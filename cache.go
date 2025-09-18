package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileCache provides a simple filesystem based cache with TTL semantics.
// This is intentionally tiny and dependency‑free. It is designed so the
// composite wall image only has to be regenerated at most once per day.
type FileCache struct {
	Dir string        // Directory root where cache files are stored
	TTL time.Duration // Time to live for each cached artifact

	mu        sync.RWMutex           // (reserved for future dir-wide ops)
	perKeyMu  map[string]*sync.Mutex // per-key mutexes avoid duplicate generation
	perKeyMuG sync.Mutex             // guards perKeyMu map
}

// NewFileCache constructs a new cache. The directory is created if missing.
func NewFileCache(dir string, ttl time.Duration) (*FileCache, error) {
	if dir == "" {
		return nil, errors.New("cache directory cannot be empty")
	}
	if ttl <= 0 {
		return nil, errors.New("ttl must be > 0")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}
	return &FileCache{
		Dir:      dir,
		TTL:      ttl,
		perKeyMu: make(map[string]*sync.Mutex),
	}, nil
}

// filePath returns an absolute path for a given cache key.
func (c *FileCache) filePath(key string) string {
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:16]) // first 16 bytes (128 bits) is enough
	return filepath.Join(c.Dir, name+".bin")
}

// getPerKeyMutex returns a mutex pointer for the provided key.
func (c *FileCache) getPerKeyMutex(key string) *sync.Mutex {
	c.perKeyMuG.Lock()
	defer c.perKeyMuG.Unlock()
	m, ok := c.perKeyMu[key]
	if !ok {
		m = &sync.Mutex{}
		c.perKeyMu[key] = m
	}
	return m
}

// Get attempts to read a valid (non-expired) cache entry for key.
// Returns (data, true, nil) on a cache hit.
// Returns (_, false, nil) if the item is missing or expired.
func (c *FileCache) Get(key string) ([]byte, bool, error) {
	path := c.filePath(key)
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("stat cache file: %w", err)
	}
	if fi.Size() == 0 {
		// Treat zero length as invalid (possibly interrupted write)
		return nil, false, nil
	}
	if time.Since(fi.ModTime()) > c.TTL {
		return nil, false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("open cache file: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, false, fmt.Errorf("read cache file: %w", err)
	}
	return b, true, nil
}

// GetOrCreate returns cached data for key if present and fresh; otherwise
// it calls generator, stores the result, and returns it. The bool indicates
// whether the returned bytes came from cache (true) or were freshly generated (false).
func (c *FileCache) GetOrCreate(key string, generator func() ([]byte, error)) ([]byte, bool, error) {
	// Fast path: attempt read without locking generation path
	if data, ok, err := c.Get(key); err != nil {
		return nil, false, err
	} else if ok {
		return data, true, nil
	}

	m := c.getPerKeyMutex(key)
	m.Lock()
	defer m.Unlock()

	// Re-check after acquiring lock to avoid duplicate generation
	if data, ok, err := c.Get(key); err != nil {
		return nil, false, err
	} else if ok {
		return data, true, nil
	}

	data, err := generator()
	if err != nil {
		return nil, false, err
	}

	if err := c.writeFileAtomically(key, data); err != nil {
		return nil, false, err
	}
	return data, false, nil
}

// writeFileAtomically writes bytes to the final cache path using a temp file + rename.
func (c *FileCache) writeFileAtomically(key string, data []byte) error {
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return fmt.Errorf("ensure cache dir: %w", err)
	}
	finalPath := c.filePath(key)
	tmp, err := os.CreateTemp(c.Dir, "tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Ensure cleanup on failure
	defer func() {
		tmp.Close()
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// PurgeExpired scans the cache directory and deletes any expired entries.
// It is optional; the cache still works without calling it. Best effort only.
func (c *FileCache) PurgeExpired() error {
	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read cache dir: %w", err)
	}
	deadline := time.Now().Add(-c.TTL)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(deadline) {
			_ = os.Remove(filepath.Join(c.Dir, e.Name()))
		}
	}
	return nil
}

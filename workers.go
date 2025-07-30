package main

import (
	"fmt"
	"image"
	"log"
	"net/http"
	"time"

	"github.com/kovidgoyal/imaging"
)

type ImageJob struct {
	Index     int
	UUID      string
	URL       string
	PicWidth  int
	PicHeight int
}

type ImageResult struct {
	Index int
	Image image.Image
	Error error
}

func downloadImagesWithWorkerPool(pictureUUIDs []string, grid GridDimensions, maxWorkers int) map[int]image.Image {
	jobs := make(chan ImageJob, len(pictureUUIDs))
	results := make(chan ImageResult, len(pictureUUIDs))

	for w := 0; w < maxWorkers; w++ {
		go func() {
			for job := range jobs {
				img, err := openRemoteImageWithTimeout(job.URL, 30*time.Second)
				var resizedImg image.Image
				if err == nil {
					resizedImg = imaging.Fill(img, job.PicWidth, job.PicHeight, imaging.Center, imaging.Lanczos)
				}
				results <- ImageResult{
					Index: job.Index,
					Image: resizedImg,
					Error: err,
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for i, uuid := range pictureUUIDs {
			if i >= grid.Cols*grid.Rows {
				break
			}
			jobs <- ImageJob{
				Index:     i,
				UUID:      uuid,
				URL:       fmt.Sprintf("https://api.imreally.gay/assets/%s", uuid),
				PicWidth:  grid.PicWidth,
				PicHeight: grid.PicHeight,
			}
		}
	}()

	images := make(map[int]image.Image)
	for i := 0; i < len(pictureUUIDs) && i < grid.Cols*grid.Rows; i++ {
		result := <-results
		if result.Error != nil {
			log.Printf("Warning: Failed to process image at index %d: %v", result.Index, result.Error)
		} else {
			images[result.Index] = result.Image
		}
	}

	return images
}

func openRemoteImageWithTimeout(url string, timeout time.Duration) (image.Image, error) {
	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image: status code %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return img, nil
}

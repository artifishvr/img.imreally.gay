package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/kovidgoyal/imaging"
)

type APIResponse struct {
	Data []struct {
		ID          int    `json:"id"`
		DateCreated string `json:"date_created"`
		DateUpdated string `json:"date_updated"`
		Picture     string `json:"picture"`
		Name        string `json:"name"`
		VrcID       string `json:"vrc_ID"`
	} `json:"data"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Print("Error loading .env file")
	}

	app := fiber.New()
	app.Use(logger.New())
	client := resty.New()

	directusToken := os.Getenv("DIRECTUS_TOKEN")

	// Initialize a 24h filesystem cache for the composite image
	cache, err := NewFileCache("cache", 24*time.Hour)
	if err != nil {
		log.Fatalf("failed to init cache: %v", err)
	}

	// Define a route for the GET method on the root path '/'
	app.Get("/", func(c fiber.Ctx) error {
		data, fromCache, err := cache.GetOrCreate("wall", func() ([]byte, error) {
			var apiResponse APIResponse
			resp, err := client.R().
				SetHeader("Authorization", "Bearer "+directusToken).
				SetResult(&apiResponse).
				Get("https://api.imreally.gay/items/thewall")
			if err != nil {
				return nil, fmt.Errorf("api request: %w", err)
			}
			if resp.StatusCode() != 200 {
				return nil, fmt.Errorf("api status %d", resp.StatusCode())
			}

			var pictureUUIDs []string
			for _, item := range apiResponse.Data {
				pictureUUIDs = append(pictureUUIDs, item.Picture)
			}
			if len(pictureUUIDs) == 0 {
				return nil, fmt.Errorf("no images found")
			}

			height := 2048
			width := 1024
			picturecount := len(pictureUUIDs)
			grid := calculateOptimalGrid(width, height, picturecount)

			combined := imaging.New(width, height, image.Black)

			maxWorkers := 10
			images := downloadImagesWithWorkerPool(pictureUUIDs, grid, maxWorkers)

			for i := 0; i < grid.Cols*grid.Rows; i++ {
				img, exists := images[i]
				if !exists {
					continue
				}
				col := i % grid.Cols
				row := i / grid.Cols
				x := col * grid.PicWidth
				y := row * grid.PicHeight
				combined = imaging.Paste(combined, img, image.Pt(x, y))
			}

			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, combined, &jpeg.Options{Quality: 90}); err != nil {
				return nil, fmt.Errorf("encode jpeg: %w", err)
			}
			return buf.Bytes(), nil
		})
		if err != nil {
			if strings.Contains(err.Error(), "no images found") {
				return c.Status(404).SendString("No images found")
			}
			log.Printf("Error generating wall image: %v", err)
			return c.Status(500).SendString("Failed to generate image")
		}

		c.Set("Content-Type", "image/jpeg")
		c.Set("Content-Length", fmt.Sprintf("%d", len(data)))
		if fromCache {
			c.Set("X-Cache", "HIT")
		} else {
			c.Set("X-Cache", "MISS")
		}
		return c.Send(data)
	})

	// Start the server on port 3000
	log.Fatal(app.Listen(":3000"))
}

func openRemoteImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
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

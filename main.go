package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"

	"github.com/disintegration/imaging"
	"github.com/go-resty/resty/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/joho/godotenv"
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
		log.Fatal("Error loading .env file")
	}

	app := fiber.New()
	app.Use(logger.New())
	client := resty.New()

	directusToken := os.Getenv("DIRECTUS_TOKEN")

	// Define a route for the GET method on the root path '/'
	app.Get("/", func(c fiber.Ctx) error {
		var apiResponse APIResponse
		resp, err := client.R().
			SetHeader("Authorization", "Bearer "+directusToken).
			SetResult(&apiResponse).
			Get("https://api.imreally.gay/items/thewall")

		if err != nil {
			log.Printf("Error making API request: %v", err)
			return c.Status(500).SendString("Failed to fetch images")
		}

		if resp.StatusCode() != 200 {
			log.Printf("API returned status code: %d", resp.StatusCode())
			return c.Status(500).SendString("API request failed")
		}

		var pictureUUIDs []string
		for _, item := range apiResponse.Data {
			pictureUUIDs = append(pictureUUIDs, item.Picture)
		}

		if len(pictureUUIDs) == 0 {
			return c.Status(404).SendString("No images found")
		}

		height := 2048
		width := 1024

		picturecount := len(pictureUUIDs)

		grid := calculateOptimalGrid(width, height, picturecount)

		combined := imaging.New(width, height, image.Black)

		for i, pictureUUID := range pictureUUIDs {
			if i >= grid.Cols*grid.Rows {
				log.Printf("Warning: More images provided than grid slots available. Skipping extra images.")
				break
			}

			imageURL := fmt.Sprintf("https://api.imreally.gay/assets/%s", pictureUUID)

			img, err := openRemoteImage(imageURL)
			if err != nil {
				log.Printf("Warning: Failed to open image %s: %v", imageURL, err)
				continue
			}

			resizedImg := imaging.Fill(img, grid.PicWidth, grid.PicHeight, imaging.Center, imaging.Lanczos)

			col := i % grid.Cols
			row := i / grid.Cols

			x := col * grid.PicWidth
			y := row * grid.PicHeight

			combined = imaging.Paste(combined, resizedImg, image.Pt(x, y))
		}

		// Send a string response to the client
		var buf bytes.Buffer
		err = jpeg.Encode(&buf, combined, &jpeg.Options{Quality: 90})
		if err != nil {
			return err
		}

		c.Set("Content-Type", "image/jpeg")
		c.Set("Content-Length", string(rune(buf.Len())))

		return c.Send(buf.Bytes())
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

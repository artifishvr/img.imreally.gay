import directus from "../utils/directus";
import { readItems } from "@directus/sdk";
import Sharp from "sharp";

const DIRECTUS_URL = process.env.DIRECTUS_URL;

export default defineCachedEventHandler(
  async (event) => {
    // Fetch images from Directus
    const pictures = await directus.request(readItems("thewall"));

    // Create blank canvas
    const canvas = Sharp({
      create: {
        width: 1024,
        height: 2048,
        channels: 4,
        background: { r: 0, g: 0, b: 0, alpha: 1 },
      },
    });

    // Calculate grid dimensions
    const COLS = Math.floor(0.167 * pictures.length);
    const TILE_WIDTH = Math.floor(1024 / COLS);
    const TILE_HEIGHT = Math.floor(TILE_WIDTH * 1.5);

    // Prepare composite array
    const compositeArray = await Promise.all(
      pictures.map(async (picture, index) => {
        // Calculate position
        const row = Math.floor(index / COLS);
        const col = index % COLS;
        const x = col * TILE_WIDTH;
        const y = row * TILE_HEIGHT;

        // Fetch and resize image
        const imageUrl = `${DIRECTUS_URL}/assets/${picture.picture}`;
        const response = await fetch(imageUrl);
        const imageBuffer = Buffer.from(await response.arrayBuffer());

        const resized = await Sharp(imageBuffer)
          .resize(TILE_WIDTH, TILE_HEIGHT, {
            fit: "cover",
            position: "center",
          })
          .toBuffer();

        return {
          input: resized,
          top: y,
          left: x,
        };
      })
    );

    // Compose final image
    const finalImage = await canvas.composite(compositeArray).png().toBuffer();

    return finalImage;
  },
  { maxAge: 60 * 60 * 6 /* 6 hour */ }
);

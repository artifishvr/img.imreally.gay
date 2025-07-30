package main

import (
	"log"
	"math"
)

type GridDimensions struct {
	Cols      int
	Rows      int
	PicWidth  int
	PicHeight int
}

// finds the best grid arrangement for the given parameters
func calculateOptimalGrid(canvasWidth, canvasHeight, pictureCount int) GridDimensions {
	var bestCols, bestRows int
	var bestPicWidth, bestPicHeight int
	bestFit := 0.0

	for c := 1; c <= pictureCount; c++ {
		r := int(math.Ceil(float64(pictureCount) / float64(c)))

		// Calculate picture dimensions for this arrangement
		tempWidth := canvasWidth / c
		tempHeight := canvasHeight / r

		// Check which dimension is the limiting factor for 2:1 ratio
		// If height needs to be 2*width for 2:1 ratio
		if tempHeight > 2*tempWidth {
			// Width is limiting factor
			actualHeight := 2 * tempWidth
			actualWidth := tempWidth

			// Check if this fits in the grid
			if actualHeight*r <= canvasHeight {
				utilization := float64(actualWidth*c*actualHeight*r) / float64(canvasWidth*canvasHeight)
				if utilization > bestFit {
					bestFit = utilization
					bestCols = c
					bestRows = r
					bestPicWidth = actualWidth
					bestPicHeight = actualHeight
				}
			}
		} else {
			// Height is limiting factor
			actualHeight := tempHeight
			actualWidth := actualHeight / 2

			// Check if this fits in the grid
			if actualWidth*c <= canvasWidth {
				utilization := float64(actualWidth*c*actualHeight*r) / float64(canvasWidth*canvasHeight)
				if utilization > bestFit {
					bestFit = utilization
					bestCols = c
					bestRows = r
					bestPicWidth = actualWidth
					bestPicHeight = actualHeight
				}
			}
		}
	}

	log.Printf("Best arrangement: %dx%d grid, Picture size: %dx%d (2:1 ratio maintained)",
		bestCols, bestRows, bestPicWidth, bestPicHeight)

	return GridDimensions{
		Cols:      bestCols,
		Rows:      bestRows,
		PicWidth:  bestPicWidth,
		PicHeight: bestPicHeight,
	}
}

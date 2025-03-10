/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: © 2025 Dmitry Kireev
 */

package ux

import (
	"github.com/pterm/pterm"
	"math/rand"
	"time"
)

const (
	width      = 50 // Total width of the terminal
	height     = 5  // Number of rows for fish animation
	frameRate  = 110 * time.Millisecond
	bubbleRate = 150 * time.Millisecond // Bubbles move upward 2x slower
)

// Fish struct to hold fish position and row
type Fish struct {
	Pos      int // Horizontal position
	Row      int // Vertical position (y)
	StartPos int // Initial starting position
}

// Bubble struct to hold bubble position and column
type Bubble struct {
	Row int // Vertical position (y)
	Col int // Horizontal position
}

// RenderAsciiArt animates big text (50x10) and fishes moving right-to-left below it
func RenderAsciiArt() {
	// Define initial positions of the fishes
	fishes := []Fish{
		{Pos: 10, Row: 0, StartPos: 10},
		{Pos: 25, Row: 1, StartPos: 25},
		{Pos: 30, Row: 2, StartPos: 30},
		{Pos: 15, Row: 3, StartPos: 15},
		{Pos: 50, Row: 4, StartPos: 50},
	}

	// Define initial positions of bubbles with varying start coordinates
	bubbles := []Bubble{
		{Row: 1, Col: 5},
		{Row: 3, Col: 45},
	}

	// Start two separate areas
	bigTextArea, _ := pterm.DefaultArea.WithCenter(false).Start()
	fishArea, _ := pterm.DefaultArea.WithCenter(false).Start()

	defer bigTextArea.Stop()
	defer fishArea.Stop()

	ticker := time.NewTicker(frameRate)
	bubbleTicker := time.NewTicker(bubbleRate)
	defer ticker.Stop()
	defer bubbleTicker.Stop()

	stopChan := make(chan struct{})

	// Animation loop
	go func() {
		for {
			select {
			case <-ticker.C:
				// Generate the fish animation frame
				fishFrame := generateFrame(fishes, bubbles, width, height)
				fishArea.Update(fishFrame)

				// Update positions for all fish
				for i := range fishes {
					variation := rand.Intn(4) + 1
					if fishes[i].Pos > 0 {
						fishes[i].Pos -= 1 * variation
					} else {
						fishes[i].Pos = width - 1
					}
				}

			case <-bubbleTicker.C:
				// Move bubbles upward and randomly left or right
				for i := range bubbles {
					if bubbles[i].Row > 0 {
						bubbles[i].Row -= 1
					} else {
						bubbles[i].Row = height - 1
					}

					// Randomize horizontal movement
					randomStep := rand.Intn(5) - 2
					newCol := bubbles[i].Col + randomStep
					if newCol >= 3 && newCol < width {
						bubbles[i].Col = newCol
					}
				}
			}
		}
		close(stopChan)
	}()

	<-stopChan

	// Clear the fish area and reset terminal display
	fishArea.Update("") // Clears the fish animation area
}

// generateFrame generates a frame with multiple fishes and bubbles at their positions
func generateFrame(fishes []Fish, bubbles []Bubble, width int, height int) string {
	var output string

	// Define the number of bars for each row
	//barPattern := []int{1, 1, 1, 1, 1}
	for y := 0; y < height; y++ {
		line := ""
		// Generate animation to the right of the bars
		for x := 3; x < width; x++ {
			char := " "

			// Check for fish presence
			for _, fish := range fishes {
				if y == fish.Row && x == fish.Pos {
					char = "🐟"
					break
				}
			}

			// Check for bubble presence (only render bubble if no fish)
			if char == " " {
				for _, bubble := range bubbles {
					if y == bubble.Row && x == bubble.Col {
						char = "🫧"
						break
					}
				}
			}

			line += char
		}
		output += line + "\n"
	}
	output += ".................. ctrl+c to exit .................."
	output += "\n"

	return output
}

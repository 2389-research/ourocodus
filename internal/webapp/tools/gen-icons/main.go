package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <output-dir>\n", os.Args[0])
		os.Exit(1)
	}

	outputDir := os.Args[1]

	// Generate all icon sizes
	// PWA icons: 192x192, 512x512
	// Favicon sizes: 16x16, 32x32
	icons := []struct {
		size int
		name string
	}{
		{16, "favicon-16x16.png"},
		{32, "favicon-32x32.png"},
		{192, "icon-192.png"},
		{512, "icon-512.png"},
	}

	for _, icon := range icons {
		filename := filepath.Join(outputDir, icon.name)
		if err := generateIcon(filename, icon.size); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating %s: %v\n", filename, err)
			os.Exit(1)
		}
		fmt.Printf("✓ Generated %s\n", filename)
	}
}

// generateIcon creates a simple circular icon with "O" for Ourocodus
func generateIcon(filename string, size int) error {
	// Create image
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Background color - light blue/purple gradient approximation
	bgColor := color.RGBA{R: 99, G: 102, B: 241, A: 255} // Indigo-500

	// Fill background
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Draw a circle with white "O"
	center := size / 2
	radius := size * 2 / 5

	// Draw outer circle (white ring)
	drawCircle(img, center, center, radius, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	// Draw inner circle (transparent - creates the "O" shape)
	innerRadius := radius * 3 / 5
	drawCircle(img, center, center, innerRadius, bgColor)

	// Write to file
	file, err := os.Create(filename) // nolint:gosec // G304: CLI tool creates icon files
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return png.Encode(file, img)
}

// drawCircle draws a filled circle on the image
func drawCircle(img *image.RGBA, x0, y0, radius int, col color.Color) {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			dx := x - x0
			dy := y - y0
			if dx*dx+dy*dy <= radius*radius {
				img.Set(x, y, col)
			}
		}
	}
}

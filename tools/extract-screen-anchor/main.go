package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	source := flag.String("source", "", "source PNG")
	output := flag.String("output", "", "output PNG")
	x := flag.Int("x", 0, "crop left")
	y := flag.Int("y", 0, "crop top")
	w := flag.Int("width", 0, "crop width")
	h := flag.Int("height", 0, "crop height")
	flag.Parse()
	if err := run(*source, *output, image.Rect(*x, *y, *x+*w, *y+*h)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(source, output string, rect image.Rectangle) error {
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("decode source: %w", err)
	}
	if rect.Empty() || !rect.In(img.Bounds()) {
		return fmt.Errorf("crop %v outside source %v", rect, img.Bounds())
	}
	crop := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(crop, crop.Bounds(), img, rect.Min, draw.Src)
	if mkdirErr := os.MkdirAll(filepath.Dir(output), 0o755); mkdirErr != nil {
		return fmt.Errorf("create output directory: %w", mkdirErr)
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".anchor-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := png.Encode(tmp, crop); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode crop: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close crop: %w", err)
	}
	if err := os.Rename(tmpPath, output); err != nil {
		return fmt.Errorf("publish crop: %w", err)
	}
	return nil
}

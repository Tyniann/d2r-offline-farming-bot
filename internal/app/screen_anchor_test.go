package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchScreenAnchor(t *testing.T) {
	template := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			template.Set(x, y, color.RGBA{R: 40, G: 80, B: 120, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "anchor.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, template); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	actual := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 1; y < 3; y++ {
		for x := 1; x < 3; x++ {
			actual.Set(x, y, color.RGBA{R: 40, G: 80, B: 120, A: 255})
		}
	}
	difference, err := matchScreenAnchor(actual, screenAnchor{name: "test", path: path, rect: image.Rect(1, 1, 3, 3)})
	if err != nil {
		t.Fatal(err)
	}
	if difference != 0 {
		t.Fatalf("difference = %f, want 0", difference)
	}
}

func TestMatchScreenAnchorRejectsBounds(t *testing.T) {
	_, err := matchScreenAnchor(image.NewRGBA(image.Rect(0, 0, 2, 2)), screenAnchor{name: "test", path: "missing.png", rect: image.Rect(1, 1, 3, 3)})
	if err == nil {
		t.Fatal("expected error")
	}
}

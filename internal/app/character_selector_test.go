package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestCharacterNavigationHomeDownAndStableMatch(t *testing.T) {
	machine := &characterNavigationMachine{characterCount: 3}
	sequence := []struct {
		matched bool
		want    characterNavigationAction
	}{{false, characterNavigationHome}, {false, characterNavigationDown}, {true, characterNavigationNone}, {true, characterNavigationNone}, {true, characterNavigationComplete}}
	for index, step := range sequence {
		action, err := machine.tick(step.matched)
		if err != nil || action != step.want {
			t.Fatalf("tick %d action=%d err=%v", index, action, err)
		}
	}
	if machine.downSteps != 1 {
		t.Fatalf("Down steps = %d", machine.downSteps)
	}
}

func TestCharacterScreenFixtureClassifiesCompetingAnchors(t *testing.T) {
	directory := t.TempDir()
	playPath := filepath.Join(directory, "play.png")
	dialogPath := filepath.Join(directory, "dialog.png")
	playColor := color.RGBA{R: 255, A: 255}
	dialogColor := color.RGBA{B: 255, A: 255}
	otherColor := color.RGBA{G: 255, A: 255}
	writeSolidPNG(t, playPath, 203, 47, playColor)
	writeSolidPNG(t, dialogPath, 180, 175, dialogColor)
	play := screenAnchor{name: "play", path: playPath, rect: image.Rect(538, 624, 741, 671)}
	dialog := screenAnchor{name: "dialog", path: dialogPath, rect: image.Rect(550, 245, 730, 420)}

	characterScreen := solidCapture(1280, 720, otherColor)
	fillRect(characterScreen, play.rect, playColor)
	if err := verifyCharacterScreenCapture(characterScreen, play, dialog); err != nil {
		t.Fatalf("character screen rejected: %v", err)
	}

	difficultyScreen := solidCapture(1280, 720, otherColor)
	fillRect(difficultyScreen, dialog.rect, dialogColor)
	if err := verifyCharacterScreenCapture(difficultyScreen, play, dialog); err == nil {
		t.Fatal("open difficulty dialog was accepted as character screen")
	}

	ambiguousScreen := solidCapture(1280, 720, color.RGBA{A: 255})
	if err := verifyCharacterScreenCapture(ambiguousScreen, play, dialog); err == nil {
		t.Fatal("ambiguous screen anchors were accepted as character screen")
	}
}

func solidCapture(width, height int, value color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	fillRect(img, img.Bounds(), value)
	return img
}

func fillRect(img *image.RGBA, rect image.Rectangle, value color.Color) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.Set(x, y, value)
		}
	}
}

func writeSolidPNG(t *testing.T, path string, width, height int, value color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, value)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCharacterNavigationNoMatchIsBounded(t *testing.T) {
	machine := &characterNavigationMachine{characterCount: 2}
	if action, err := machine.tick(false); err != nil || action != characterNavigationHome {
		t.Fatalf("Home action=%d err=%v", action, err)
	}
	if action, err := machine.tick(false); err != nil || action != characterNavigationDown {
		t.Fatalf("Down action=%d err=%v", action, err)
	}
	if _, err := machine.tick(false); err == nil {
		t.Fatal("selector exceeded character_count-1 Down steps")
	}
}

package app

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
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
	} else {
		var ambiguous *characterScreenAmbiguousError
		if !errors.As(err, &ambiguous) {
			t.Fatalf("ambiguous screen error = %T, want retryable classification", err)
		}
	}
}

func TestCharacterNameAnchorIgnoresMutableLevelAndRejectsDifferentName(t *testing.T) {
	directory := t.TempDir()
	anchorPath := filepath.Join(directory, "character.png")
	background := color.RGBA{R: 35, G: 31, B: 31, A: 255}
	text := color.RGBA{R: 220, G: 220, B: 210, A: 255}
	anchorImage := solidCapture(210, 60, background)
	nameGlyphs := image.Rect(12, 21, 82, 31)
	fillRect(anchorImage, nameGlyphs, text)
	mutableLevel := image.Rect(12, 40, 145, 51)
	fillRect(anchorImage, mutableLevel, text)
	writePNG(t, anchorPath, anchorImage)
	anchor := screenAnchor{
		name: "selected_character", path: anchorPath, rect: image.Rect(1035, 48, 1245, 108),
		comparisonRegion: characterNameAnchorRegion, maxMeanDifference: characterNameAnchorMaxDifference,
		brightThreshold: characterNameAnchorBrightThreshold, brightShiftRadius: characterNameAnchorShiftRadius,
	}

	sameNameNewLevel := solidCapture(1280, 720, background)
	fillRect(sameNameNewLevel, anchor.rect, background)
	fillRect(sameNameNewLevel, nameGlyphs.Add(anchor.rect.Min), text)
	fillRect(sameNameNewLevel, mutableLevel.Add(anchor.rect.Min), color.RGBA{R: 90, G: 45, B: 20, A: 255})
	score, err := matchScreenAnchor(sameNameNewLevel, anchor)
	if err != nil {
		t.Fatal(err)
	}
	if score > characterNameAnchorMaxDifference {
		t.Fatalf("same name with changed level score=%f, want <= %f", score, characterNameAnchorMaxDifference)
	}

	differentName := solidCapture(1280, 720, background)
	fillRect(differentName, anchor.rect, background)
	fillRect(differentName, image.Rect(90, 21, 150, 31).Add(anchor.rect.Min), text)
	score, err = matchScreenAnchor(differentName, anchor)
	if err != nil {
		t.Fatal(err)
	}
	if score <= characterNameAnchorMaxDifference {
		t.Fatalf("different name score=%f, want > %f", score, characterNameAnchorMaxDifference)
	}
}

func TestCharacterSelectionRowRectFollowsVisibleSelection(t *testing.T) {
	cases := map[int]image.Rectangle{
		0: phase16CharacterAnchorRect,
		1: image.Rect(1035, 108, 1245, 168),
		2: image.Rect(1035, 168, 1245, 228),
		8: image.Rect(1035, 528, 1245, 588),
		9: image.Rect(1035, 528, 1245, 588),
	}
	for downSteps, want := range cases {
		if got := characterSelectionRowRect(downSteps); got != want {
			t.Fatalf("downSteps=%d rect=%v, want %v", downSteps, got, want)
		}
	}
}

func TestCharacterSelectionRequiresSelectedBorder(t *testing.T) {
	directory := t.TempDir()
	anchorPath := filepath.Join(directory, "selected.png")
	background := color.RGBA{R: 35, G: 31, B: 31, A: 255}
	gold := color.RGBA{R: 190, G: 135, B: 60, A: 255}
	anchorImage := solidCapture(210, 60, background)
	fillCharacterBorder(anchorImage, anchorImage.Bounds(), gold)
	writePNG(t, anchorPath, anchorImage)
	anchor := screenAnchor{name: "selected_character", path: anchorPath, rect: phase16CharacterAnchorRect}

	selected := solidCapture(1280, 720, background)
	fillCharacterBorder(selected, anchor.rect, gold)
	matched, err := matchSelectedCharacterBorder(selected, anchor)
	if err != nil || !matched {
		t.Fatalf("selected border matched=%t err=%v", matched, err)
	}

	unselected := solidCapture(1280, 720, background)
	matched, err = matchSelectedCharacterBorder(unselected, anchor)
	if err != nil || matched {
		t.Fatalf("unselected border matched=%t err=%v", matched, err)
	}
}

func fillCharacterBorder(img *image.RGBA, rect image.Rectangle, value color.Color) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			localX, localY := x-rect.Min.X, y-rect.Min.Y
			if localX < characterSelectionBorderWidth || localX >= rect.Dx()-characterSelectionBorderWidth ||
				localY < characterSelectionBorderWidth || localY >= rect.Dy()-characterSelectionBorderWidth {
				img.Set(x, y, value)
			}
		}
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
	writePNG(t, path, img)
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
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

func TestCanceledSelectionExplainsEmergencyStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := canceledInputOperationError(ctx, input.Status{Stopped: true}, "f11")
	if err == nil || !strings.Contains(err.Error(), "Not-Aus (F11)") || strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("error=%q, want understandable emergency-stop guidance", err)
	}
}

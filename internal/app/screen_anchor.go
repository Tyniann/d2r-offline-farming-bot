package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

const screenAnchorMaxMeanDifference = 0.16

type screenAnchor struct {
	name string
	path string
	rect image.Rectangle
}

func matchScreenAnchor(actual image.Image, anchor screenAnchor) (float64, error) {
	file, err := os.Open(anchor.path)
	if err != nil {
		return 0, fmt.Errorf("open %s screen anchor: %w", anchor.name, err)
	}
	defer file.Close()
	expected, err := png.Decode(file)
	if err != nil {
		return 0, fmt.Errorf("decode %s screen anchor: %w", anchor.name, err)
	}
	if anchor.rect.Empty() || !anchor.rect.In(actual.Bounds()) {
		return 0, fmt.Errorf("%s screen anchor rectangle %v outside capture %v", anchor.name, anchor.rect, actual.Bounds())
	}
	if expected.Bounds().Dx() != anchor.rect.Dx() || expected.Bounds().Dy() != anchor.rect.Dy() {
		return 0, fmt.Errorf("%s screen anchor size %v does not match rectangle %v", anchor.name, expected.Bounds().Size(), anchor.rect.Size())
	}

	var difference uint64
	for y := 0; y < anchor.rect.Dy(); y++ {
		for x := 0; x < anchor.rect.Dx(); x++ {
			er, eg, eb, _ := expected.At(expected.Bounds().Min.X+x, expected.Bounds().Min.Y+y).RGBA()
			ar, ag, ab, _ := actual.At(anchor.rect.Min.X+x, anchor.rect.Min.Y+y).RGBA()
			difference += channelDifference(er, ar) + channelDifference(eg, ag) + channelDifference(eb, ab)
		}
	}
	channels := uint64(anchor.rect.Dx() * anchor.rect.Dy() * 3)
	return float64(difference) / float64(channels*0xffff), nil
}

func channelDifference(a, b uint32) uint64 {
	if a >= b {
		return uint64(a - b)
	}
	return uint64(b - a)
}

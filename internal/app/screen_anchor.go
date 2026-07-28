package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

const screenAnchorMaxMeanDifference = 0.16

type screenAnchor struct {
	name                  string
	path                  string
	rect                  image.Rectangle
	comparisonRegion      image.Rectangle
	maxMeanDifference     float64
	brightThreshold       uint8
	brightShiftRadius     int
	requireSelectedBorder bool
}

func matchScreenAnchor(actual image.Image, anchor screenAnchor) (float64, error) {
	region := anchor.comparisonRegion
	if region.Empty() {
		region = image.Rect(0, 0, anchor.rect.Dx(), anchor.rect.Dy())
	}
	if anchor.brightThreshold > 0 {
		return matchBrightScreenAnchorRegion(actual, anchor, region)
	}
	return matchScreenAnchorRegion(actual, anchor, region)
}

func (a screenAnchor) maximumMeanDifference() float64 {
	if a.maxMeanDifference > 0 {
		return a.maxMeanDifference
	}
	return screenAnchorMaxMeanDifference
}

func matchScreenAnchorRegion(actual image.Image, anchor screenAnchor, region image.Rectangle) (float64, error) {
	expected, err := loadScreenAnchor(actual, anchor, region)
	if err != nil {
		return 0, err
	}

	var difference uint64
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			er, eg, eb, _ := expected.At(expected.Bounds().Min.X+x, expected.Bounds().Min.Y+y).RGBA()
			ar, ag, ab, _ := actual.At(anchor.rect.Min.X+x, anchor.rect.Min.Y+y).RGBA()
			difference += channelDifference(er, ar) + channelDifference(eg, ag) + channelDifference(eb, ab)
		}
	}
	channels := uint64(region.Dx() * region.Dy() * 3)
	return float64(difference) / float64(channels*0xffff), nil
}

func matchBrightScreenAnchorRegion(actual image.Image, anchor screenAnchor, region image.Rectangle) (float64, error) {
	expected, err := loadScreenAnchor(actual, anchor, region)
	if err != nil {
		return 0, err
	}
	threshold := uint32(anchor.brightThreshold) * 0x101
	bestDifference := 1.0
	found := false
	for shiftY := -anchor.brightShiftRadius; shiftY <= anchor.brightShiftRadius; shiftY++ {
		for shiftX := -anchor.brightShiftRadius; shiftX <= anchor.brightShiftRadius; shiftX++ {
			var intersection, union uint64
			for y := region.Min.Y; y < region.Max.Y; y++ {
				for x := region.Min.X; x < region.Max.X; x++ {
					er, eg, eb, _ := expected.At(expected.Bounds().Min.X+x, expected.Bounds().Min.Y+y).RGBA()
					ar, ag, ab, _ := actual.At(anchor.rect.Min.X+x+shiftX, anchor.rect.Min.Y+y+shiftY).RGBA()
					expectedBright := er >= threshold || eg >= threshold || eb >= threshold
					actualBright := ar >= threshold || ag >= threshold || ab >= threshold
					if expectedBright && actualBright {
						intersection++
					}
					if expectedBright || actualBright {
						union++
					}
				}
			}
			if union == 0 {
				continue
			}
			found = true
			difference := 1 - float64(intersection)/float64(union)
			if difference < bestDifference {
				bestDifference = difference
			}
		}
	}
	if !found {
		return 0, fmt.Errorf("%s screen anchor comparison region contains no bright identity pixels", anchor.name)
	}
	return bestDifference, nil
}

func loadScreenAnchor(actual image.Image, anchor screenAnchor, region image.Rectangle) (image.Image, error) {
	file, err := os.Open(anchor.path)
	if err != nil {
		return nil, fmt.Errorf("open %s screen anchor: %w", anchor.name, err)
	}
	defer file.Close()
	expected, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode %s screen anchor: %w", anchor.name, err)
	}
	if anchor.rect.Empty() || !anchor.rect.In(actual.Bounds()) {
		return nil, fmt.Errorf("%s screen anchor rectangle %v outside capture %v", anchor.name, anchor.rect, actual.Bounds())
	}
	if expected.Bounds().Dx() != anchor.rect.Dx() || expected.Bounds().Dy() != anchor.rect.Dy() {
		return nil, fmt.Errorf("%s screen anchor size %v does not match rectangle %v", anchor.name, expected.Bounds().Size(), anchor.rect.Size())
	}
	localBounds := image.Rect(0, 0, anchor.rect.Dx(), anchor.rect.Dy())
	if region.Empty() || !region.In(localBounds) {
		return nil, fmt.Errorf("%s screen anchor comparison region %v outside anchor %v", anchor.name, region, localBounds)
	}
	return expected, nil
}

func channelDifference(a, b uint32) uint64 {
	if a >= b {
		return uint64(a - b)
	}
	return uint64(b - a)
}

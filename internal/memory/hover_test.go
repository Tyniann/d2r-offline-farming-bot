package memory

import "testing"

func TestParseHoverBufferHovered(t *testing.T) {
	buf := make([]byte, hoverBufferSize)
	buf[0] = 1 // is_hovered uint16 > 0
	binaryPutU32(buf[4:], HoverUnitTypeEntrance)
	binaryPutU32(buf[8:], 4711)

	got := parseHoverBuffer(buf)
	want := HoverState{IsHovered: true, UnitType: HoverUnitTypeEntrance, UnitID: 4711}
	if got != want {
		t.Fatalf("parseHoverBuffer() = %+v, want %+v", got, want)
	}
}

func TestParseHoverBufferNotHovered(t *testing.T) {
	buf := make([]byte, hoverBufferSize)
	// Stale type/ID bytes must be ignored when the hovered flag is zero.
	binaryPutU32(buf[4:], HoverUnitTypeMonster)
	binaryPutU32(buf[8:], 99)

	if got := parseHoverBuffer(buf); got != (HoverState{}) {
		t.Fatalf("parseHoverBuffer() = %+v, want zero HoverState", got)
	}
}

func TestParseHoverBufferTooShort(t *testing.T) {
	if got := parseHoverBuffer([]byte{1, 0, 0}); got != (HoverState{}) {
		t.Fatalf("parseHoverBuffer(short) = %+v, want zero HoverState", got)
	}
}

func TestHoverUnitTypeItemValue(t *testing.T) {
	if HoverUnitTypeItem != 4 {
		t.Fatalf("HoverUnitTypeItem = %d, want 4", HoverUnitTypeItem)
	}
}

func TestScanHoverOffsetFromImage(t *testing.T) {
	image := make([]byte, 64)
	copy(image[9:], []byte{0xC6, 0x84, 0xC2})
	// disp32 operand at pattern+3; hover offset = disp32 - 1.
	binaryPutU32(image[12:], 0x2811D31)
	// imm8 wildcard at +7, then fixed tail bytes.
	copy(image[17:], []byte{0x48, 0x8B, 0x74})

	got, err := scanHoverOffset(image)
	if err != nil {
		t.Fatalf("scanHoverOffset() error = %v", err)
	}
	if got != 0x2811D30 {
		t.Fatalf("scanHoverOffset() = %#x, want 0x2811D30", got)
	}
}

func TestScanHoverOffsetNotFound(t *testing.T) {
	if _, err := scanHoverOffset(make([]byte, 64)); err == nil {
		t.Fatal("scanHoverOffset() expected error for missing pattern")
	}
}

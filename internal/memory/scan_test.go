package memory

import "testing"

func TestFindPattern(t *testing.T) {
	image := []byte{
		0x00, 0x01, 0x02,
		0x48, 0x03, 0xC7, 0x49, 0x8B, 0x8C, 0xC6,
		0x10, 0x20, 0x30, 0x40,
	}

	idx, err := findPattern(image, patternSpec{
		bytes: []byte{0x48, 0x03, 0xC7, 0x49, 0x8B, 0x8C, 0xC6},
		mask:  "xxxxxxx",
	})
	if err != nil {
		t.Fatalf("findPattern() error = %v", err)
	}
	if idx != 3 {
		t.Fatalf("findPattern() = %d, want 3", idx)
	}
}

func TestScanUnitTableOffsetFromImage(t *testing.T) {
	image := make([]byte, 32)
	copy(image[5:], []byte{0x48, 0x03, 0xC7, 0x49, 0x8B, 0x8C, 0xC6})
	binaryPutU32(image[12:], 0x22C6090)

	got, err := scanUnitTableOffset(image)
	if err != nil {
		t.Fatalf("scanUnitTableOffset() error = %v", err)
	}
	if got != 0x22C6090 {
		t.Fatalf("scanUnitTableOffset() = %#x, want 0x22C6090", got)
	}
}

func TestReadModuleImageSkipsUnreadablePages(t *testing.T) {
	access := newMockAccess()
	access.moduleBase = 0x10000000
	access.moduleSize = 8192
	page := make([]byte, 4096)
	copy(page, []byte{1, 2, 3, 4})
	access.setBytes(access.moduleBase+4096, page)

	reader := newTestReader(access)
	reader.Bind(access)

	image, err := reader.readModuleImage(access.moduleBase, access.moduleSize)
	if err != nil {
		t.Fatalf("readModuleImage() error = %v", err)
	}
	if len(image) != int(access.moduleSize) {
		t.Fatalf("len(image) = %d, want %d", len(image), access.moduleSize)
	}
	if image[0] != 0 {
		t.Fatalf("unreadable first page should remain zero, got %d", image[0])
	}
	if image[4096] != 1 || image[4099] != 4 {
		t.Fatalf("readable second page not copied: %v", image[4096:4100])
	}
}

func binaryPutU32(buf []byte, v uint32) {
	buf[0] = byte(v)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 24)
}

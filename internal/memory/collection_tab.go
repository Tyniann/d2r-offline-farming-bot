package memory

import (
	"encoding/binary"
	"fmt"
)

// Gate 25.4, D2R 3.2.92777: runtime code at RVA 0x1EC7D1 reads the
// PanelManager global; BankExpansionLayout's initializer assigns the named
// material groups. Captured objects and provenance live in testdata/collection-tabs.json.
// These are UI structure offsets, not item IDs. No heap scan or screenshot
// interpretation participates in a gameplay snapshot.
const (
	collectionPanelManagerRVA = 0x1EF1758
	collectionStashVTableRVA  = 0x171C138
	collectionWidgetParentAt  = 0x30
	collectionWidgetStateAt   = 0x50
	collectionWidgetListAt    = 0x58
	collectionWidgetLengthAt  = 0x60
)

type collectionTabEvidence struct {
	manager, stash uintptr
	groups         [4]uintptr
	tab            string
}

func (p *ProbeReader) readCollectionTab(moduleBase uintptr) (collectionTabEvidence, error) {
	manager, err := p.reader.ReadUint64(moduleBase + collectionPanelManagerRVA)
	if err != nil || manager == 0 {
		return collectionTabEvidence{}, fmt.Errorf("collection panel manager unavailable")
	}
	panel, err := p.reader.ReadBytes(uintptr(manager), 0x80)
	if err != nil || !p.collectionWidgetNamed(panel, "PanelManager") {
		return collectionTabEvidence{}, fmt.Errorf("collection panel manager invalid")
	}
	length := binary.LittleEndian.Uint64(panel[collectionWidgetLengthAt:])
	list := binary.LittleEndian.Uint64(panel[collectionWidgetListAt:])
	if length == 0 || length > 64 || list == 0 {
		return collectionTabEvidence{}, fmt.Errorf("collection panel list unavailable")
	}
	children, err := p.reader.ReadBytes(uintptr(list), int(length)*8)
	if err != nil {
		return collectionTabEvidence{}, fmt.Errorf("collection panel list: %w", err)
	}
	var stash uintptr
	for i := 0; i < len(children); i += 8 {
		child := uintptr(binary.LittleEndian.Uint64(children[i:]))
		if child == 0 {
			return collectionTabEvidence{}, fmt.Errorf("collection panel child unavailable")
		}
		vtable, e := p.reader.ReadUint64(child)
		if e != nil {
			return collectionTabEvidence{}, fmt.Errorf("collection panel child: %w", e)
		}
		if uintptr(vtable) == moduleBase+collectionStashVTableRVA {
			if stash != 0 {
				return collectionTabEvidence{}, fmt.Errorf("collection stash widget ambiguous")
			}
			stash = child
		}
	}
	if stash == 0 {
		return collectionTabEvidence{}, fmt.Errorf("collection stash widget unavailable")
	}
	widget, err := p.reader.ReadBytes(stash, 0x2A0)
	if err != nil || !p.collectionWidgetNamed(widget, "BankExpansionLayout") || binary.LittleEndian.Uint64(widget[collectionWidgetParentAt:]) != manager || !collectionWidgetActive(widget) {
		return collectionTabEvidence{}, fmt.Errorf("collection stash widget invalid or inactive")
	}
	active := ""
	evidence := collectionTabEvidence{manager: uintptr(manager), stash: stash}
	for index, group := range []struct {
		offset    int
		name, tab string
	}{
		{0x278, "basicstash_container", "basic"},
		{0x288, "advancedstash_gems", "gems"},
		{0x290, "advancedstash_materials", "materials"},
		{0x298, "advancedstash_runes", "runes"},
	} {
		address := uintptr(binary.LittleEndian.Uint64(widget[group.offset:]))
		evidence.groups[index] = address
		if address == 0 {
			return collectionTabEvidence{}, fmt.Errorf("collection tab widget unavailable")
		}
		data, e := p.reader.ReadBytes(address, 0x80)
		if e != nil || !p.collectionWidgetNamed(data, group.name) || binary.LittleEndian.Uint64(data[collectionWidgetParentAt:]) != uint64(stash) || data[0x50] > 1 || data[0x51] > 1 || data[0x50] != data[0x51] {
			return collectionTabEvidence{}, fmt.Errorf("collection tab widget invalid")
		}
		if collectionWidgetActive(data) {
			if active != "" {
				return collectionTabEvidence{}, fmt.Errorf("collection tab ambiguous")
			}
			active = group.tab
		}
	}
	// Basic (Personal/Shared) and Materials are valid navigation starts. Retain
	// their evidence; ingredient actions still require confirmed Gems or Runes.
	if active == "" {
		return collectionTabEvidence{}, fmt.Errorf("collection stash tab inactive")
	}
	evidence.tab = active
	return evidence, nil
}

func (p *ProbeReader) collectionWidgetNamed(data []byte, name string) bool {
	// Captured widget names are pointer/length/capacity strings, not item names.
	length := binary.LittleEndian.Uint64(data[0x10:])
	// PanelManager's captured short name marks inline storage in the high bit.
	capacity := binary.LittleEndian.Uint64(data[0x18:]) &^ (uint64(1) << 63)
	address := binary.LittleEndian.Uint64(data[0x08:])
	if length != uint64(len(name)) || capacity < length || capacity > 128 || address == 0 {
		return false
	}
	text, err := p.reader.ReadBytes(uintptr(address), len(name))
	return err == nil && string(text) == name
}

func collectionWidgetActive(data []byte) bool {
	// The setters at RVA 0x14F2F0 and 0x66FB60 assign visibility and activation.
	// Both must agree; half-switched panels do not authorize a click.
	return data[collectionWidgetStateAt] == 1 && data[collectionWidgetStateAt+1] == 1
}

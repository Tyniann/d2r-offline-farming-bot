package memory

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func capturedCollectionTabs(t *testing.T, access *mockAccess, runes bool) map[string]uintptr {
	t.Helper()
	path := "testdata/collection-tabs.json"
	if runes {
		path = "testdata/collection-tabs-runes.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		ModuleBase uintptr `json:"module_base"`
		Segments   []struct {
			Address   uintptr
			Hex, Role string
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	access.moduleBase = fixture.ModuleBase
	roles := make(map[string]uintptr)
	for _, segment := range fixture.Segments {
		bytes, err := hex.DecodeString(segment.Hex)
		if err != nil {
			t.Fatal(err)
		}
		access.setBytes(segment.Address, bytes)
		roles[segment.Role] = segment.Address
	}
	return roles
}

func TestCollectionTabCapturedWidgetsAndUnavailableStates(t *testing.T) {
	activate := func(name string) func(*mockAccess, map[string]uintptr) {
		return func(a *mockAccess, r map[string]uintptr) {
			a.memory[r["advancedstash_gems"]][0x50], a.memory[r["advancedstash_gems"]][0x51] = 0, 0
			a.memory[r[name]][0x50], a.memory[r[name]][0x51] = 1, 1
		}
	}
	for _, test := range []struct {
		name, want string
		change     func(*mockAccess, map[string]uintptr)
	}{
		{"captured_gems", "gems", nil},
		{"captured_runes_new_process", "runes", nil},
		{"basic_start", "basic", activate("basicstash_container")},
		{"materials_start", "materials", activate("advancedstash_materials")},
		{"half_switched", "", func(a *mockAccess, r map[string]uintptr) { a.memory[r["advancedstash_gems"]][0x51] = 0 }},
		{"half_switched_sibling", "", func(a *mockAccess, r map[string]uintptr) { a.memory[r["advancedstash_runes"]][0x50] = 1 }},
		{"ambiguous", "", func(a *mockAccess, r map[string]uintptr) {
			a.memory[r["advancedstash_runes"]][0x50], a.memory[r["advancedstash_runes"]][0x51] = 1, 1
		}},
		{"wrong_parent", "", func(a *mockAccess, r map[string]uintptr) { collectionWriteU64(a, r["advancedstash_gems"]+0x30, 0) }},
		{"unreadable", "", func(a *mockAccess, r map[string]uintptr) { a.partialAt = r["advancedstash_runes"] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := newMockAccess()
			roles := capturedCollectionTabs(t, a, test.want == "runes")
			if test.change != nil {
				test.change(a, roles)
			}
			r := NewReader(testLogger())
			r.Bind(a)
			got, err := NewProbeReader(r, DefaultOffsetSet()).readCollectionTab(a.moduleBase)
			if test.want == "" {
				if err == nil {
					t.Fatal("unproved tab accepted")
				}
			} else if err != nil || got.tab != test.want {
				t.Fatalf("tab=%q, err=%v, want %q", got.tab, err, test.want)
			}
		})
	}
}

type changingTabAccess struct {
	*mockAccess
	roles map[string]uintptr
	reads int
}

func (a *changingTabAccess) ReadAt(address uintptr, data []byte) error {
	if address == a.moduleBase+collectionPanelManagerRVA {
		a.reads++
		if a.reads == 2 {
			a.memory[a.roles["advancedstash_gems"]][0x50], a.memory[a.roles["advancedstash_gems"]][0x51] = 0, 0
			a.memory[a.roles["advancedstash_runes"]][0x50], a.memory[a.roles["advancedstash_runes"]][0x51] = 1, 1
		}
	}
	return a.mockAccess.ReadAt(address, data)
}

func TestCollectionStorageChangingTabPreservesCountsAndRevokesTab(t *testing.T) {
	p, a, off, snap := collectionFixture(t)
	roles := capturedCollectionTabs(t, a, false)
	writeSegmentHead(a, a.moduleBase, off.UnitTable, unitSegmentPlayer, 0x30000)
	ui := make([]byte, uiBufferSize)
	ui[uiInventoryIndex], ui[uiStashIndex], ui[uiGateIndex] = 1, 1, 1
	a.setBytes(a.moduleBase+off.UI-uiBufferBefore, ui)
	p.reader.Bind(&changingTabAccess{mockAccess: a, roles: roles})
	got := p.readCollectionStorage(a.moduleBase, off, snap)
	if !got.Available || got.TabKnown || got.Tab != "" || len(got.Counts) != 1 || got.Counts[0].Count != 22 {
		t.Fatalf("changing tab evidence=%+v", got)
	}
}

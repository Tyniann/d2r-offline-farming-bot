package world

import (
	"fmt"
	"sort"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

// ItemQuality describes the item quality tier reported by D2R.
type ItemQuality uint32

// Item quality values used for read-only item enumeration.
const (
	ItemQualityUnknown    ItemQuality = 0
	ItemQualityLowQuality ItemQuality = 1
	ItemQualityNormal     ItemQuality = 2
	ItemQualitySuperior   ItemQuality = 3
	ItemQualityMagic      ItemQuality = 4
	ItemQualitySet        ItemQuality = 5
	ItemQualityRare       ItemQuality = 6
	ItemQualityUnique     ItemQuality = 7
	ItemQualityCrafted    ItemQuality = 8
)

// String returns a stable label for structured logging.
func (q ItemQuality) String() string {
	switch q {
	case ItemQualityLowQuality:
		return "low_quality"
	case ItemQualityNormal:
		return "normal"
	case ItemQualitySuperior:
		return "superior"
	case ItemQualityMagic:
		return "magic"
	case ItemQualitySet:
		return "set"
	case ItemQualityRare:
		return "rare"
	case ItemQualityUnique:
		return "unique"
	case ItemQualityCrafted:
		return "crafted"
	default:
		return "unknown"
	}
}

// BaseTier classifies weapon and armor bases from their generated D2R tier chain.
type BaseTier string

const (
	// BaseTierUnknown covers misc items and incomplete or absent equipment chains.
	BaseTierUnknown BaseTier = "unknown"
	// BaseTierNormal identifies the normal member of an equipment base chain.
	BaseTierNormal BaseTier = "normal"
	// BaseTierExceptional identifies the exceptional member of an equipment base chain.
	BaseTierExceptional BaseTier = "exceptional"
	// BaseTierElite identifies the elite member of an equipment base chain.
	BaseTierElite BaseTier = "elite"
)

// String returns the stable Pickit and telemetry label.
func (t BaseTier) String() string {
	if t == "" {
		return string(BaseTierUnknown)
	}
	return string(t)
}

// ItemLocation describes where an item is located in the game state.
type ItemLocation string

// Item location values modeled for loot and inventory work.
const (
	ItemLocationUnknown      ItemLocation = "unknown"
	ItemLocationGround       ItemLocation = "ground"
	ItemLocationInventory    ItemLocation = "inventory"
	ItemLocationEquipped     ItemLocation = "equipped"
	ItemLocationBelt         ItemLocation = "belt"
	ItemLocationCursor       ItemLocation = "cursor"
	ItemLocationCube         ItemLocation = "cube"
	ItemLocationStash        ItemLocation = "stash"
	ItemLocationSharedStash1 ItemLocation = "shared_stash_1"
	ItemLocationSharedStash2 ItemLocation = "shared_stash_2"
	ItemLocationSharedStash3 ItemLocation = "shared_stash_3"
	ItemLocationSocket       ItemLocation = "socket"
	ItemLocationVendor       ItemLocation = "vendor"
)

// String returns a stable label for structured logging.
func (l ItemLocation) String() string {
	if l == "" {
		return string(ItemLocationUnknown)
	}
	return string(l)
}

// ItemStat is a raw item stat entry retained for later Pickit evaluation.
type ItemStat struct {
	ID    uint16
	Layer uint16
	Value int32
}

// ItemIdentityReason beschreibt, warum eine Set-/Unique-Identität nicht konsistent aufgelöst wurde.
type ItemIdentityReason string

const (
	// ItemIdentityReasonUnavailable bedeutet, dass die rohe Referenz nicht gelesen werden konnte.
	ItemIdentityReasonUnavailable ItemIdentityReason = "item_identity_unavailable"
	// ItemIdentityReasonUnknown bedeutet, dass die rohe Referenz im patchgenauen Katalog fehlt.
	ItemIdentityReasonUnknown ItemIdentityReason = "item_identity_unknown"
	// ItemIdentityReasonQualityMismatch bedeutet, dass Katalogart und Item-Qualität widersprechen.
	ItemIdentityReasonQualityMismatch ItemIdentityReason = "item_identity_quality_mismatch"
	// ItemIdentityReasonBaseMismatch bedeutet, dass Katalog- und Item-Basiscode widersprechen.
	ItemIdentityReasonBaseMismatch ItemIdentityReason = "item_identity_base_mismatch"
)

// Item is a semantic item in the world model.
type Item struct {
	TxtFileNo         uint32
	UnitID            uint32
	Code              string
	Name              string
	Type              string
	NormalCode        string
	UberCode          string
	UltraCode         string
	BaseTier          BaseTier
	Quality           ItemQuality
	IdentityKind      ItemIdentityKind
	IdentityRawID     uint32
	IdentityKey       string
	IdentityName      string
	IdentityAvailable bool
	IdentityValid     bool
	IdentityReason    ItemIdentityReason
	Location          ItemLocation
	RawLocation       uint32
	OwnerID           uint32
	PlayerOwned       bool
	Page              int
	GridX             int // Inventory column 0..9; [Item.Width] expands across columns.
	GridY             int // Inventory row 0..3; [Item.Height] expands across rows.
	Width             int // Inventory footprint width in columns.
	Height            int // Inventory footprint height in rows.
	Position          Position
	Flags             uint32
	Identified        bool
	Ethereal          bool
	IsHovered         bool
	Stats             []ItemStat
	// SocketStatActive and SocketStatBase remain verbose diagnosis from memory.
	SocketStatActive SocketStatEvidence
	SocketStatBase   SocketStatEvidence
	// Sockets, SocketsAvailable and Socketed are the fail-closed Gate-19.0 projection.
	Sockets          int
	SocketsAvailable bool
	Socketed         bool
}

// SocketStatEvidence mirrors memory Active/Base Stat-194 diagnosis for verbose logs.
type SocketStatEvidence struct {
	ListReadable bool
	Present      bool
	Value        int32
}

type itemCatalogEntry struct {
	Code       string
	Name       string
	Type       string
	NormalCode string
	UberCode   string
	UltraCode  string
	BaseTier   BaseTier
	Width      int
	Height     int
}

// ItemCatalogEntry beschreibt einen stabilen Basiseintrag des eingebetteten Item-Katalogs.
type ItemCatalogEntry struct {
	TxtFileNo uint32
	Code      string
	Name      string
	Type      string
	BaseTier  BaseTier
}

// ItemCatalogEntries liefert eine defensive, nach TxtFileNo geordnete Katalogkopie.
func ItemCatalogEntries() []ItemCatalogEntry {
	entries := make([]ItemCatalogEntry, 0, len(itemCatalog))
	for id, entry := range itemCatalog {
		entries = append(entries, ItemCatalogEntry{TxtFileNo: id, Code: entry.Code, Name: entry.Name, Type: entry.Type, BaseTier: entry.BaseTier})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].TxtFileNo < entries[j].TxtFileNo })
	return entries
}

// LookupItemCode returns the stable item code for a D2R item text ID.
func LookupItemCode(txtFileNo uint32) string {
	return lookupItemCatalog(txtFileNo).Code
}

// LookupItemName returns the display name for a D2R item text ID.
func LookupItemName(txtFileNo uint32) string {
	return lookupItemCatalog(txtFileNo).Name
}

// LookupItemType returns the item type code for a D2R item text ID.
func LookupItemType(txtFileNo uint32) string {
	return lookupItemCatalog(txtFileNo).Type
}

// LookupItemDimensions returns the inventory footprint for a D2R item text ID.
func LookupItemDimensions(txtFileNo uint32) (width, height int) {
	entry := lookupItemCatalog(txtFileNo)
	return entry.Width, entry.Height
}

func lookupItemCatalog(txtFileNo uint32) itemCatalogEntry {
	if entry, ok := itemCatalog[txtFileNo]; ok {
		if entry.Type == "rune" && (entry.Width <= 0 || entry.Height <= 0) {
			entry.Width = 1
			entry.Height = 1
		}
		return entry
	}
	return itemCatalogEntry{Name: "Unknown Item"}
}

func mapItem(i memory.ItemUnit, hover HoverInfo) Item {
	stats := make([]ItemStat, 0, len(i.Stats))
	for _, s := range i.Stats {
		stats = append(stats, ItemStat{ID: s.ID, Layer: s.Layer, Value: s.Value})
	}
	entry := lookupItemCatalog(i.TxtFileNo)
	item := Item{
		TxtFileNo:        i.TxtFileNo,
		UnitID:           i.UnitID,
		Code:             entry.Code,
		Name:             entry.Name,
		Type:             entry.Type,
		NormalCode:       entry.NormalCode,
		UberCode:         entry.UberCode,
		UltraCode:        entry.UltraCode,
		BaseTier:         entry.BaseTier,
		Quality:          ItemQuality(i.Quality),
		Location:         mapItemLocation(i),
		RawLocation:      i.RawLocation,
		OwnerID:          i.OwnerID,
		PlayerOwned:      i.PlayerOwned,
		Page:             int(i.Page),
		GridX:            int(i.GridX),
		GridY:            int(i.GridY),
		Width:            entry.Width,
		Height:           entry.Height,
		Position:         Position{X: i.PosX, Y: i.PosY},
		Flags:            i.Flags,
		Identified:       i.Identified,
		Ethereal:         i.Ethereal,
		IsHovered:        hover.Matches(HoverUnitTypeItem, i.UnitID),
		Stats:            stats,
		SocketStatActive: mapSocketStatEvidence(i.SocketStatActive),
		SocketStatBase:   mapSocketStatEvidence(i.SocketStatBase),
		Sockets:          i.Sockets,
		SocketsAvailable: i.SocketsAvailable,
		Socketed:         i.Socketed,
	}
	applyItemIdentity(&item, i.UniqueSetID, i.UniqueSetIDAvailable, LookupItemIdentity)
	return item
}

func mapSocketStatEvidence(ev memory.SocketStatEvidence) SocketStatEvidence {
	return SocketStatEvidence{ListReadable: ev.ListReadable, Present: ev.Present, Value: ev.Value}
}

// FormatSocketStatEvidence returns a stable Gate-19.0 log token for one list.
func FormatSocketStatEvidence(ev SocketStatEvidence) string {
	switch {
	case !ev.ListReadable:
		return "unreadable"
	case !ev.Present:
		return "absent"
	default:
		return fmt.Sprintf("value:%d", ev.Value)
	}
}

type itemIdentityLookup func(ItemIdentityKind, uint32) (ItemIdentityCatalogEntry, bool)

func applyItemIdentity(item *Item, rawID int32, available bool, lookup itemIdentityLookup) {
	switch item.Quality {
	case ItemQualitySet:
		item.IdentityKind = ItemIdentitySet
	case ItemQualityUnique:
		item.IdentityKind = ItemIdentityUnique
	default:
		return
	}

	item.IdentityAvailable = available
	if !available {
		item.IdentityReason = ItemIdentityReasonUnavailable
		return
	}
	if rawID < 0 {
		item.IdentityReason = ItemIdentityReasonUnknown
		return
	}
	item.IdentityRawID = uint32(rawID)
	entry, ok := lookup(item.IdentityKind, item.IdentityRawID)
	if !ok {
		item.IdentityReason = ItemIdentityReasonUnknown
		return
	}
	if entry.Kind != item.IdentityKind {
		item.IdentityReason = ItemIdentityReasonQualityMismatch
		return
	}
	if entry.BaseCode != item.Code {
		item.IdentityReason = ItemIdentityReasonBaseMismatch
		return
	}
	item.IdentityKey = entry.Key
	item.IdentityName = entry.DisplayName
	item.IdentityValid = true
}

func mapItemLocation(i memory.ItemUnit) ItemLocation {
	switch i.RawLocation {
	case 0:
		if i.Flags&0x00002000 != 0 && i.OwnerID == ^uint32(0) {
			return ItemLocationVendor
		}
		if !i.PlayerOwned {
			return ItemLocationUnknown
		}
		switch i.Page {
		case 0:
			return ItemLocationInventory
		case 3:
			return ItemLocationCube
		default:
			return ItemLocationStash
		}
	case 1:
		if !i.PlayerOwned {
			return ItemLocationUnknown
		}
		return ItemLocationEquipped
	case 2:
		if !i.PlayerOwned {
			return ItemLocationUnknown
		}
		return ItemLocationBelt
	case 3, 5:
		return ItemLocationGround
	case 4:
		return ItemLocationCursor
	case 6:
		return ItemLocationSocket
	default:
		return ItemLocationUnknown
	}
}

// InventoryItems returns validated personal inventory items only.
func (s State) InventoryItems() []Item {
	items := s.ItemsByLocation(ItemLocationInventory)
	out := make([]Item, 0, len(items))
	for _, item := range items {
		if item.PlayerOwned && item.Page == 0 {
			out = append(out, item)
		}
	}
	return out
}

// GroundItems returns all items currently modeled at ground location.
func (s State) GroundItems() []Item {
	return s.ItemsByLocation(ItemLocationGround)
}

// ItemsByLocation returns items whose location matches any requested location.
func (s State) ItemsByLocation(locations ...ItemLocation) []Item {
	if len(locations) == 0 {
		out := make([]Item, len(s.Items))
		copy(out, s.Items)
		return out
	}
	want := make(map[ItemLocation]struct{}, len(locations))
	for _, l := range locations {
		want[l] = struct{}{}
	}
	out := make([]Item, 0)
	for _, item := range s.Items {
		if _, ok := want[item.Location]; ok {
			out = append(out, item)
		}
	}
	return out
}

// FindItemByUnitID returns the item with unitID, or false when none match.
func (s State) FindItemByUnitID(unitID uint32) (Item, bool) {
	for _, item := range s.Items {
		if item.UnitID == unitID {
			return item, true
		}
	}
	return Item{}, false
}

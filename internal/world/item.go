package world

import "github.com/Tyniann/d2r-offline-farming-bot/internal/memory"

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

// Item is a semantic item in the world model.
type Item struct {
	TxtFileNo   uint32
	UnitID      uint32
	Code        string
	Name        string
	Type        string
	NormalCode  string
	UberCode    string
	UltraCode   string
	Quality     ItemQuality
	Location    ItemLocation
	RawLocation uint32
	OwnerID     uint32
	PlayerOwned bool
	Page        int
	GridX       int // Inventory column 0..9; [Item.Width] expands across columns.
	GridY       int // Inventory row 0..3; [Item.Height] expands across rows.
	Width       int // Inventory footprint width in columns.
	Height      int // Inventory footprint height in rows.
	Position    Position
	Flags       uint32
	Identified  bool
	Ethereal    bool
	IsHovered   bool
	Stats       []ItemStat
}

type itemCatalogEntry struct {
	Code       string
	Name       string
	Type       string
	NormalCode string
	UberCode   string
	UltraCode  string
	Width      int
	Height     int
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
	return Item{
		TxtFileNo:   i.TxtFileNo,
		UnitID:      i.UnitID,
		Code:        entry.Code,
		Name:        entry.Name,
		Type:        entry.Type,
		NormalCode:  entry.NormalCode,
		UberCode:    entry.UberCode,
		UltraCode:   entry.UltraCode,
		Quality:     ItemQuality(i.Quality),
		Location:    mapItemLocation(i),
		RawLocation: i.RawLocation,
		OwnerID:     i.OwnerID,
		PlayerOwned: i.PlayerOwned,
		Page:        int(i.Page),
		GridX:       int(i.GridX),
		GridY:       int(i.GridY),
		Width:       entry.Width,
		Height:      entry.Height,
		Position:    Position{X: i.PosX, Y: i.PosY},
		Flags:       i.Flags,
		Identified:  i.Identified,
		Ethereal:    i.Ethereal,
		IsHovered:   hover.Matches(HoverUnitTypeItem, i.UnitID),
		Stats:       stats,
	}
}

func mapItemLocation(i memory.ItemUnit) ItemLocation {
	switch i.RawLocation {
	case 0:
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

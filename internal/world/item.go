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

func lookupItemCatalog(txtFileNo uint32) itemCatalogEntry {
	if entry, ok := itemCatalog[txtFileNo]; ok {
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
		Location:    mapItemLocation(i.RawLocation),
		RawLocation: i.RawLocation,
		Position:    Position{X: i.PosX, Y: i.PosY},
		Flags:       i.Flags,
		Identified:  i.Identified,
		Ethereal:    i.Ethereal,
		IsHovered:   hover.Matches(HoverUnitTypeItem, i.UnitID),
		Stats:       stats,
	}
}

func mapItemLocation(raw uint32) ItemLocation {
	switch raw {
	case 0:
		return ItemLocationInventory
	case 1:
		return ItemLocationEquipped
	case 2:
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

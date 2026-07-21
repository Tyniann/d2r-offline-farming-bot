package world

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestMapItemResolvesSetIdentityAgainstQualityAndBase(t *testing.T) {
	got := mapItem(memory.ItemUnit{
		TxtFileNo:            535,
		Quality:              uint32(ItemQualitySet),
		UniqueSetID:          77,
		UniqueSetIDAvailable: true,
	}, HoverInfo{})

	if !got.IdentityAvailable || !got.IdentityValid {
		t.Fatalf("identity available=%t valid=%t reason=%q", got.IdentityAvailable, got.IdentityValid, got.IdentityReason)
	}
	if got.IdentityKind != ItemIdentitySet || got.IdentityRawID != 77 || got.IdentityKey != "Tal Rasha's Adjudication" || got.IdentityName != "Tal Rasha's Adjudication" {
		t.Fatalf("identity = %+v", got)
	}
}

func TestMapItemIdentityFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		item memory.ItemUnit
		want ItemIdentityReason
	}{
		{
			name: "read unavailable",
			item: memory.ItemUnit{TxtFileNo: 535, Quality: uint32(ItemQualitySet)},
			want: ItemIdentityReasonUnavailable,
		},
		{
			name: "negative sentinel",
			item: memory.ItemUnit{TxtFileNo: 535, Quality: uint32(ItemQualitySet), UniqueSetID: -1, UniqueSetIDAvailable: true},
			want: ItemIdentityReasonUnknown,
		},
		{
			name: "unknown raw id",
			item: memory.ItemUnit{TxtFileNo: 535, Quality: uint32(ItemQualitySet), UniqueSetID: 9999, UniqueSetIDAvailable: true},
			want: ItemIdentityReasonUnknown,
		},
		{
			name: "base mismatch",
			item: memory.ItemUnit{TxtFileNo: 625, Quality: uint32(ItemQualitySet), UniqueSetID: 77, UniqueSetIDAvailable: true},
			want: ItemIdentityReasonBaseMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapItem(tt.item, HoverInfo{})
			if got.IdentityValid || got.IdentityName != "" || got.IdentityReason != tt.want {
				t.Fatalf("identity valid=%t name=%q reason=%q, want invalid/%q", got.IdentityValid, got.IdentityName, got.IdentityReason, tt.want)
			}
		})
	}
}

func TestApplyItemIdentityRejectsQualityMismatch(t *testing.T) {
	item := Item{Code: "amu", Quality: ItemQualitySet}
	lookup := func(ItemIdentityKind, uint32) (ItemIdentityCatalogEntry, bool) {
		return ItemIdentityCatalogEntry{Kind: ItemIdentityUnique, BaseCode: "amu", DisplayName: "wrong space"}, true
	}

	applyItemIdentity(&item, 77, true, lookup)
	if item.IdentityValid || item.IdentityReason != ItemIdentityReasonQualityMismatch {
		t.Fatalf("identity valid=%t reason=%q", item.IdentityValid, item.IdentityReason)
	}
}

func TestMapItemIgnoresIdentityReferenceForOtherQualities(t *testing.T) {
	got := mapItem(memory.ItemUnit{
		TxtFileNo:            535,
		Quality:              uint32(ItemQualityRare),
		UniqueSetID:          77,
		UniqueSetIDAvailable: true,
	}, HoverInfo{})
	if got.IdentityKind != "" || got.IdentityAvailable || got.IdentityValid || got.IdentityReason != "" {
		t.Fatalf("non-set/unique identity should be not applicable: %+v", got)
	}
}

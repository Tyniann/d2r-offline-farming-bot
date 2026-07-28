package pathing

import (
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestMatchSystemEgressLayoutToleratesSmallAnchorDrift(t *testing.T) {
	live := LayoutFingerprint{
		Version:     1,
		AreaID:      world.LutGholein,
		AnchorCount: 1,
		Hash:        "live-hash",
		Anchors:     []string{"o:267:5121,5073"},
	}
	err := MatchSystemEgressLayout(live, 1, world.LutGholein, 1, "bound-hash", []string{"o:267:5121,5076"})
	if err != nil {
		t.Fatalf("expected stash Y±3 within tolerance: %v", err)
	}
}

func TestMatchSystemEgressLayoutRejectsLargeDriftAndIdentityChange(t *testing.T) {
	live := LayoutFingerprint{
		Version:     1,
		AreaID:      world.LutGholein,
		AnchorCount: 1,
		Hash:        "live-hash",
		Anchors:     []string{"o:267:5121,5073"},
	}
	if err := MatchSystemEgressLayout(live, 1, world.LutGholein, 1, "bound-hash", []string{"o:267:5121,5080"}); !errors.Is(err, ErrRouteLayoutMismatch) {
		t.Fatalf("large drift err = %v", err)
	}
	if err := MatchSystemEgressLayout(live, 1, world.LutGholein, 1, "bound-hash", []string{"o:156:5121,5073"}); !errors.Is(err, ErrRouteLayoutMismatch) {
		t.Fatalf("identity change err = %v", err)
	}
}

func TestMatchSystemEgressLayoutLegacyExactHash(t *testing.T) {
	live := LayoutFingerprint{Version: 1, AreaID: world.LutGholein, AnchorCount: 1, Hash: "abc", Anchors: []string{"o:267:1,1"}}
	if err := MatchSystemEgressLayout(live, 1, world.LutGholein, 1, "abc", nil); err != nil {
		t.Fatalf("exact hash should pass: %v", err)
	}
	if err := MatchSystemEgressLayout(live, 1, world.LutGholein, 1, "other", nil); !errors.Is(err, ErrRouteLayoutMismatch) {
		t.Fatalf("hash mismatch err = %v", err)
	}
}

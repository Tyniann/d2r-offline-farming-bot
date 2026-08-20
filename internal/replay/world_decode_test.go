package replay

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestParseObjectKindAcceptsLowerKurastKinds(t *testing.T) {
	chest := parseObjectKind(EntityFrame{Kind: "super_chest", ID: world.JungleChestID})
	if chest != world.ObjectKindSuperChest {
		t.Fatalf("catalog id = %s, want super_chest", chest)
	}
	rack := parseObjectKind(EntityFrame{Kind: "rack", ID: world.WeaponRack2ID})
	if rack != world.ObjectKindRack {
		t.Fatalf("catalog id = %s, want rack", rack)
	}
	legacyUnknown := parseObjectKind(EntityFrame{Kind: "unknown", ID: world.JungleChestID})
	if legacyUnknown != world.ObjectKindUnknown {
		t.Fatalf("legacy unknown label = %s, want unknown", legacyUnknown)
	}
	byLabel := parseObjectKind(EntityFrame{Kind: "super_chest", ID: 9999})
	if byLabel != world.ObjectKindSuperChest {
		t.Fatalf("kind label without catalog id = %s, want super_chest", byLabel)
	}
}

func TestWorldDecodePreservesObjectModeAndItemQuantity(t *testing.T) {
	now := time.Now()
	state := worldStateFromFrame(Frame{World: WorldFrame{
		Phase: "in_game", Valid: true,
		Objects: []EntityFrame{{Kind: "super_chest", ID: world.JungleChestID, UnitID: 183, X: 5032, Y: 2994, Mode: world.ObjectModeClosed, ModeKnown: true}},
		Items:   []ItemFrame{{Code: "key", Location: "inventory", PlayerOwned: true, Quantity: 7, QuantityKnown: true}},
	}}, now)
	if len(state.Objects) != 1 || !state.Objects[0].ModeKnown || state.Objects[0].Mode != world.ObjectModeClosed {
		t.Fatalf("decoded object = %+v", state.Objects)
	}
	if len(state.Items) != 1 || !state.Items[0].QuantityKnown || state.Items[0].Quantity != 7 {
		t.Fatalf("decoded item = %+v", state.Items)
	}
	frame := NormalizeWorld(state)
	if len(frame.Objects) != 1 || !frame.Objects[0].ModeKnown || frame.Objects[0].Mode != world.ObjectModeClosed {
		t.Fatalf("normalized object = %+v", frame.Objects)
	}
	if len(frame.Items) != 1 || !frame.Items[0].QuantityKnown || frame.Items[0].Quantity != 7 {
		t.Fatalf("normalized item = %+v", frame.Items)
	}
}

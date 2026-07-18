package tasks

import (
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestDefaultRunRegistryMetadataAndOrder(t *testing.T) {
	definitions := DefaultRunRegistry().Definitions()
	if len(definitions) != 2 || definitions[0].ID != RunIDCountess || definitions[1].ID != RunIDMephisto {
		t.Fatalf("definitions = %+v", definitions)
	}
	if len(definitions[0].BossEngageSequence) != 1 || !definitions[0].Boss.RequireSuperUnique || definitions[0].ReturnOrigin != town.OriginAct1 {
		t.Fatalf("Countess definition = %+v", definitions[0])
	}
	if len(definitions[1].BossEngageSequence) != 2 || definitions[1].Boss.NPCID != 242 || definitions[1].Boss.RequireSuperUnique || definitions[1].ReturnOrigin != town.OriginAct3 {
		t.Fatalf("Mephisto definition = %+v", definitions[1])
	}
	if definitions[0].Recording.StartWaypoint != pathing.WaypointTargetBlackMarsh || definitions[0].Recording.TerminalArea != world.TowerCellarLevel5 || definitions[0].Recording.Boss.NPCID != definitions[0].Boss.NPCID || definitions[0].Recording.TerminalMaxDistanceTiles != 80 {
		t.Fatalf("Countess recording contract = %+v", definitions[0].Recording)
	}
	if definitions[1].Recording.StartWaypoint != pathing.WaypointTargetDuranceOfHateLevel2 || definitions[1].Recording.TerminalArea != world.DuranceOfHateLevel3 || definitions[1].Recording.Boss.NPCID != definitions[1].Boss.NPCID || definitions[1].Recording.TerminalMaxDistanceTiles != 60 {
		t.Fatalf("Mephisto recording contract = %+v", definitions[1].Recording)
	}
}

func TestRunRegistryRejectsDuplicateAndInvalidIDs(t *testing.T) {
	countess, _ := DefaultRunRegistry().Definition(RunIDCountess)
	if _, err := NewRunRegistry(countess, countess); err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("duplicate error = %v", err)
	}
	countess.ID = "Bad ID"
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), string(RunReasonDefinitionInvalid)) {
		t.Fatalf("invalid id error = %v", err)
	}
}

func TestRunRegistryRejectsInvalidCapabilityCombinations(t *testing.T) {
	mephisto, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	mephisto.RequiredCaps = withoutCapability(mephisto.RequiredCaps, RunCapabilityForeignTownEgress)
	if _, err := NewRunRegistry(mephisto); err == nil || !strings.Contains(err.Error(), string(RunReasonCapabilityMissing)) {
		t.Fatalf("missing egress capability error = %v", err)
	}

	countess, _ := DefaultRunRegistry().Definition(RunIDCountess)
	countess.RequiredCaps = append(countess.RequiredCaps, RunCapabilityForeignTownEgress)
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), "must not require foreign Town egress") {
		t.Fatalf("invalid Act-1 capability error = %v", err)
	}

	countess, _ = DefaultRunRegistry().Definition(RunIDCountess)
	countess.BossEngageSequence = []EncounterAction{{Hook: profile.HookTownReady}}
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), "boss_engage") {
		t.Fatalf("invalid encounter action error = %v", err)
	}

	countess, _ = DefaultRunRegistry().Definition(RunIDCountess)
	countess.Boss.RequireSuperUnique = false
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), "fallback requires") {
		t.Fatalf("invalid boss identity error = %v", err)
	}
}

func TestRunRegistryResolveRejectsUnknownAndMissingConfig(t *testing.T) {
	registry := DefaultRunRegistry()
	if _, err := registry.Resolve("baal", nil); err == nil || !strings.Contains(err.Error(), string(RunReasonUnknown)) {
		t.Fatalf("unknown run error = %v", err)
	}
	if _, err := registry.Resolve(RunIDCountess, nil); err == nil || !strings.Contains(err.Error(), string(RunReasonConfigMissing)) {
		t.Fatalf("missing config error = %v", err)
	}

	config := RunConfig{RouteID: "route", Loot: RunLootConfig{PickupFile: "pickit/countess.nip"}}
	resolved, err := registry.Resolve(RunIDCountess, map[RunID]RunConfig{RunIDCountess: config})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Definition.ID != RunIDCountess || resolved.Config.Loot.PickupFile != "pickit/countess.nip" {
		t.Fatalf("resolved run = %+v", resolved)
	}
}

func TestRunRegistryReturnsDefensiveDefinitionCopies(t *testing.T) {
	registry := DefaultRunRegistry()
	definition, _ := registry.Definition(RunIDMephisto)
	definition.BossEngageSequence[0].Hook = profile.HookTownReady
	definition.RequiredCaps[0] = "mutated"
	definition.Recording.AllowedRouteAreas[0] = world.None
	again, _ := registry.Definition(RunIDMephisto)
	if again.BossEngageSequence[0].Hook != profile.HookBossEngage || again.RequiredCaps[0] == "mutated" || again.Recording.AllowedRouteAreas[0] == world.None {
		t.Fatalf("registry definition mutated through returned slices: %+v", again)
	}
}

func withoutCapability(capabilities []RunCapability, remove RunCapability) []RunCapability {
	result := make([]RunCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		if capability != remove {
			result = append(result, capability)
		}
	}
	return result
}

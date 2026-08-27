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
	if len(definitions) != 6 ||
		definitions[0].ID != RunIDCountess ||
		definitions[1].ID != RunIDCows ||
		definitions[2].ID != RunIDLowerKurast ||
		definitions[3].ID != RunIDMephisto ||
		definitions[4].ID != RunIDNihlathak ||
		definitions[5].ID != RunIDSummoner {
		t.Fatalf("definitions = %+v", definitions)
	}
	countess := definitions[0]
	cows := definitions[1]
	lowerKurast := definitions[2]
	mephisto := definitions[3]
	nihlathak := definitions[4]
	summoner := definitions[5]
	if len(countess.BossEngageSequence) != 1 || !countess.Boss.RequireSuperUnique || countess.ReturnOrigin != town.OriginAct1 || countess.ClearNearbyAfterBoss {
		t.Fatalf("Countess definition = %+v", countess)
	}
	if len(mephisto.BossEngageSequence) != 2 || mephisto.Boss.NPCID != world.Mephisto || mephisto.Boss.RequireSuperUnique || mephisto.ReturnOrigin != town.OriginAct3 {
		t.Fatalf("Mephisto definition = %+v", mephisto)
	}
	if len(nihlathak.BossEngageSequence) != 0 || nihlathak.Boss.NPCID != world.Nihlathak ||
		nihlathak.Boss.SearchAnchorObject != world.ObjectKindUnknown || nihlathak.Boss.SearchAnchorEntrance != world.EntranceKindUnknown ||
		!nihlathak.ClearNearbyAfterBoss || nihlathak.ReturnOrigin != town.OriginAct5 {
		t.Fatalf("Nihlathak definition = %+v", nihlathak)
	}
	if len(summoner.BossEngageSequence) != 0 || summoner.Boss.NPCID != world.Summoner ||
		summoner.Boss.SearchAnchorObject != world.ObjectKindUnknown || summoner.Boss.SearchAnchorEntrance != world.EntranceKindUnknown ||
		summoner.ReturnOrigin != town.OriginAct2 {
		t.Fatalf("Summoner definition = %+v", summoner)
	}
	if countess.Recording.StartWaypoint != pathing.WaypointTargetBlackMarsh || countess.Recording.TerminalArea != world.TowerCellarLevel5 || countess.Recording.Boss.NPCID != countess.Boss.NPCID || countess.Recording.TerminalMaxDistanceTiles != 80 {
		t.Fatalf("Countess recording contract = %+v", countess.Recording)
	}
	if mephisto.Recording.StartWaypoint != pathing.WaypointTargetDuranceOfHateLevel2 || mephisto.Recording.TerminalArea != world.DuranceOfHateLevel3 || mephisto.Recording.Boss.NPCID != mephisto.Boss.NPCID || mephisto.Recording.TerminalMaxDistanceTiles != 60 {
		t.Fatalf("Mephisto recording contract = %+v", mephisto.Recording)
	}
	if summoner.Recording.StartWaypoint != pathing.WaypointTargetArcaneSanctuary || summoner.Recording.TerminalArea != world.ArcaneSanctuary || summoner.Recording.EgressOriginAct != town.OriginAct2 {
		t.Fatalf("Summoner recording contract = %+v", summoner.Recording)
	}
	if !summoner.HasCapability(RunCapabilityRouteClear) ||
		!summoner.AllowsRouteHostile(world.ArcaneSpecter) ||
		!summoner.AllowsRouteHostile(world.ArcaneHellClan) ||
		!summoner.AllowsRouteHostile(world.ArcaneGhoulLord) ||
		len(summoner.RouteHostileNPCIDs) != 3 {
		t.Fatalf("Summoner route-clear contract = %+v", summoner)
	}
	for _, definition := range []RunDefinition{countess, mephisto, nihlathak} {
		if !definition.HasCapability(RunCapabilityLocalRecoveryClear) {
			t.Fatalf("%s must enable local recovery clear: %+v", definition.ID, definition.RequiredCaps)
		}
	}
	for _, definition := range []RunDefinition{summoner, cows, lowerKurast} {
		if definition.HasCapability(RunCapabilityLocalRecoveryClear) {
			t.Fatalf("%s unexpectedly enables local recovery clear: %+v", definition.ID, definition.RequiredCaps)
		}
	}
	for _, definition := range []RunDefinition{countess, mephisto, nihlathak, lowerKurast} {
		if definition.HasCapability(RunCapabilityRouteClear) || len(definition.RouteHostileNPCIDs) != 0 {
			t.Fatalf("%s unexpectedly enables route clear: %+v", definition.ID, definition)
		}
	}
	if nihlathak.Recording.StartWaypoint != pathing.WaypointTargetHallsOfPain || nihlathak.Recording.TerminalArea != world.HallsOfVaught || nihlathak.Recording.EgressOriginAct != town.OriginAct5 {
		t.Fatalf("Nihlathak recording contract = %+v", nihlathak.Recording)
	}
	if cows.RouteSet == nil || cows.RouteSet.PrimaryRole != pathing.RouteRoleCowSweep || len(cows.RouteRoles()) != 2 {
		t.Fatalf("Cow route set = %+v", cows.RouteSet)
	}
	leg, legOK := cows.RecordingForRole(pathing.RouteRoleLegAcquisition)
	sweep, sweepOK := cows.RecordingForRole(pathing.RouteRoleCowSweep)
	if !legOK || !sweepOK || leg.StartWaypoint != pathing.WaypointTargetStonyField || leg.TerminalObjectKind != world.ObjectKindWirtsBody || sweep.StartKind != RecordingStartObjectPortalArrival || sweep.TerminalKind != RecordingTerminalEndpoint {
		t.Fatalf("Cow recording contracts leg=%+v sweep=%+v", leg, sweep)
	}
	if lowerKurast.DisplayName != "Lower Kurast" ||
		lowerKurast.EntryArea != world.LowerKurast ||
		lowerKurast.RouteTerminalArea != world.LowerKurast ||
		lowerKurast.WaypointTarget != pathing.WaypointTargetLowerKurast ||
		lowerKurast.Boss.NPCID != 0 ||
		strings.TrimSpace(lowerKurast.Boss.Name) != "" ||
		len(lowerKurast.BossEngageSequence) != 0 ||
		lowerKurast.ClearNearbyAfterBoss ||
		lowerKurast.ReturnOrigin != town.OriginAct3 ||
		!lowerKurast.HasCapability(RunCapabilityChestSweep) ||
		!lowerKurast.HasCapability(RunCapabilityForeignTownEgress) ||
		lowerKurast.RouteSet != nil {
		t.Fatalf("Lower Kurast definition = %+v", lowerKurast)
	}
	if lowerKurast.Recording.StartKind != RecordingStartWaypoint ||
		lowerKurast.Recording.StartWaypoint != pathing.WaypointTargetLowerKurast ||
		lowerKurast.Recording.AllowedStartArea != world.LowerKurast ||
		lowerKurast.Recording.TerminalKind != RecordingTerminalEndpoint ||
		lowerKurast.Recording.TerminalArea != world.LowerKurast ||
		lowerKurast.Recording.Boss.NPCID != 0 ||
		lowerKurast.Recording.TerminalObjectKind != world.ObjectKindUnknown ||
		lowerKurast.Recording.TerminalMaxDistanceTiles != 60 ||
		lowerKurast.Recording.Movement != pathing.RouteMovementTeleport ||
		lowerKurast.Recording.SafetyReturn != RecordingSafetyReturnTownPortal ||
		lowerKurast.Recording.EgressOriginAct != town.OriginAct3 ||
		lowerKurast.Recording.InstructionCode != "record_lower_kurast" ||
		town.KeyRestockNextRun != string(RunIDLowerKurast) {
		t.Fatalf("Lower Kurast recording contract = %+v", lowerKurast.Recording)
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

func TestRunRegistryRequiresExplicitRecordingKinds(t *testing.T) {
	countess, _ := DefaultRunRegistry().Definition(RunIDCountess)
	countess.Recording.StartKind = ""
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), "start kind") {
		t.Fatalf("empty recording start kind error = %v", err)
	}

	countess, _ = DefaultRunRegistry().Definition(RunIDCountess)
	countess.Recording.TerminalKind = ""
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), "terminal kind") {
		t.Fatalf("empty recording terminal kind error = %v", err)
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

	summoner, _ := DefaultRunRegistry().Definition(RunIDSummoner)
	if len(summoner.BossEngageSequence) != 0 {
		t.Fatalf("Summoner engage sequence should be empty, got %+v", summoner.BossEngageSequence)
	}
	if _, err := NewRunRegistry(summoner); err != nil {
		t.Fatalf("empty engage sequence should be valid: %v", err)
	}

	countess, _ = DefaultRunRegistry().Definition(RunIDCountess)
	countess.Boss.RequireSuperUnique = false
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), "fallback requires") {
		t.Fatalf("invalid boss identity error = %v", err)
	}

	countess, _ = DefaultRunRegistry().Definition(RunIDCountess)
	countess.RouteHostileNPCIDs = []uint32{world.ArcaneSpecter}
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), string(RunCapabilityRouteClear)) {
		t.Fatalf("allowlist without capability error = %v", err)
	}

	summoner, _ = DefaultRunRegistry().Definition(RunIDSummoner)
	summoner.RouteHostileNPCIDs = nil
	if _, err := NewRunRegistry(summoner); err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("capability without allowlist error = %v", err)
	}

	lowerKurast, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	lowerKurast.RequiredCaps = withoutCapability(lowerKurast.RequiredCaps, RunCapabilityChestSweep)
	if _, err := NewRunRegistry(lowerKurast); err == nil || !strings.Contains(err.Error(), "boss descriptor is required for a single-route run") {
		t.Fatalf("boss-less single-route without chest_sweep error = %v", err)
	}

	countess, _ = DefaultRunRegistry().Definition(RunIDCountess)
	countess.RequiredCaps = append(countess.RequiredCaps, RunCapabilityChestSweep)
	if _, err := NewRunRegistry(countess); err == nil || !strings.Contains(err.Error(), "chest_sweep run must not declare a boss descriptor") {
		t.Fatalf("chest_sweep with boss error = %v", err)
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

	config := RunConfig{RouteID: "route"}
	resolved, err := registry.Resolve(RunIDCountess, map[RunID]RunConfig{RunIDCountess: config})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Definition.ID != RunIDCountess || resolved.Config.RouteID != "route" {
		t.Fatalf("resolved run = %+v", resolved)
	}
}

func TestRunRegistryReturnsDefensiveDefinitionCopies(t *testing.T) {
	registry := DefaultRunRegistry()
	definition, _ := registry.Definition(RunIDMephisto)
	definition.BossEngageSequence[0].Hook = profile.HookTownReady
	definition.RequiredCaps[0] = "mutated"
	definition.Recording.AllowedRouteAreas[0] = world.None
	summoner, _ := registry.Definition(RunIDSummoner)
	summoner.RouteHostileNPCIDs[0] = 999
	again, _ := registry.Definition(RunIDMephisto)
	summonerAgain, _ := registry.Definition(RunIDSummoner)
	if again.BossEngageSequence[0].Hook != profile.HookBossEngage || again.RequiredCaps[0] == "mutated" || again.Recording.AllowedRouteAreas[0] == world.None ||
		summonerAgain.RouteHostileNPCIDs[0] == 999 {
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

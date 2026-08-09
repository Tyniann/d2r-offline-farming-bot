package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type cowClearMock struct {
	routeResults []profile.Result
	ceResults    []profile.Result
	requests     []profile.RouteClearRequest
	ceUnitIDs    []uint32
	resets       int
}

func (m *cowClearMock) TickRouteClear(_ context.Context, request profile.RouteClearRequest, _ time.Time) profile.Result {
	m.requests = append(m.requests, request)
	if len(m.routeResults) == 0 {
		return profile.Result{Status: profile.StatusPending}
	}
	result := m.routeResults[0]
	m.routeResults = m.routeResults[1:]
	return result
}

func (m *cowClearMock) TickAuthorizedCorpseExplosion(_ context.Context, _ world.State, unitID uint32, _ time.Time) profile.Result {
	m.ceUnitIDs = append(m.ceUnitIDs, unitID)
	if len(m.ceResults) == 0 {
		return profile.Result{Status: profile.StatusPending}
	}
	result := m.ceResults[0]
	m.ceResults = m.ceResults[1:]
	return result
}

func (m *cowClearMock) ResetRouteClear() { m.resets++ }

func cowHoldState(at time.Time, generation uint64, monsters []world.Monster, corpses []world.CowCorpse) world.State {
	for index := range corpses {
		corpses[index].ObservedAt = at
		corpses[index].SnapshotGeneration = generation
		corpses[index].ConsumptionKnown = true
	}
	return world.State{
		At: at, Generation: generation, Valid: true, Phase: world.GamePhaseInGame,
		Area: world.LookupArea(world.MooMooFarm), Player: world.Player{Position: world.Position{X: 100, Y: 100}},
		Monsters: monsters, CowCorpsesComplete: true, CowCorpses: corpses,
	}
}

func cowClearRequest(state world.State, target world.Monster) profile.RouteClearRequest {
	return profile.RouteClearRequest{RunID: string(RunIDCows), DefinitionID: "necro_bone_spear", Player: state.Player, Target: target, AssessmentAt: state.At}
}

func cowLivingPack(count int) []world.Monster {
	monsters := make([]world.Monster, count)
	for index := range monsters {
		monsters[index] = world.Monster{
			NPCID: world.HellBovine, UnitID: uint32(index + 7),
			Position: world.Position{X: uint32(106 + index), Y: 100},
		}
	}
	return monsters
}

func TestCowHoldUsesAmplifyDamageOnceThenBoneSpear(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 0, 0, 0, time.UTC)
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 106, Y: 100}}
	state := cowHoldState(now, 1, []world.Monster{cow}, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(state)

	first := executor.TickRouteClear(context.Background(), cowClearRequest(state, cow), now)
	second := executor.TickRouteClear(context.Background(), cowClearRequest(state, cow), now.Add(time.Second))
	if first.ActionKind != profile.RouteClearActionCurse || second.ActionKind != profile.RouteClearActionAttack || len(delegate.requests) != 2 {
		t.Fatalf("first=%+v second=%+v requests=%d", first, second, len(delegate.requests))
	}
}

func TestCowHoldLatchesFreshPackBoundaryUntilRouteResumes(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 0, 30, 0, time.UTC)
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 102, Y: 100}}
	oldCorpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 41, Position: world.Position{X: 112, Y: 100}}
	pack := cowLivingPack(cowCorpseExplosionMinimumLiving)
	pack[0] = cow
	state := cowHoldState(now, 1, pack, []world.CowCorpse{oldCorpse})
	delegate := &cowClearMock{
		routeResults: []profile.Result{
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
		},
		ceResults: []profile.Result{{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCorpseExplosion}},
	}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(state)
	request := cowClearRequest(state, cow)

	_ = executor.TickRouteClear(context.Background(), request, now)
	attack := executor.TickRouteClear(context.Background(), request, now.Add(time.Second))
	if attack.ActionKind != profile.RouteClearActionAttack || len(delegate.ceUnitIDs) != 0 {
		t.Fatalf("old-pack decision=%+v CE=%v", attack, delegate.ceUnitIDs)
	}

	// Even when the old corpse later becomes the nearest object, the latched
	// boundary permits only a corpse first observed after this Hold began.
	oldCorpse.Position = world.Position{X: 101, Y: 100}
	newCorpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 104, Y: 100}}
	fresh := cowHoldState(now.Add(2*time.Second), 2, pack, []world.CowCorpse{oldCorpse, newCorpse})
	executor.ObserveObjectiveProgress(fresh)
	ce := executor.TickRouteClear(context.Background(), cowClearRequest(fresh, cow), fresh.At)
	if ce.ActionKind != profile.RouteClearActionCorpseExplosion || len(delegate.ceUnitIDs) != 1 || delegate.ceUnitIDs[0] != 42 {
		t.Fatalf("fresh-pack result=%+v CE=%v", ce, delegate.ceUnitIDs)
	}
}

func TestCowHoldSkipsUnprojectableLivingTargetBeforeRangeRecovery(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 0, 45, 0, time.UTC)
	firstCow := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 102, Y: 100}}
	secondCow := world.Monster{NPCID: world.HellBovine, UnitID: 8, Position: world.Position{X: 104, Y: 100}}
	state := cowHoldState(now, 1, []world.Monster{firstCow, secondCow}, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusPending, Reason: profile.RouteClearReasonTargetUnprojectable},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(state)

	first := executor.TickRouteClear(context.Background(), cowClearRequest(state, firstCow), now)
	second := executor.TickRouteClear(context.Background(), cowClearRequest(state, firstCow), now.Add(time.Second))
	if first.Reason != "" || second.ActionKind != profile.RouteClearActionCurse || len(delegate.requests) != 2 ||
		delegate.requests[0].Target.UnitID != 7 || delegate.requests[1].Target.UnitID != 8 {
		t.Fatalf("first=%+v second=%+v requests=%+v", first, second, delegate.requests)
	}
}

func TestCowHoldPrefersCorpseAndBoundsIneffectiveAttempts(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 0, 0, time.UTC)
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 106, Y: 100}}
	corpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 105, Y: 100}}
	pack := cowLivingPack(cowCorpseExplosionMinimumLiving)
	pack[0] = cow
	state := cowHoldState(now, 1, pack, []world.CowCorpse{corpse})
	delegate := &cowClearMock{
		routeResults: []profile.Result{
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
		},
		ceResults: []profile.Result{
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCorpseExplosion},
			{Status: profile.StatusComplete, Reason: profile.CorpseExplosionReasonSettled},
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCorpseExplosion},
			{Status: profile.StatusComplete, Reason: profile.CorpseExplosionReasonSettled},
		},
	}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(state)
	request := cowClearRequest(state, cow)

	_ = executor.TickRouteClear(context.Background(), request, now)
	_ = executor.TickRouteClear(context.Background(), request, now.Add(time.Second))
	_ = executor.TickRouteClear(context.Background(), request, now.Add(2*time.Second))
	last := executor.TickRouteClear(context.Background(), request, now.Add(3*time.Second))
	if len(delegate.ceUnitIDs) != 4 || delegate.ceUnitIDs[0] != 42 || delegate.ceUnitIDs[2] != 42 {
		t.Fatalf("CE calls=%v", delegate.ceUnitIDs)
	}
	if last.ActionKind != profile.RouteClearActionAttack || len(delegate.requests) != 2 {
		t.Fatalf("post-attempt result=%+v requests=%d", last, len(delegate.requests))
	}
}

func TestCowHoldUsesActiveGroupDensityInsteadOfCombinedDistantCows(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 15, 0, time.UTC)
	monsters := []world.Monster{
		{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 102, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 8, Position: world.Position{X: 103, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 9, Position: world.Position{X: 104, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 10, Position: world.Position{X: 105, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 11, Position: world.Position{X: 120, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 12, Position: world.Position{X: 121, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 13, Position: world.Position{X: 122, Y: 100}},
	}
	state := cowHoldState(now, 1, monsters, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(state)

	_ = executor.TickRouteClear(context.Background(), cowClearRequest(state, monsters[0]), now)
	attack := executor.TickRouteClear(context.Background(), cowClearRequest(state, monsters[0]), now.Add(time.Second))
	if attack.ActionKind != profile.RouteClearActionAttack || len(delegate.ceUnitIDs) != 0 || delegate.requests[1].Target.UnitID != 7 {
		t.Fatalf("split-group result=%+v CE=%v requests=%+v", attack, delegate.ceUnitIDs, delegate.requests)
	}
}

func TestCowHoldIncludesTwelveTileGroupMembersForCorpseExplosion(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 16, 0, time.UTC)
	monsters := []world.Monster{
		{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 102, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 8, Position: world.Position{X: 111, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 9, Position: world.Position{X: 112, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 10, Position: world.Position{X: 113, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 11, Position: world.Position{X: 114, Y: 100}},
	}
	initial := cowHoldState(now, 1, monsters, nil)
	delegate := &cowClearMock{
		routeResults: []profile.Result{{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse}},
		ceResults:    []profile.Result{{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCorpseExplosion}},
	}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	opener := executor.TickRouteClear(context.Background(), cowClearRequest(initial, monsters[0]), now)
	if opener.CowGroupLivingCount != 5 {
		t.Fatalf("twelve-tile group living=%d, want 5", opener.CowGroupLivingCount)
	}

	corpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 108, Y: 100}}
	fresh := cowHoldState(now.Add(time.Second), 2, monsters, []world.CowCorpse{corpse})
	executor.ObserveObjectiveProgress(fresh)
	ce := executor.TickRouteClear(context.Background(), cowClearRequest(fresh, monsters[0]), fresh.At)
	if ce.ActionKind != profile.RouteClearActionCorpseExplosion || len(delegate.ceUnitIDs) != 1 || delegate.ceUnitIDs[0] != corpse.UnitID ||
		ce.CowGroupLivingCount != 5 || ce.CowCorpseCoverageCount != 5 {
		t.Fatalf("twelve-tile CE result=%+v calls=%v", ce, delegate.ceUnitIDs)
	}
}

func TestCowHoldPinsAnchorWithinGroupWithoutEmergency(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 17, 0, time.UTC)
	first := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 102, Y: 100}}
	second := world.Monster{NPCID: world.HellBovine, UnitID: 8, Position: world.Position{X: 103, Y: 100}}
	distant := world.Monster{NPCID: world.HellBovine, UnitID: 20, Position: world.Position{X: 113, Y: 100}}
	initial := cowHoldState(now, 1, []world.Monster{first, second, distant}, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(initial, first), now)

	// Unit 8 becomes nearer, but the still-living anchor 7 remains authoritative.
	first.Position = world.Position{X: 105, Y: 100}
	second.Position = world.Position{X: 101, Y: 100}
	moved := cowHoldState(now.Add(time.Second), 2, []world.Monster{first, second, distant}, nil)
	executor.ObserveObjectiveProgress(moved)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(moved, second), moved.At)
	if len(delegate.requests) != 2 || delegate.requests[0].Target.UnitID != 7 || delegate.requests[1].Target.UnitID != 7 {
		t.Fatalf("pinned group requests=%+v", delegate.requests)
	}
}

func TestCowHoldPreemptsRemoteAnchorForImmediateGroupThreat(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 30, 0, 0, time.UTC)
	anchor := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 103, Y: 100}}
	approaching := world.Monster{NPCID: world.HellBovine, UnitID: 8, Position: world.Position{X: 108, Y: 100}}
	initial := cowHoldState(now, 1, []world.Monster{anchor, approaching}, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(initial, anchor), now)

	anchor.Position = world.Position{X: 125, Y: 100}
	approaching.Position = world.Position{X: 114, Y: 100}
	threatened := cowHoldState(now.Add(time.Second), 2, []world.Monster{anchor, approaching}, nil)
	executor.ObserveObjectiveProgress(threatened)
	result := executor.TickRouteClear(context.Background(), cowClearRequest(threatened, approaching), threatened.At)
	if result.CowGroupAnchorUnitID != approaching.UnitID || len(delegate.requests) != 2 || delegate.requests[1].Target.UnitID != approaching.UnitID {
		t.Fatalf("immediate group preemption result=%+v requests=%+v", result, delegate.requests)
	}
}

func TestCowHoldPreemptsRemoteGroupForImmediateOutsideThreat(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 18, 0, time.UTC)
	anchor := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 103, Y: 100}}
	outside := world.Monster{NPCID: world.HellBovine, UnitID: 20, Position: world.Position{X: 116, Y: 100}}
	initial := cowHoldState(now, 1, []world.Monster{anchor, outside}, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(initial, anchor), now)

	anchor.Position = world.Position{X: 125, Y: 100}
	threatened := cowHoldState(now.Add(time.Second), 2, []world.Monster{anchor, outside}, nil)
	executor.ObserveObjectiveProgress(threatened)
	result := executor.TickRouteClear(context.Background(), cowClearRequest(threatened, outside), threatened.At)
	if result.CowGroupAnchorUnitID != outside.UnitID || len(delegate.requests) != 2 || delegate.requests[1].Target.UnitID != outside.UnitID {
		t.Fatalf("preemption result=%+v requests=%+v", result, delegate.requests)
	}
}

func TestCowHoldKeepsPinnedGroupWithinEmergencyHysteresis(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 19, 0, time.UTC)
	anchor := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 103, Y: 100}}
	outside := world.Monster{NPCID: world.HellBovine, UnitID: 20, Position: world.Position{X: 116, Y: 100}}
	initial := cowHoldState(now, 1, []world.Monster{anchor, outside}, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(initial, anchor), now)

	anchor.Position = world.Position{X: 117, Y: 100}
	moved := cowHoldState(now.Add(time.Second), 2, []world.Monster{anchor, outside}, nil)
	executor.ObserveObjectiveProgress(moved)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(moved, outside), moved.At)
	if len(delegate.requests) != 2 || delegate.requests[1].Target.UnitID != anchor.UnitID {
		t.Fatalf("hysteresis requests=%+v", delegate.requests)
	}
}

func TestCowHoldDenseGroupIgnoresFreshCorpseOutsideAnchorRadius(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 20, 0, time.UTC)
	monsters := cowLivingPack(cowCorpseExplosionMinimumLiving)
	king := world.Monster{
		NPCID: world.CowKing, UnitID: 99, Position: world.Position{X: 125, Y: 100},
		MonsterTypeFlag: world.SuperUniqueMonsterFlag,
	}
	monsters = append(monsters, king)
	initial := cowHoldState(now, 1, monsters, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(initial, monsters[0]), now)

	farCorpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 120, Y: 100}}
	fresh := cowHoldState(now.Add(time.Second), 2, monsters, []world.CowCorpse{farCorpse})
	executor.ObserveObjectiveProgress(fresh)
	attack := executor.TickRouteClear(context.Background(), cowClearRequest(fresh, monsters[0]), fresh.At)
	if attack.ActionKind != profile.RouteClearActionAttack || len(delegate.ceUnitIDs) != 0 || delegate.requests[1].Target.UnitID != monsters[0].UnitID {
		t.Fatalf("far-corpse result=%+v CE=%v requests=%+v", attack, delegate.ceUnitIDs, delegate.requests)
	}
}

func TestCowHoldChoosesCorpseWithBestActiveGroupCoverage(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 25, 0, time.UTC)
	monsters := []world.Monster{
		{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 105, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 8, Position: world.Position{X: 112, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 9, Position: world.Position{X: 113, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 10, Position: world.Position{X: 114, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 11, Position: world.Position{X: 115, Y: 100}},
	}
	initial := cowHoldState(now, 1, monsters, nil)
	delegate := &cowClearMock{
		routeResults: []profile.Result{{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse}},
		ceResults:    []profile.Result{{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCorpseExplosion}},
	}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(initial, monsters[0]), now)

	nearPlayer := world.CowCorpse{NPCID: world.HellBovine, UnitID: 41, Position: world.Position{X: 96, Y: 100}}
	bestCoverage := world.CowCorpse{NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 112, Y: 100}}
	fresh := cowHoldState(now.Add(time.Second), 2, monsters, []world.CowCorpse{nearPlayer, bestCoverage})
	executor.ObserveObjectiveProgress(fresh)
	ce := executor.TickRouteClear(context.Background(), cowClearRequest(fresh, monsters[0]), fresh.At)
	if ce.ActionKind != profile.RouteClearActionCorpseExplosion || len(delegate.ceUnitIDs) != 1 || delegate.ceUnitIDs[0] != 42 {
		t.Fatalf("coverage result=%+v CE=%v", ce, delegate.ceUnitIDs)
	}
	if ce.CowGroupAnchorUnitID != 7 || ce.CowGroupLivingCount != 5 ||
		ce.CowCorpseAnchorDistanceTiles != 7 || ce.CowCorpseCoverageCount != 5 {
		t.Fatalf("coverage telemetry context=%+v", ce)
	}
}

func TestCowHoldUsesBoneSpearBelowCorpseExplosionDensity(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 30, 0, time.UTC)
	pack := cowLivingPack(cowCorpseExplosionMinimumLiving - 1)
	corpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 101, Y: 100}}
	state := cowHoldState(now, 1, pack, []world.CowCorpse{corpse})
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(state)
	request := cowClearRequest(state, pack[0])

	first := executor.TickRouteClear(context.Background(), request, now)
	second := executor.TickRouteClear(context.Background(), request, now.Add(time.Second))
	if first.ActionKind != profile.RouteClearActionCurse || second.ActionKind != profile.RouteClearActionAttack || len(delegate.ceUnitIDs) != 0 {
		t.Fatalf("first=%+v second=%+v CE=%v", first, second, delegate.ceUnitIDs)
	}
}

func TestCowHoldRejectsLatchedCorpseExplosionWithLowCoverage(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 35, 0, time.UTC)
	dense := cowLivingPack(cowCorpseExplosionMinimumLiving)
	initial := cowHoldState(now, 1, dense, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
		{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
	}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(initial, dense[0]), now)

	remnants := dense[:2]
	corpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 107, Y: 100}}
	shrunk := cowHoldState(now.Add(time.Second), 2, remnants, []world.CowCorpse{corpse})
	executor.ObserveObjectiveProgress(shrunk)
	result := executor.TickRouteClear(context.Background(), cowClearRequest(shrunk, remnants[0]), shrunk.At)
	if result.ActionKind != profile.RouteClearActionAttack || len(delegate.ceUnitIDs) != 0 || executor.combatPhase != cowHoldCleanup {
		t.Fatalf("low-coverage result=%+v CE=%v phase=%v", result, delegate.ceUnitIDs, executor.combatPhase)
	}
}

func TestCowHoldUsesCorpseExplosionBeforeCleaningUpShrunkenPack(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 1, 45, 0, time.UTC)
	initialPack := cowLivingPack(cowCorpseExplosionMinimumLiving - 1)
	initial := cowHoldState(now, 1, initialPack, nil)
	delegate := &cowClearMock{
		routeResults: []profile.Result{
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse},
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack},
		},
		ceResults: []profile.Result{
			{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCorpseExplosion},
			{Status: profile.StatusComplete, Reason: profile.CorpseExplosionReasonSettled},
		},
	}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	executor.ObserveObjectiveProgress(initial)
	request := cowClearRequest(initial, initialPack[0])

	_ = executor.TickRouteClear(context.Background(), request, now)
	attack := executor.TickRouteClear(context.Background(), request, now.Add(time.Second))
	if attack.ActionKind != profile.RouteClearActionAttack {
		t.Fatalf("low-density result=%+v", attack)
	}

	densePack := cowLivingPack(cowCorpseExplosionMinimumLiving)
	dense := cowHoldState(now.Add(2*time.Second), 2, densePack, nil)
	executor.ObserveObjectiveProgress(dense)
	denseAttack := executor.TickRouteClear(context.Background(), cowClearRequest(dense, densePack[0]), dense.At)
	if denseAttack.ActionKind != profile.RouteClearActionAttack {
		t.Fatalf("dense pre-corpse result=%+v", denseAttack)
	}

	firstCorpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 104, Y: 100}}
	shrunkPack := cowLivingPack(cowCorpseExplosionMinimumLiving - 1)
	shrunk := cowHoldState(now.Add(3*time.Second), 3, shrunkPack, []world.CowCorpse{firstCorpse})
	executor.ObserveObjectiveProgress(shrunk)
	firstCE := executor.TickRouteClear(context.Background(), cowClearRequest(shrunk, shrunkPack[0]), shrunk.At)
	if firstCE.ActionKind != profile.RouteClearActionCorpseExplosion || len(delegate.ceUnitIDs) != 1 || delegate.ceUnitIDs[0] != 42 {
		t.Fatalf("first CE result=%+v CE=%v", firstCE, delegate.ceUnitIDs)
	}

	firstCorpse.Consumed = true
	secondCorpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 43, Position: world.Position{X: 105, Y: 100}}
	cleanupPack := cowLivingPack(2)
	cleanup := cowHoldState(now.Add(4*time.Second), 4, cleanupPack, []world.CowCorpse{firstCorpse, secondCorpse})
	executor.ObserveObjectiveProgress(cleanup)
	cleanupAttack := executor.TickRouteClear(context.Background(), cowClearRequest(cleanup, cleanupPack[0]), cleanup.At)
	if cleanupAttack.ActionKind != profile.RouteClearActionAttack || len(delegate.ceUnitIDs) != 2 || delegate.ceUnitIDs[1] != 42 {
		t.Fatalf("cleanup result=%+v CE=%v", cleanupAttack, delegate.ceUnitIDs)
	}
}

func TestCowHoldPrioritizesCowKingAndReportsCorpseProgress(t *testing.T) {
	now := time.Date(2026, 8, 1, 20, 2, 0, 0, time.UTC)
	regular := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 102, Y: 100}, IsHovered: true}
	king := world.Monster{NPCID: world.CowKing, UnitID: 9, Position: world.Position{X: 110, Y: 100}, MonsterTypeFlag: world.SuperUniqueMonsterFlag}
	state := cowHoldState(now, 1, []world.Monster{regular, king}, nil)
	delegate := &cowClearMock{routeResults: []profile.Result{{Status: profile.StatusAction, ActionKind: profile.RouteClearActionCurse}}}
	executor := newCowHoldExecutor(phase17ThreatConfig())
	executor.bind(delegate)
	if executor.ObserveObjectiveProgress(state) {
		t.Fatal("initial corpse baseline counted as progress")
	}
	_ = executor.TickRouteClear(context.Background(), cowClearRequest(state, regular), now)
	if len(delegate.requests) != 1 || delegate.requests[0].Target.UnitID != king.UnitID {
		t.Fatalf("target requests=%+v", delegate.requests)
	}

	newCorpse := world.CowCorpse{NPCID: world.HellBovine, UnitID: 44, Position: world.Position{X: 104, Y: 100}}
	fresh := cowHoldState(now.Add(time.Second), 2, []world.Monster{regular, king}, []world.CowCorpse{newCorpse})
	if !executor.ObserveObjectiveProgress(fresh) {
		t.Fatal("new current corpse did not count as objective progress")
	}
	consumed := cowHoldState(now.Add(2*time.Second), 3, []world.Monster{regular, king}, []world.CowCorpse{newCorpse})
	consumed.CowCorpses[0].Consumed = true
	if !executor.ObserveObjectiveProgress(consumed) {
		t.Fatal("consumed overlapping corpse did not count as objective progress")
	}
}

type terminalCowRoute struct {
	progress  RouteProgress
	holdCalls int
	tickCalls int
}

func (r *terminalCowRoute) Start(string, world.State) error { return nil }
func (r *terminalCowRoute) Progress(world.State) (RouteProgress, bool) {
	return r.progress, true
}
func (r *terminalCowRoute) Hold(world.State) error { r.holdCalls++; return nil }
func (r *terminalCowRoute) Tick(context.Context, world.State) (bool, error) {
	r.tickCalls++
	return true, nil
}
func (r *terminalCowRoute) Reset() {}

func TestCowSweepRequiresThreeFreshSafeTerminalSnapshots(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	config := phase17ThreatConfig()
	route := &terminalCowRoute{progress: RouteProgress{RouteID: "cow", Mode: RouteProgressTransition}}
	pipeline := runPipeline{definition: definition, routeID: "cow", routeCombat: config, requireTerminalSafe: true}
	clear := &routeClearMock{}
	base := time.Date(2026, 8, 1, 20, 3, 0, 0, time.UTC)
	for snapshot := 1; snapshot <= Phase17StableClearSnapshots; snapshot++ {
		state := cowHoldState(base.Add(time.Duration(snapshot)*time.Second), uint64(snapshot), nil, nil)
		result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: clear}, pipelineStepPlayRoute, state, state.At, base)
		if snapshot < Phase17StableClearSnapshots && (result.complete || result.failed || route.tickCalls != 0) {
			t.Fatalf("snapshot %d result=%+v ticks=%d", snapshot, result, route.tickCalls)
		}
		if snapshot == Phase17StableClearSnapshots && (!result.complete || result.failed || route.tickCalls != 1) {
			t.Fatalf("terminal result=%+v ticks=%d", result, route.tickCalls)
		}
	}
}

func TestCowSweepInventoryFullSkipsLootAndContinuesRoute(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	config := phase17ThreatConfig()
	route := &mockRoutePlayback{progressOK: true, progress: RouteProgress{
		RouteID: "cow", Mode: RouteProgressMovement, TargetAvailable: true,
		MovementTarget: world.Position{X: 120, Y: 100},
	}}
	loot := &mockLootActions{scans: []LootScanResult{{InventoryFull: true, InventoryFullCandidateCount: 1}}}
	pipeline := runPipeline{definition: definition, routeID: "cow", routeCombat: config}
	state := cowHoldState(time.Date(2026, 8, 1, 20, 4, 0, 0, time.UTC), 1, nil, nil)
	result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: &routeClearMock{}, Loot: loot}, pipelineStepPlayRoute, state, state.At, state.At)
	if result.failed || result.complete || route.tickCalls != 1 || len(loot.startCalls) != 0 {
		t.Fatalf("result=%+v route ticks=%d pickup starts=%d", result, route.tickCalls, len(loot.startCalls))
	}
}

func TestCowRouteThreatUsesDedicatedNoProgressReason(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	config := phase17ThreatConfig()
	config.NoProgressTimeout = time.Second
	progress := phase17ThreatProgress()
	route := controllerRoute(progress)
	clear := &routeClearMock{result: profile.Result{Status: profile.StatusAction}}
	var controller RouteThreatController
	base := time.Date(2026, 8, 1, 20, 5, 0, 0, time.UTC)
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 105, Y: 100}}
	for index := 0; index < 2; index++ {
		state := cowHoldState(base.Add(time.Duration(index)*time.Second), uint64(index+1), []world.Monster{cow}, nil)
		assessment := assessThreats(state, progress, definition.RouteHostileNPCIDs, config)
		result := controller.Tick(context.Background(), route, clear, state, progress, assessment, definition, config, "necro_bone_spear", state.At)
		if index == 0 && result.Failed {
			t.Fatalf("first tick failed: %+v", result)
		}
		if index == 1 && (!result.Failed || result.Reason != RouteThreatReasonCowNoProgress) {
			t.Fatalf("no-progress result=%+v", result)
		}
	}
}

type cowApproachClear struct {
	routeClearMock
	confirmedApproaches int
}

func (c *cowApproachClear) ObserveRouteClearApproachProgress() {
	c.confirmedApproaches++
}

func TestCowDensityProjectionLossUsesBoundedRouteApproach(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	base := time.Date(2026, 8, 1, 20, 6, 0, 0, time.UTC)
	config := phase17ThreatConfig()
	config.NoProgressTimeout = 12 * time.Second
	progress := RouteProgress{
		RouteID: "cow", Mode: RouteProgressMovement, TargetAvailable: true,
		MovementTarget: world.Position{X: 103, Y: 100},
	}
	// The cow is within attack range but off the next short route edge. Moving
	// five tiles along the command overshoots its three-tile goal and moves away
	// from the cow; only command-vector progress is valid evidence here.
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 77, Position: world.Position{X: 100, Y: 125}}
	route := controllerRoute(progress)
	clear := &cowApproachClear{routeClearMock: routeClearMock{result: profile.Result{
		Status: profile.StatusPending, Reason: profile.RouteClearReasonTargetUnprojectable,
	}}}
	projectable := true
	combat := &mockCombatActions{farthestOK: &projectable, farthestPosition: world.Position{X: 100, Y: 105}, farthestDistance: 20}
	trace := &pipelineTelemetry{}
	pipeline := &runPipeline{
		definition: definition, routeID: "cow", combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: config,
	}
	deps := Deps{Route: route, RouteClear: clear, Combat: combat, Telemetry: trace}

	for index := 0; index < Phase17StableClearSnapshots; index++ {
		state := cowHoldState(base.Add(time.Duration(index)*100*time.Millisecond), uint64(index+1), []world.Monster{cow}, nil)
		result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, base)
		if result.failed {
			t.Fatalf("projection tick %d failed: %+v", index, result)
		}
	}
	if combat.teleportCalls != 1 || combat.forceMoveCalls != 0 || combat.lastTeleportTarget != cow.Position || route.tickCalls != 0 {
		t.Fatalf("teleports=%d force moves=%d target=%+v route ticks=%d", combat.teleportCalls, combat.forceMoveCalls, combat.lastTeleportTarget, route.tickCalls)
	}

	// The next snapshot may already make combat projectable. Recovery progress
	// must still be confirmed before the normal clear result can discard it.
	clear.result = profile.Result{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack, TargetUnitID: cow.UnitID}
	progressed := cowHoldState(base.Add(time.Second), 4, []world.Monster{cow}, nil)
	progressed.Player.Position = world.Position{X: 100, Y: 105}
	result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, progressed, progressed.At, base)
	if result.failed || clear.confirmedApproaches != 1 || pipeline.routeApproachPending {
		t.Fatalf("confirmed approach result=%+v confirmations=%d pending=%t", result, clear.confirmedApproaches, pipeline.routeApproachPending)
	}
	approachRecorded := false
	for _, event := range trace.events {
		if event.Event == telemetry.RouteClearAction && event.ModeName == "approach" && event.ActionKind != "teleport" {
			t.Fatalf("Cow approach action=%+v", event)
		}
		if event.Event == telemetry.RouteClearAction && event.ModeName == "approach" && event.ActionKind == "teleport" {
			approachRecorded = true
		}
	}
	if !approachRecorded {
		t.Fatalf("missing Cow teleport approach: %+v", trace.events)
	}
}

func TestCowProjectionLossApproachesDuringRouteRecovery(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	base := time.Date(2026, 8, 1, 20, 7, 0, 0, time.UTC)
	config := phase17ThreatConfig()
	config.NoProgressTimeout = 12 * time.Second
	progress := RouteProgress{
		RouteID: "cow", Mode: RouteProgressRecovery, TargetAvailable: true,
		MovementTarget: world.Position{X: 103, Y: 100},
	}
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 78, Position: world.Position{X: 100, Y: 125}}
	route := controllerRoute(progress)
	clear := &cowApproachClear{routeClearMock: routeClearMock{result: profile.Result{
		Status: profile.StatusPending, Reason: profile.RouteClearReasonTargetUnprojectable,
	}}}
	projectable := true
	combat := &mockCombatActions{farthestOK: &projectable, farthestPosition: world.Position{X: 100, Y: 105}, farthestDistance: 20}
	pipeline := &runPipeline{
		definition: definition, routeID: "cow", combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: config,
	}
	deps := Deps{Route: route, RouteClear: clear, Combat: combat}

	for index := 0; index < Phase17StableClearSnapshots; index++ {
		state := cowHoldState(base.Add(time.Duration(index)*100*time.Millisecond), uint64(index+1), []world.Monster{cow}, nil)
		result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, base)
		if result.failed {
			t.Fatalf("recovery projection tick %d failed: %+v", index, result)
		}
	}
	if combat.teleportCalls != 1 || combat.lastTeleportTarget != cow.Position || route.tickCalls != 0 {
		t.Fatalf("teleports=%d target=%+v route ticks=%d", combat.teleportCalls, combat.lastTeleportTarget, route.tickCalls)
	}
}

func TestCowProjectionApproachRejectsLandingInsidePack(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	base := time.Date(2026, 8, 9, 13, 31, 0, 0, time.UTC)
	config := phase17ThreatConfig()
	config.NoProgressTimeout = 12 * time.Second
	progress := RouteProgress{
		RouteID: "cow", Mode: RouteProgressMovement, TargetAvailable: true,
		MovementTarget: world.Position{X: 103, Y: 100},
	}
	target := world.Monster{NPCID: world.HellBovine, UnitID: 78, Position: world.Position{X: 100, Y: 125}}
	nearLanding := world.Monster{NPCID: world.HellBovine, UnitID: 79, Position: world.Position{X: 101, Y: 106}}
	route := controllerRoute(progress)
	clear := &cowApproachClear{routeClearMock: routeClearMock{result: profile.Result{
		Status: profile.StatusPending, Reason: profile.RouteClearReasonTargetUnprojectable,
		TargetUnitID: target.UnitID, TargetNPCID: target.NPCID,
	}}}
	projectable := true
	combat := &mockCombatActions{
		farthestOK: &projectable, farthestPosition: world.Position{X: 100, Y: 105}, farthestDistance: 20,
	}
	pipeline := &runPipeline{
		definition: definition, routeID: "cow", combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: config,
	}
	deps := Deps{Route: route, RouteClear: clear, Combat: combat}

	for index := 0; index < Phase17StableClearSnapshots; index++ {
		state := cowHoldState(base.Add(time.Duration(index)*100*time.Millisecond), uint64(index+1), []world.Monster{target, nearLanding}, nil)
		if result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, base); result.failed {
			t.Fatalf("unsafe projection tick %d failed: %+v", index, result)
		}
	}
	if combat.teleportCalls != 0 || pipeline.routeApproachExhaustedUnitID != target.UnitID {
		t.Fatalf("unsafe landing teleports=%d exhausted=%d", combat.teleportCalls, pipeline.routeApproachExhaustedUnitID)
	}
}

func TestCowUnprojectableTargetDefersRunFailureToCombatWatchdog(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	base := time.Date(2026, 8, 9, 9, 0, 0, 0, time.UTC)
	config := phase17ThreatConfig()
	config.NoProgressTimeout = 12 * time.Second
	progress := RouteProgress{
		RouteID: "cow", Mode: RouteProgressMovement, TargetAvailable: true,
		MovementTarget: world.Position{X: 103, Y: 100},
	}
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 79, Position: world.Position{X: 100, Y: 125}}
	route := controllerRoute(progress)
	clear := &cowApproachClear{routeClearMock: routeClearMock{result: profile.Result{
		Status: profile.StatusPending, Reason: profile.RouteClearReasonTargetUnprojectable,
	}}}
	projectable := false
	combat := &mockCombatActions{farthestOK: &projectable}
	pipeline := &runPipeline{
		definition: definition, routeID: "cow", combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: config,
	}
	deps := Deps{Route: route, RouteClear: clear, Combat: combat}

	for index := 0; index < Phase17StableClearSnapshots; index++ {
		state := cowHoldState(base.Add(time.Duration(index)*100*time.Millisecond), uint64(index+1), []world.Monster{cow}, nil)
		if result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, base); result.failed {
			t.Fatalf("projection tick %d bypassed watchdog: %+v", index, result)
		}
	}
	if pipeline.routeApproachExhaustedUnitID != cow.UnitID || combat.teleportCalls != 0 {
		t.Fatalf("exhausted=%d teleports=%d", pipeline.routeApproachExhaustedUnitID, combat.teleportCalls)
	}

	timedOut := cowHoldState(base.Add(config.NoProgressTimeout+time.Second), 4, []world.Monster{cow}, nil)
	result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, timedOut, timedOut.At, base)
	if !result.failed || result.reason != string(RouteThreatReasonCowNoProgress) {
		t.Fatalf("watchdog result=%+v, want %s", result, RouteThreatReasonCowNoProgress)
	}
}

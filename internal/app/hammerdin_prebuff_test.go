package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type hammerdinPrebuffInputMock struct {
	keys       []string
	selections []uint16
	clicks     []input.MouseButton
	moves      [][2]int
	window     input.WindowInfo
}

func (m *hammerdinPrebuffInputMock) PressKey(key string) error {
	m.keys = append(m.keys, key)
	return nil
}
func (m *hammerdinPrebuffInputMock) SelectSkill(_ input.BindingSource, skillID uint16) error {
	m.selections = append(m.selections, skillID)
	return nil
}
func (m *hammerdinPrebuffInputMock) Click(button input.MouseButton) error {
	m.clicks = append(m.clicks, button)
	return nil
}
func (m *hammerdinPrebuffInputMock) Focus() error { return nil }
func (m *hammerdinPrebuffInputMock) MoveTo(x, y int) error {
	m.moves = append(m.moves, [2]int{x, y})
	return nil
}
func (m *hammerdinPrebuffInputMock) Window() (input.WindowInfo, bool) { return m.window, true }

func TestHammerdinCTAPrebuffConfirmsEveryStepAndRestoresCombatSlots(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newHammerdinPrebuff(hammerdinPrebuffBindings(true), in, true)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(100, 0)
	state := hammerdinTownState(at, 1, world.WeaponSetPrimary, 0, 0)
	tick := func() hammerdinPrebuffResult {
		t.Helper()
		state.Generation++
		state.At = state.At.Add(400 * time.Millisecond)
		result, tickErr := prebuff.tick(state, state.At)
		if tickErr != nil {
			t.Fatalf("tick state=%d: %v", prebuff.state, tickErr)
		}
		return result
	}

	tick() // Confirm Primary.
	if result := tick(); result.Action != "weapon_set_secondary" {
		t.Fatalf("secondary action=%q", result.Action)
	}
	state.Player.ActiveWeaponSet.Set = world.WeaponSetSecondary
	tick() // Confirm Secondary.

	for _, skillID := range []uint16{
		memory.MustSkillID("battle_command"), memory.MustSkillID("battle_orders"),
		memory.MustSkillID("battle_command"), memory.MustSkillID("holy_shield"),
	} {
		tick() // Select.
		state.Player.RightSkillID = skillID
		tick() // Confirm and cast.
	}
	state.At = state.At.Add(prebuffWeaponSwapSettle)

	if result := tick(); result.Action != "weapon_set_primary" {
		t.Fatalf("primary action=%q", result.Action)
	}
	state.Player.ActiveWeaponSet.Set = world.WeaponSetPrimary
	tick() // Confirm Primary.
	tick() // Select Blessed Hammer.
	state.Player.LeftSkillID = memory.MustSkillID("blessed_hammer")
	tick() // Confirm Blessed Hammer.
	tick() // Select Concentration.
	state.Player.RightSkillID = memory.MustSkillID("concentration")
	if result := tick(); !result.Done {
		t.Fatalf("final result=%+v state=%d", result, prebuff.state)
	}

	if got, want := strings.Join(in.keys, ","), "w,w"; got != want {
		t.Fatalf("swap keys=%q want=%q", got, want)
	}
	wantSelections := []uint16{155, 149, 155, 117, 112, 113}
	if len(in.selections) != len(wantSelections) {
		t.Fatalf("selections=%v", in.selections)
	}
	for i, want := range wantSelections {
		if in.selections[i] != want {
			t.Fatalf("selection[%d]=%d want=%d", i, in.selections[i], want)
		}
	}
	if len(in.clicks) != 4 || len(in.moves) != 4 {
		t.Fatalf("clicks=%v moves=%v", in.clicks, in.moves)
	}
	for _, move := range in.moves {
		if move != [2]int{640, 360} {
			t.Fatalf("self-cast move=%v", move)
		}
	}
	if prebuff.ctaAnchor.due(state.At) {
		t.Fatal("CTA anchor due immediately after confirmed second Battle Command")
	}
	if !prebuff.ctaAnchor.due(state.At.Add(ctaRecastAfter)) {
		t.Fatal("CTA anchor not due after 150 seconds")
	}
}

func TestHammerdinNoCTAPrebuffNeverSwapsOrSelectsCTA(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newHammerdinPrebuff(hammerdinPrebuffBindings(false), in, false)
	if err != nil {
		t.Fatal(err)
	}
	state := hammerdinTownState(time.Unix(200, 0), 1, world.WeaponSetPrimary, 0, 0)
	tick := func() hammerdinPrebuffResult {
		t.Helper()
		state.Generation++
		state.At = state.At.Add(400 * time.Millisecond)
		result, tickErr := prebuff.tick(state, state.At)
		if tickErr != nil {
			t.Fatal(tickErr)
		}
		return result
	}
	tick()
	tick()
	state.Player.RightSkillID = memory.MustSkillID("holy_shield")
	tick()
	tick()
	state.Player.LeftSkillID = memory.MustSkillID("blessed_hammer")
	tick()
	tick()
	state.Player.RightSkillID = memory.MustSkillID("concentration")
	if result := tick(); !result.Done {
		t.Fatalf("result=%+v", result)
	}
	if len(in.keys) != 0 {
		t.Fatalf("no-CTA swap keys=%v", in.keys)
	}
	if got, want := in.selections, []uint16{117, 112, 113}; len(got) != len(want) {
		t.Fatalf("no-CTA selections=%v", got)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("no-CTA selections=%v", got)
			}
		}
	}
	if len(in.clicks) != 1 {
		t.Fatalf("no-CTA clicks=%v", in.clicks)
	}
}

func TestHammerdinPrebuffFailsClosedWithoutToggleLoop(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newHammerdinPrebuff(hammerdinPrebuffBindings(true), in, true)
	if err != nil {
		t.Fatal(err)
	}
	state := hammerdinTownState(time.Unix(300, 0), 1, world.WeaponSetPrimary, 0, 0)
	if _, err := prebuff.tick(state, state.At); err != nil {
		t.Fatal(err)
	}
	state.Generation++
	state.At = state.At.Add(100 * time.Millisecond)
	if _, err := prebuff.tick(state, state.At); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		state.Generation++
		state.At = state.At.Add(400 * time.Millisecond)
		if _, err := prebuff.tick(state, state.At); err != nil {
			t.Fatalf("early confirmation wait: %v", err)
		}
	}
	state.Generation++
	state.At = state.At.Add(400 * time.Millisecond)
	if _, err := prebuff.tick(state, state.At); err == nil || !strings.Contains(err.Error(), reasonWeaponSetUnconfirmed) {
		t.Fatalf("unconfirmed swap error=%v", err)
	}
	if len(in.keys) != 1 {
		t.Fatalf("swap sent %d times, want exactly one", len(in.keys))
	}

	prebuff.reset()
	state.Player.ActiveWeaponSet = world.WeaponSetState{}
	if _, err := prebuff.tick(state, state.At); err == nil || !strings.Contains(err.Error(), reasonWeaponSetUnavailable) {
		t.Fatalf("unavailable error=%v", err)
	}
	if len(in.keys) != 1 {
		t.Fatalf("unavailable read sent input: %v", in.keys)
	}
}

func TestHammerdinPrebuffMapsWrongCTASelectionToStableReason(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newHammerdinPrebuff(hammerdinPrebuffBindings(true), in, true)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(400, 0)
	state := hammerdinTownState(at, 1, world.WeaponSetPrimary, 0, 0)
	_, _ = prebuff.tick(state, at)
	state.Generation++
	state.At = at.Add(100 * time.Millisecond)
	_, _ = prebuff.tick(state, state.At)
	state.Player.ActiveWeaponSet.Set = world.WeaponSetSecondary
	state.Generation++
	state.At = state.At.Add(100 * time.Millisecond)
	_, _ = prebuff.tick(state, state.At)
	state.Generation++
	state.At = state.At.Add(100 * time.Millisecond)
	_, _ = prebuff.tick(state, state.At) // Select BC.
	state.Generation++
	state.At = state.At.Add(2 * time.Second)
	if _, err := prebuff.tick(state, state.At); err == nil || !strings.Contains(err.Error(), reasonCTASkillUnconfirmed) {
		t.Fatalf("wrong CTA selection error=%v", err)
	}
	if len(in.clicks) != 0 {
		t.Fatalf("wrong CTA selection cast clicks=%v", in.clicks)
	}
}

func TestHammerdinPrebuffRejectsPartialOrMismatchedCTAContract(t *testing.T) {
	bindings := hammerdinPrebuffBindings(false)
	bindings.skills[memory.MustSkillID("battle_command")] = input.SkillCast{SkillID: 155, SelectKey: "f6", CastButton: input.MouseRight}
	in := &hammerdinPrebuffInputMock{}
	if _, err := newHammerdinPrebuff(bindings, in, true); err == nil || !strings.Contains(err.Error(), reasonCTABindingsIncomplete) {
		t.Fatalf("partial CTA error=%v", err)
	}
	if _, err := newInferredHammerdinPrebuff(bindings, in); err == nil || !strings.Contains(err.Error(), hammerdinCTAPairRequiredMessage) {
		t.Fatalf("inferred partial CTA error=%v", err)
	}
	if _, err := newHammerdinPrebuff(hammerdinPrebuffBindings(true), in, false); err == nil {
		t.Fatal("no-CTA test accepted configured CTA")
	}
}

func TestHammerdinCTAPrebuffWaitsForHolyShieldSettleBeforePrimarySwap(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newHammerdinPrebuff(hammerdinPrebuffBindings(true), in, true)
	if err != nil {
		t.Fatal(err)
	}
	state := hammerdinTownState(time.Unix(900, 0), 1, world.WeaponSetPrimary, 0, 0)
	tick := func() hammerdinPrebuffResult {
		t.Helper()
		state.Generation++
		state.At = state.At.Add(400 * time.Millisecond)
		result, tickErr := prebuff.tick(state, state.At)
		if tickErr != nil {
			t.Fatalf("tick state=%d: %v", prebuff.state, tickErr)
		}
		return result
	}
	tick()
	if result := tick(); result.Action != "weapon_set_secondary" {
		t.Fatalf("secondary action=%q", result.Action)
	}
	state.Player.ActiveWeaponSet.Set = world.WeaponSetSecondary
	tick()
	for _, skillID := range []uint16{
		memory.MustSkillID("battle_command"), memory.MustSkillID("battle_orders"),
		memory.MustSkillID("battle_command"), memory.MustSkillID("holy_shield"),
	} {
		tick()
		state.Player.RightSkillID = skillID
		tick()
	}
	if got, want := strings.Join(in.keys, ","), "w"; got != want {
		t.Fatalf("keys after holy shield=%q", got)
	}
	if result := tick(); result.Action != "" {
		t.Fatalf("restore W during holy shield animation: %+v", result)
	}
	if len(in.keys) != 1 {
		t.Fatalf("restore W sent during settle: %v", in.keys)
	}
	state.At = state.At.Add(prebuffWeaponSwapSettle)
	if result := tick(); result.Action != "weapon_set_primary" {
		t.Fatalf("primary action after settle=%q", result.Action)
	}
	if got, want := strings.Join(in.keys, ","), "w,w"; got != want {
		t.Fatalf("keys after settle=%q", got)
	}
}

func TestHammerdinPrebuffRejectsTownCasts(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newInferredHammerdinPrebuff(hammerdinPrebuffBindings(true), in)
	if err != nil {
		t.Fatal(err)
	}
	state := hammerdinTownState(time.Unix(800, 0), 1, world.WeaponSetPrimary, 0, 0)
	state.Area = world.LookupArea(world.RogueEncampment)
	if _, tickErr := prebuff.tick(state, state.At); tickErr == nil || !strings.Contains(tickErr.Error(), reasonPrebuffRequiresField) {
		t.Fatalf("town cast error=%v", tickErr)
	}
	if len(in.keys) != 0 || len(in.clicks) != 0 {
		t.Fatalf("town sent input keys=%v clicks=%v", in.keys, in.clicks)
	}
}

func TestHammerdinCTAPrebuffSkipsWhileAnchorNotDueAndRecastsAfter150s(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newInferredHammerdinPrebuff(hammerdinPrebuffBindings(true), in)
	if err != nil {
		t.Fatal(err)
	}
	state := hammerdinTownState(time.Unix(500, 0), 1, world.WeaponSetPrimary, 0, 0)
	driveHammerdinCTAToComplete(t, prebuff, in, &state)
	if got, want := strings.Join(in.keys, ","), "w,w"; got != want {
		t.Fatalf("initial swap keys=%q", got)
	}

	state.Generation++
	state.At = state.At.Add(time.Second)
	result, tickErr := prebuff.tick(state, state.At)
	if tickErr != nil || !result.Done {
		t.Fatalf("not-due result=%+v err=%v", result, tickErr)
	}
	if got, want := strings.Join(in.keys, ","), "w,w"; got != want {
		t.Fatalf("not-due sent extra input: %v", in.keys)
	}

	state.Generation++
	state.At = state.At.Add(ctaRecastAfter)
	state.Player.ActiveWeaponSet.Set = world.WeaponSetPrimary
	state.Player.LeftSkillID = 0
	state.Player.RightSkillID = 0
	result, tickErr = prebuff.tick(state, state.At)
	if tickErr != nil || result.Done {
		t.Fatalf("due restart result=%+v err=%v", result, tickErr)
	}
	state.Generation++
	state.At = state.At.Add(400 * time.Millisecond)
	result, tickErr = prebuff.tick(state, state.At)
	if tickErr != nil || result.Action != "weapon_set_secondary" {
		t.Fatalf("due recast action=%+v err=%v", result, tickErr)
	}
	if got, want := strings.Join(in.keys, ","), "w,w,w"; got != want {
		t.Fatalf("due recast keys=%q", got)
	}
}

func TestHammerdinPrebuffResetsAnchorOnMenuPhase(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newInferredHammerdinPrebuff(hammerdinPrebuffBindings(true), in)
	if err != nil {
		t.Fatal(err)
	}
	state := hammerdinTownState(time.Unix(600, 0), 1, world.WeaponSetPrimary, 0, 0)
	driveHammerdinCTAToComplete(t, prebuff, in, &state)

	state.Phase = world.GamePhaseMenu
	state.Valid = false
	if _, tickErr := prebuff.tick(state, state.At.Add(time.Second)); tickErr != nil {
		t.Fatal(tickErr)
	}
	if !prebuff.ctaAnchor.due(state.At) {
		t.Fatal("menu phase left the CTA anchor armed")
	}

	state = hammerdinTownState(state.At.Add(time.Second), state.Generation+1, world.WeaponSetPrimary, 0, 0)
	if _, tickErr := prebuff.tick(state, state.At); tickErr != nil {
		t.Fatal(tickErr)
	}
	state.Generation++
	state.At = state.At.Add(400 * time.Millisecond)
	result, tickErr := prebuff.tick(state, state.At)
	if tickErr != nil || result.Action != "weapon_set_secondary" {
		t.Fatalf("post-menu recast=%+v err=%v", result, tickErr)
	}
}

func TestHammerdinTownReadyHookCompletesWithoutInputWhenCTANotDue(t *testing.T) {
	in := &hammerdinPrebuffInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	prebuff, err := newInferredHammerdinPrebuff(hammerdinPrebuffBindings(true), in)
	if err != nil {
		t.Fatal(err)
	}
	wrapper := &hammerdinTownReadyProfile{Executor: &profile.Executor{}, prebuff: prebuff}
	state := hammerdinTownState(time.Unix(700, 0), 1, world.WeaponSetPrimary, 0, 0)
	state.Identity = world.GameIdentity{Valid: true, Class: world.CharacterClassPaladin}
	driveHammerdinCTAToComplete(t, prebuff, in, &state)

	state.Generation++
	state.At = state.At.Add(time.Second)
	got := wrapper.TickHook(context.Background(), profile.HookFieldReady, state, profile.EncounterTarget{}, state.At)
	if got.Status != profile.StatusComplete {
		t.Fatalf("not-due hook=%+v", got)
	}
	if got, want := strings.Join(in.keys, ","), "w,w"; got != want {
		t.Fatalf("not-due hook sent extra input: %v", in.keys)
	}

	menu := state
	menu.Phase = world.GamePhaseMenu
	menu.Valid = false
	if got := wrapper.TickHook(context.Background(), profile.HookFieldReady, menu, profile.EncounterTarget{}, menu.At); got.Status != profile.StatusPending {
		t.Fatalf("menu hook=%+v", got)
	}
	wrapper.Reset()
	if !prebuff.ctaAnchor.due(state.At) {
		t.Fatal("generation reset left the CTA anchor armed")
	}
}

func driveHammerdinCTAToComplete(t *testing.T, prebuff *hammerdinPrebuff, in *hammerdinPrebuffInputMock, state *world.State) {
	t.Helper()
	tick := func() hammerdinPrebuffResult {
		t.Helper()
		state.Generation++
		state.At = state.At.Add(400 * time.Millisecond)
		result, tickErr := prebuff.tick(*state, state.At)
		if tickErr != nil {
			t.Fatalf("tick state=%d: %v", prebuff.state, tickErr)
		}
		return result
	}
	tick()
	if result := tick(); result.Action != "weapon_set_secondary" {
		t.Fatalf("secondary action=%q", result.Action)
	}
	state.Player.ActiveWeaponSet.Set = world.WeaponSetSecondary
	tick()
	for _, skillID := range []uint16{
		memory.MustSkillID("battle_command"), memory.MustSkillID("battle_orders"),
		memory.MustSkillID("battle_command"), memory.MustSkillID("holy_shield"),
	} {
		tick()
		state.Player.RightSkillID = skillID
		tick()
	}
	state.At = state.At.Add(prebuffWeaponSwapSettle)
	if result := tick(); result.Action != "weapon_set_primary" {
		t.Fatalf("primary action=%q", result.Action)
	}
	state.Player.ActiveWeaponSet.Set = world.WeaponSetPrimary
	tick()
	tick()
	state.Player.LeftSkillID = memory.MustSkillID("blessed_hammer")
	tick()
	tick()
	state.Player.RightSkillID = memory.MustSkillID("concentration")
	if result := tick(); !result.Done {
		t.Fatalf("final result=%+v state=%d", result, prebuff.state)
	}
}

func hammerdinPrebuffBindings(cta bool) configBindingSource {
	skills := map[uint16]input.SkillCast{}
	for _, entry := range []struct {
		name   string
		key    string
		button input.MouseButton
	}{
		{"blessed_hammer", "f1", input.MouseLeft},
		{"concentration", "f2", input.MouseRight},
		{"holy_shield", "f4", input.MouseRight},
	} {
		id := memory.MustSkillID(entry.name)
		skills[id] = input.SkillCast{SkillID: id, SelectKey: entry.key, CastButton: entry.button}
	}
	if cta {
		for _, entry := range []struct{ name, key string }{{"battle_command", "f6"}, {"battle_orders", "f7"}} {
			id := memory.MustSkillID(entry.name)
			skills[id] = input.SkillCast{SkillID: id, SelectKey: entry.key, CastButton: input.MouseRight}
		}
	}
	return configBindingSource{skills: skills}
}

func hammerdinTownState(at time.Time, generation uint64, set world.WeaponSet, left, right uint16) world.State {
	return world.State{
		At: at, Generation: generation, Valid: true, Phase: world.GamePhaseInGame,
		Area: world.LookupArea(world.BloodMoor),
		Player: world.Player{
			ActiveWeaponSet: world.WeaponSetState{Set: set, Available: true},
			LeftSkillID:     left, RightSkillID: right,
		},
	}
}

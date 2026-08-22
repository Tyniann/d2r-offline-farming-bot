package app

import (
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func safeOfflineExitState() world.State {
	return world.State{
		Valid:    true,
		Phase:    world.GamePhaseInGame,
		Area:     world.LookupArea(world.RogueEncampment),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer},
	}
}

func TestOfflineExitMachineRequiresVerifiedSequence(t *testing.T) {
	machine := &offlineExitMachine{}
	now := time.Unix(100, 0)
	state := safeOfflineExitState()
	const maximumTownSettle = 500 * time.Millisecond

	for i := 0; i < offlineExitStableTicks; i++ {
		action, done, err := machine.tick(now.Add(time.Duration(i)*time.Millisecond), state)
		if err != nil || done || action != offlineExitNoAction {
			t.Fatalf("safe tick %d = action %d done %v err %v", i, action, done, err)
		}
	}
	action, done, err := machine.tick(now.Add(maximumTownSettle), state)
	if err != nil || done || action != offlineExitPressEscape {
		t.Fatalf("escape tick = action %d done %v err %v", action, done, err)
	}

	state.UI.QuitMenuOpen = true
	for i := 0; i < offlineExitStableTicks-1; i++ {
		action, done, err = machine.tick(now.Add(offlineExitTownSettle+time.Duration(i+1)*time.Millisecond), state)
		if err != nil || done || action != offlineExitNoAction {
			t.Fatalf("quit tick %d = action %d done %v err %v", i, action, done, err)
		}
	}
	action, done, err = machine.tick(now.Add(offlineExitTownSettle+3*time.Millisecond), state)
	if err != nil || done || action != offlineExitClickSave {
		t.Fatalf("save tick = action %d done %v err %v", action, done, err)
	}

	menu := world.State{Phase: world.GamePhaseMenu, Valid: false}
	for i := 0; i < offlineExitStableTicks-1; i++ {
		action, done, err = machine.tick(now.Add(offlineExitTownSettle+time.Duration(4+i)*time.Millisecond), menu)
		if err != nil || done || action != offlineExitNoAction {
			t.Fatalf("menu tick %d = action %d done %v err %v", i, action, done, err)
		}
	}
	action, done, err = machine.tick(now.Add(offlineExitTownSettle+6*time.Millisecond), menu)
	if err != nil || !done || action != offlineExitNoAction {
		t.Fatalf("completion tick = action %d done %v err %v", action, done, err)
	}
}

func TestOfflineExitMachineRejectsUnsafeInitialStates(t *testing.T) {
	tests := []struct {
		name  string
		state world.State
		want  string
	}{
		{name: "wrong area", state: func() world.State { s := safeOfflineExitState(); s.Area = world.LookupArea(world.BlackMarsh); return s }(), want: "Rogue Encampment"},
		{name: "menu", state: world.State{Phase: world.GamePhaseMenu}, want: "requires in_game"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine := &offlineExitMachine{}
			_, _, err := machine.tick(time.Unix(100, 0), tc.state)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
	t.Run("quit already open advances without settle", func(t *testing.T) {
		machine := &offlineExitMachine{}
		state := safeOfflineExitState()
		state.UI.QuitMenuOpen = true
		action, done, err := machine.tick(time.Unix(100, 0), state)
		if err != nil || done || action != offlineExitNoAction || machine.stage != offlineExitAwaitQuitMenu {
			t.Fatalf("action=%d done=%v stage=%s err=%v", action, done, machine.stage, err)
		}
	})
}

func TestOfflineExitMachineDoesNotExtendSettleForStaleTownUI(t *testing.T) {
	machine := &offlineExitMachine{}
	now := time.Unix(100, 0)
	state := safeOfflineExitState()
	state.UI.WaypointOpen = true
	action, done, err := machine.tick(now, state)
	if err != nil || done || action != offlineExitNoAction {
		t.Fatalf("open waypoint tick = action %d done %v err %v", action, done, err)
	}
	state.UI.WaypointOpen = false
	action, done, err = machine.tick(now.Add(offlineExitTownSettle-time.Millisecond), state)
	if err != nil || done || action != offlineExitNoAction {
		t.Fatalf("premature settle tick = action %d done %v err %v", action, done, err)
	}
	action, done, err = machine.tick(now.Add(offlineExitTownSettle), state)
	if err != nil || done || action != offlineExitPressEscape {
		t.Fatalf("closed waypoint settle = action %d done %v err %v", action, done, err)
	}
}

func TestOfflineExitMachineTimesOutWithoutQuitConfirmation(t *testing.T) {
	machine := &offlineExitMachine{}
	now := time.Unix(100, 0)
	state := safeOfflineExitState()
	for i := 0; i < offlineExitStableTicks; i++ {
		_, _, _ = machine.tick(now.Add(time.Duration(i)*time.Millisecond), state)
	}
	_, _, _ = machine.tick(now.Add(offlineExitTownSettle), state)
	_, _, err := machine.tick(now.Add(offlineExitTownSettle+offlineExitQuitMenuTimeout+time.Second), state)
	if err == nil || !strings.Contains(err.Error(), "quit menu") {
		t.Fatalf("err = %v, want quit-menu timeout", err)
	}
}

func TestOfflineExitMachineRetriesOneMemoryUnconfirmedEscape(t *testing.T) {
	machine := &offlineExitMachine{}
	now := time.Unix(100, 0)
	state := safeOfflineExitState()
	var action offlineExitAction
	for i := 0; i < offlineExitStableTicks; i++ {
		_, _, _ = machine.tick(now.Add(time.Duration(i)*time.Millisecond), state)
	}
	action, _, _ = machine.tick(now.Add(offlineExitTownSettle), state)
	if action != offlineExitPressEscape || machine.quitMenuRequests != 1 {
		t.Fatalf("first request action=%d requests=%d", action, machine.quitMenuRequests)
	}
	action, _, err := machine.tick(now.Add(offlineExitTownSettle+offlineExitQuitMenuRetry-time.Millisecond), state)
	if err != nil || action != offlineExitNoAction {
		t.Fatalf("premature retry action=%d err=%v", action, err)
	}
	action, _, err = machine.tick(now.Add(offlineExitTownSettle+offlineExitQuitMenuRetry+2*time.Millisecond), state)
	if err != nil || action != offlineExitPressEscape || machine.quitMenuRequests != 2 {
		t.Fatalf("retry action=%d requests=%d err=%v", action, machine.quitMenuRequests, err)
	}

	state.UI.QuitMenuOpen = true
	action, _, err = machine.tick(now.Add(offlineExitTownSettle+offlineExitQuitMenuRetry+3*time.Millisecond), state)
	if err != nil || action != offlineExitNoAction || machine.quitMenuRequests != 2 {
		t.Fatalf("confirmed menu action=%d requests=%d err=%v", action, machine.quitMenuRequests, err)
	}
}

func TestOfflineExitMachineRestartsSettleWhenPlayerMoves(t *testing.T) {
	machine := &offlineExitMachine{}
	now := time.Unix(100, 0)
	state := safeOfflineExitState()
	state.Player.Position = world.Position{X: 100, Y: 100}

	_, _, _ = machine.tick(now, state)
	state.Player.Position.X++
	action, done, err := machine.tick(now.Add(offlineExitTownSettle), state)
	if err != nil || done || action != offlineExitPressEscape {
		t.Fatalf("moving settle tick = action %d done %v err %v", action, done, err)
	}
}

func TestValidateOfflineExitWindow(t *testing.T) {
	valid := &offlineSelectionMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	if err := validateOfflineExitWindow(valid); err != nil {
		t.Fatalf("valid window error = %v", err)
	}
	invalid := &offlineSelectionMock{window: input.WindowInfo{ClientWidth: 1920, ClientHeight: 1080}}
	if err := validateOfflineExitWindow(invalid); err == nil {
		t.Fatal("expected geometry error")
	}
}

func TestResolveActiveRunDisabledForOfflineExitTest(t *testing.T) {
	cfg := fullCountessConfig(t)
	cfg.Runs.Active = "countess"
	if got := resolveActiveRun(Options{OfflineExitTest: true}, cfg); got != "" {
		t.Fatalf("active run = %q, want empty", got)
	}
}

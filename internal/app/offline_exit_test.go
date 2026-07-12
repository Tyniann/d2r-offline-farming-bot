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

	for i := 0; i < offlineExitStableTicks-1; i++ {
		action, done, err := machine.tick(now.Add(time.Duration(i)*time.Millisecond), state)
		if err != nil || done || action != offlineExitNoAction {
			t.Fatalf("safe tick %d = action %d done %v err %v", i, action, done, err)
		}
	}
	action, done, err := machine.tick(now.Add(2*time.Millisecond), state)
	if err != nil || done || action != offlineExitPressEscape {
		t.Fatalf("escape tick = action %d done %v err %v", action, done, err)
	}

	state.UI.QuitMenuOpen = true
	for i := 0; i < offlineExitStableTicks-1; i++ {
		action, done, err = machine.tick(now.Add(time.Duration(3+i)*time.Millisecond), state)
		if err != nil || done || action != offlineExitNoAction {
			t.Fatalf("quit tick %d = action %d done %v err %v", i, action, done, err)
		}
	}
	action, done, err = machine.tick(now.Add(5*time.Millisecond), state)
	if err != nil || done || action != offlineExitClickSave {
		t.Fatalf("save tick = action %d done %v err %v", action, done, err)
	}

	menu := world.State{Phase: world.GamePhaseMenu, Valid: false}
	for i := 0; i < offlineExitStableTicks-1; i++ {
		action, done, err = machine.tick(now.Add(time.Duration(6+i)*time.Millisecond), menu)
		if err != nil || done || action != offlineExitNoAction {
			t.Fatalf("menu tick %d = action %d done %v err %v", i, action, done, err)
		}
	}
	action, done, err = machine.tick(now.Add(8*time.Millisecond), menu)
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
		{name: "inventory", state: func() world.State { s := safeOfflineExitState(); s.UI.InventoryOpen = true; return s }(), want: "inventory or stash"},
		{name: "stash", state: func() world.State { s := safeOfflineExitState(); s.UI.StashOpen = true; return s }(), want: "inventory or stash"},
		{name: "quit already open", state: func() world.State { s := safeOfflineExitState(); s.UI.QuitMenuOpen = true; return s }(), want: "initially closed"},
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
}

func TestOfflineExitMachineTimesOutWithoutQuitConfirmation(t *testing.T) {
	machine := &offlineExitMachine{}
	now := time.Unix(100, 0)
	state := safeOfflineExitState()
	for i := 0; i < offlineExitStableTicks; i++ {
		_, _, _ = machine.tick(now.Add(time.Duration(i)*time.Millisecond), state)
	}
	_, _, err := machine.tick(now.Add(offlineExitQuitMenuTimeout+time.Second), state)
	if err == nil || !strings.Contains(err.Error(), "quit menu") {
		t.Fatalf("err = %v, want quit-menu timeout", err)
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
	cfg := fullCountessConfig()
	cfg.Runs.Active = "countess"
	if got := resolveActiveRun(Options{OfflineExitTest: true}, cfg); got != "" {
		t.Fatalf("active run = %q, want empty", got)
	}
}

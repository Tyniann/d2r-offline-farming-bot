package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type recordingChatSender struct {
	command string
	err     error
}

func (s *recordingChatSender) SendChatCommand(command string) error {
	s.command = command
	return s.err
}

func TestEffectivePlayersDefaultsMissingToOne(t *testing.T) {
	if got := EffectivePlayers(0); got != 1 {
		t.Fatalf("missing = %d", got)
	}
	if got := EffectivePlayers(8); got != 8 {
		t.Fatalf("explicit = %d", got)
	}
	if got := LoadoutPlayers(nil); got != 1 {
		t.Fatalf("nil loadout = %d", got)
	}
	if got := LoadoutPlayers(&CharacterLoadoutSnapshot{Players: 5}); got != 5 {
		t.Fatalf("frozen = %d", got)
	}
}

func TestApplyOfflinePlayersCommandSendsAfterCertifiedGame(t *testing.T) {
	sender := &recordingChatSender{}
	state := world.State{Valid: true, Phase: world.GamePhaseInGame}
	if err := applyOfflinePlayersCommand(state, sender, 8); err != nil {
		t.Fatal(err)
	}
	if sender.command != "/players 8" {
		t.Fatalf("command = %q", sender.command)
	}
}

func TestApplyOfflinePlayersCommandSendsDefaultOne(t *testing.T) {
	sender := &recordingChatSender{}
	state := world.State{Valid: true, Phase: world.GamePhaseInGame}
	if err := applyOfflinePlayersCommand(state, sender, 0); err != nil {
		t.Fatal(err)
	}
	if sender.command != "/players 1" {
		t.Fatalf("command = %q", sender.command)
	}
}

func TestApplyOfflinePlayersCommandRejectsUncertifiedOrBlockedGame(t *testing.T) {
	sender := &recordingChatSender{}
	if err := applyOfflinePlayersCommand(world.State{Phase: world.GamePhaseLoading}, sender, 3); err == nil {
		t.Fatal("expected loading rejection")
	}
	if sender.command != "" {
		t.Fatalf("sent %q during loading", sender.command)
	}
	blocked := world.State{Valid: true, Phase: world.GamePhaseInGame, UI: world.UIState{StashOpen: true}}
	if err := applyOfflinePlayersCommand(blocked, sender, 3); err == nil || !strings.Contains(err.Error(), "open UI") {
		t.Fatalf("stash error = %v", err)
	}
	if sender.command != "" {
		t.Fatalf("sent %q with stash open", sender.command)
	}
}

func TestApplyOfflinePlayersCommandPropagatesSendFailure(t *testing.T) {
	sender := &recordingChatSender{err: errors.New("focus lost")}
	state := world.State{Valid: true, Phase: world.GamePhaseInGame}
	if err := applyOfflinePlayersCommand(state, sender, 2); err == nil || !strings.Contains(err.Error(), "focus lost") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateOfflinePlayersRange(t *testing.T) {
	if err := validateOfflinePlayers(1); err != nil {
		t.Fatal(err)
	}
	if err := validateOfflinePlayers(9); err == nil {
		t.Fatal("expected 9 to fail")
	}
	if _, err := offlinePlayersCommand(0); err == nil {
		t.Fatal("expected raw 0 command to fail")
	}
}

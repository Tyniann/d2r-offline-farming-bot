package app

import (
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// DefaultOfflinePlayers is the stored and effective /players value when the
	// field is missing from Schema-3 operator settings.
	DefaultOfflinePlayers = 1
	// MinOfflinePlayers is the lowest accepted offline /players value.
	MinOfflinePlayers = 1
	// MaxOfflinePlayers is the highest accepted offline /players value.
	MaxOfflinePlayers = 8
	// offlinePlayersNewGameSettle is the load-fade wait before `/players`. Memory
	// can already report InGame while D2R still swallows keyboard, same as wait_entry_area.
	offlinePlayersNewGameSettle = 3 * time.Second
)

type chatCommandSender interface {
	SendChatCommand(command string) error
}

// EffectivePlayers maps a missing Schema-3 `players` field (0) to 1.
func EffectivePlayers(value int) int {
	if value == 0 {
		return DefaultOfflinePlayers
	}
	return value
}

// LoadoutPlayers returns the frozen /players value, or 1 when no loadout exists.
func LoadoutPlayers(loadout *CharacterLoadoutSnapshot) int {
	if loadout == nil {
		return DefaultOfflinePlayers
	}
	return EffectivePlayers(loadout.Players)
}

func validateOfflinePlayers(value int) error {
	if value < MinOfflinePlayers || value > MaxOfflinePlayers {
		return fmt.Errorf("players must be between %d and %d", MinOfflinePlayers, MaxOfflinePlayers)
	}
	return nil
}

func offlinePlayersCommand(players int) (string, error) {
	if err := validateOfflinePlayers(players); err != nil {
		return "", err
	}
	return fmt.Sprintf("/players %d", players), nil
}

func offlinePlayersFadeDelay(alreadyActive bool) time.Duration {
	if alreadyActive {
		return 0
	}
	return offlinePlayersNewGameSettle
}

func applyOfflinePlayersCommand(state world.State, sender chatCommandSender, players int) error {
	if sender == nil {
		return fmt.Errorf("offline players chat input is unavailable")
	}
	players = EffectivePlayers(players)
	command, err := offlinePlayersCommand(players)
	if err != nil {
		return err
	}
	if state.Phase != world.GamePhaseInGame || !state.Valid {
		return fmt.Errorf("offline players requires a certified in-game snapshot")
	}
	if state.UI.InventoryOpen || state.UI.NPCInteractOpen || state.UI.NPCShopOpen || state.UI.StashOpen || state.UI.QuitMenuOpen {
		return fmt.Errorf("offline players blocked by open UI")
	}
	if err := sender.SendChatCommand(command); err != nil {
		return fmt.Errorf("offline players send %s: %w", command, err)
	}
	return nil
}

func (rt *Runtime) applyOfflinePlayersCommand() error {
	if rt == nil || rt.Input == nil || rt.World == nil {
		return fmt.Errorf("offline players runtime input is unavailable")
	}
	sender, ok := rt.Input.(chatCommandSender)
	if !ok {
		return fmt.Errorf("offline players chat input is unavailable")
	}
	players := LoadoutPlayers(rt.Options.Loadout)
	if err := applyOfflinePlayersCommand(rt.World.Current(), sender, players); err != nil {
		return err
	}
	rt.Log.Info("offline players command sent", "players", players, "command", fmt.Sprintf("/players %d", players))
	return nil
}

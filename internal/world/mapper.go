package world

import "github.com/Tyniann/d2r-offline-farming-bot/internal/memory"

// FromSnapshot maps a Phase-1 memory snapshot into a semantic world State.
//
// When snap.Valid is true, Area and Player are populated. Phase is set to
// GamePhaseInGame as a best-effort heuristic because player position and
// vitals were readable; this is not a hard in-game guarantee (the probe may
// still succeed when InGameGate is zero). Do not derive GamePhase solely from
// State.Valid — reliable Menu/Loading/InGame detection needs additional
// signals in later phases.
//
// Unknown area IDs (including 0 with a valid snapshot) do not invalidate the
// state; LookupArea supplies synthetic names and AreaKindUnknown.
//
// When snap.Valid is false, Reason is forwarded when present, Phase is
// GamePhaseUnknown, and Area/Player are zero values that must not be read.
// Invalid reasons such as memory.ReasonNotInGame are not interpreted as
// GamePhaseMenu or GamePhaseLoading.
func FromSnapshot(snap memory.Snapshot) State {
	if !snap.Valid {
		return State{
			At:     snap.At,
			Valid:  false,
			Reason: snap.Reason,
			Phase:  GamePhaseUnknown,
		}
	}

	return State{
		At:    snap.At,
		Valid: true,
		Phase: GamePhaseInGame,
		Area:  LookupArea(AreaID(snap.AreaID)),
		Player: Player{
			Position: Position{X: snap.PosX, Y: snap.PosY},
			HP:       snap.HP,
			MaxHP:    snap.MaxHP,
			Mana:     snap.Mana,
			MaxMana:  snap.MaxMana,
		},
	}
}

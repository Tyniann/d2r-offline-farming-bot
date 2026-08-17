package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const sessionGameStableTicks = 3

type sessionGameExpectation struct {
	Character   string
	GameVersion string
	StartArea   world.AreaID
}

type sessionGameVerification struct {
	Generation     uint64
	CharacterName  string
	CharacterClass string
	AreaID         world.AreaID
	ConfirmedAt    time.Time
}

type sessionGameVerifier struct {
	expectation sessionGameExpectation
	generation  uint64
	stableTicks int
	lastAt      time.Time
}

func newSessionGameVerifier(expectation sessionGameExpectation) *sessionGameVerifier {
	return &sessionGameVerifier{expectation: expectation}
}

func (v *sessionGameVerifier) ResetForNextGame() {
	v.generation++
	v.stableTicks = 0
	v.lastAt = time.Time{}
}

func (v *sessionGameVerifier) Observe(state world.State, gameVersion string) (sessionGameVerification, bool, error) {
	if v.generation == 0 {
		return sessionGameVerification{}, false, fmt.Errorf("session game verifier requires cycle reset")
	}
	if state.Phase == world.GamePhaseLoading || state.Phase == world.GamePhaseMenu || state.Phase == world.GamePhaseUnknown ||
		!state.Valid || !state.Identity.Valid || state.Area.ID == 0 {
		v.stableTicks = 0
		return sessionGameVerification{}, false, nil
	}
	if state.Phase != world.GamePhaseInGame {
		v.stableTicks = 0
		return sessionGameVerification{}, false, nil
	}
	if gameVersion == "" || gameVersion != v.expectation.GameVersion {
		return sessionGameVerification{}, false, fmt.Errorf("session game version mismatch: active=%q expected=%q", gameVersion, v.expectation.GameVersion)
	}
	if !strings.EqualFold(state.Identity.CharacterName, v.expectation.Character) {
		return sessionGameVerification{}, false, fmt.Errorf("session character mismatch: active=%q expected=%q", state.Identity.CharacterName, v.expectation.Character)
	}
	if state.Area.ID != v.expectation.StartArea {
		return sessionGameVerification{}, false, fmt.Errorf("session start area mismatch: active=%s expected=%s", state.Area.Name, world.LookupArea(v.expectation.StartArea).Name)
	}
	if state.UI.InventoryOpen || state.UI.NPCInteractOpen || state.UI.NPCShopOpen || state.UI.StashOpen || state.UI.QuitMenuOpen {
		return sessionGameVerification{}, false, fmt.Errorf("session game verification blocked by open UI")
	}
	if !v.lastAt.IsZero() && !state.At.After(v.lastAt) {
		return sessionGameVerification{}, false, fmt.Errorf("session game verification requires fresh snapshots")
	}
	v.lastAt = state.At
	v.stableTicks++
	if v.stableTicks < sessionGameStableTicks {
		return sessionGameVerification{}, false, nil
	}
	return sessionGameVerification{
		Generation: v.generation, CharacterName: state.Identity.CharacterName,
		CharacterClass: state.Identity.Class.String(), AreaID: state.Area.ID, ConfirmedAt: state.At,
	}, true, nil
}

func verifySessionRouteStart(route pathing.Route, state world.State, gameVersion string) (pathing.LayoutFingerprint, error) {
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		return pathing.LayoutFingerprint{}, fmt.Errorf("session route layout: %w", err)
	}
	if err := pathing.ValidateRoutePrecheck(route, pathing.RoutePrecheckInput{
		Identity: state.Identity, GameVersion: gameVersion, Layout: fingerprint, World: state,
	}); err != nil {
		return pathing.LayoutFingerprint{}, fmt.Errorf("session route precheck: %w", err)
	}
	return fingerprint, nil
}

type sessionStateResetter interface {
	Reset()
}

type sessionNamedResetter struct {
	name     string
	resetter sessionStateResetter
}

type sessionResetBarrier struct {
	components []sessionNamedResetter
	resetWorld func(time.Time, string)
}

func (b sessionResetBarrier) ResetForNextGame(at time.Time, reason string) error {
	for _, component := range b.components {
		if component.resetter == nil {
			return fmt.Errorf("session reset component %q is nil", component.name)
		}
		component.resetter.Reset()
	}
	if b.resetWorld == nil {
		return fmt.Errorf("session world reset is unavailable")
	}
	b.resetWorld(at, reason)
	return nil
}

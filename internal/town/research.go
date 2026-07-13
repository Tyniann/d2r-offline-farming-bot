package town

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// EvidenceGrade states whether a field is safe to consume in a later Town action.
type EvidenceGrade string

const (
	EvidenceReliable    EvidenceGrade = "reliable"
	EvidenceOptional    EvidenceGrade = "optional"
	EvidenceShifted     EvidenceGrade = "shifted"
	EvidenceUnavailable EvidenceGrade = "unavailable"
)

// FieldReport classifies one required Town datum without inferring missing values.
type FieldReport struct {
	Grade   EvidenceGrade `json:"grade"`
	Present bool          `json:"present"`
	Detail  string        `json:"detail"`
}

// ResearchReport inventories all Town inputs that Phase 9 must prove before input is authorized.
type ResearchReport struct {
	Act              FieldReport `json:"act"`
	NPCs             FieldReport `json:"npcs"`
	Belt             FieldReport `json:"belt"`
	ScrollCounters   FieldReport `json:"scroll_counters"`
	Identified       FieldReport `json:"identified"`
	VendorCandidates FieldReport `json:"vendor_candidates"`
	Gold             FieldReport `json:"gold"`
	Durability       FieldReport `json:"durability"`
	BulkPurchaseSafe bool        `json:"bulk_purchase_safe"`
	BulkReason       string      `json:"bulk_reason"`
}

// Research inspects one immutable World snapshot without sending input.
//
// The research surface intentionally reports bulk buying as unproven. Productive
// approval belongs to the separate profile, shop, item-pin, and verifier gates;
// it is never inherited from this read-only diagnostic report.
func Research(state world.State) ResearchReport {
	valid := state.Valid && state.Phase == world.GamePhaseInGame
	report := ResearchReport{
		Act:              FieldReport{Grade: EvidenceReliable, Present: valid && state.Area.Act != world.ActUnknown, Detail: "world area act"},
		NPCs:             FieldReport{Grade: EvidenceOptional, Present: valid && len(state.Monsters) > 0, Detail: "monster enumeration; town NPC identity is not yet modeled"},
		Belt:             FieldReport{Grade: EvidenceReliable, Present: valid, Detail: "player-owned belt items with item type and grid position"},
		ScrollCounters:   FieldReport{Grade: EvidenceShifted, Present: false, Detail: "tomes are enumerated, but scroll quantity has no validated stat decoder"},
		Identified:       FieldReport{Grade: EvidenceReliable, Present: valid, Detail: "item identified flag"},
		VendorCandidates: FieldReport{Grade: EvidenceUnavailable, Present: false, Detail: "requires a later decision handoff"},
		Gold:             goldFieldReport(state),
		Durability:       FieldReport{Grade: EvidenceShifted, Present: false, Detail: "raw item stats exist, but durability IDs and scaling are unvalidated"},
		BulkPurchaseSafe: false,
		BulkReason:       "shift_rmb_unverified",
	}
	if !valid {
		report.Belt.Present = false
		report.Identified.Present = false
	}
	return report
}

func goldFieldReport(state world.State) FieldReport {
	if !state.Valid || !state.Player.GoldKnown || !state.Player.PrivateStashGoldKnown {
		return FieldReport{Grade: EvidenceUnavailable, Present: false, Detail: "carried or private-stash gold stat unavailable"}
	}
	return FieldReport{Grade: EvidenceReliable, Present: true, Detail: fmt.Sprintf("player stats: carried=%d private_stash=%d", state.Player.Gold, state.Player.PrivateStashGold)}
}

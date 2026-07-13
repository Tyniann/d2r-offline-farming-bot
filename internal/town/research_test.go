package town

import (
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	"testing"
)

func TestResearchClassifiesEveryRequiredFieldAndDisablesBulk(t *testing.T) {
	report := Research(world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment)})
	if report.Act.Grade != EvidenceReliable || !report.Act.Present {
		t.Fatalf("act = %+v", report.Act)
	}
	if report.Belt.Grade != EvidenceReliable || report.ScrollCounters.Grade != EvidenceShifted || report.Gold.Grade != EvidenceUnavailable {
		t.Fatalf("report = %+v", report)
	}
	if report.BulkPurchaseSafe || report.BulkReason != "shift_rmb_unverified" {
		t.Fatalf("bulk = %v/%q", report.BulkPurchaseSafe, report.BulkReason)
	}
}

func TestResearchDoesNotTreatInvalidWorldAsEvidence(t *testing.T) {
	report := Research(world.State{})
	if report.Act.Present || report.Belt.Present || report.Identified.Present {
		t.Fatalf("invalid report = %+v", report)
	}
}

func TestResearchReportsValidatedGoldValues(t *testing.T) {
	state := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), Player: world.Player{Gold: 50938, PrivateStashGold: 2401390, GoldKnown: true, PrivateStashGoldKnown: true}}
	report := Research(state)
	if report.Gold.Grade != EvidenceReliable || !report.Gold.Present || report.Gold.Detail != "player stats: carried=50938 private_stash=2401390" {
		t.Fatalf("gold = %+v", report.Gold)
	}
}

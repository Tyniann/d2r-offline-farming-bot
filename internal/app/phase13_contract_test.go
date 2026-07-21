package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

func TestPhase13ProfileAndAssignmentSchemas(t *testing.T) {
	profile := PickitProfileDocument{
		SchemaVersion: 1,
		Revision:      1,
		ID:            "tal-rasha",
		Name:          "Tal Rasha Set",
		Rules: []PickitProfileRuleDocument{{
			ID: "tal-rasha-adjudication", Action: loot.ActionKeep,
			Expression: `[setitem] == "Tal Rasha's Adjudication"`,
		}},
	}
	if err := profile.Validate(); err != nil {
		t.Fatalf("valid profile: %v", err)
	}
	assignment := PickitAssignmentManifest{
		SchemaVersion: 1,
		Revision:      12,
		Assignments: map[string]map[tasks.RunID][]string{
			"MrBones": {tasks.RunIDCountess: {"gems", "keys", "countess-standard"}, tasks.RunIDMephisto: {"tal-rasha", "gems", "mephisto-standard"}},
		},
	}
	if err := assignment.Validate(); err != nil {
		t.Fatalf("valid assignment: %v", err)
	}
}

func TestPhase13SchemasRejectAmbiguity(t *testing.T) {
	base := PickitProfileDocument{SchemaVersion: 1, Revision: 1, ID: "profile", Name: "Profil", Rules: []PickitProfileRuleDocument{{ID: "rule", Action: loot.ActionKeep, Expression: "[type] == rune"}}}
	tests := []struct {
		name   string
		mutate func(*PickitProfileDocument)
	}{
		{name: "zero revision", mutate: func(p *PickitProfileDocument) { p.Revision = 0 }},
		{name: "mutable looking id", mutate: func(p *PickitProfileDocument) { p.ID = "Profile Name" }},
		{name: "no rules", mutate: func(p *PickitProfileDocument) { p.Rules = nil }},
		{name: "duplicate rule", mutate: func(p *PickitProfileDocument) { p.Rules = append(p.Rules, p.Rules[0]) }},
		{name: "unknown action", mutate: func(p *PickitProfileDocument) { p.Rules[0].Action = "drop" }},
		{name: "empty expression", mutate: func(p *PickitProfileDocument) { p.Rules[0].Expression = " " }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := base
			profile.Rules = append([]PickitProfileRuleDocument(nil), base.Rules...)
			test.mutate(&profile)
			if err := profile.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	assignments := PickitAssignmentManifest{SchemaVersion: 1, Revision: 1, Assignments: map[string]map[tasks.RunID][]string{"MrBones": {tasks.RunIDCountess: {"gems", "gems"}}}}
	if err := assignments.Validate(); err == nil {
		t.Fatal("duplicate profile assignment was accepted")
	}
}

func TestPhase13ContractTablesAreCompleteAndOrdered(t *testing.T) {
	owners := PickitContractOwners()
	if len(owners) != 6 || owners[0].Owner != "internal/memory" || owners[len(owners)-1].Owner != "web/src/features/pickit" {
		t.Fatalf("owners = %+v", owners)
	}
	migration := PickitMigrationMatrix()
	if len(migration) != 3 || migration[0].LegacyAuthority != "countess pickup_file" || migration[2].LegacyAuthority != "mephisto sell_file" {
		t.Fatalf("migration = %+v", migration)
	}
}

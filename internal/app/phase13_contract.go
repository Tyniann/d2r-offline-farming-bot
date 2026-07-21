package app

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

const (
	// PickitProfileSchemaVersion ist die erste persistente Profilversion.
	PickitProfileSchemaVersion = 1
	// PickitAssignmentSchemaVersion ist die erste persistente Pickit-Zuordnungsversion.
	PickitAssignmentSchemaVersion = 1
)

var pickitSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// PickitProfileDocument ist die einzige persistente Autorität für ein globales Pickit-Profil.
type PickitProfileDocument struct {
	SchemaVersion int                         `yaml:"schema_version"`
	Revision      uint64                      `yaml:"revision"`
	ID            string                      `yaml:"id"`
	Name          string                      `yaml:"name"`
	Rules         []PickitProfileRuleDocument `yaml:"rules"`
}

// PickitProfileRuleDocument hält eine geordnete Regel mit genau einer Aktion.
type PickitProfileRuleDocument struct {
	ID         string      `yaml:"id"`
	Action     loot.Action `yaml:"action"`
	Expression string      `yaml:"expression"`
}

// Validate prüft das persistente Profilschema vor Parser- und Katalogauflösung.
func (d PickitProfileDocument) Validate() error {
	if d.SchemaVersion != PickitProfileSchemaVersion {
		return fmt.Errorf("pickit profile schema_version must be %d", PickitProfileSchemaVersion)
	}
	if d.Revision == 0 {
		return fmt.Errorf("pickit profile revision must be positive")
	}
	if !pickitSlugPattern.MatchString(d.ID) {
		return fmt.Errorf("pickit profile id %q must be a normalized slug", d.ID)
	}
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("pickit profile name is required")
	}
	if len(d.Rules) == 0 {
		return fmt.Errorf("pickit profile requires at least one rule")
	}
	seen := make(map[string]struct{}, len(d.Rules))
	for index, rule := range d.Rules {
		if !pickitSlugPattern.MatchString(rule.ID) {
			return fmt.Errorf("pickit profile rule %d id %q must be a normalized slug", index, rule.ID)
		}
		if _, duplicate := seen[rule.ID]; duplicate {
			return fmt.Errorf("pickit profile rule id %q is duplicated", rule.ID)
		}
		seen[rule.ID] = struct{}{}
		if !rule.Action.Valid() {
			return fmt.Errorf("pickit profile rule %q action %q is unsupported", rule.ID, rule.Action)
		}
		if strings.TrimSpace(rule.Expression) == "" {
			return fmt.Errorf("pickit profile rule %q expression is required", rule.ID)
		}
	}
	return nil
}

// PickitAssignmentManifest ist die einzige persistente Zuordnungsautorität pro Charakter und Run.
type PickitAssignmentManifest struct {
	SchemaVersion int                                 `yaml:"schema_version"`
	Revision      uint64                              `yaml:"revision"`
	Assignments   map[string]map[tasks.RunID][]string `yaml:"assignments"`
}

// Validate verwirft leere, doppelte oder unbekannte Zuordnungen fail-closed.
func (m PickitAssignmentManifest) Validate() error {
	if m.SchemaVersion != PickitAssignmentSchemaVersion {
		return fmt.Errorf("pickit assignment schema_version must be %d", PickitAssignmentSchemaVersion)
	}
	if m.Revision == 0 || m.Assignments == nil {
		return fmt.Errorf("pickit assignment positive revision and assignments are required")
	}
	characters := make(map[string]struct{}, len(m.Assignments))
	for character, runs := range m.Assignments {
		trimmed := strings.TrimSpace(character)
		folded := strings.ToLower(trimmed)
		if trimmed == "" || runs == nil {
			return fmt.Errorf("pickit assignment character %q is invalid", character)
		}
		if _, duplicate := characters[folded]; duplicate {
			return fmt.Errorf("pickit assignment character %q is duplicated case-insensitively", character)
		}
		characters[folded] = struct{}{}
		for runID, profileIDs := range runs {
			if _, ok := tasks.DefaultRunRegistry().Definition(runID); !ok {
				return fmt.Errorf("pickit assignment %s/%s uses an unknown run", character, runID)
			}
			if len(profileIDs) == 0 {
				return fmt.Errorf("pickit assignment %s/%s requires at least one profile", character, runID)
			}
			profiles := make(map[string]struct{}, len(profileIDs))
			for _, profileID := range profileIDs {
				if !pickitSlugPattern.MatchString(profileID) {
					return fmt.Errorf("pickit assignment %s/%s profile id %q must be a normalized slug", character, runID, profileID)
				}
				if _, duplicate := profiles[profileID]; duplicate {
					return fmt.Errorf("pickit assignment %s/%s profile %q is duplicated", character, runID, profileID)
				}
				profiles[profileID] = struct{}{}
			}
		}
	}
	return nil
}

// PickitContractOwner benennt den einzigen Owner eines Phase-13-Vertrags.
type PickitContractOwner struct {
	Contract string
	Owner    string
}

// PickitContractOwners liefert die verbindlichen Paketgrenzen für Phase 13.
func PickitContractOwners() []PickitContractOwner {
	return []PickitContractOwner{
		{Contract: "raw_item_identity", Owner: "internal/memory"},
		{Contract: "item_identity_resolution", Owner: "internal/world"},
		{Contract: "parser_action_and_trace", Owner: "internal/loot"},
		{Contract: "profile_assignment_and_run_snapshot", Owner: "internal/app"},
		{Contract: "http_json_and_sse", Owner: "internal/api"},
		{Contract: "pickit_user_interface", Owner: "web/src/features/pickit"},
	}
}

// PickitMigrationContract beschreibt genau eine alte Autorität und ihr Phase-13-Ziel.
type PickitMigrationContract struct {
	LegacyAuthority string
	TargetAuthority string
	RemovalGate     string
}

// PickitMigrationMatrix liefert die einmalige, fallback-freie Policy-Migration.
func PickitMigrationMatrix() []PickitMigrationContract {
	return []PickitMigrationContract{
		{LegacyAuthority: "countess pickup_file", TargetAuthority: "countess-standard profile with keep actions", RemovalGate: "countess match matrix unchanged"},
		{LegacyAuthority: "mephisto pickup_file", TargetAuthority: "mephisto-standard profile with keep and sell actions", RemovalGate: "mephisto pickup/keep/sell matrix unchanged"},
		{LegacyAuthority: "mephisto sell_file", TargetAuthority: "sell actions in mephisto-standard profile", RemovalGate: "identified and unidentified service candidates unchanged"},
	}
}

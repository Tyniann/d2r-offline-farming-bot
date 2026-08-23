package api

import (
	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
)

// PickitCatalogDTO ist der patchgenaue Katalogvertrag des Regel-Editors.
type PickitCatalogDTO struct {
	SchemaVersion   int                 `json:"schema_version"`
	CatalogVersion  string              `json:"catalog_version"`
	Bases           []PickitBaseDTO     `json:"bases"`
	Identities      []PickitIdentityDTO `json:"identities"`
	Actions         []string            `json:"actions"`
	Qualities       []string            `json:"qualities"`
	SpeedCategories []string            `json:"speed_categories"`
}

// PickitBaseDTO beschreibt eine auswählbare Item-Basis.
type PickitBaseDTO struct {
	TxtFileNo uint32 `json:"txt_file_no"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	BaseTier  string `json:"base_tier"`
}

// PickitIdentityDTO beschreibt eine Set- oder Unique-Identität.
type PickitIdentityDTO struct {
	Kind        string `json:"kind"`
	RawID       uint32 `json:"raw_id"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	BaseCode    string `json:"base_code"`
	SetKey      string `json:"set_key"`
	SetName     string `json:"set_name"`
	Spawnable   bool   `json:"spawnable"`
}

// PickitRuleDTO ist eine geordnete Editor-Regel.
type PickitRuleDTO struct {
	ID         string                `json:"id"`
	Action     string                `json:"action"`
	Expression string                `json:"expression"`
	Summary    *PickitRuleSummaryDTO `json:"summary,omitempty"`
}

// PickitRuleSummaryDTO projiziert einen sprachneutralen Darstellungshinweis ohne Parser-Interna.
type PickitRuleSummaryDTO struct {
	Kind   string                     `json:"kind"`
	Params PickitRuleSummaryParamsDTO `json:"params"`
}

// PickitRuleSummaryParamsDTO projiziert typisierte, sprachneutrale Katalog- und Filterwerte.
type PickitRuleSummaryParamsDTO struct {
	Codes          []string `json:"codes,omitempty"`
	Types          []string `json:"types,omitempty"`
	Qualities      []string `json:"qualities,omitempty"`
	Tiers          []string `json:"tiers,omitempty"`
	SetKey         string   `json:"set_key,omitempty"`
	UniqueKey      string   `json:"unique_key,omitempty"`
	SocketOperator string   `json:"socket_operator,omitempty"`
	SocketCount    *int     `json:"socket_count,omitempty"`
	Ethereal       *bool    `json:"ethereal,omitempty"`
}

// PickitProfileDTO ist die JSON-Projektion eines persistierten Profils.
type PickitProfileDTO struct {
	SchemaVersion int             `json:"schema_version"`
	Revision      uint64          `json:"revision"`
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Rules         []PickitRuleDTO `json:"rules"`
}

// PickitProfilesDTO bündelt Profile und die aktuelle Assignment-Revision.
type PickitProfilesDTO struct {
	Profiles           []PickitProfileDTO `json:"profiles"`
	AssignmentRevision uint64             `json:"assignment_revision"`
}

// PickitValidationRequest validiert einen nicht persistierten Profilentwurf.
type PickitValidationRequest struct {
	Profile PickitProfileDTO `json:"profile"`
}

// PickitValidationDTO meldet die kanonische, kompilierbare Form.
type PickitValidationDTO struct {
	Valid   bool             `json:"valid"`
	Profile PickitProfileDTO `json:"profile"`
}

// PickitPreviewItemDTO ist ein kontrolliertes Test-Item ohne Zugriff auf Live-Memory.
type PickitPreviewItemDTO struct {
	Code              string `json:"code"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Quality           string `json:"quality"`
	BaseTier          string `json:"base_tier"`
	IdentityKind      string `json:"identity_kind"`
	IdentityKey       string `json:"identity_key"`
	IdentityAvailable bool   `json:"identity_available"`
	IdentityValid     bool   `json:"identity_valid"`
	Identified        bool   `json:"identified"`
	Ethereal          bool   `json:"ethereal"`
	Sockets           int    `json:"sockets"`
	SocketsAvailable  bool   `json:"sockets_available"`
	Socketed          bool   `json:"socketed"`
}

// PickitPreviewRequest wertet einen Entwurf gegen ein kontrolliertes Test-Item aus.
type PickitPreviewRequest struct {
	Profile PickitProfileDTO     `json:"profile"`
	Item    PickitPreviewItemDTO `json:"item"`
}

// PickitTraceDTO beschreibt eine tatsächlich ausgewertete Regel.
type PickitTraceDTO struct {
	RuleIndex          int    `json:"rule_index"`
	ProfileID          string `json:"profile_id"`
	RuleID             string `json:"rule_id"`
	Action             string `json:"action"`
	Expression         string `json:"expression"`
	Matched            bool   `json:"matched"`
	ProfileRevision    uint64 `json:"profile_revision"`
	AssignmentRevision uint64 `json:"assignment_revision"`
}

// PickitPreviewDTO enthält First-Match-Ergebnis und vollständigen Trace bis zum Treffer.
type PickitPreviewDTO struct {
	Matched            bool             `json:"matched"`
	RuleIndex          int              `json:"rule_index"`
	ProfileID          string           `json:"profile_id"`
	RuleID             string           `json:"rule_id"`
	Action             string           `json:"action"`
	ProfileRevision    uint64           `json:"profile_revision"`
	AssignmentRevision uint64           `json:"assignment_revision"`
	Trace              []PickitTraceDTO `json:"trace"`
}

// PickitCreateRequest legt ein Profil an.
type PickitCreateRequest struct {
	Profile PickitProfileDTO `json:"profile"`
}

// PickitUpdateRequest ersetzt ein Profil revisionsgebunden.
type PickitUpdateRequest struct {
	ExpectedRevision uint64           `json:"expected_revision"`
	Profile          PickitProfileDTO `json:"profile"`
}

// PickitDuplicateRequest dupliziert ein Profil unter einer neuen ID.
type PickitDuplicateRequest struct {
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
}

// PickitDeleteRequest löscht ein Profil und optional alle bestätigten Zuordnungen revisionsgebunden.
type PickitDeleteRequest struct {
	ExpectedRevision           uint64 `json:"expected_revision"`
	ExpectedAssignmentRevision uint64 `json:"expected_assignment_revision"`
	RemoveAssignments          bool   `json:"remove_assignments"`
}

// PickitAssignmentUsageDTO bezeichnet eine entfernte Profilverwendung.
type PickitAssignmentUsageDTO struct {
	Character string `json:"character"`
	RouteID   string `json:"route_id"`
}

// PickitDeleteResultDTO liefert nach dem Löschen den autoritativen Assignment-Stand.
type PickitDeleteResultDTO struct {
	Assignments PickitAssignmentsDTO       `json:"assignments"`
	Removed     []PickitAssignmentUsageDTO `json:"removed"`
}

// PickitAssignmentsDTO ist die JSON-Projektion der einzigen Assignment-Autorität.
type PickitAssignmentsDTO struct {
	SchemaVersion int                            `json:"schema_version"`
	Revision      uint64                         `json:"revision"`
	Assignments   map[string]map[string][]string `json:"assignments"`
}

// PickitAssignmentUpdateRequest ersetzt genau eine geordnete Zuordnung.
type PickitAssignmentUpdateRequest struct {
	Character        string   `json:"character"`
	RunID            string   `json:"run_id"`
	ProfileIDs       []string `json:"profile_ids"`
	ExpectedRevision uint64   `json:"expected_revision"`
}

// PickitImportRequest importiert alten NIP-Text ausschließlich als Entwurf.
type PickitImportRequest struct {
	Text   string `json:"text"`
	Action string `json:"action"`
}

// PickitImportDTO liefert kanonische Entwurfsregeln ohne Persistenz.
type PickitImportDTO struct {
	Rules    []PickitRuleDTO `json:"rules"`
	Warnings []string        `json:"warnings"`
}

// PickitExportDTO liefert einen portablen NIP-Text und den unvermeidbaren Aktionshinweis.
type PickitExportDTO struct {
	ProfileID string `json:"profile_id"`
	Text      string `json:"text"`
	Revision  uint64 `json:"revision"`
	Warning   string `json:"warning"`
}

type pickitBackend interface {
	PickitCatalog() PickitCatalogDTO
	PickitProfiles() (PickitProfilesDTO, error)
	ValidatePickit(PickitValidationRequest) (PickitValidationDTO, error)
	PreviewPickit(PickitPreviewRequest) (PickitPreviewDTO, error)
	CreatePickit(PickitCreateRequest) (PickitProfileDTO, error)
	UpdatePickit(string, PickitUpdateRequest) (PickitProfileDTO, error)
	DuplicatePickit(string, PickitDuplicateRequest) (PickitProfileDTO, error)
	DeletePickit(string, PickitDeleteRequest) (PickitDeleteResultDTO, error)
	PickitAssignments() (PickitAssignmentsDTO, error)
	UpdatePickitAssignment(PickitAssignmentUpdateRequest) (PickitAssignmentsDTO, error)
	ImportPickit(PickitImportRequest) (PickitImportDTO, error)
	ExportPickit(string) (PickitExportDTO, error)
}

func profileDTO(document app.PickitProfileDocument) PickitProfileDTO {
	rules := make([]PickitRuleDTO, len(document.Rules))
	for i, rule := range document.Rules {
		summary, err := loot.SummarizePickitExpression(rule.Expression)
		if err != nil {
			summary = loot.PickitRuleSummary{Kind: "custom"}
		}
		rules[i] = PickitRuleDTO{ID: rule.ID, Action: string(rule.Action), Expression: rule.Expression, Summary: pickitRuleSummaryDTO(summary)}
	}
	return PickitProfileDTO{SchemaVersion: document.SchemaVersion, Revision: document.Revision, ID: document.ID, Name: document.Name, Rules: rules}
}

func pickitRuleSummaryDTO(summary loot.PickitRuleSummary) *PickitRuleSummaryDTO {
	params := summary.Params
	return &PickitRuleSummaryDTO{Kind: summary.Kind, Params: PickitRuleSummaryParamsDTO{
		Codes: append([]string(nil), params.Codes...), Types: append([]string(nil), params.Types...),
		Qualities: append([]string(nil), params.Qualities...), Tiers: append([]string(nil), params.Tiers...),
		SetKey: params.SetKey, UniqueKey: params.UniqueKey, SocketOperator: params.SocketOperator,
		SocketCount: params.SocketCount, Ethereal: params.Ethereal,
	}}
}

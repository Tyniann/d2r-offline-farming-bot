package api

import (
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func (b *LiveBackend) PickitCatalog() PickitCatalogDTO {
	bases := world.ItemCatalogEntries()
	baseDTOs := make([]PickitBaseDTO, len(bases))
	for i, entry := range bases {
		baseDTOs[i] = PickitBaseDTO{TxtFileNo: entry.TxtFileNo, Code: entry.Code, Name: entry.Name, Type: entry.Type, BaseTier: string(entry.BaseTier)}
	}
	identities := world.ItemIdentityCatalogEntries()
	identityDTOs := make([]PickitIdentityDTO, len(identities))
	for i, entry := range identities {
		identityDTOs[i] = PickitIdentityDTO{Kind: string(entry.Kind), RawID: entry.RawID, Key: entry.Key, DisplayName: entry.DisplayName, BaseCode: entry.BaseCode, SetKey: entry.SetKey, SetName: entry.SetName, Spawnable: entry.Spawnable}
	}
	return PickitCatalogDTO{SchemaVersion: app.PickitProfileSchemaVersion, CatalogVersion: world.ItemIdentityCatalogVersion(), Bases: baseDTOs, Identities: identityDTOs, Actions: []string{"keep", "sell"}, Qualities: []string{"low_quality", "normal", "superior", "magic", "set", "rare", "unique", "crafted"}, SpeedCategories: []string{"slow", "normal", "fast"}}
}

func (b *LiveBackend) PickitProfiles() (PickitProfilesDTO, error) {
	profiles, err := b.pickitProfiles.List()
	if err != nil {
		return PickitProfilesDTO{}, err
	}
	manifest, err := b.pickitAssignments.Snapshot()
	if err != nil {
		return PickitProfilesDTO{}, err
	}
	dtos := make([]PickitProfileDTO, len(profiles))
	for i, profile := range profiles {
		dtos[i] = profileDTO(profile)
	}
	return PickitProfilesDTO{Profiles: dtos, AssignmentRevision: manifest.Revision}, nil
}

func pickitDocument(dto PickitProfileDTO) app.PickitProfileDocument {
	rules := make([]app.PickitProfileRuleDocument, len(dto.Rules))
	for i, rule := range dto.Rules {
		rules[i] = app.PickitProfileRuleDocument{ID: rule.ID, Action: loot.Action(rule.Action), Expression: rule.Expression}
	}
	return app.PickitProfileDocument{SchemaVersion: dto.SchemaVersion, Revision: dto.Revision, ID: dto.ID, Name: dto.Name, Rules: rules}
}

func (b *LiveBackend) ValidatePickit(request PickitValidationRequest) (PickitValidationDTO, error) {
	document := pickitDocument(request.Profile)
	for i := range document.Rules {
		canonical, err := loot.CanonicalPickitExpression(document.Rules[i].Expression)
		if err != nil {
			return PickitValidationDTO{}, err
		}
		document.Rules[i].Expression = canonical
	}
	specs := make([]loot.PickitRuleSpec, len(document.Rules))
	for i, rule := range document.Rules {
		specs[i] = loot.PickitRuleSpec{ProfileID: document.ID, RuleID: rule.ID, Action: rule.Action, Expression: rule.Expression, ProfileRevision: document.Revision}
	}
	if err := document.Validate(); err != nil {
		return PickitValidationDTO{}, err
	}
	if _, err := loot.CompilePickitRules("pickit API validation", specs); err != nil {
		return PickitValidationDTO{}, err
	}
	return PickitValidationDTO{Valid: true, Profile: profileDTO(document)}, nil
}

func (b *LiveBackend) PreviewPickit(request PickitPreviewRequest) (PickitPreviewDTO, error) {
	validated, err := b.ValidatePickit(PickitValidationRequest{Profile: request.Profile})
	if err != nil {
		return PickitPreviewDTO{}, err
	}
	document := pickitDocument(validated.Profile)
	specs := make([]loot.PickitRuleSpec, len(document.Rules))
	for i, rule := range document.Rules {
		specs[i] = loot.PickitRuleSpec{ProfileID: document.ID, RuleID: rule.ID, Action: rule.Action, Expression: rule.Expression, ProfileRevision: document.Revision}
	}
	compiled, _ := loot.CompilePickitRules("pickit API preview", specs)
	item := world.Item{Code: request.Item.Code, Name: request.Item.Name, Type: request.Item.Type, Quality: previewQuality(request.Item.Quality), IdentityKind: world.ItemIdentityKind(request.Item.IdentityKind), IdentityKey: request.Item.IdentityKey, IdentityAvailable: request.Item.IdentityAvailable, IdentityValid: request.Item.IdentityValid, Identified: request.Item.Identified, Ethereal: request.Item.Ethereal}
	result := compiled.Evaluate(item)
	trace := make([]PickitTraceDTO, len(result.Trace))
	for i, entry := range result.Trace {
		trace[i] = PickitTraceDTO{RuleIndex: entry.RuleIndex, ProfileID: entry.ProfileID, RuleID: entry.RuleID, Action: string(entry.Action), Expression: entry.Expression, Matched: entry.Matched, ProfileRevision: entry.ProfileRevision, AssignmentRevision: entry.AssignmentRevision}
	}
	return PickitPreviewDTO{Matched: result.Matched, RuleIndex: result.RuleIndex, ProfileID: result.ProfileID, RuleID: result.RuleID, Action: string(result.Action), ProfileRevision: result.ProfileRevision, AssignmentRevision: result.AssignmentRevision, Trace: trace}, nil
}

func previewQuality(value string) world.ItemQuality {
	switch value {
	case "low_quality":
		return world.ItemQualityLowQuality
	case "normal":
		return world.ItemQualityNormal
	case "superior":
		return world.ItemQualitySuperior
	case "magic":
		return world.ItemQualityMagic
	case "set":
		return world.ItemQualitySet
	case "rare":
		return world.ItemQualityRare
	case "unique":
		return world.ItemQualityUnique
	case "crafted":
		return world.ItemQualityCrafted
	}
	return world.ItemQualityUnknown
}

func (b *LiveBackend) pickitMutation(operation func() error) error {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	b.mu.RLock()
	state, workflow := b.status.State, b.routeWorkflow.State
	b.mu.RUnlock()
	if state != string(app.SupervisorStateIdle) && state != string(app.SupervisorStateIdleInGame) {
		return &commandError{code: "command_conflict", message: "Pickit kann während eines aktiven Runs nicht geändert werden."}
	}
	if routeWorkflowBusy(workflow) {
		return &commandError{code: "command_conflict", message: "Pickit ist während eines Routen-Workflows gesperrt."}
	}
	return operation()
}

func (b *LiveBackend) CreatePickit(request PickitCreateRequest) (result PickitProfileDTO, err error) {
	err = b.pickitMutation(func() error {
		value, e := b.pickitProfiles.Create(pickitDocument(request.Profile))
		if e == nil {
			result = profileDTO(value)
		}
		return e
	})
	if err == nil {
		b.publishPickit("pickit_profile_changed")
	}
	return
}
func (b *LiveBackend) UpdatePickit(id string, request PickitUpdateRequest) (result PickitProfileDTO, err error) {
	err = b.pickitMutation(func() error {
		value, e := b.pickitProfiles.Update(id, request.ExpectedRevision, pickitDocument(request.Profile))
		if e != nil && strings.Contains(e.Error(), "revision_conflict") {
			current, _ := b.pickitProfiles.Get(id)
			return &commandError{code: "revision_conflict", message: "Das Pickit-Profil wurde zwischenzeitlich geändert.", details: map[string]any{"expected_revision": request.ExpectedRevision, "current_revision": current.Revision, "path": "profile.revision"}}
		}
		if e == nil {
			result = profileDTO(value)
		}
		return e
	})
	if err == nil {
		b.publishPickit("pickit_profile_changed")
	}
	return
}
func (b *LiveBackend) DuplicatePickit(id string, request PickitDuplicateRequest) (result PickitProfileDTO, err error) {
	err = b.pickitMutation(func() error {
		value, e := b.pickitProfiles.Duplicate(id, request.TargetID, request.TargetName)
		if e == nil {
			result = profileDTO(value)
		}
		return e
	})
	if err == nil {
		b.publishPickit("pickit_profile_changed")
	}
	return
}
func (b *LiveBackend) DeletePickit(id string, request PickitDeleteRequest) error {
	err := b.pickitMutation(func() error {
		current, e := b.pickitProfiles.Get(id)
		if e != nil {
			return e
		}
		if current.Revision != request.ExpectedRevision {
			return &commandError{code: "revision_conflict", message: "Das Pickit-Profil wurde zwischenzeitlich geändert.", details: map[string]any{"expected_revision": request.ExpectedRevision, "current_revision": current.Revision, "path": "profile.revision"}}
		}
		return b.pickitProfiles.Delete(id, b.pickitAssignments)
	})
	if err == nil {
		b.publishPickit("pickit_profile_changed")
	}
	return err
}

func (b *LiveBackend) PickitAssignments() (PickitAssignmentsDTO, error) {
	manifest, err := b.pickitAssignments.Snapshot()
	if err != nil {
		return PickitAssignmentsDTO{}, err
	}
	return assignmentsDTO(manifest), nil
}
func assignmentsDTO(manifest app.PickitAssignmentManifest) PickitAssignmentsDTO {
	values := make(map[string]map[string][]string, len(manifest.Assignments))
	for character, runs := range manifest.Assignments {
		values[character] = make(map[string][]string, len(runs))
		for runID, profiles := range runs {
			values[character][string(runID)] = append([]string(nil), profiles...)
		}
	}
	return PickitAssignmentsDTO{SchemaVersion: manifest.SchemaVersion, Revision: manifest.Revision, Assignments: values}
}
func (b *LiveBackend) UpdatePickitAssignment(request PickitAssignmentUpdateRequest) (result PickitAssignmentsDTO, err error) {
	err = b.pickitMutation(func() error {
		value, e := b.pickitAssignments.Replace(request.Character, tasks.RunID(request.RunID), request.ProfileIDs, request.ExpectedRevision)
		if e != nil && strings.Contains(e.Error(), "revision_conflict") {
			current, _ := b.pickitAssignments.Snapshot()
			return &commandError{code: "revision_conflict", message: "Die Pickit-Zuordnung wurde zwischenzeitlich geändert.", details: map[string]any{"expected_revision": request.ExpectedRevision, "current_revision": current.Revision, "path": "assignments.revision"}}
		}
		if e == nil {
			result = assignmentsDTO(value)
		}
		return e
	})
	if err == nil {
		b.publishPickit("pickit_assignment_changed")
	}
	return
}

func (b *LiveBackend) ImportPickit(request PickitImportRequest) (PickitImportDTO, error) {
	action := loot.Action(request.Action)
	if !action.Valid() {
		return PickitImportDTO{}, fmt.Errorf("unsupported import action %q", request.Action)
	}
	rules := []PickitRuleDTO{}
	for number, line := range strings.Split(request.Text, "\n") {
		expression := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if expression == "" {
			continue
		}
		canonical, err := loot.CanonicalPickitExpression(expression)
		if err != nil {
			return PickitImportDTO{}, fmt.Errorf("import line %d: %w", number+1, err)
		}
		rules = append(rules, PickitRuleDTO{ID: fmt.Sprintf("import-%d", len(rules)+1), Action: string(action), Expression: canonical})
	}
	if len(rules) == 0 {
		return PickitImportDTO{}, fmt.Errorf("import contains no rules")
	}
	return PickitImportDTO{Rules: rules, Warnings: []string{"NIP kennt keine Phase-13-Aktion; die gewählte Aktion wurde allen Regeln zugewiesen."}}, nil
}
func (b *LiveBackend) ExportPickit(id string) (PickitExportDTO, error) {
	profile, err := b.pickitProfiles.Get(id)
	if err != nil {
		return PickitExportDTO{}, err
	}
	lines := make([]string, len(profile.Rules))
	for i, rule := range profile.Rules {
		lines[i] = rule.Expression
	}
	return PickitExportDTO{ProfileID: id, Revision: profile.Revision, Text: strings.Join(lines, "\n") + "\n", Warning: "NIP transportiert keep/sell/ignore nicht; Aktionen bleiben nur im Profil erhalten."}, nil
}
func (b *LiveBackend) publishPickit(event string) {
	b.publisher.Publish(telemetry.LiveEvent{Event: event, Details: map[string]any{"source": "pickit_api"}})
}

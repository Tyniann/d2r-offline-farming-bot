package app

import (
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

// CharacterLoadoutSnapshot freezes character, profile, revision, bindings and inventory for one workflow/queue session.
type CharacterLoadoutSnapshot struct {
	Character           string
	ProfileID           string
	Revision            uint64
	Bindings            configBindingSource // cast buttons follow the frozen profile slot contract
	BeltLayout          OperatorBeltLayout  // empty means combat-profile YAML defaults
	BindingsComplete    bool
	InventoryConfigured bool
	InventoryGrid       [][]int // defensive copy; nil when unconfigured
	Players             int     // frozen /players 1–8; 0 means default 1
}

// CharacterLoadoutResolver resolves OperatorSettings Schema 3 into an immutable runtime snapshot.
type CharacterLoadoutResolver struct {
	store      *OperatorSettingsStore
	profiles   config.ProfilesConfig
	botHotkeys OperatorInputSettings
}

// NewCharacterLoadoutResolver builds a resolver over the installed operator settings store.
func NewCharacterLoadoutResolver(store *OperatorSettingsStore, profiles config.ProfilesConfig, botHotkeys OperatorInputSettings) *CharacterLoadoutResolver {
	clonedProfiles := make(config.ProfilesConfig, len(profiles))
	for id, profile := range profiles {
		clonedProfiles[id] = profile
	}
	return &CharacterLoadoutResolver{store: store, profiles: clonedProfiles, botHotkeys: botHotkeys}
}

// Resolve builds a defensive snapshot for character. Incomplete bindings still produce a snapshot for inspection but mark Completeness.
func (r *CharacterLoadoutResolver) Resolve(character string) (CharacterLoadoutSnapshot, error) {
	if r == nil || r.store == nil {
		return CharacterLoadoutSnapshot{}, fmt.Errorf("character loadout resolver is unavailable")
	}
	settings, err := r.store.Snapshot()
	if err != nil {
		return CharacterLoadoutSnapshot{}, err
	}
	key := strings.ToLower(strings.TrimSpace(character))
	if key == "" {
		return CharacterLoadoutSnapshot{}, fmt.Errorf("character loadout requires a character name")
	}
	value, ok := settings.Characters[key]
	if !ok {
		return CharacterLoadoutSnapshot{}, fmt.Errorf("character %q is unknown in operator settings", character)
	}
	profileID := strings.TrimSpace(value.CombatProfile)
	if profileID == "" {
		return CharacterLoadoutSnapshot{}, fmt.Errorf("character %q has no combat profile", character)
	}
	profile, ok := r.profiles[profileID]
	if !ok {
		return CharacterLoadoutSnapshot{}, fmt.Errorf("character %q combat profile %q is unknown", character, profileID)
	}
	bindings := value.ProfileBindings[profileID]
	source, err := bindingSourceFromOperatorBindings(bindings, profile)
	if err != nil {
		return CharacterLoadoutSnapshot{}, fmt.Errorf("character %q profile %q bindings: %w", character, profileID, err)
	}
	snapshot := CharacterLoadoutSnapshot{
		Character:           strings.TrimSpace(character),
		ProfileID:           profileID,
		Revision:            settings.Revision,
		Bindings:            source,
		BeltLayout:          bindings.BeltLayout,
		BindingsComplete:    ProfileBindingsComplete(bindings, profile),
		InventoryConfigured: value.InventoryLock != nil,
		Players:             EffectivePlayers(value.Players),
	}
	if value.InventoryLock != nil {
		snapshot.InventoryGrid = cloneInventoryGrid(value.InventoryLock.Grid)
	}
	return snapshot, nil
}

// CloneCharacterLoadoutSnapshot returns a defensive copy of a frozen loadout.
func CloneCharacterLoadoutSnapshot(snapshot CharacterLoadoutSnapshot) CharacterLoadoutSnapshot {
	clone := snapshot
	clone.Bindings = cloneConfigBindingSource(snapshot.Bindings)
	clone.InventoryGrid = cloneInventoryGrid(snapshot.InventoryGrid)
	return clone
}

// EffectiveInventoryGrid returns the runtime 4×10 lock grid. Unconfigured loadouts are fully locked.
func EffectiveInventoryGrid(snapshot CharacterLoadoutSnapshot) [][]int {
	if snapshot.InventoryConfigured && len(snapshot.InventoryGrid) == 4 {
		return cloneInventoryGrid(snapshot.InventoryGrid)
	}
	return allLockedInventoryGrid()
}

// ProfileBindingsComplete reports whether active profile has every required_skill + all 4 belt slots.
func ProfileBindingsComplete(bindings OperatorProfileBindings, profile config.ProfileConfig) bool {
	if len(profile.RequiredSkills) == 0 {
		return false
	}
	for _, required := range profile.RequiredSkills {
		key := strings.ToLower(strings.TrimSpace(required.Skill))
		bound, ok := bindings.Skills[key]
		if !ok || !isOperatorSkillFKey(strings.ToLower(strings.TrimSpace(bound))) {
			return false
		}
	}
	if err := validateOptionalSkillPairBindings(bindings, profile); err != nil {
		return false
	}
	return strings.TrimSpace(bindings.Belt.Slot1) != "" &&
		strings.TrimSpace(bindings.Belt.Slot2) != "" &&
		strings.TrimSpace(bindings.Belt.Slot3) != "" &&
		strings.TrimSpace(bindings.Belt.Slot4) != ""
}

// EvaluateLoadoutReadiness returns stable reason codes for static queue gates (bindings then inventory).
func EvaluateLoadoutReadiness(settings OperatorSettings, character string, profiles config.ProfilesConfig) []string {
	key := strings.ToLower(strings.TrimSpace(character))
	value, ok := settings.Characters[key]
	if !ok {
		return []string{string(QueueReasonProfileBindingsIncomplete)}
	}
	profileID := strings.TrimSpace(value.CombatProfile)
	if profileID == "" {
		return []string{string(QueueReasonProfileBindingsIncomplete)}
	}
	profile, ok := profiles[profileID]
	if !ok {
		return []string{string(QueueReasonProfileBindingsIncomplete)}
	}
	bindings := OperatorProfileBindings{}
	if value.ProfileBindings != nil {
		bindings = value.ProfileBindings[profileID]
	}
	if !ProfileBindingsComplete(bindings, profile) {
		return []string{string(QueueReasonProfileBindingsIncomplete)}
	}
	if value.InventoryLock == nil {
		return []string{string(QueueReasonCharacterInventoryUnconfigured)}
	}
	return nil
}

// InventoryCowSuitable reports whether a configured lock grid is statically cow-capable.
// It requires any locked 2×2 rectangle and disjoint free cells for 1×3 and 1×2 footprints.
func InventoryCowSuitable(grid [][]int) bool {
	if len(grid) != 4 {
		return false
	}
	locked := [4][10]bool{}
	for row := 0; row < 4; row++ {
		if len(grid[row]) != 10 {
			return false
		}
		for col := 0; col < 10; col++ {
			locked[row][col] = grid[row][col] == 1
		}
	}
	hasLocked2x2 := false
	for row := 0; row < 3 && !hasLocked2x2; row++ {
		for col := 0; col < 9; col++ {
			if locked[row][col] && locked[row][col+1] && locked[row+1][col] && locked[row+1][col+1] {
				hasLocked2x2 = true
				break
			}
		}
	}
	if !hasLocked2x2 {
		return false
	}
	return cowStaticCanPlace(locked, 1, 3) && cowStaticCanPlace(locked, 1, 2)
}

func cowStaticCanPlace(locked [4][10]bool, width, height int) bool {
	for row := 0; row <= 4-height; row++ {
		for col := 0; col <= 10-width; col++ {
			fits := true
			for dy := 0; dy < height && fits; dy++ {
				for dx := 0; dx < width; dx++ {
					if locked[row+dy][col+dx] {
						fits = false
						break
					}
				}
			}
			if fits {
				return true
			}
		}
	}
	return false
}

func bindingSourceFromOperatorBindings(bindings OperatorProfileBindings, profileCfg config.ProfileConfig) (configBindingSource, error) {
	out := configBindingSource{
		skills: make(map[uint16]input.SkillCast, len(bindings.Skills)),
		belt: [4]string{
			strings.ToLower(strings.TrimSpace(bindings.Belt.Slot1)),
			strings.ToLower(strings.TrimSpace(bindings.Belt.Slot2)),
			strings.ToLower(strings.TrimSpace(bindings.Belt.Slot3)),
			strings.ToLower(strings.TrimSpace(bindings.Belt.Slot4)),
		},
	}
	for rawName, rawKey := range bindings.Skills {
		skillID, err := memory.ParseSkillTestName(rawName)
		if err != nil {
			return configBindingSource{}, fmt.Errorf("skills.%s: %w", rawName, err)
		}
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if key == "" {
			continue
		}
		button := input.MouseRight
		if profileSkillSlot(profileCfg, rawName) == "left" {
			button = input.MouseLeft
		}
		out.skills[skillID] = input.SkillCast{
			SkillID:    skillID,
			SelectKey:  key,
			CastButton: button,
		}
	}
	return out, nil
}

func profileSkillSlot(profileCfg config.ProfileConfig, skill string) string {
	for _, required := range profileCfg.RequiredSkills {
		if required.Skill == skill {
			return required.Slot
		}
	}
	for _, pair := range profileCfg.OptionalSkillPairs {
		for _, optional := range pair.Skills {
			if optional.Skill == skill {
				return optional.Slot
			}
		}
	}
	return ""
}

func cloneConfigBindingSource(src configBindingSource) configBindingSource {
	clone := configBindingSource{belt: src.belt, skills: make(map[uint16]input.SkillCast, len(src.skills))}
	for skillID, cast := range src.skills {
		clone.skills[skillID] = cast
	}
	return clone
}

func cloneInventoryGrid(grid [][]int) [][]int {
	if grid == nil {
		return nil
	}
	clone := make([][]int, len(grid))
	for row, values := range grid {
		clone[row] = append([]int(nil), values...)
	}
	return clone
}

func allLockedInventoryGrid() [][]int {
	grid := make([][]int, 4)
	for row := range grid {
		grid[row] = []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	}
	return grid
}

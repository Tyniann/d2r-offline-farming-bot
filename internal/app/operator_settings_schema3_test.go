package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestOperatorSettingsSchema3BindingsRoundTripAndValidation(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}

	replacement := cloneOperatorSettings(assigned.Settings)
	value := replacement.Characters["mrbones"]
	value.ProfileBindings = map[string]OperatorProfileBindings{
		"necro_bone_spear": necroBoneSpearBindingsFixture(),
	}
	value.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	replacement.Characters["mrbones"] = value
	change, err := store.Update(assigned.Settings.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}
	got := change.Settings.Characters["mrbones"]
	if !reflect.DeepEqual(got.ProfileBindings["necro_bone_spear"], necroBoneSpearBindingsFixture()) {
		t.Fatalf("bindings=%+v", got.ProfileBindings)
	}
	if !reflect.DeepEqual(got.InventoryLock.Grid, sampleInventoryGrid(false)) {
		t.Fatalf("inventory=%+v", got.InventoryLock)
	}

	schema2 := cloneOperatorSettings(change.Settings)
	schema2.SchemaVersion = 2
	if mkdirErr := os.MkdirAll(filepath.Dir(store.path), 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(store.path, mustMarshalOperatorSettings(schema2), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	_, err = store.Snapshot()
	var settingsErr *OperatorSettingsError
	if !errors.As(err, &settingsErr) || settingsErr.Code != Phase15ReasonConfigSchemaUnsupported {
		t.Fatalf("schema-2 error=%v", err)
	}
}

func TestOperatorSettingsPartialBindingsAndRejects(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}

	partial := cloneOperatorSettings(assigned.Settings)
	value := partial.Characters["mrbones"]
	value.ProfileBindings = map[string]OperatorProfileBindings{
		"necro_bone_spear": {Skills: map[string]string{"teleport": "f7"}},
	}
	partial.Characters["mrbones"] = value
	if _, err := store.Update(assigned.Settings.Revision, partial); err != nil {
		t.Fatalf("partial bindings should store: %v", err)
	}
	current, _ := store.Snapshot()

	duplicate := cloneOperatorSettings(current)
	dupValue := duplicate.Characters["mrbones"]
	dupValue.ProfileBindings = map[string]OperatorProfileBindings{
		"necro_bone_spear": {Skills: map[string]string{"teleport": "f7", "town_portal": "f7"}},
	}
	duplicate.Characters["mrbones"] = dupValue
	if _, err := store.Update(current.Revision, duplicate); err == nil {
		t.Fatal("duplicate F-key accepted")
	}

	unsafe := cloneOperatorSettings(current)
	unsafeValue := unsafe.Characters["mrbones"]
	unsafeValue.ProfileBindings = map[string]OperatorProfileBindings{
		"necro_bone_spear": {Belt: OperatorBeltBindings{Slot1: "f10"}},
	}
	unsafe.Characters["mrbones"] = unsafeValue
	if _, err := store.Update(current.Revision, unsafe); err == nil {
		t.Fatal("unsafe belt key accepted")
	}
}

func TestOperatorSettingsInventoryLockVariants(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	current := assigned.Settings

	absent := cloneOperatorSettings(current)
	value := absent.Characters["mrbones"]
	value.InventoryLock = nil
	absent.Characters["mrbones"] = value
	if _, err := store.Update(current.Revision, absent); err != nil {
		t.Fatalf("absent inventory should store: %v", err)
	}
	current, _ = store.Snapshot()

	locked := cloneOperatorSettings(current)
	lockedValue := locked.Characters["mrbones"]
	lockedValue.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(true)}
	locked.Characters["mrbones"] = lockedValue
	if _, err := store.Update(current.Revision, locked); err != nil {
		t.Fatalf("all-locked inventory should store: %v", err)
	}
	current, _ = store.Snapshot()

	malformed := cloneOperatorSettings(current)
	bad := malformed.Characters["mrbones"]
	bad.InventoryLock = &OperatorInventoryLock{Grid: [][]int{{1, 0}}}
	malformed.Characters["mrbones"] = bad
	if _, err := store.Update(current.Revision, malformed); err == nil {
		t.Fatal("malformed inventory accepted")
	}
}

func TestOperatorSettingsBindingsDoNotCrossCharactersAndCloneIsDefensive(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := cloneOperatorSettings(assigned.Settings)
	value := replacement.Characters["mrbones"]
	value.ProfileBindings = map[string]OperatorProfileBindings{"necro_bone_spear": necroBoneSpearBindingsFixture()}
	value.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	replacement.Characters["mrbones"] = value
	change, err := store.Update(assigned.Settings.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}
	hammer := change.Settings.Characters["mrhammer"]
	if hammer.ProfileBindings != nil || hammer.InventoryLock != nil {
		t.Fatalf("cross-character leak=%+v", hammer)
	}

	clone := cloneOperatorSettings(change.Settings)
	clone.Characters["mrbones"].ProfileBindings["necro_bone_spear"].Skills["teleport"] = "f1"
	clone.Characters["mrbones"].InventoryLock.Grid[0][0] = 0
	original := change.Settings.Characters["mrbones"]
	if original.ProfileBindings["necro_bone_spear"].Skills["teleport"] != "f7" {
		t.Fatal("clone aliased skill map")
	}
	if original.InventoryLock.Grid[0][0] != 1 {
		t.Fatal("clone aliased inventory grid")
	}
}

func TestOperatorSettingsResetPreservesBindingsAndInventory(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := cloneOperatorSettings(assigned.Settings)
	value := replacement.Characters["mrbones"]
	value.ProfileBindings = map[string]OperatorProfileBindings{"necro_bone_spear": necroBoneSpearBindingsFixture()}
	value.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	replacement.Characters["mrbones"] = value
	updated, err := store.Update(assigned.Settings.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}
	reset, err := store.Reset(updated.Settings.Revision)
	if err != nil {
		t.Fatal(err)
	}
	got := reset.Settings.Characters["mrbones"]
	if !reflect.DeepEqual(got.ProfileBindings["necro_bone_spear"], necroBoneSpearBindingsFixture()) {
		t.Fatalf("reset wiped bindings=%+v", got.ProfileBindings)
	}
	if !reflect.DeepEqual(got.InventoryLock.Grid, sampleInventoryGrid(false)) {
		t.Fatalf("reset wiped inventory=%+v", got.InventoryLock)
	}
}

func TestAssignCharacterProfilePreservesQueueInventoryAndInactiveBindings(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	second := store.profiles["necro_bone_spear"]
	second.DisplayName = "Knochengeist"
	second.Setup.Default = false
	second.Combat.StandardAttack = "bone_spirit"
	store.profiles["necro_bone_spirit"] = second

	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := cloneOperatorSettings(assigned.Settings)
	value := replacement.Characters["mrbones"]
	value.Queue = []string{"countess", "cows"}
	value.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	value.ProfileBindings = map[string]OperatorProfileBindings{
		"necro_bone_spear":  necroBoneSpearBindingsFixture(),
		"necro_bone_spirit": {Skills: map[string]string{"teleport": "f7"}, Belt: OperatorBeltBindings{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"}},
	}
	replacement.Characters["mrbones"] = value
	updated, err := store.Update(assigned.Settings.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}
	switched, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spirit", updated.Settings.Revision)
	if err != nil {
		t.Fatal(err)
	}
	got := switched.Settings.Characters["mrbones"]
	if got.CombatProfile != "necro_bone_spirit" {
		t.Fatalf("profile=%q", got.CombatProfile)
	}
	if !reflect.DeepEqual(got.Queue, []string{"countess", "cows"}) {
		t.Fatalf("queue wiped=%v", got.Queue)
	}
	if !reflect.DeepEqual(got.InventoryLock.Grid, sampleInventoryGrid(false)) {
		t.Fatalf("inventory wiped=%v", got.InventoryLock)
	}
	if !reflect.DeepEqual(got.ProfileBindings["necro_bone_spear"], necroBoneSpearBindingsFixture()) {
		t.Fatalf("inactive bindings wiped=%v", got.ProfileBindings)
	}
	if got.ProfileBindings["necro_bone_spirit"].Skills["teleport"] != "f7" {
		t.Fatalf("active bindings wiped=%v", got.ProfileBindings["necro_bone_spirit"])
	}
}

func TestCharacterLoadoutResolverAndReadiness(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := cloneOperatorSettings(assigned.Settings)
	value := replacement.Characters["mrbones"]
	value.ProfileBindings = map[string]OperatorProfileBindings{"necro_bone_spear": necroBoneSpearBindingsFixture()}
	value.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	replacement.Characters["mrbones"] = value
	updated, err := store.Update(assigned.Settings.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}

	resolver := NewCharacterLoadoutResolver(store, store.profiles, updated.Settings.Input)
	snapshot, err := resolver.Resolve("MrBones")
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.BindingsComplete || snapshot.ProfileID != "necro_bone_spear" || !snapshot.InventoryConfigured {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	cast, err := snapshot.Bindings.Resolve(memory.SkillBoneSpear)
	if err != nil || cast.SelectKey != "f8" || cast.CastButton != input.MouseRight {
		t.Fatalf("bone spear cast=%+v err=%v", cast, err)
	}
	if reasons := EvaluateLoadoutReadiness(updated.Settings, "MrBones", store.profiles); len(reasons) != 0 {
		t.Fatalf("ready reasons=%v", reasons)
	}

	partial := cloneOperatorSettings(updated.Settings)
	partialValue := partial.Characters["mrbones"]
	partialValue.ProfileBindings = map[string]OperatorProfileBindings{
		"necro_bone_spear": {Skills: map[string]string{"teleport": "f7"}},
	}
	partial.Characters["mrbones"] = partialValue
	if reasons := EvaluateLoadoutReadiness(partial, "MrBones", store.profiles); len(reasons) != 1 || reasons[0] != string(QueueReasonProfileBindingsIncomplete) {
		t.Fatalf("incomplete reasons=%v", reasons)
	}

	noInventory := cloneOperatorSettings(updated.Settings)
	noInventoryValue := noInventory.Characters["mrbones"]
	noInventoryValue.InventoryLock = nil
	noInventory.Characters["mrbones"] = noInventoryValue
	if reasons := EvaluateLoadoutReadiness(noInventory, "MrBones", store.profiles); len(reasons) != 1 || reasons[0] != string(QueueReasonCharacterInventoryUnconfigured) {
		t.Fatalf("inventory reasons=%v", reasons)
	}
	if !InventoryCowSuitable(sampleInventoryGrid(false)) {
		t.Fatal("expected cow-suitable left-locked grid")
	}
	if InventoryCowSuitable(sampleInventoryGrid(true)) {
		t.Fatal("all-locked grid must warn for cows")
	}
}

func necroBoneSpearBindingsFixture() OperatorProfileBindings {
	return OperatorProfileBindings{
		Skills: map[string]string{
			"teleport":         "f7",
			"town_portal":      "f6",
			"amplify_damage":   "f1",
			"corpse_explosion": "f2",
			"bone_prison":      "f3",
			"bone_armor":       "f5",
			"bone_spear":       "f8",
		},
		Belt: OperatorBeltBindings{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"},
	}
}

func sampleInventoryGrid(allLocked bool) [][]int {
	grid := make([][]int, 4)
	for row := range grid {
		grid[row] = make([]int, 10)
		for col := range grid[row] {
			if allLocked || col < 4 {
				grid[row][col] = 1
			}
		}
	}
	return grid
}

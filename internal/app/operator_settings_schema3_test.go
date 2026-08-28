package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestOperatorSettingsPotionRestockRoundTripAndRejects(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}

	healing, mana := 3, 8
	ok := cloneOperatorSettings(assigned.Settings)
	value := ok.Characters["mrbones"]
	bindings := necroBoneSpearBindingsFixture()
	bindings.PotionRestock = OperatorPotionRestock{Healing: &healing, Mana: &mana}
	value.ProfileBindings = map[string]OperatorProfileBindings{"necro_bone_spear": bindings}
	ok.Characters["mrbones"] = value
	change, err := store.Update(assigned.Settings.Revision, ok)
	if err != nil {
		t.Fatal(err)
	}
	got := change.Settings.Characters["mrbones"].ProfileBindings["necro_bone_spear"].PotionRestock
	if got.Healing == nil || *got.Healing != 3 || got.Mana == nil || *got.Mana != 8 {
		t.Fatalf("potion_restock=%+v", got)
	}

	current, _ := store.Snapshot()
	tooHigh := cloneOperatorSettings(current)
	high := 5
	tooHighValue := tooHigh.Characters["mrbones"]
	highBindings := necroBoneSpearBindingsFixture()
	highBindings.PotionRestock = OperatorPotionRestock{Healing: &high}
	tooHighValue.ProfileBindings = map[string]OperatorProfileBindings{"necro_bone_spear": highBindings}
	tooHigh.Characters["mrbones"] = tooHighValue
	if _, err := store.Update(current.Revision, tooHigh); err == nil {
		t.Fatal("healing restock above column capacity accepted")
	}

	noMana := cloneOperatorSettings(current)
	manaOnly := 2
	noManaValue := noMana.Characters["mrbones"]
	noManaBindings := necroBoneSpearBindingsFixture()
	noManaBindings.BeltLayout = OperatorBeltLayout{Slot1: beltPotionHealing, Slot2: beltPotionHealing, Slot3: beltPotionRejuvenation, Slot4: beltPotionRejuvenation}
	noManaBindings.PotionRestock = OperatorPotionRestock{Mana: &manaOnly}
	noManaValue.ProfileBindings = map[string]OperatorProfileBindings{"necro_bone_spear": noManaBindings}
	noMana.Characters["mrbones"] = noManaValue
	if _, err := store.Update(current.Revision, noMana); err == nil {
		t.Fatal("mana restock without mana column accepted")
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
	if !snapshot.BindingsComplete || snapshot.ProfileID != "necro_bone_spear" || !snapshot.InventoryConfigured || snapshot.Players != DefaultOfflinePlayers {
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

func TestHammerdinBindingsRequireDutySkillsAndAllOrNothingCTA(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	assigned, err := store.AssignCharacterProfile("MrHammer", "paladin", "paladin_hammerdin", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}

	replacement := cloneOperatorSettings(assigned.Settings)
	value := replacement.Characters["mrhammer"]
	value.ProfileBindings = map[string]OperatorProfileBindings{"paladin_hammerdin": hammerdinBindingsFixture(false)}
	value.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	replacement.Characters["mrhammer"] = value
	withoutCTA, err := store.Update(assigned.Settings.Revision, replacement)
	if err != nil {
		t.Fatalf("required bindings with empty CTA: %v", err)
	}
	profile := store.profiles["paladin_hammerdin"]
	explicitEmptyCTA := hammerdinBindingsFixture(false)
	explicitEmptyCTA.Skills["battle_command"] = ""
	explicitEmptyCTA.Skills["battle_orders"] = ""
	if err = validateOperatorProfileBindings("mrhammer", map[string]OperatorProfileBindings{"paladin_hammerdin": explicitEmptyCTA}, store.profiles, assigned.Settings.Input); err != nil {
		t.Fatalf("explicit empty/empty CTA: %v", err)
	}
	if !ProfileBindingsComplete(withoutCTA.Settings.Characters["mrhammer"].ProfileBindings["paladin_hammerdin"], profile) {
		t.Fatal("valid required bindings with empty CTA are incomplete")
	}

	withCTA := cloneOperatorSettings(withoutCTA.Settings)
	withCTAValue := withCTA.Characters["mrhammer"]
	withCTAValue.ProfileBindings["paladin_hammerdin"] = hammerdinBindingsFixture(true)
	withCTA.Characters["mrhammer"] = withCTAValue
	updated, err := store.Update(withoutCTA.Settings.Revision, withCTA)
	if err != nil {
		t.Fatalf("complete CTA pair: %v", err)
	}
	resolver := NewCharacterLoadoutResolver(store, store.profiles, updated.Settings.Input)
	loadout, err := resolver.Resolve("MrHammer")
	if err != nil || !loadout.BindingsComplete {
		t.Fatalf("Hammerdin loadout=%+v err=%v", loadout, err)
	}
	hammer, err := loadout.Bindings.Resolve(memory.MustSkillID("blessed_hammer"))
	if err != nil || hammer.CastButton != input.MouseLeft {
		t.Fatalf("Blessed Hammer binding=%+v err=%v", hammer, err)
	}
	for _, skill := range []string{"teleport", "town_portal", "concentration", "holy_shield", "battle_command", "battle_orders"} {
		cast, resolveErr := loadout.Bindings.Resolve(memory.MustSkillID(skill))
		if resolveErr != nil || cast.CastButton != input.MouseRight {
			t.Fatalf("%s binding=%+v err=%v", skill, cast, resolveErr)
		}
	}

	for _, missing := range []string{"battle_command", "battle_orders"} {
		t.Run("missing "+missing, func(t *testing.T) {
			partial := cloneOperatorSettings(updated.Settings)
			partialValue := partial.Characters["mrhammer"]
			bindings := hammerdinBindingsFixture(true)
			delete(bindings.Skills, missing)
			partialValue.ProfileBindings["paladin_hammerdin"] = bindings
			partial.Characters["mrhammer"] = partialValue
			if _, updateErr := store.Update(updated.Settings.Revision, partial); updateErr == nil || !strings.Contains(updateErr.Error(), "Für Call to Arms müssen Battle Command und Battle Orders beide belegt sein") {
				t.Fatalf("partial CTA error=%v", updateErr)
			}
		})
	}

	missingRequired := hammerdinBindingsFixture(false)
	delete(missingRequired.Skills, "holy_shield")
	if ProfileBindingsComplete(missingRequired, profile) {
		t.Fatal("missing Holy Shield binding accepted as complete")
	}
}

func TestOperatorSettingsRejectsCTAEnabledToggle(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	assigned, err := store.AssignCharacterProfile("MrHammer", "paladin", "paladin_hammerdin", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	settings := cloneOperatorSettings(assigned.Settings)
	value := settings.Characters["mrhammer"]
	value.ProfileBindings = map[string]OperatorProfileBindings{"paladin_hammerdin": hammerdinBindingsFixture(false)}
	settings.Characters["mrhammer"] = value
	encoded := string(mustMarshalOperatorSettings(settings))
	needle := "\n                skills:\n"
	withToggle := strings.Replace(encoded, needle, "\n                cta:\n                    enabled: true"+needle, 1)
	if withToggle == encoded {
		t.Fatal("test fixture could not insert CTA toggle")
	}
	if err := os.WriteFile(store.path, []byte(withToggle), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Snapshot(); err == nil || !strings.Contains(err.Error(), "field cta not found") {
		t.Fatalf("CTA toggle error=%v", err)
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

func hammerdinBindingsFixture(withCTA bool) OperatorProfileBindings {
	bindings := OperatorProfileBindings{
		Skills: map[string]string{
			"blessed_hammer": "f1",
			"concentration":  "f2",
			"teleport":       "f3",
			"holy_shield":    "f4",
			"town_portal":    "f5",
		},
		Belt: OperatorBeltBindings{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"},
	}
	if withCTA {
		bindings.Skills["battle_command"] = "f6"
		bindings.Skills["battle_orders"] = "f7"
	}
	return bindings
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

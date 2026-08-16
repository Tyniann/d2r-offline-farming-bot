package app

import (
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestNewCharacterWorkflowRuntimeUsesSelectedCharacterLoadout(t *testing.T) {
	store, root := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := cloneOperatorSettings(assigned.Settings)
	character := replacement.Characters["mrbones"]
	character.ProfileBindings = map[string]OperatorProfileBindings{
		"necro_bone_spear": necroBoneSpearBindingsFixture(),
	}
	character.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	replacement.Characters["mrbones"] = character
	if _, updateErr := store.Update(assigned.Settings.Revision, replacement); updateErr != nil {
		t.Fatal(updateErr)
	}

	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataRoot = root
	cfg.Session.Character = "MrHammer"
	resolver := NewCharacterLoadoutResolver(store, cfg.Profiles, replacement.Input)
	runtime, err := NewCharacterWorkflowRuntime(cfg, resolver, "MrBones")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := runtime.CloseLog(); closeErr != nil {
			t.Errorf("CloseLog() error = %v", closeErr)
		}
	})

	teleport, err := runtime.Bindings.Resolve(memory.SkillTeleport)
	if err != nil {
		t.Fatalf("resolve Teleport binding: %v", err)
	}
	if teleport.SelectKey != "f7" {
		t.Fatalf("Teleport select key = %q, want f7", teleport.SelectKey)
	}
	if runtime.Config.Session.Character != "MrBones" {
		t.Fatalf("runtime character = %q, want MrBones", runtime.Config.Session.Character)
	}
	if runtime.Config.Session.Enabled {
		t.Fatal("isolated route workflow runtime must not start the farming session")
	}
	if cfg.Session.Character != "MrHammer" {
		t.Fatalf("source config character mutated to %q", cfg.Session.Character)
	}
}

func TestNewCharacterWorkflowRuntimeAllowsHammerdinWithoutCountessPickit(t *testing.T) {
	store, root := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.AssignCharacterProfile("MrHammer", "paladin", "paladin_hammerdin", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := cloneOperatorSettings(assigned.Settings)
	character := replacement.Characters["mrhammer"]
	character.ProfileBindings = map[string]OperatorProfileBindings{
		"paladin_hammerdin": hammerdinBindingsFixture(true),
	}
	character.InventoryLock = &OperatorInventoryLock{Grid: sampleInventoryGrid(false)}
	replacement.Characters["mrhammer"] = character
	if _, updateErr := store.Update(assigned.Settings.Revision, replacement); updateErr != nil {
		t.Fatal(updateErr)
	}

	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataRoot = root
	cfg.Session.Character = "MrBones"
	resolver := NewCharacterLoadoutResolver(store, cfg.Profiles, replacement.Input)
	runtime, err := NewCharacterWorkflowRuntime(cfg, resolver, "MrHammer")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := runtime.CloseLog(); closeErr != nil {
			t.Errorf("CloseLog() error = %v", closeErr)
		}
	})
	if runtime.Config.Session.Character != "MrHammer" {
		t.Fatalf("runtime character = %q, want MrHammer", runtime.Config.Session.Character)
	}
	if runtime.combatProfileID != "paladin_hammerdin" {
		t.Fatalf("runtime profile = %q, want paladin_hammerdin", runtime.combatProfileID)
	}
}

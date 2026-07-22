package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPickitProfileCRUDRevisionDuplicateDeleteAndWriteRollback(t *testing.T) {
	profiles, assignments := newTestPickitStores(t)
	base := testPickitProfile("base", "Basis", "runes", loot.ActionKeep, `[type] == rune`)
	created, err := profiles.Create(base)
	if err != nil {
		t.Fatal(err)
	}
	if created.Revision != 1 || created.Rules[0].Expression != `[type] == "rune"` {
		t.Fatalf("created profile = %+v", created)
	}
	if _, createErr := profiles.Create(base); createErr == nil || !strings.Contains(createErr.Error(), "pickit_profile_id_conflict") {
		t.Fatalf("duplicate create error = %v", createErr)
	}

	replacement := created
	replacement.Name = "Geändert"
	updated, err := profiles.Update("base", 1, replacement)
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update = %+v error=%v", updated, err)
	}
	// Wiederholte bestätigte Mutation ist auch mit der alten Revision idempotent.
	idempotent, err := profiles.Update("base", 1, replacement)
	if err != nil || idempotent.Revision != 2 {
		t.Fatalf("idempotent update = %+v error=%v", idempotent, err)
	}
	conflict := updated
	conflict.Name = "Konflikt"
	if _, updateErr := profiles.Update("base", 1, conflict); updateErr == nil || !strings.Contains(updateErr.Error(), "revision_conflict") {
		t.Fatalf("revision conflict error = %v", updateErr)
	}
	conflict.ID = "renamed"
	if _, updateErr := profiles.Update("base", 2, conflict); updateErr == nil || !strings.Contains(updateErr.Error(), "immutable") {
		t.Fatalf("immutable id error = %v", updateErr)
	}

	copy, err := profiles.Duplicate("base", "copy", "Kopie")
	if err != nil || copy.ID != "copy" || copy.Revision != 1 || copy.Rules[0].ID != "runes" {
		t.Fatalf("duplicate = %+v error=%v", copy, err)
	}
	if _, initErr := assignments.Initialize(map[string]map[tasks.RunID][]string{"MrBones": {tasks.RunIDCountess: {"base"}}}); initErr != nil {
		t.Fatal(initErr)
	}
	if deleteErr := profiles.Delete("base", assignments); deleteErr == nil || !strings.Contains(deleteErr.Error(), "pickit_profile_assigned") {
		t.Fatalf("assigned delete error = %v", deleteErr)
	}
	if deleteErr := profiles.Delete("copy", assignments); deleteErr != nil {
		t.Fatal(deleteErr)
	}
	if _, getErr := profiles.Get("copy"); !errors.Is(rootCause(getErr), os.ErrNotExist) {
		t.Fatalf("deleted profile error = %v", getErr)
	}

	before, err := profiles.Get("base")
	if err != nil {
		t.Fatal(err)
	}
	profiles.write = func(string, []byte, string) error { return errors.New("injected write failure") }
	failed := before
	failed.Name = "Darf nicht landen"
	if _, updateErr := profiles.Update("base", before.Revision, failed); updateErr == nil || !strings.Contains(updateErr.Error(), "write_failed") {
		t.Fatalf("write failure error = %v", updateErr)
	}
	after, err := profiles.Get("base")
	if err != nil || after.Revision != before.Revision || !samePickitProfileContent(after, before) {
		t.Fatalf("profile changed after failed write: before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestPickitAssignmentsRevisionDuplicateUnknownAndRollback(t *testing.T) {
	profiles, assignments := newTestPickitStores(t)
	for _, id := range []string{"one", "two"} {
		if _, err := profiles.Create(testPickitProfile(id, id, "rule", loot.ActionKeep, `[type] == rune`)); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := assignments.Initialize(map[string]map[tasks.RunID][]string{"MrBones": {tasks.RunIDCountess: {"one"}}})
	if err != nil || manifest.Revision != 1 {
		t.Fatalf("initialize = %+v error=%v", manifest, err)
	}
	updated, err := assignments.Replace("mrbones", tasks.RunIDCountess, []string{"two", "one"}, 1)
	if err != nil || updated.Revision != 2 || len(updated.Assignments) != 1 {
		t.Fatalf("replace = %+v error=%v", updated, err)
	}
	idempotent, err := assignments.Replace("MRBONES", tasks.RunIDCountess, []string{"two", "one"}, 1)
	if err != nil || idempotent.Revision != 2 {
		t.Fatalf("idempotent replace = %+v error=%v", idempotent, err)
	}
	if _, replaceErr := assignments.Replace("MrBones", tasks.RunIDCountess, []string{"one"}, 1); replaceErr == nil || !strings.Contains(replaceErr.Error(), "revision_conflict") {
		t.Fatalf("revision conflict error = %v", replaceErr)
	}
	if _, replaceErr := assignments.Replace("MrBones", tasks.RunIDCountess, []string{"one", "one"}, 2); replaceErr == nil || !strings.Contains(replaceErr.Error(), "duplicated") {
		t.Fatalf("duplicate assignment error = %v", replaceErr)
	}
	if _, replaceErr := assignments.Replace("MrBones", tasks.RunIDCountess, []string{"missing"}, 2); replaceErr == nil || !strings.Contains(replaceErr.Error(), "unavailable") {
		t.Fatalf("unknown profile error = %v", replaceErr)
	}

	before, err := assignments.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assignments.write = func(string, []byte, string) error { return errors.New("injected assignment write failure") }
	if _, replaceErr := assignments.Replace("MrBones", tasks.RunIDCountess, []string{"one"}, 2); replaceErr == nil {
		t.Fatal("expected assignment write failure")
	}
	after, err := assignments.Snapshot()
	if err != nil || after.Revision != before.Revision || !equalStrings(findPickitAssignment(after, "MrBones", tasks.RunIDCountess), findPickitAssignment(before, "MrBones", tasks.RunIDCountess)) {
		t.Fatalf("assignment changed after failed write: before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestMigratedPickitProfilesReproduceCountessAndMephistoPolicies(t *testing.T) {
	profiles, err := NewPickitProfileService(filepath.Join("..", "..", "configs", "pickit", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	allProfiles, err := profiles.List()
	if err != nil || len(allProfiles) != 4 {
		t.Fatalf("initial profiles = %d error=%v", len(allProfiles), err)
	}
	assignments, err := NewPickitAssignmentStore(filepath.Join(t.TempDir(), "assignments.yaml"), profiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, initErr := assignments.Initialize(map[string]map[tasks.RunID][]string{"MrBones": {
		tasks.RunIDCountess: {"gems", "keys", "countess-standard"},
		tasks.RunIDMephisto: {"gems", "mephisto-standard"},
	}}); initErr != nil {
		t.Fatal(initErr)
	}
	countess, err := assignments.Resolve("mrbones", tasks.RunIDCountess)
	if err != nil {
		t.Fatal(err)
	}
	mephisto, err := assignments.Resolve("MrBones", tasks.RunIDMephisto)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name                 string
		item                 world.Item
		countess, meph, sell bool
	}{
		{name: "rune", item: world.Item{Code: "r33", Type: "rune"}, countess: true},
		{name: "key", item: world.Item{Code: "pk1"}, countess: true},
		{name: "rejuvenation", item: world.Item{Type: "rpot"}, countess: true},
		{name: "flawless gem", item: world.Item{Code: "gzv"}, countess: true, meph: true},
		{name: "set exceptional", item: world.Item{Quality: world.ItemQualitySet, BaseTier: world.BaseTierExceptional}, meph: true, sell: true},
		{name: "unique elite", item: world.Item{Quality: world.ItemQualityUnique, BaseTier: world.BaseTierElite}, meph: true, sell: true},
		{name: "set normal", item: world.Item{Quality: world.ItemQualitySet, BaseTier: world.BaseTierNormal}},
		{name: "rare elite", item: world.Item{Quality: world.ItemQualityRare, BaseTier: world.BaseTierElite}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := countess.All.Evaluate(test.item).Matched; got != test.countess {
				t.Fatalf("countess=%t want=%t", got, test.countess)
			}
			if got := mephisto.All.Evaluate(test.item).Matched; got != test.meph {
				t.Fatalf("mephisto=%t want=%t", got, test.meph)
			}
			result := mephisto.All.Evaluate(test.item)
			if got := result.Matched && result.Action == loot.ActionSell; got != test.sell {
				t.Fatalf("sell=%t want=%t", got, test.sell)
			}
		})
	}

}

func TestRepositoryPickitAssignmentExampleReferencesValidProfiles(t *testing.T) {
	profiles, err := NewPickitProfileService(filepath.Join("..", "..", "configs", "pickit", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := NewPickitAssignmentStore(filepath.Join("..", "..", "configs", "pickit-assignments.example.yaml"), profiles)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := assignments.Snapshot()
	if err != nil || manifest.Revision != 1 {
		t.Fatalf("example manifest = %+v error=%v", manifest, err)
	}
	for _, runID := range []tasks.RunID{tasks.RunIDCountess, tasks.RunIDMephisto} {
		if _, err := assignments.Resolve("mrbones", runID); err != nil {
			t.Fatalf("resolve example %s: %v", runID, err)
		}
	}
}

func TestPickitProfileStrictDecodeRejectsUnknownFields(t *testing.T) {
	root := filepath.Join(t.TempDir(), "profiles")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad.yaml"), []byte("schema_version: 1\nrevision: 1\nid: bad\nname: Bad\nunknown: true\nrules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	profiles, err := NewPickitProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profiles.List(); err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("strict decode error = %v", err)
	}
}

func newTestPickitStores(t *testing.T) (*PickitProfileService, *PickitAssignmentStore) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "profiles")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	profiles, err := NewPickitProfileService(root)
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := NewPickitAssignmentStore(filepath.Join(t.TempDir(), "assignments.yaml"), profiles)
	if err != nil {
		t.Fatal(err)
	}
	return profiles, assignments
}

func testPickitProfile(id, name, ruleID string, action loot.Action, expression string) PickitProfileDocument {
	return PickitProfileDocument{
		SchemaVersion: PickitProfileSchemaVersion, Revision: 1, ID: id, Name: name,
		Rules: []PickitProfileRuleDocument{{ID: ruleID, Action: action, Expression: expression}},
	}
}

func rootCause(err error) error {
	for err != nil {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
	return nil
}

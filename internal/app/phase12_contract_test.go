package app

import (
	"os"
	"reflect"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"gopkg.in/yaml.v3"
)

func TestPhase12SchemaFixturesValidate(t *testing.T) {
	assignmentData, err := os.ReadFile("testdata/phase12/route-assignments.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var assignments RouteAssignmentManifest
	if decodeErr := yaml.Unmarshal(assignmentData, &assignments); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if validationErr := assignments.Validate(); validationErr != nil {
		t.Fatalf("assignment fixture: %v", validationErr)
	}

	candidateData, err := os.ReadFile("testdata/phase12/route-candidate.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var candidate RouteCandidate
	if decodeErr := yaml.Unmarshal(candidateData, &candidate); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if validationErr := candidate.Validate(); validationErr != nil {
		t.Fatalf("candidate fixture: %v", validationErr)
	}
}

func TestPhase12WorkflowAndLockContractsAreComplete(t *testing.T) {
	transitions := RouteWorkflowTransitions()
	if len(transitions) != 14 {
		t.Fatalf("workflow transitions = %d, want 14", len(transitions))
	}
	if transitions[0] != (RouteWorkflowTransition{From: RouteWorkflowIdle, To: RouteWorkflowPreflight}) || transitions[len(transitions)-1] != (RouteWorkflowTransition{From: RouteWorkflowRecording, To: RouteWorkflowEmergencyCancelled}) {
		t.Fatalf("workflow boundaries = first %+v last %+v", transitions[0], transitions[len(transitions)-1])
	}
	wantLocks := []RouteLock{RouteLockWorkflow, RouteLockCatalog, RouteLockLifecycle, RouteLockAssignment, RouteLockCandidate, RouteLockJournal}
	if got := RouteLockOrder(); !reflect.DeepEqual(got, wantLocks) {
		t.Fatalf("lock order = %v, want %v", got, wantLocks)
	}
	seen := map[RouteMutationOperation]bool{}
	for _, checkpoint := range RouteCrashMatrix() {
		if checkpoint.Checkpoint == "" || checkpoint.Recovery == "" {
			t.Fatalf("incomplete crash contract = %+v", checkpoint)
		}
		seen[checkpoint.Operation] = true
	}
	for _, operation := range []RouteMutationOperation{RouteMutationPublish, RouteMutationReplace, RouteMutationArchive, RouteMutationRestore, RouteMutationDelete} {
		if !seen[operation] {
			t.Fatalf("missing crash contract for %s", operation)
		}
	}
	owners := RouteContractOwners()
	if len(owners) != 9 {
		t.Fatalf("contract owners = %d, want 9", len(owners))
	}
	for _, owner := range owners {
		if owner.Contract == "" || owner.Owner == "" {
			t.Fatalf("unnamed contract owner = %+v", owner)
		}
	}
}

func TestPhase12SchemaContractsRejectUnsafeValues(t *testing.T) {
	if err := (RouteAssignmentManifest{SchemaVersion: 1, Revision: 1, Assignments: map[string]map[tasks.RunID]string{"MrBones": {tasks.RunIDCountess: "route"}}}).Validate(); err == nil {
		t.Fatal("mixed-case character slug accepted")
	}
	if err := (RouteCandidate{SchemaVersion: 1}).Validate(); err == nil {
		t.Fatal("incomplete candidate accepted")
	}
}

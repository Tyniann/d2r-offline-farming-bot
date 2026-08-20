package tasks

import (
	"reflect"
	"testing"
)

func TestSharedRunStepContractIsFiniteAndConnected(t *testing.T) {
	contracts := SharedRunStepContracts()
	if len(contracts) == 0 || contracts[0].Step != RunStepResolveDefinition || contracts[len(contracts)-1].Step != RunStepComplete {
		t.Fatalf("shared contracts have invalid boundaries: %+v", contracts)
	}

	known := make(map[RunStep]bool, len(contracts))
	for _, contract := range contracts {
		if contract.Step == "" || known[contract.Step] {
			t.Fatalf("duplicate or empty run step %q", contract.Step)
		}
		known[contract.Step] = true
	}
	for _, contract := range contracts {
		for _, next := range contract.AllowedNext {
			if !known[next] {
				t.Fatalf("step %q references unknown successor %q", contract.Step, next)
			}
		}
		if contract.Step != RunStepComplete && len(contract.AllowedNext) == 0 {
			t.Fatalf("non-terminal step %q has no successor", contract.Step)
		}
	}
	if len(contracts[len(contracts)-1].AllowedNext) != 0 {
		t.Fatalf("complete step must be terminal: %+v", contracts[len(contracts)-1])
	}
}

func TestSharedRunContractReturnsDefensiveCopies(t *testing.T) {
	contracts := SharedRunStepContracts()
	contracts[0].AllowedNext[0] = RunStepComplete
	if got := SharedRunStepContracts()[0].AllowedNext[0]; got != RunStepPrecheck {
		t.Fatalf("transition contract was mutated through returned slice: %q", got)
	}

	scopes := RequiredRunResetScopes()
	scopes[0] = RunResetTelemetryBinding
	if got := RequiredRunResetScopes()[0]; got != RunResetBossPin {
		t.Fatalf("reset contract was mutated through returned slice: %q", got)
	}
}

func TestRequiredRunResetScopesFreezePhase10Barrier(t *testing.T) {
	want := []RunResetScope{
		RunResetBossPin,
		RunResetEncounterAction,
		RunResetRoutePlayback,
		RunResetProfileExecutor,
		RunResetLootExecutor,
		RunResetTownExecutor,
		RunResetTelemetryBinding,
	}
	if got := RequiredRunResetScopes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("reset scopes = %v, want %v", got, want)
	}
}

func TestPhase10ReasonCodesAreUnique(t *testing.T) {
	reasons := []RunReason{
		RunReasonUnknown, RunReasonConfigMissing, RunReasonDefinitionInvalid, RunReasonCapabilityMissing,
		RunReasonRouteMissing, RunReasonRouteBindingMismatch, RunReasonRouteLayoutMismatch, RunReasonRouteRuntimeValidation,
		RunReasonProfileClassMismatch, RunReasonProfileRunStrategyUnavailable, RunReasonCharacterProfileRunIncompatible, RunReasonWaypointTargetUnsupported, RunReasonWaypointUIUnconfirmed,
		RunReasonWaypointDestinationTimeout, RunReasonUnexpectedArea, RunReasonChestSweepEmpty, RunReasonBossNotFound, RunReasonBossPinLost,
		RunReasonEncounterActionFailed, RunReasonBossKillUnconfirmed, RunReasonLootPolicyInvalid, RunReasonItemTierUnknown,
		RunReasonItemClassificationConflict, RunReasonItemIdentifyFailed, RunReasonItemSellFailed, RunReasonTownEgressMissing,
		RunReasonTownEgressBindingMismatch, RunReasonHubTransferUnsupported, RunReasonTownServiceVerifyTimeout,
	}
	seen := make(map[RunReason]bool, len(reasons))
	for _, reason := range reasons {
		if reason == "" || seen[reason] {
			t.Fatalf("duplicate or empty Phase-10 reason %q", reason)
		}
		seen[reason] = true
	}
}

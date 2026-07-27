package app

import "testing"

func TestPhase15OwnershipDataRootAndNonGoalsAreComplete(t *testing.T) {
	owners := Phase15ContractOwners()
	if len(owners) != 8 || owners[0].Owner != "internal/app and Go Core" || owners[len(owners)-1].Owner != "Electron Main" {
		t.Fatalf("owners=%+v", owners)
	}
	layout := Phase15DataRootLayout()
	if len(layout) != 5 || layout[0].RelativePath != "configs" || layout[len(layout)-1].RelativePath != "desktop-settings.json" {
		t.Fatalf("layout=%+v", layout)
	}
	if Phase15DataRootDirectoryName != "D2ROfflineFarmingBot" || Phase15DefaultHistoryRetentionDays != 60 || Phase15MaximumConfigBackups != 10 {
		t.Fatal("phase-15 product defaults drifted")
	}
	if goals := Phase15NonGoals(); len(goals) != 10 {
		t.Fatalf("non-goals=%+v", goals)
	}
}

func TestPhase15DesktopActiveStatesMatchTrayContract(t *testing.T) {
	states := Phase15DesktopActiveStates()
	want := []SupervisorState{SupervisorStateStartingGame, SupervisorStateStartingRun, SupervisorStateRunningRun, SupervisorStatePausedBetweenRuns, SupervisorStateExitingGame, SupervisorStateCancelling}
	if len(states) != len(want) {
		t.Fatalf("states=%+v", states)
	}
	for index := range want {
		if states[index] != want[index] {
			t.Fatalf("state[%d]=%q want=%q", index, states[index], want[index])
		}
	}
}

func TestPhase15ReasonCodesAreUniqueAndGrouped(t *testing.T) {
	groups := Phase15ReasonGroups()
	if len(groups) != 6 {
		t.Fatalf("groups=%+v", groups)
	}
	seen := make(map[Phase15ReasonCode]string)
	for _, group := range groups {
		if group.Area == "" || len(group.Codes) == 0 {
			t.Fatalf("incomplete group=%+v", group)
		}
		for _, code := range group.Codes {
			if code == "" {
				t.Fatalf("empty code in %s", group.Area)
			}
			if previous, duplicate := seen[code]; duplicate {
				t.Fatalf("code %q appears in %s and %s", code, previous, group.Area)
			}
			seen[code] = group.Area
		}
	}
	if len(seen) != 34 {
		t.Fatalf("reason code count=%d want=34", len(seen))
	}
}

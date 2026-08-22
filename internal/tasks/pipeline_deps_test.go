package tasks

import (
	"reflect"
	"testing"
)

func TestPipelineDependencyViewsStayNarrow(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		fields []string
	}{
		{name: "travel", value: pipelineTravelDeps{}, fields: []string{"Waypoint", "TownWalk", "Combat", "Loot", "Route", "RouteClear", "Profile", "Telemetry", "Chest"}},
		{name: "chest", value: pipelineChestDeps{}, fields: []string{"Chest", "Combat", "Loot", "Route", "RouteClear", "Telemetry"}},
		{name: "boss", value: pipelineBossDeps{}, fields: []string{"Pathing", "Combat", "RouteClear", "Profile", "Telemetry"}},
		{name: "loot", value: pipelineLootDeps{}, fields: []string{"Combat", "Loot"}},
		{name: "return", value: pipelineReturnDeps{}, fields: []string{"Waypoint", "Portal", "Stash", "Combat", "Actions", "Loot", "RouteClear", "TownEgress", "Town", "Telemetry"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			typ := reflect.TypeOf(test.value)
			got := make([]string, typ.NumField())
			for i := range typ.NumField() {
				got[i] = typ.Field(i).Name
			}
			if !reflect.DeepEqual(got, test.fields) {
				t.Fatalf("dependency fields = %v, want %v", got, test.fields)
			}
		})
	}
}

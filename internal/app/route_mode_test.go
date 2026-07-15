package app

import "testing"

func TestParseRouteCommand(t *testing.T) {
	tests := map[string]routeCommand{
		"list":                                {action: "list"},
		"inspect:test-route":                  {action: "inspect", id: "test-route"},
		"validate:test-route":                 {action: "validate", id: "test-route"},
		"record:test-route":                   {action: "record", id: "test-route"},
		"play-segment:test-route/black-marsh": {action: "play-segment", id: "test-route", segmentID: "black-marsh"},
		"play:test-route":                     {action: "play", id: "test-route"},
		"inspect-egress:act3":                 {action: "inspect-egress", id: "act3"},
		"record-egress:act3":                  {action: "record-egress", id: "act3"},
		"validate-egress:act3":                {action: "validate-egress", id: "act3"},
		"play-egress:act3":                    {action: "play-egress", id: "act3"},
	}
	for raw, want := range tests {
		got, err := parseRouteCommand(raw)
		if err != nil || got != want {
			t.Fatalf("parseRouteCommand(%q) = %+v, %v; want %+v", raw, got, err, want)
		}
	}
}

func TestParseRouteCommandRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"", "inspect", "inspect:", "play:", "play-segment:test", "list:any", "play-egress:act1"} {
		if _, err := parseRouteCommand(raw); err == nil {
			t.Fatalf("parseRouteCommand(%q) expected error", raw)
		}
	}
}

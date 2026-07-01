package app

import (
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestParseInputTestSpecSingleActions(t *testing.T) {
	cases := []struct {
		spec string
		want []inputTestAction
	}{
		{"belt:1", []inputTestAction{{kind: inputTestBelt, slot: 1}}},
		{"potion:4", []inputTestAction{{kind: inputTestBelt, slot: 4}}},
		{"portal", []inputTestAction{{kind: inputTestPortal}}},
		{"skill:teleport", []inputTestAction{{kind: inputTestSkill, skillID: memory.SkillTeleport}}},
		{"skill:town_portal", []inputTestAction{{kind: inputTestSkill, skillID: memory.SkillTownPortal}}},
		{"center-click", []inputTestAction{{kind: inputTestCenterClick}}},
		{"click:10,20", []inputTestAction{{kind: inputTestClick, x: 10, y: 20}}},
		{"click:10, 20", []inputTestAction{{kind: inputTestClick, x: 10, y: 20}}},
		{"click:-5,-10", []inputTestAction{{kind: inputTestClick, x: -5, y: -10}}},
	}

	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			got, err := parseInputTestSpec(tc.spec)
			if err != nil {
				t.Fatalf("parseInputTestSpec() err = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("action[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseInputTestSpecSequence(t *testing.T) {
	got, err := parseInputTestSpec("belt:1,portal,skill:teleport")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].kind != inputTestBelt || got[0].slot != 1 {
		t.Fatalf("first action = %+v", got[0])
	}
	if got[1].kind != inputTestPortal {
		t.Fatalf("second action = %+v", got[1])
	}
	if got[2].kind != inputTestSkill || got[2].skillID != memory.SkillTeleport {
		t.Fatalf("third action = %+v", got[2])
	}
}

func TestParseInputTestSpecErrors(t *testing.T) {
	cases := []string{
		"",
		"unknown",
		"belt:0",
		"belt:5",
		"skill:0",
		"skill:9",
		"skill:unknown",
		"click:bad,20",
		"click:10",
		"belt:1,,portal",
		"portal:extra",
		"center-click:1",
	}

	for _, spec := range cases {
		t.Run(spec, func(t *testing.T) {
			_, err := parseInputTestSpec(spec)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseInputTestSpecSequenceWithClick(t *testing.T) {
	got, err := parseInputTestSpec("belt:1,click:10,20,portal")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[1].kind != inputTestClick || got[1].x != 10 || got[1].y != 20 {
		t.Fatalf("click action = %+v", got[1])
	}
}

func TestParseInputTestSpecUnknownShowsExamples(t *testing.T) {
	_, err := parseInputTestSpec("foo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "belt:1") {
		t.Fatalf("error = %q, want allowed examples", err.Error())
	}
}

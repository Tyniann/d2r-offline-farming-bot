package world

import "testing"

func TestParseAreaSpec(t *testing.T) {
	cases := []struct {
		spec string
		want AreaID
	}{
		{"black_marsh", BlackMarsh},
		{"Black Marsh", BlackMarsh},
		{"BLACK-MARSH", BlackMarsh},
		{"6", BlackMarsh},
		{"forgotten_tower", ForgottenTower},
		{"2", BloodMoor},
	}
	for _, tc := range cases {
		got, err := ParseAreaSpec(tc.spec)
		if err != nil {
			t.Fatalf("ParseAreaSpec(%q) error = %v", tc.spec, err)
		}
		if got != tc.want {
			t.Fatalf("ParseAreaSpec(%q) = %d, want %d", tc.spec, got, tc.want)
		}
	}
}

func TestParseAreaSpecInvalid(t *testing.T) {
	for _, spec := range []string{"", "not_an_area", "0", "99999"} {
		if _, err := ParseAreaSpec(spec); err == nil {
			t.Fatalf("ParseAreaSpec(%q) expected error", spec)
		}
	}
}

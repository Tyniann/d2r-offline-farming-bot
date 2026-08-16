package memory

import "testing"

func TestConfigureWeaponSetSkillEvidenceRejectsInvalidContract(t *testing.T) {
	probe := &ProbeReader{}
	for _, pair := range [][2]uint16{{0, 1}, {1, 0}, {1, 1}} {
		if err := probe.ConfigureWeaponSetSkillEvidence(pair[0], pair[1]); err == nil {
			t.Fatalf("ConfigureWeaponSetSkillEvidence(%d, %d) should fail", pair[0], pair[1])
		}
	}
}

func TestReadActiveWeaponSetFromCompleteSkillEvidence(t *testing.T) {
	probe := &ProbeReader{}
	if err := probe.ConfigureWeaponSetSkillEvidence(149, 155); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		skills PlayerSkills
		want   WeaponSetSnapshot
	}{
		{name: "primary", skills: PlayerSkills{Complete: true, SkillsKnown: map[uint16]bool{}}, want: WeaponSetSnapshot{Value: 0, Available: true}},
		{name: "secondary", skills: PlayerSkills{Complete: true, SkillsKnown: map[uint16]bool{149: true, 155: true}}, want: WeaponSetSnapshot{Value: 1, Available: true}},
		{name: "partial pair", skills: PlayerSkills{Complete: true, SkillsKnown: map[uint16]bool{149: true}}},
		{name: "incomplete", skills: PlayerSkills{Complete: false, SkillsKnown: map[uint16]bool{149: true, 155: true}}},
		{name: "missing map", skills: PlayerSkills{Complete: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := probe.readActiveWeaponSetFromSkills(test.skills); got != test.want {
				t.Fatalf("readActiveWeaponSetFromSkills() = %+v, want %+v", got, test.want)
			}
		})
	}
}

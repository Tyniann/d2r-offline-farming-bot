package world

import "testing"

func TestMercenaryHPPercent(t *testing.T) {
	tests := []struct {
		name string
		merc Mercenary
		want uint8
	}{
		{name: "unknown", merc: Mercenary{}, want: 0},
		{name: "unknown vitals", merc: Mercenary{Alive: true, MaxHP: 90}, want: 0},
		{name: "zero max", merc: Mercenary{Alive: true, VitalsKnown: true}, want: 0},
		{name: "zero", merc: Mercenary{Alive: true, VitalsKnown: true, MaxHP: 90}, want: 0},
		{name: "below half", merc: Mercenary{Alive: true, VitalsKnown: true, HP: 44, MaxHP: 90}, want: 48},
		{name: "exact half", merc: Mercenary{Alive: true, VitalsKnown: true, HP: 45, MaxHP: 90}, want: 50},
		{name: "above half", merc: Mercenary{Alive: true, VitalsKnown: true, HP: 46, MaxHP: 90}, want: 51},
		{name: "full", merc: Mercenary{Alive: true, VitalsKnown: true, HP: 90, MaxHP: 90}, want: 100},
		{name: "clamp", merc: Mercenary{Alive: true, VitalsKnown: true, HP: 100, MaxHP: 90}, want: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.merc.HPPercent(); got != tt.want {
				t.Fatalf("HPPercent() = %d, want %d", got, tt.want)
			}
		})
	}
}

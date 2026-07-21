package loot

// Action beschreibt die einzige fachliche Folge einer gematchten Pickit-Regel.
type Action string

const (
	// ActionKeep hebt ein Item auf und behält es nach allen bestehenden Sicherheitsprüfungen.
	ActionKeep Action = "keep"
	// ActionSell hebt ein Item auf, identifiziert es bei Bedarf und verkauft es nach erneuter Prüfung.
	ActionSell Action = "sell"
)

// Valid meldet, ob die Aktion zum verbindlichen Phase-13-Vertrag gehört.
func (a Action) Valid() bool {
	return a == ActionKeep || a == ActionSell
}

package world

// MonsterCoverage ist die semantische World-Projektion der read-only
// Monster-Reservoir-Vollständigkeit.
type MonsterCoverage struct {
	// EligibleMonsterCount zählt alle vollständig gelesenen lebenden Runtime-Kandidaten.
	EligibleMonsterCount int
	// MonstersTruncated meldet einen Verlust nicht priorisierter Kandidaten am 512er Reservoir.
	MonstersTruncated bool
	// MonsterCoverageRadiusTiles begrenzt bei Truncation den garantiert vollständigen Nahbereich.
	MonsterCoverageRadiusTiles float64
}

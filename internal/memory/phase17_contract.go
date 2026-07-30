package memory

const (
	// Phase17MaxRuntimeMonsters begrenzt ausschließlich das nearest-to-player Reservoir
	// nicht priorisierter lebender Runtime-Monster. Priority-Einheiten liegen außerhalb.
	Phase17MaxRuntimeMonsters = 512
	maxRuntimeMonsters        = Phase17MaxRuntimeMonsters
)

// MonsterCoverage beschreibt die Vollständigkeit des read-only Monster-Reservoirs.
type MonsterCoverage struct {
	// EligibleMonsterCount zählt alle vollständig gelesenen lebenden Kandidaten vor der Reservoirentscheidung.
	EligibleMonsterCount int
	// MonstersTruncated meldet, dass mehr als [Phase17MaxRuntimeMonsters] nicht priorisierte Kandidaten konkurrierten.
	MonstersTruncated bool
	// MonsterCoverageRadiusTiles ist bei Truncation die Distanz des am weitesten entfernten retained Kandidaten.
	MonsterCoverageRadiusTiles float64
}

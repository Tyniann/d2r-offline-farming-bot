package world

import "math"

// NearestObject returns the nearest object of kind to the player position, or false when none match.
func (s State) NearestObject(kind ObjectKind) (Object, bool) {
	if !s.Valid {
		return Object{}, false
	}
	var best Object
	var bestDist float64
	found := false
	for _, o := range s.Objects {
		if o.Kind != kind {
			continue
		}
		d := distanceSquared(s.Player.Position, o.Position)
		if !found || d < bestDist {
			best = o
			bestDist = d
			found = true
		}
	}
	return best, found
}

// NearestEntrance returns the nearest entrance of kind to the player position, or false when none match.
func (s State) NearestEntrance(kind EntranceKind) (Entrance, bool) {
	if !s.Valid {
		return Entrance{}, false
	}
	var best Entrance
	var bestDist float64
	found := false
	for _, e := range s.Entrances {
		if e.Kind != kind {
			continue
		}
		d := distanceSquared(s.Player.Position, e.Position)
		if !found || d < bestDist {
			best = e
			bestDist = d
			found = true
		}
	}
	return best, found
}

// FindSuperUnique returns the nearest living monster with SuperUnique flag (10).
// When npcID is 0, any super-unique matches (used for The Countess regardless of base NPC type).
func (s State) FindSuperUnique(npcID uint32) (Monster, bool) {
	if !s.Valid {
		return Monster{}, false
	}
	var best Monster
	var bestDist float64
	found := false
	for _, m := range s.Monsters {
		if m.MonsterTypeFlag != SuperUniqueMonsterFlag {
			continue
		}
		if npcID != 0 && m.NPCID != npcID {
			continue
		}
		d := distanceSquared(s.Player.Position, m.Position)
		if !found || d < bestDist {
			best = m
			bestDist = d
			found = true
		}
	}
	return best, found
}

func distanceSquared(a, b Position) float64 {
	dx := float64(a.X) - float64(b.X)
	dy := float64(a.Y) - float64(b.Y)
	return dx*dx + dy*dy
}

// Distance returns the Euclidean distance between two positions.
func Distance(a, b Position) float64 {
	return math.Sqrt(distanceSquared(a, b))
}

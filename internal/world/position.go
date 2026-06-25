package world

// Position holds raw world tile coordinates matching memory.Snapshot PosX/PosY.
// No area-origin or pathing normalization is applied in Phase 2.1.
type Position struct {
	X uint32 // Horizontal tile coordinate.
	Y uint32 // Vertical tile coordinate.
}

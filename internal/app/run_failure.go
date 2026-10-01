package app

func isRestartableSessionFailure(reason string, allowed []string) bool {
	// A new game cannot clear shared material capacity or an unconfirmed Cube.
	// Even a custom retry list must not restart these terminal storage failures.
	if isStorageCompactionFailure(reason) {
		return false
	}
	for _, candidate := range allowed {
		if reason == candidate {
			return true
		}
	}
	return false
}

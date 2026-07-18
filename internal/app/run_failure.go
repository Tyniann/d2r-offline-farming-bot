package app

func isRestartableSessionFailure(reason string, allowed []string) bool {
	for _, candidate := range allowed {
		if reason == candidate {
			return true
		}
	}
	return false
}

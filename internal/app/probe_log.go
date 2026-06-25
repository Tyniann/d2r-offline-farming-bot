package app

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

const probeHeartbeat = 5 * time.Second

// probeLogIsHeartbeat reports whether an invalid probe log should use Debug (heartbeat) vs Info.
func probeLogIsHeartbeat(lastLog time.Time, heartbeat time.Duration) bool {
	if heartbeat <= 0 {
		heartbeat = probeHeartbeat
	}
	return !lastLog.IsZero() && time.Since(lastLog) >= heartbeat
}

// isPositionOnlyProbeChange reports a valid snapshot change limited to PosX/PosY.
func isPositionOnlyProbeChange(prev, cur memory.Snapshot) bool {
	if !prev.Valid || !cur.Valid {
		return false
	}
	return prev.HP == cur.HP &&
		prev.MaxHP == cur.MaxHP &&
		prev.Mana == cur.Mana &&
		prev.MaxMana == cur.MaxMana &&
		prev.AreaID == cur.AreaID &&
		prev.StatsSource == cur.StatsSource &&
		(prev.PosX != cur.PosX || prev.PosY != cur.PosY)
}

// probeShouldLog reports whether a probe snapshot should be emitted to operator logs.
// force is true immediately after attach/re-attach to ensure one log even when values match.
// verbose allows position-only changes to log on Debug (see logProbeSnapshot).
func probeShouldLog(prev, cur memory.Snapshot, lastLog time.Time, heartbeat time.Duration, force, verbose bool) bool {
	if force {
		return true
	}
	if heartbeat <= 0 {
		heartbeat = probeHeartbeat
	}
	if prev.Valid != cur.Valid {
		return true
	}
	if !cur.Valid {
		if prev.Reason != cur.Reason {
			return true
		}
		return time.Since(lastLog) >= heartbeat
	}
	if prev.HP != cur.HP ||
		prev.MaxHP != cur.MaxHP ||
		prev.Mana != cur.Mana ||
		prev.MaxMana != cur.MaxMana ||
		prev.AreaID != cur.AreaID ||
		prev.StatsSource != cur.StatsSource {
		return true
	}
	if verbose && isPositionOnlyProbeChange(prev, cur) {
		return true
	}
	if time.Since(lastLog) >= heartbeat {
		if isPositionOnlyProbeChange(prev, cur) {
			return verbose
		}
		return true
	}
	return false
}

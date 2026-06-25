package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	worldHeartbeat              = 5 * time.Second
	worldResetReasonProcessLost = "process_lost"
)

// worldLogIsHeartbeat reports whether an invalid world log should use Debug (heartbeat) vs Info.
func worldLogIsHeartbeat(lastLog time.Time, heartbeat time.Duration) bool {
	if heartbeat <= 0 {
		heartbeat = worldHeartbeat
	}
	return !lastLog.IsZero() && time.Since(lastLog) >= heartbeat
}

// isPositionOnlyWorldChange reports a valid state change limited to player position.
func isPositionOnlyWorldChange(prev, cur world.State) bool {
	if !prev.Valid || !cur.Valid {
		return false
	}
	return prev.Phase == cur.Phase &&
		prev.Area.ID == cur.Area.ID &&
		prev.Player.HP == cur.Player.HP &&
		prev.Player.MaxHP == cur.Player.MaxHP &&
		prev.Player.Mana == cur.Player.Mana &&
		prev.Player.MaxMana == cur.Player.MaxMana &&
		(prev.Player.Position.X != cur.Player.Position.X || prev.Player.Position.Y != cur.Player.Position.Y)
}

// worldShouldLog reports whether a world state should be emitted to operator logs.
// force is true immediately after attach/re-attach to ensure one log even when values match.
// verbose allows position-only changes to log on Debug (see logWorldState).
// State.At is intentionally ignored because every tick gets a new timestamp.
func worldShouldLog(prev, cur world.State, lastLog time.Time, heartbeat time.Duration, force, verbose bool) bool {
	if force {
		return true
	}
	if heartbeat <= 0 {
		heartbeat = worldHeartbeat
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
	if prev.Phase != cur.Phase ||
		prev.Area.ID != cur.Area.ID ||
		prev.Player.HP != cur.Player.HP ||
		prev.Player.MaxHP != cur.Player.MaxHP ||
		prev.Player.Mana != cur.Player.Mana ||
		prev.Player.MaxMana != cur.Player.MaxMana {
		return true
	}
	if verbose && isPositionOnlyWorldChange(prev, cur) {
		return true
	}
	if time.Since(lastLog) >= heartbeat {
		if isPositionOnlyWorldChange(prev, cur) {
			return verbose
		}
		return true
	}
	return false
}

// worldLogAttrs builds structured log attributes for a world state.
func worldLogAttrs(cur world.State) []slog.Attr {
	if !cur.Valid {
		return []slog.Attr{slog.String("reason", cur.Reason)}
	}
	return []slog.Attr{
		slog.String("phase", cur.Phase.String()),
		slog.String("area_name", cur.Area.Name),
		slog.Uint64("area_id", uint64(cur.Area.ID)),
		slog.String("act", cur.Area.Act.String()),
		slog.String("area_kind", cur.Area.Kind.String()),
		slog.Uint64("hp", uint64(cur.Player.HP)),
		slog.Uint64("max_hp", uint64(cur.Player.MaxHP)),
		slog.Uint64("hp_pct", uint64(cur.Player.HPPercent())),
		slog.Uint64("mana", uint64(cur.Player.Mana)),
		slog.Uint64("max_mana", uint64(cur.Player.MaxMana)),
		slog.Uint64("mana_pct", uint64(cur.Player.ManaPercent())),
		slog.Uint64("pos_x", uint64(cur.Player.Position.X)),
		slog.Uint64("pos_y", uint64(cur.Player.Position.Y)),
	}
}

func (rt *Runtime) logWorldState(prev, cur world.State, heartbeat, verbose bool) {
	if cur.Valid {
		level := slog.LevelInfo
		if verbose && isPositionOnlyWorldChange(prev, cur) {
			level = slog.LevelDebug
		}
		rt.Log.Log(context.Background(), level, "world state", attrsToArgs(worldLogAttrs(cur))...)
		return
	}

	if heartbeat {
		rt.Log.Debug("world unavailable", "reason", cur.Reason)
		return
	}
	rt.Log.Info("world unavailable", "reason", cur.Reason)
}

func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, 0, len(attrs)*2)
	for _, a := range attrs {
		args = append(args, a.Key, a.Value.Any())
	}
	return args
}

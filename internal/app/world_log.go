package app

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
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

// isPositionOnlyWorldChange reports a valid state change limited to player position and unchanged entity fingerprint.
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
		prev.Mercenary == cur.Mercenary &&
		prev.UI == cur.UI &&
		entityFingerprint(prev) == entityFingerprint(cur) &&
		(prev.Player.Position.X != cur.Player.Position.X || prev.Player.Position.Y != cur.Player.Position.Y)
}

type entityFPKey struct {
	kind   string
	unitID uint32
}

// entityFingerprint builds a stable signature from entity counts and sorted (kind, unitID) pairs.
func entityFingerprint(s world.State) string {
	keys := make([]entityFPKey, 0, len(s.Objects)+len(s.Entrances)+len(s.Monsters)+len(s.Items))
	for _, o := range s.Objects {
		keys = append(keys, entityFPKey{kind: "o:" + o.Kind.String(), unitID: o.UnitID})
	}
	for _, e := range s.Entrances {
		keys = append(keys, entityFPKey{kind: "e:" + e.Kind.String(), unitID: e.UnitID})
	}
	for _, m := range s.Monsters {
		keys = append(keys, entityFPKey{kind: "m:" + fmt.Sprintf("%d", m.NPCID), unitID: m.UnitID})
	}
	for _, i := range s.Items {
		if i.Location != world.ItemLocationGround {
			continue
		}
		keys = append(keys, entityFPKey{kind: fmt.Sprintf("i:%d:%s", i.TxtFileNo, i.Location.String()), unitID: i.UnitID})
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].kind != keys[j].kind {
			return keys[i].kind < keys[j].kind
		}
		return keys[i].unitID < keys[j].unitID
	})

	var b strings.Builder
	fmt.Fprintf(&b, "%d|%d|%d|%d|", len(s.Objects), len(s.Entrances), len(s.Monsters), len(s.GroundItems()))
	for _, k := range keys {
		fmt.Fprintf(&b, "%s:%d;", k.kind, k.unitID)
	}
	return b.String()
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
		if prev.Reason != cur.Reason || prev.Phase != cur.Phase {
			return true
		}
		return time.Since(lastLog) >= heartbeat
	}
	if prev.Phase != cur.Phase ||
		prev.Area.ID != cur.Area.ID ||
		prev.Player.HP != cur.Player.HP ||
		prev.Player.MaxHP != cur.Player.MaxHP ||
		prev.Player.Mana != cur.Player.Mana ||
		prev.Player.MaxMana != cur.Player.MaxMana ||
		prev.Mercenary != cur.Mercenary {
		return true
	}
	if prev.UI != cur.UI {
		return true
	}
	if entityFingerprint(prev) != entityFingerprint(cur) {
		return true
	}
	if prev.Hover != cur.Hover {
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
func worldLogAttrs(cur world.State, verbose bool) []slog.Attr {
	if !cur.Valid {
		attrs := []slog.Attr{slog.String("reason", cur.Reason)}
		if cur.Phase != world.GamePhaseUnknown {
			attrs = append(attrs, slog.String("phase", cur.Phase.String()))
		}
		return attrs
	}
	attrs := []slog.Attr{
		slog.String("phase", cur.Phase.String()),
		slog.String("area_name", cur.Area.Name),
		slog.Uint64("area_id", uint64(cur.Area.ID)),
		slog.String("act", cur.Area.Act.String()),
		slog.String("area_kind", cur.Area.Kind.String()),
		slog.Uint64("object_count", uint64(len(cur.Objects))),
		slog.Uint64("entrance_count", uint64(len(cur.Entrances))),
		slog.Uint64("monster_count", uint64(len(cur.Monsters))),
		slog.Uint64("item_count", uint64(len(cur.Items))),
		slog.Uint64("ground_item_count", uint64(len(cur.GroundItems()))),
		slog.Uint64("hp", uint64(cur.Player.HP)),
		slog.Uint64("max_hp", uint64(cur.Player.MaxHP)),
		slog.Uint64("hp_pct", uint64(cur.Player.HPPercent())),
		slog.Uint64("mana", uint64(cur.Player.Mana)),
		slog.Uint64("max_mana", uint64(cur.Player.MaxMana)),
		slog.Uint64("mana_pct", uint64(cur.Player.ManaPercent())),
		slog.Bool("mercenary_hired_known", cur.Mercenary.HiredKnown),
		slog.Bool("mercenary_hired", cur.Mercenary.Hired),
		slog.Bool("mercenary_alive", cur.Mercenary.Alive),
		slog.Bool("mercenary_dead", cur.Mercenary.Dead),
		slog.Bool("mercenary_vitals_known", cur.Mercenary.VitalsKnown),
		slog.Uint64("mercenary_unit_id", uint64(cur.Mercenary.UnitID)),
		slog.Uint64("mercenary_npc_id", uint64(cur.Mercenary.NPCID)),
		slog.Uint64("mercenary_hp", uint64(cur.Mercenary.HP)),
		slog.Uint64("mercenary_max_hp", uint64(cur.Mercenary.MaxHP)),
		slog.Uint64("mercenary_hp_pct", uint64(cur.Mercenary.HPPercent())),
		slog.Uint64("left_skill_id", uint64(cur.Player.LeftSkillID)),
		slog.String("left_skill", memory.SkillName(cur.Player.LeftSkillID)),
		slog.Uint64("right_skill_id", uint64(cur.Player.RightSkillID)),
		slog.String("right_skill", memory.SkillName(cur.Player.RightSkillID)),
		slog.Uint64("pos_x", uint64(cur.Player.Position.X)),
		slog.Uint64("pos_y", uint64(cur.Player.Position.Y)),
		slog.Bool("ui_inventory_open", cur.UI.InventoryOpen),
		slog.Bool("ui_npc_interact_open", cur.UI.NPCInteractOpen),
		slog.Bool("ui_npc_shop_open", cur.UI.NPCShopOpen),
		slog.Bool("ui_waypoint_open", cur.UI.WaypointOpen),
		slog.Bool("ui_stash_open", cur.UI.StashOpen),
		slog.Bool("ui_quit_menu_open", cur.UI.QuitMenuOpen),
	}
	if cur.Hover.IsHovered {
		attrs = append(attrs,
			slog.String("hover_type", cur.Hover.UnitType.String()),
			slog.Uint64("hover_unit_id", uint64(cur.Hover.UnitID)),
		)
	}
	if hint := verboseEntityHint(cur); hint != "" {
		attrs = append(attrs, slog.String("entity_hint", hint))
	}
	if hint := verboseEntrancesHint(cur); hint != "" {
		attrs = append(attrs, slog.String("entrances_hint", hint))
	}
	if verbose {
		if hint := verboseGroundItemsHint(cur); hint != "" {
			attrs = append(attrs, slog.String("ground_items_hint", hint))
		}
	}
	return attrs
}

func verboseEntityHint(cur world.State) string {
	if o, ok := cur.NearestObject(world.ObjectKindPersonalStash); ok {
		return fmt.Sprintf("personal_stash id=%d unit=%d x=%d y=%d distance=%.1f", o.ID, o.UnitID, o.Position.X, o.Position.Y, world.Distance(cur.Player.Position, o.Position))
	}
	if o, ok := cur.NearestObject(world.ObjectKindWaypoint); ok {
		return fmt.Sprintf("waypoint id=%d unit=%d", o.ID, o.UnitID)
	}
	if o, ok := cur.NearestObject(world.ObjectKindGoodChest); ok {
		return fmt.Sprintf("good_chest id=%d unit=%d", o.ID, o.UnitID)
	}
	if m, ok := cur.FindSuperUnique(0); ok {
		return fmt.Sprintf("countess npc=%d unit=%d", m.NPCID, m.UnitID)
	}
	if e, ok := cur.NearestEntrance(world.EntranceKindWildernessToTower); ok {
		return fmt.Sprintf("tower_entrance id=%d unit=%d", e.ID, e.UnitID)
	}
	return ""
}

func verboseEntrancesHint(cur world.State) string {
	if len(cur.Entrances) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cur.Entrances))
	for _, e := range cur.Entrances {
		name := e.Name
		if name == "" {
			name = "unknown"
		}
		parts = append(parts, fmt.Sprintf("id=%d unit=%d kind=%s x=%d y=%d name=%q", e.ID, e.UnitID, e.Kind.String(), e.Position.X, e.Position.Y, name))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func verboseGroundItemsHint(cur world.State) string {
	items := hintGroundItems(cur)
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UnitID < items[j].UnitID
	})
	const maxHintItems = 32
	if len(items) > maxHintItems {
		items = items[:maxHintItems]
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		name := item.Name
		if name == "" {
			name = "Unknown Item"
		}
		parts = append(parts, fmt.Sprintf("unit=%d id=%d code=%q type=%q name=%q quality=%s ethereal=%t identity_kind=%q identity_raw_id=%d identity_available=%t identity_name=%q identity_consistent=%t identity_reason=%q x=%d y=%d",
			item.UnitID,
			item.TxtFileNo,
			item.Code,
			item.Type,
			name,
			item.Quality.String(),
			item.Ethereal,
			item.IdentityKind,
			item.IdentityRawID,
			item.IdentityAvailable,
			item.IdentityName,
			item.IdentityValid,
			item.IdentityReason,
			item.Position.X,
			item.Position.Y,
		))
	}
	return strings.Join(parts, "; ")
}

func hintGroundItems(cur world.State) []world.Item {
	items := cur.GroundItems()
	out := make([]world.Item, 0, len(items))
	for _, item := range items {
		if item.Type == "body" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (rt *Runtime) runtimeWorldLogAttrs(cur world.State, verbose bool) []slog.Attr {
	attrs := worldLogAttrs(cur, verbose)
	if !verbose || !cur.Valid || rt == nil || rt.Loot == nil {
		return attrs
	}

	items := cur.InventoryItems()
	grid := loot.NewInventoryGrid(rt.Loot.InventoryLock(), items)
	capacity := grid.Capacity()
	attrs = append(attrs,
		slog.Uint64("inventory_item_count", uint64(len(items))),
		slog.Int("inventory_free_slots", capacity.FreeSlots),
		slog.Int("inventory_locked_slots", capacity.LockedSlots),
		slog.Bool("inventory_capacity_unsafe", capacity.Unsafe),
	)
	if capacity.Reason != "" {
		attrs = append(attrs, slog.String("inventory_capacity_reason", capacity.Reason))
	}
	if hint := verboseInventoryItemsHint(items); hint != "" {
		attrs = append(attrs, slog.String("inventory_items_hint", hint))
	}
	return attrs
}

func verboseInventoryItemsHint(items []world.Item) string {
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UnitID < items[j].UnitID
	})
	const maxHintItems = 32
	if len(items) > maxHintItems {
		items = items[:maxHintItems]
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		name := item.Name
		if name == "" {
			name = "Unknown Item"
		}
		parts = append(parts, fmt.Sprintf("unit=%d id=%d code=%q name=%q quality=%s ethereal=%t identity_kind=%q identity_raw_id=%d identity_available=%t identity_name=%q identity_consistent=%t identity_reason=%q grid=%d,%d size=%dx%d",
			item.UnitID,
			item.TxtFileNo,
			item.Code,
			name,
			item.Quality.String(),
			item.Ethereal,
			item.IdentityKind,
			item.IdentityRawID,
			item.IdentityAvailable,
			item.IdentityName,
			item.IdentityValid,
			item.IdentityReason,
			item.GridX,
			item.GridY,
			item.Width,
			item.Height,
		))
	}
	return strings.Join(parts, "; ")
}

func (rt *Runtime) logWorldState(prev, cur world.State, heartbeat, verbose bool) {
	if cur.Valid {
		level := slog.LevelInfo
		if verbose && isPositionOnlyWorldChange(prev, cur) {
			level = slog.LevelDebug
		}
		rt.Log.Log(context.Background(), level, "world state", attrsToArgs(rt.runtimeWorldLogAttrs(cur, verbose))...)
		return
	}

	if heartbeat {
		rt.Log.Debug("world unavailable", attrsToArgs(rt.runtimeWorldLogAttrs(cur, verbose))...)
		return
	}
	rt.Log.Info("world unavailable", attrsToArgs(rt.runtimeWorldLogAttrs(cur, verbose))...)
}

func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, 0, len(attrs)*2)
	for _, a := range attrs {
		args = append(args, a.Key, a.Value.Any())
	}
	return args
}

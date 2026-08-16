package app

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
)

// WaypointTargetsInspectReport is the read-only JSON calibration contract.
type WaypointTargetsInspectReport struct {
	SchemaVersion int                            `json:"schema_version"`
	Targets       []pathing.WaypointTargetAction `json:"targets"`
}

// ResolveWaypointTargetsInspectReport returns the immutable target registry
// without process attach, hotkeys, input, or session startup.
func ResolveWaypointTargetsInspectReport(opts Options) (WaypointTargetsInspectReport, error) {
	if !opts.WaypointTargetsInspect {
		return WaypointTargetsInspectReport{}, fmt.Errorf("waypoint target inspect mode is not selected")
	}
	if opts.SessionInspect || opts.RunsInspect || opts.Probe || opts.InputTest != "" || opts.Run != "" || opts.RunPhase != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.WeaponSetProbe != "" || opts.Route != "" || opts.TownInspect || opts.TownTest != "" {
		return WaypointTargetsInspectReport{}, fmt.Errorf("--waypoint-targets-inspect is mutually exclusive with session, run, probe, route, town, and test modes")
	}
	return WaypointTargetsInspectReport{SchemaVersion: 1, Targets: pathing.DefaultWaypointTargetRegistry().Actions()}, nil
}

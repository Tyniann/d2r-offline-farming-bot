package app

import (
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
)

type routeCommand struct {
	action    string
	id        string
	segmentID string
}

func parseRouteCommand(raw string) (routeCommand, error) {
	raw = strings.TrimSpace(raw)
	if raw == "list" {
		return routeCommand{action: "list"}, nil
	}
	action, id, ok := strings.Cut(raw, ":")
	if !ok || strings.TrimSpace(id) == "" {
		return routeCommand{}, fmt.Errorf("route command must be list, inspect:<route-id>, or validate:<route-id>")
	}
	if action == "inspect-egress" || action == "record-egress" || action == "validate-egress" || action == "play-egress" {
		if strings.TrimSpace(id) != "act3" {
			return routeCommand{}, fmt.Errorf("%s expects act3", action)
		}
		return routeCommand{action: action, id: "act3"}, nil
	}
	if action == "play-segment" {
		routeID, segmentID, ok := strings.Cut(id, "/")
		if !ok || strings.TrimSpace(routeID) == "" || strings.TrimSpace(segmentID) == "" {
			return routeCommand{}, fmt.Errorf("play-segment expects <route-id>/<segment-id>")
		}
		return routeCommand{action: action, id: strings.TrimSpace(routeID), segmentID: strings.TrimSpace(segmentID)}, nil
	}
	if action == "play" {
		return routeCommand{action: action, id: strings.TrimSpace(id)}, nil
	}
	if action != "inspect" && action != "validate" && action != "record" {
		return routeCommand{}, fmt.Errorf("unsupported route command %q", action)
	}
	return routeCommand{action: action, id: strings.TrimSpace(id)}, nil
}

// RunRouteCommand executes a read-only Phase 6.2 registry command.
func (rt *Runtime) RunRouteCommand(raw string) error {
	command, err := parseRouteCommand(raw)
	if err != nil {
		return err
	}
	if command.action == "record" {
		return rt.RunRouteRecord(command.id, rt.Options.RouteName, rt.Options.RouteDifficulty)
	}
	if command.action == "inspect-egress" {
		return rt.RunTownEgressInspect()
	}
	if command.action == "record-egress" {
		return rt.RunTownEgressRecord(rt.Options.RouteName, rt.Options.RouteDifficulty)
	}
	if command.action == "validate-egress" {
		return rt.RunTownEgressValidate()
	}
	if command.action == "play-egress" {
		return rt.RunTownEgressPlay()
	}
	if command.action == "play-segment" {
		return rt.RunRouteSegment(command.id, command.segmentID)
	}
	if command.action == "play" {
		return rt.RunRoutePlay(command.id)
	}
	directory, err := configuredFarmingRouteDirectory(rt.Config)
	if err != nil {
		return err
	}
	registry, err := pathing.LoadRouteRegistry(directory)
	if err != nil {
		return err
	}
	if command.action == "list" {
		entries := registry.Entries()
		rt.Log.Info("route registry", "directory", registry.Directory(), "route_file_count", len(entries))
		for _, entry := range entries {
			rt.Log.Info("route registry entry", "route_id", entry.ID, "name", entry.Name, "status", entry.Status, "reason", entry.Reason, "path", entry.Path, "character", entry.Binding.CharacterName, "difficulty", entry.Binding.Difficulty, "game_version", entry.Binding.GameVersion, "tags", entry.Tags)
		}
		return nil
	}
	route, err := registry.Get(command.id)
	if err != nil {
		return err
	}
	if command.action == "validate" {
		rt.Log.Info("route valid", "route_id", route.ID, "path_directory", registry.Directory(), "segment_count", len(route.Segments), "layout_fingerprint", route.Binding.LayoutFingerprint.Hash)
		return nil
	}
	rt.Log.Info("route details", "route_id", route.ID, "name", route.Name, "kind", route.Kind, "tags", route.Tags, "character", route.Binding.CharacterName, "character_class", route.Binding.CharacterClass, "difficulty", route.Binding.Difficulty, "game_version", route.Binding.GameVersion, "recorded_at", route.Recording.RecordedAt, "segment_count", len(route.Segments), "layout_fingerprint", route.Binding.LayoutFingerprint.Hash)
	for _, segment := range route.Segments {
		rt.Log.Info("route segment", "route_id", route.ID, "segment_id", segment.ID, "from_area_id", segment.FromAreaID, "to_area_id", segment.ToAreaID, "movement", segment.Movement, "point_count", len(segment.Points), "entrance_kind", segment.Transition.EntranceKind)
	}
	return nil
}

func (rt *Runtime) routeCommandIsReadOnly() bool {
	if rt.Options.Route == "" {
		return false
	}
	command, err := parseRouteCommand(rt.Options.Route)
	return err == nil && (command.action == "list" || command.action == "inspect" || command.action == "validate" || command.action == "record" || command.action == "inspect-egress" || command.action == "record-egress" || command.action == "validate-egress")
}

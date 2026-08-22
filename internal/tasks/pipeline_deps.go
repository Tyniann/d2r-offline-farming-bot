package tasks

// Domain dependency views keep each pipeline component from reaching inputs
// owned by another domain. The orchestrator narrows the generation-wide Deps
// snapshot once at the component boundary.
type pipelineTravelDeps struct {
	Waypoint   WaypointActions
	TownWalk   TownWalker
	Combat     CombatActions
	Loot       LootActions
	Route      RoutePlayback
	RouteClear RouteClearExecutor
	Profile    ProfileActions
	Telemetry  RunTelemetry
	Chest      ChestOperateActions
}

type pipelineChestDeps struct {
	Chest      ChestOperateActions
	Combat     CombatActions
	Loot       LootActions
	Route      RoutePlayback
	RouteClear RouteClearExecutor
	Telemetry  RunTelemetry
}

type pipelineBossDeps struct {
	Pathing    Navigator
	Combat     CombatActions
	RouteClear RouteClearExecutor
	Profile    ProfileActions
	Telemetry  RunTelemetry
}

type pipelineLootDeps struct {
	Combat CombatActions
	Loot   LootActions
}

type pipelineReturnDeps struct {
	Waypoint   WaypointActions
	Portal     TownPortalActions
	Stash      PersonalStashActions
	Combat     CombatActions
	Actions    RunActions
	Loot       LootActions
	RouteClear RouteClearExecutor
	TownEgress TownEgressPlayback
	Town       TownPreparationActions
	Telemetry  RunTelemetry
}

func narrowTravelDeps(deps Deps) pipelineTravelDeps {
	return pipelineTravelDeps{Waypoint: deps.Waypoint, TownWalk: deps.TownWalk, Combat: deps.Combat, Loot: deps.Loot, Route: deps.Route, RouteClear: deps.RouteClear, Profile: deps.Profile, Telemetry: deps.Telemetry, Chest: deps.Chest}
}

func narrowChestDeps(deps Deps) pipelineChestDeps {
	// Terminal leftover sweep runs after play_bound_route is done. Hold on that
	// finished adapter fails closed with no active progress; operate-on-sight
	// during playback keeps Route via [narrowChestDepsFromTravel].
	return pipelineChestDeps{Chest: deps.Chest, Combat: deps.Combat, Loot: deps.Loot, RouteClear: deps.RouteClear, Telemetry: deps.Telemetry}
}

func narrowChestDepsFromTravel(deps pipelineTravelDeps) pipelineChestDeps {
	return pipelineChestDeps{Chest: deps.Chest, Combat: deps.Combat, Loot: deps.Loot, Route: deps.Route, RouteClear: deps.RouteClear, Telemetry: deps.Telemetry}
}

func narrowBossDeps(deps Deps) pipelineBossDeps {
	return pipelineBossDeps{Pathing: deps.Pathing, Combat: deps.Combat, RouteClear: deps.RouteClear, Profile: deps.Profile, Telemetry: deps.Telemetry}
}

func narrowLootDeps(deps Deps) pipelineLootDeps {
	return pipelineLootDeps{Combat: deps.Combat, Loot: deps.Loot}
}

func narrowReturnDeps(deps Deps) pipelineReturnDeps {
	return pipelineReturnDeps{Waypoint: deps.Waypoint, Portal: deps.Portal, Stash: deps.Stash, Combat: deps.Combat, Actions: deps.Actions, Loot: deps.Loot, RouteClear: deps.RouteClear, TownEgress: deps.TownEgress, Town: deps.Town, Telemetry: deps.Telemetry}
}

func (deps pipelineTravelDeps) lootDeps() pipelineLootDeps {
	return pipelineLootDeps{Combat: deps.Combat, Loot: deps.Loot}
}

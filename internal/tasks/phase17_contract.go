package tasks

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// Phase17StableClearSnapshots ist die Anzahl frischer, lokal vollständiger
	// threat-freier Snapshots vor der Rückkehr zu Route-Movement.
	Phase17StableClearSnapshots = 3
)

// RouteCombatConfig is the resolved task-side tuning for route threat assessment.
type RouteCombatConfig struct {
	Enabled                    bool
	ImmediateRadiusTiles       float64
	CorridorWidthTiles         float64
	LandingRadiusTiles         float64
	AttackDistanceTiles        float64
	NoProgressTimeout          time.Duration
	TeleportManaReservePercent int
	ResumeManaPercent          int
	EmergencyManaPercent       int
	ManaRecoveryTimeout        time.Duration
}

// RouteProgressMode beschreibt die Quelle des tatsächlich nächsten Route-Ziels.
type RouteProgressMode string

const (
	// RouteProgressMovement bezeichnet den nächsten regulären aufgezeichneten Punkt.
	RouteProgressMovement RouteProgressMode = "movement"
	// RouteProgressRecovery bezeichnet den tatsächlich beauftragten vorherigen Korrekturpunkt.
	RouteProgressRecovery RouteProgressMode = "recovery"
	// RouteProgressTransition bezeichnet einen erwarteten Area-Übergang ohne Positionsziel.
	RouteProgressTransition RouteProgressMode = "transition"
)

// RouteProgress ist die read-only Task-Projektion des aktiven RoutePlayers.
// Sie rekonstruiert weder Route-YAML noch Telemetrie und autorisiert keinen Input.
type RouteProgress struct {
	// RouteID identifiziert die bereits gestartete Route.
	RouteID string
	// RouteRole identifies the fixed role of a route-set member. Single-route
	// runs leave it empty and continue to use RouteID as their telemetry route.
	RouteRole pathing.RouteRole
	// SegmentID identifiziert das aktive Segment.
	SegmentID string
	// SegmentIndex ist der aktive nullbasierte Segmentindex.
	SegmentIndex int
	// PointIndex ist der nächste noch nicht bestätigte Punktindex.
	PointIndex int
	// PreviousConfirmed ist der letzte vom RoutePlayer bestätigte Punkt.
	PreviousConfirmed world.Position
	// MovementTarget ist das tatsächliche nächste Movement- oder Recovery-Ziel.
	MovementTarget world.Position
	// TargetAvailable meldet, ob MovementTarget für eine Positionsprüfung gültig ist.
	TargetAvailable bool
	// Mode unterscheidet reguläres Movement, Recovery und Transition.
	Mode RouteProgressMode
	// DriftTiles ist der aktuelle Abstand zur aktiven Routenkante.
	DriftTiles float64
	// LocalRecoveryAttempts zählt verbrauchte lokale Korrekturen des aktiven Segments.
	LocalRecoveryAttempts int
	// RecoveryInputSent meldet einen tatsächlich gesendeten Cast auf das aktuelle Recovery-Ziel.
	RecoveryInputSent bool
	// RecoveryInputAt bindet den Cast an den exakten Route-Tick.
	RecoveryInputAt time.Time
	// RecoveryInputOrigin ist die Spielerposition beim gesendeten Cast.
	RecoveryInputOrigin world.Position
	// RecoveryNextInputAt ist der früheste Zeitpunkt eines throttle-seitig möglichen Folgecasts.
	RecoveryNextInputAt time.Time
	// RecoveryOutcomeAt ist der früheste Zeitpunkt, an dem eine weiterhin
	// unveränderte Position den gesendeten Teleport als wirkungslos belegt.
	RecoveryOutcomeAt time.Time
	// RecoveryProgressTiles ist der für diesen Cast autoritative Mindestfortschritt.
	RecoveryProgressTiles float64
}

// ThreatZone benennt die fachliche Zone eines ausgewählten Route-Blockers.
type ThreatZone string

const (
	// ThreatZoneNone bezeichnet ein Assessment ohne bekannten Route-Blocker.
	ThreatZoneNone ThreatZone = ""
	// ThreatZoneImmediate priorisiert erlaubte Gegner im Spielerumfeld.
	ThreatZoneImmediate ThreatZone = "immediate"
	// ThreatZoneLanding priorisiert danach Gegner am effektiven Landepunkt.
	ThreatZoneLanding ThreatZone = "landing"
	// ThreatZoneCorridor priorisiert danach Gegner auf der vorausliegenden Routenkante.
	ThreatZoneCorridor ThreatZone = "corridor"
)

// ThreatAssessment ist die immutable, höchstens einmal pro [world.State.At]
// gebildete Route-Threat-Entscheidung. Sie liest keinen Prozessspeicher und sendet keinen Input.
type ThreatAssessment struct {
	// SnapshotAt bindet das Assessment an genau einen World-Snapshot.
	SnapshotAt time.Time
	// RouteTarget ist das priorisierte bekannte Movement-Hindernis.
	RouteTarget world.Monster
	// RouteTargetFound meldet, ob RouteTarget gültig ist.
	RouteTargetFound bool
	// RouteZone ist die priorisierte Zone des RouteTarget.
	RouteZone ThreatZone
	// HoveredRouteTarget ist das aktuell gehovte lebende Monster. Während ein
	// Allowlist-Blocker die Route hält, darf dieses Ziel ohne weitere Aim-Prüfung
	// unmittelbar angegriffen werden.
	HoveredRouteTarget world.Monster
	// HoveredRouteTargetFound meldet, ob Memory ein lebendes Monster unter dem
	// Cursor bestätigt.
	HoveredRouteTargetFound bool
	// DensityTarget ist der rollierende Kandidat bei unvollständiger lokaler Coverage.
	DensityTarget world.Monster
	// DensityTargetFound meldet, ob DensityTarget sicher projizierbar ist.
	DensityTargetFound bool
	// RelevantThreatCount zählt alle erlaubten Immediate-, Landing- und Corridor-Threats.
	RelevantThreatCount int
	// RequiredCoverageTiles ist die vollständige lokale Spieler-Ziel-Hülle.
	RequiredCoverageTiles float64
	// CoverageComplete meldet lokale Vollständigkeit unabhängig von globaler Truncation.
	CoverageComplete bool
}

// RouteThreatState bezeichnet einen generation-scoped Zustand des Route-Clear-Automaten.
type RouteThreatState string

const (
	// RouteThreatMoving erlaubt bei sicherem Assessment genau einen Route-Tick.
	RouteThreatMoving RouteThreatState = "route_moving"
	// RouteThreatClearing hält die Route und räumt einen bekannten Route-Blocker.
	RouteThreatClearing RouteThreatState = "route_clearing"
	// RouteThreatDensityRelief hält die Route bei einer lokalen Coverage-Lücke aktiv handlungsfähig.
	RouteThreatDensityRelief RouteThreatState = "density_relief"
	// RouteThreatManaRecovery hält die Route bis zur bestätigten Mana-Resume-Schwelle.
	RouteThreatManaRecovery RouteThreatState = "route_mana_recovery"
	// RouteThreatRecoveryGuard schützt das tatsächliche Route-Recovery-Ziel.
	RouteThreatRecoveryGuard RouteThreatState = "route_recovery_guard"
)

// RouteThreatReason ist ein stabiler maschinenlesbarer Phase-17-Fehlergrund.
type RouteThreatReason string

const (
	// RouteThreatReasonClearNoProgress bezeichnet zwölf Sekunden ohne objektiven Clear-Fortschritt.
	RouteThreatReasonClearNoProgress RouteThreatReason = "route_clear_no_progress"
	// RouteThreatReasonCowNoProgress bezeichnet einen Cow-Hold ohne objektiven
	// Fortschritt durch weniger Lebende, neue/verbrauchte Leichen oder Coverage.
	RouteThreatReasonCowNoProgress RouteThreatReason = "cow_combat_no_progress"
	// RouteThreatReasonOutOfRange bezeichnet einen dreifach bestätigten, nicht sicher angreifbaren Blocker.
	RouteThreatReasonOutOfRange RouteThreatReason = "route_threat_out_of_range"
	// RouteThreatReasonManaRecoveryFailed bezeichnet fehlende oder nicht bestätigte Mana-Erholung.
	RouteThreatReasonManaRecoveryFailed RouteThreatReason = "route_mana_recovery_failed"
	// RouteThreatReasonRecoveryUnsafe bezeichnet ein bedrohtes oder wirkungslos wiederholtes Recovery-Ziel.
	RouteThreatReasonRecoveryUnsafe RouteThreatReason = "route_recovery_unsafe"
	// RouteThreatReasonStateInvalid bezeichnet eine interne Vertragsverletzung.
	RouteThreatReasonStateInvalid RouteThreatReason = "route_threat_state_invalid"
)

// Phase17RouteThreatReasons liefert die vollständige geordnete Reason-Code-Menge.
func Phase17RouteThreatReasons() []RouteThreatReason {
	return []RouteThreatReason{
		RouteThreatReasonClearNoProgress,
		RouteThreatReasonOutOfRange,
		RouteThreatReasonManaRecoveryFailed,
		RouteThreatReasonRecoveryUnsafe,
		RouteThreatReasonStateInvalid,
	}
}

// Phase17ContractOwner benennt die einzige Autorität einer Route-Threat-Verantwortung.
type Phase17ContractOwner struct {
	// Responsibility ist der stabile maschinenlesbare Verantwortungsbereich.
	Responsibility string
	// Owner ist die einzige schreibende oder entscheidende Autorität.
	Owner string
	// Boundary beschreibt die harte Grenze zu anderen Komponenten.
	Boundary string
}

// Phase17ContractOwners liefert die verbindlichen Memory-, World-, Task-, Profil- und Inputgrenzen.
func Phase17ContractOwners() []Phase17ContractOwner {
	return []Phase17ContractOwner{
		{Responsibility: "living_monsters_and_hover", Owner: "internal/memory to internal/world", Boundary: "no pixels, task-owned process read, or cached living guess"},
		{Responsibility: "monster_capacity_and_coverage", Owner: "internal/memory to internal/world", Boundary: "512 non-priority plus unbounded priority entries"},
		{Responsibility: "run_hostile_allowlist", Owner: "tasks.RunDefinition", Boundary: "no raw NPC IDs in operator YAML"},
		{Responsibility: "threat_geometry_and_state", Owner: "internal/tasks", Boundary: "one assessment per world.State.At"},
		{Responsibility: "build_clear_strategy", Owner: "internal/profile", Boundary: "no class or skill switch in tasks or pathing"},
		{Responsibility: "route_progress_and_recovery_target", Owner: "internal/pathing", Boundary: "read-only projection without task-side reconstruction"},
		{Responsibility: "route_deadline_hold", Owner: "internal/app route adapter", Boundary: "no second task timeout"},
		{Responsibility: "skill_aim_hover_and_click", Owner: "internal/app CombatActions", Boundary: "no direct input from controller or profile"},
		{Responsibility: "resource_selection_and_verification", Owner: "internal/profile", Boundary: "existing Result status remains tick authority"},
	}
}

// Phase17NonGoals liefert die ausdrücklich ausgeschlossenen Architekturpfade.
func Phase17NonGoals() []string {
	return []string{
		"online_battle_net_savegame_or_installation_mutation",
		"universal_combat_ai_rotation_dsl_or_class_switch_in_tasks",
		"corpse_model_amplify_damage_or_corpse_explosion",
		"area_clear_spatial_index_quadtree_or_second_memory_query",
		"approach_teleport_random_jitter_or_alternative_recovery_target",
		"implicit_combat_for_raw_playback_recording_or_guided_validation",
		"automatic_enablement_for_non_summoner_runs",
		"new_settings_page_history_chart_or_operator_npc_id_list",
		"hard_monster_action_or_total_clear_duration_limit",
	}
}

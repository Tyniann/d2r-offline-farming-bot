package profile

import (
	"errors"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ErrRouteClearTargetUnprojectable reports that the selected living target has
// no playable client point in the currently bound window.
var ErrRouteClearTargetUnprojectable = errors.New("route clear target unprojectable")

// Hook identifies a semantic lifecycle event emitted by a run.
type Hook string

const (
	// HookTownReady runs after a stable town start and before town navigation.
	HookTownReady Hook = "town_ready"
	// HookBossEngage runs after a boss UnitID is confirmed and before attacks.
	HookBossEngage Hook = "boss_engage"
)

// TargetKind identifies how an action target is resolved from World state.
type TargetKind string

const (
	// TargetSelf casts at the current player position.
	TargetSelf TargetKind = "self"
	// TargetBoss casts at the confirmed encounter target.
	TargetBoss TargetKind = "boss"
)

// Action is one ordered skill cast attached to a semantic hook.
type Action struct {
	SkillID          uint16
	Target           TargetKind
	OncePerGame      bool
	OncePerEncounter bool
	Delay            time.Duration
	Settle           time.Duration
}

// ResourceKind identifies a supported belt-consumable policy.
type ResourceKind string

const (
	// ResourceHealing consumes a healing potion for low HP.
	ResourceHealing ResourceKind = "healing"
	// ResourceMana consumes a mana potion for low mana.
	ResourceMana ResourceKind = "mana"
	// ResourceRejuvenation consumes rejuvenation for critical HP.
	ResourceRejuvenation ResourceKind = "rejuvenation"
)

// ResourceRule holds one percentage threshold and eligible belt columns.
type ResourceRule struct {
	UseBelowPercent uint8
	BeltSlots       []int
	Cooldown        time.Duration
}

// ResourcePolicy is evaluated before hook and run actions on valid gameplay ticks.
type ResourcePolicy struct {
	Healing       ResourceRule
	Mana          ResourceRule
	Rejuvenation  ResourceRule
	Throttle      time.Duration
	VerifyTimeout time.Duration
}

// Definition is the resolved, immutable behavior selected for one character build.
type Definition struct {
	ID             string
	CharacterClass world.CharacterClass
	Hooks          map[Hook][]Action
	Resources      ResourcePolicy
}

// EncounterTarget pins the boss identity used by encounter-scoped actions.
type EncounterTarget struct {
	UnitID   uint32
	Position world.Position
	// ActionIndex is the stable definition index; retries of one action keep the same value.
	ActionIndex int
}

// RouteClearMode identifies why stationary route combat is requested.
type RouteClearMode string

const (
	// RouteClearThreat attacks a known Immediate, Landing, or Corridor blocker.
	RouteClearThreat RouteClearMode = "route_threat"
	// RouteClearDensityRelief attacks a safe local candidate to improve incomplete coverage.
	RouteClearDensityRelief RouteClearMode = "density_relief"
)

// RouteClearStrategy identifies a code-backed build strategy.
type RouteClearStrategy string

const (
	// RouteClearSingleTarget attacks one current Memory-confirmed route blocker per cast.
	RouteClearSingleTarget RouteClearStrategy = "single_target"
)

// RouteClearActionKind identifies the purpose of one confirmed route-combat input.
type RouteClearActionKind string

const (
	// RouteClearActionCurse is the one-time opener applied to the first blocker.
	RouteClearActionCurse RouteClearActionKind = "curse"
	// RouteClearActionAttack is a regular damage cast after the opener.
	RouteClearActionAttack RouteClearActionKind = "attack"
)

// RouteClearRequest is the immutable stationary-combat request for one snapshot.
type RouteClearRequest struct {
	RunID        string
	DefinitionID string
	Player       world.Player
	Target       world.Monster
	Mode         RouteClearMode
	AssessmentAt time.Time
}

// RouteCombatActions is the movement-free action surface available to route clear.
type RouteCombatActions interface {
	CastAttackAtMonster(time.Time, uint16, world.Player, world.Monster) (bool, error)
	StopAttack() error
}

// Status is a stable hook or resource executor outcome.
type Status string

const (
	// StatusComplete indicates no further action is required for this tick.
	StatusComplete Status = "complete"
	// StatusPending indicates settle or consumption verification is in progress.
	StatusPending Status = "pending"
	// StatusAction indicates exactly one input was sent in this tick.
	StatusAction Status = "action"
	// StatusFailed indicates a fail-closed terminal executor error.
	StatusFailed Status = "failed"
)

const (
	// RouteClearReasonTargetUnprojectable preserves a safe projection miss for
	// the controller's existing three-snapshot out-of-range gate.
	RouteClearReasonTargetUnprojectable = "route_clear_target_unprojectable"
)

// Result reports one executor decision without exposing input internals.
type Result struct {
	Status     Status
	Reason     string
	Hook       Hook
	Resource   ResourceKind
	SkillID    uint16
	BeltSlot   int
	ActionKind RouteClearActionKind
}

// EventName identifies a stable profile telemetry transition.
type EventName string

const (
	// EventHookAction records a successfully requested hook skill.
	EventHookAction EventName = "profile_hook_action"
	// EventPotionRequested records a successfully requested belt potion.
	EventPotionRequested EventName = "resource_potion_requested"
	// EventPotionConfirmed records Memory-confirmed consumption.
	EventPotionConfirmed EventName = "resource_consumption_confirmed"
	// EventActionFailed records a stable input or verification failure.
	EventActionFailed EventName = "profile_action_failed"
)

// Event contains the profile context needed for correlated run telemetry.
type Event struct {
	Name             EventName
	Profile          string
	Hook             Hook
	Resource         ResourceKind
	SkillID          uint16
	Target           TargetKind
	TargetUnitID     uint32
	PotionUnitID     uint32
	ThresholdPercent uint8
	BeltSlot         int
	Confirmed        bool
	Reason           string
}

// Telemetry synchronously publishes profile events before execution advances.
type Telemetry interface {
	EmitProfile(Event) error
}

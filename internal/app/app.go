package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/replay"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Runtime bundles initialized application components.
type Runtime struct {
	Config  *config.Config
	Options Options
	Log     *slog.Logger
	logFile *os.File

	Process            processController
	Memory             *memory.Reader
	Probe              snapshotReader
	UIProbe            uiBufferCaptureReader
	HirelingProbe      hirelingEvidenceReader
	ObjectInspect      objectInspectEvidenceReader
	World              *world.Model
	Input              inputController
	Bindings           configBindingSource
	Tasks              *tasks.Runner
	Pathing            *pathing.Navigator
	Loot               *loot.Filter
	PickitProfiles     *PickitProfileService
	PickitAssignments  *PickitAssignmentStore
	ActivePickit       PickitPolicySnapshot
	Telemetry          *telemetry.Recorder
	Profile            *profile.Executor
	RuntimeTrace       *replay.Recorder
	profileTelemetry   *profileTelemetryAdapter
	compatibility      d2rCompatibilityContract
	processPreAttached atomic.Bool

	sessionReset              sessionResetBarrier
	taskDeps                  tasks.Deps
	runConfig                 tasks.RunConfig
	combatProfileID           string
	sessionSelection          tasks.RunSelection
	routePlayback             *routePlaybackAdapter
	townEgress                *townEgressAdapter
	lootActions               *lootActionsAdapter
	townLayout                *townLayoutPin
	townTelemetry             *townTelemetryRelay
	townPreparation           *townPreparationAdapter
	stashExecutor             *loot.StashExecutor
	uiStatusPublisher         func(UIStatusSnapshot)
	pauseHotkeyHandler        func() error
	stopAfterRunHotkeyHandler func() error
	runReadinessPending       bool
	productiveRunActive       bool
	mercenaryDeath            mercenaryDeathGuard
	lastSnapshot              memory.Snapshot
}

// New builds a Runtime from config and CLI/runtime options.
func New(cfg *config.Config, opts Options) (rt *Runtime, err error) {
	bindings, err := resolveRuntimeBindings(opts)
	if err != nil {
		return nil, err
	}
	combatProfileID, err := resolveRuntimeCombatProfileID(cfg, opts.Loadout)
	if err != nil {
		return nil, err
	}
	if cfg.Session.Enabled && SessionExecutionRequested(opts) {
		// Queue runtimes already own an immutable character loadout. Preserve it
		// for the read-only session preflight so Paladin routes are not checked
		// against the classless Necromancer fallback.
		if _, planErr := ResolveSessionPlan(cfg, Options{SessionInspect: true, Loadout: opts.Loadout}); planErr != nil {
			return nil, planErr
		}
		if bindingErr := validateFullRunBindingsWithProfile(cfg, cfg.Session.Run, bindings, combatProfileID); bindingErr != nil {
			return nil, fmt.Errorf("session bindings: %w", bindingErr)
		}
	}
	var runTelemetry *telemetry.Recorder
	logLevel := cfg.App.LogLevel
	if opts.Verbose {
		logLevel = "debug"
	}
	logDirectory := "logs"
	if cfg.DataRoot != "" {
		logDirectory = filepath.Join(cfg.DataRoot, "logs")
	}
	log, logFile, logFilePath, err := config.NewFileLogger(logLevel, logDirectory, cfg.App.Name, time.Now())
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}
	defer func() {
		if err != nil && logFile != nil {
			_ = logFile.Close()
		}
		if err != nil && runTelemetry != nil {
			_ = runTelemetry.Close()
		}
	}()
	log = log.With("app", cfg.App.Name)
	log.Info("file logging enabled", "path", logFilePath)

	offsetsPath := cfg.ResolvePath(cfg.Memory.OffsetsFile)
	offsetSet, err := memory.ResolveOffsetSet(offsetsPath)
	if err != nil {
		return nil, fmt.Errorf("load offsets: %w", err)
	}

	expectedVersion := cfg.Memory.GameVersion
	if expectedVersion == "" {
		expectedVersion = offsetSet.D2RVersion
	}
	if cfg.Memory.GameVersion != "" && offsetSet.D2RVersion != "" && cfg.Memory.GameVersion != offsetSet.D2RVersion {
		log.Warn("memory.game_version does not match offset set",
			"config_game_version", cfg.Memory.GameVersion,
			"offset_d2r_version", offsetSet.D2RVersion,
		)
	}

	offsetsSource := "(default)"
	if offsetsPath != "" {
		offsetsSource = offsetsPath
	}
	log.Info("offset configuration",
		"game_version", expectedVersion,
		"offset_set", offsetSet.Name,
		"offsets_file", offsetsSource,
		"attach_timeout_ms", cfg.Process.AttachTimeoutMs,
	)

	mem := memory.NewReader(log)
	proc := process.New(log, cfg.Process.ProcessName)
	inputCtrl, err := input.NewController(log, mapInputConfig(cfg.Input), mapSafetyConfig(cfg.Input))
	if err != nil {
		return nil, fmt.Errorf("input controller: %w", err)
	}
	runSelection := resolveRunSelection(opts, cfg)
	if validationErr := validateRunMode(runSelection, cfg, opts, log); validationErr != nil {
		return nil, validationErr
	}
	runtimeRunID := runSelection.Run
	if cfg.Session.Enabled && SessionExecutionRequested(opts) {
		runtimeRunID = cfg.Session.Run
	}
	if runtimeRunID == "" {
		// Passive desktop and isolated diagnostics keep Countess only as a
		// profile carrier. They must not require a Countess pickit or strategy.
		runtimeRunID = string(tasks.RunIDCountess)
	}
	if isHammerdinPrebuffTownTest(opts.TownTest) {
		// The isolated prebuff uses the registered Hammerdin/Mephisto profile
		// carrier without authorizing route playback or boss combat.
		runtimeRunID = string(tasks.RunIDMephisto)
	}
	selectedRunCfg, ok := cfg.Runs.Run(runtimeRunID)
	if !ok {
		return nil, fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, runtimeRunID)
	}
	// Der passive Desktop-UI-Start muss einen frisch provisionierten Root ohne
	// Farming-Zuweisung erklären können. Isolierte Town-/Merc-Diagnosen nutzen
	// Countess nur als Profil-/Policy-Träger und verlangen ebenfalls keine Route.
	// Erst konkrete Run-/Sessionpfade verlangen fail-closed eine Assignment-Route.
	requireFarmingRoute := farmingRouteRequired(opts, runSelection)
	strategyRegistry := NewCombatStrategyRegistry()
	if registryErr := strategyRegistry.ValidateAgainstProfiles(cfg.Profiles); registryErr != nil {
		return nil, fmt.Errorf("combat strategy registry: %w", registryErr)
	}
	runCfg, err := mapRunConfigWithProfile(cfg, runtimeRunID, combatProfileID, requireFarmingRoute)
	if err != nil {
		return nil, err
	}
	inventoryCells := allLockedInventoryGrid()
	if opts.Loadout != nil {
		inventoryCells = EffectiveInventoryGrid(*opts.Loadout)
	} else if CharacterLoadoutRequired(opts) {
		return nil, fmt.Errorf("character loadout required for inventory lock")
	}
	// Operator belt_layout overrides combat-profile potion columns for this
	// runtime only; shared config.yaml defaults stay unchanged for other characters.
	runtimeCFG := *cfg
	runtimeCFG.Profiles = cloneProfilesConfig(cfg.Profiles)
	if err = applyLoadoutBeltLayout(runtimeCFG.Profiles, opts.Loadout); err != nil {
		return nil, fmt.Errorf("belt layout: %w", err)
	}
	cfg = &runtimeCFG
	if runtimeRunID == string(tasks.RunIDCows) {
		runCfg.Cow = mapCowConfig(cfg, bindings, inventoryCells, combatProfileID)
	}

	pathingCfg := mapPathingConfig(cfg.Pathing)
	if validationErr := pathingCfg.Validate(); validationErr != nil {
		return nil, fmt.Errorf("pathing config: %w", validationErr)
	}
	nav := pathing.NewNavigator(log, pathing.Deps{
		Input:    inputCtrl,
		Bindings: bindings,
		Config:   pathingCfg,
	})
	waypoints := pathing.NewWaypointActions(log, inputCtrl, pathingCfg)
	runWaypoints := &runWaypointAdapter{actions: waypoints}
	townPortals := pathing.NewTownPortalActions(log, inputCtrl, pathingCfg)
	personalStash := pathing.NewPersonalStashActions(log, inputCtrl, pathingCfg)
	townLayout := &townLayoutPin{}
	townTrace := &townTelemetryRelay{}
	townPreparation, err := newTownPreparationAdapterWithProfile(log, inputCtrl, pathingCfg, cfg, runtimeRunID, selectedRunCfg, combatProfileID, townLayout, townTrace, true)
	if err != nil {
		return nil, err
	}
	if runtimeRunID == string(tasks.RunIDCows) {
		townPreparation.requireFullBuyableBelt = true
		townPreparation.minimumRejuvenation = 1
	}
	townStartAdapter, err := newTownPreparationAdapterWithProfile(log, inputCtrl, pathingCfg, cfg, runtimeRunID, selectedRunCfg, combatProfileID, townLayout, townTrace, false)
	if err != nil {
		return nil, err
	}
	townStartAdapter.thresholds = town.Thresholds{}
	layoutTownWalker := &layoutTownWaypointWalker{adapter: townStartAdapter}
	combat := newCombatAdapter(log, inputCtrl, bindings, pathingCfg, runCfg.Combat.AttackInterval)
	wireNavigatorRightSkill(nav, combat, inputCtrl)
	runActions := newRunActionsAdapter(log, inputCtrl, bindings, combat)
	profileTrace := &profileTelemetryAdapter{}
	profileExecutor, err := newProfileExecutor(log, cfg.Profiles, combatProfileID, runtimeRunID, strategyRegistry, inputCtrl, bindings, pathingCfg, combat, profileTrace, CharacterLoadoutRequired(opts))
	if err != nil {
		return nil, fmt.Errorf("profile config: %w", err)
	}
	profileActions, err := attachHammerdinTownReady(combatProfileID, profileExecutor, bindings, inputCtrl)
	if err != nil {
		return nil, fmt.Errorf("profile config: %w", err)
	}
	inventoryLock, err := loot.NewInventoryLock(inventoryCells)
	if err != nil {
		return nil, fmt.Errorf("loot inventory lock: %w", err)
	}
	pickitProfiles, err := NewPickitProfileService(cfg.ResolvePath(filepath.Join("pickit", "profiles")))
	if err != nil {
		return nil, fmt.Errorf("pickit profiles: %w", err)
	}
	if setupErr := ValidateCharacterSetupConfig(cfg, pickitProfiles); setupErr != nil {
		return nil, fmt.Errorf("character setup config: %w", setupErr)
	}
	pickitAssignments, err := NewPickitAssignmentStore(cfg.ResolvePath("pickit-assignments.local.yaml"), pickitProfiles)
	if err != nil {
		return nil, fmt.Errorf("pickit assignments: %w", err)
	}
	pickit, err := loadRuntimePickitPolicy(opts, pickitAssignments, cfg.Session.Character, tasks.RunID(runtimeRunID))
	if err != nil {
		return nil, err
	}
	if runSelection.Run != "" {
		runTelemetry, err = telemetry.New(cfg.Telemetry.Directory, runSelection.Run, runSelection.Phase)
		if err != nil {
			return nil, fmt.Errorf("create run telemetry: %w", err)
		}
		log.Info("run telemetry enabled", "run_id", runTelemetry.RunID(), "path", runTelemetry.Path())
		profileTrace.setTelemetry(runTelemetry)
		townTrace.setTelemetry(runTelemetry)
	}
	lootFilter := loot.NewFilter(log, inventoryLock, pickit)
	townPreparation.setItemPolicies(lootFilter, cfg.Loot.Stash)
	stashExecutor, err := loot.NewStashExecutor(log, lootFilter, inputCtrl, mapLootStashConfig(cfg.Loot.Stash))
	if err != nil {
		return nil, fmt.Errorf("loot stash config: %w", err)
	}
	profileCfg := cfg.Profiles[combatProfileID]
	lootActions := newLootActionsAdapter(log, lootFilter, profileCfg.Resources, cfg.Loot.Pickup, inputCtrl, pathingCfg, stashExecutor, runTelemetry)
	routeLifecycle, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("route lifecycle: %w", err)
	}
	routePlayback := newRoutePlaybackAdapter(log, cfg.ResolvePath(cfg.Routes.FarmingRoot), expectedVersion, nav, runTelemetry, routeLifecycle)
	townEgress := newTownEgressAdapter(log, cfg, expectedVersion, inputCtrl, pathingCfg, runTelemetry)
	var cowSetup *cowSetupAdapter
	var cowRecipe *cowPortalRecipeAdapter
	if runtimeRunID == string(tasks.RunIDCows) {
		cowSetup, err = newCowSetupAdapterWithProfile(log, inputCtrl, nav, pathingCfg, cfg, runtimeRunID, selectedRunCfg, combatProfileID, townLayout, townTrace)
		if err != nil {
			return nil, fmt.Errorf("cow setup: %w", err)
		}
		cowRecipe, err = newCowPortalRecipeAdapter(log, inputCtrl, pathingCfg, cfg.Loot)
		if err != nil {
			return nil, fmt.Errorf("cow portal recipe: %w", err)
		}
		runCfg.Cow.HasTownServices = true
	}
	chestOperate := newChestOperateAdapter(log, inputCtrl, pathingCfg)
	taskDeps := tasks.Deps{
		Input: inputCtrl, Pathing: nav, Waypoint: runWaypoints, Portal: townPortals, TownWalk: layoutTownWalker,
		Stash: personalStash, Combat: combat, Actions: runActions, Loot: lootActions, Route: routePlayback, RouteClear: profileExecutor, TownEgress: townEgress, Profile: profileActions, Town: townPreparation, Cow: cowSetup, CowRecipe: cowRecipe, Chest: chestOperate,
	}
	// Do not assign a nil *telemetry.Recorder to the interface: that would make
	// the interface non-nil and turn the first fail-closed pipeline event into a
	// false telemetry failure. Session runs bind their per-generation recorder
	// in prepareSessionRun.
	if runTelemetry != nil {
		taskDeps.Telemetry = runTelemetry
	}
	var runtimeTrace *replay.Recorder
	if opts.RuntimeTraceCapture != "" {
		runtimeTrace, err = replay.NewRecorder(replay.Config{
			Enabled:   true,
			Directory: runtimeTraceDirectory(cfg),
			Label:     opts.RuntimeTraceCapture,
		}, replay.Metadata{BotVersion: version.Version, Commit: version.Commit}, runtimeTraceContract(cfg, opts, runSelection, runCfg))
		if err != nil {
			return nil, fmt.Errorf("runtime trace capture: %w", err)
		}
		taskDeps = replay.InstrumentDeps(taskDeps, runtimeTrace)
		log.Info("runtime trace capture enabled", "label", opts.RuntimeTraceCapture, "directory", runtimeTraceDirectory(cfg))
	}

	probe := memory.NewProbeReader(mem, offsetSet)
	probe.SetScannedCachePath(cfg.ResolvePath(memory.DefaultScannedCacheFile))
	if opts.WeaponSetProbe != "" || combatProfileID == hammerdinProfileID {
		if err := probe.ConfigureWeaponSetSkillEvidence(memory.MustSkillID("battle_orders"), memory.MustSkillID("battle_command")); err != nil {
			return nil, fmt.Errorf("configure weapon-set skill evidence: %w", err)
		}
	}

	rt = &Runtime{
		Config:            cfg,
		Options:           opts,
		Log:               log,
		logFile:           logFile,
		Process:           proc,
		Memory:            mem,
		Probe:             probe,
		UIProbe:           probe,
		HirelingProbe:     probe,
		ObjectInspect:     probe,
		World:             world.NewModel(log),
		Input:             inputCtrl,
		Bindings:          bindings,
		Tasks:             tasks.NewRunner(log, runSelection, runCfg, taskDeps),
		Pathing:           nav,
		Loot:              lootFilter,
		PickitProfiles:    pickitProfiles,
		PickitAssignments: pickitAssignments,
		Telemetry:         runTelemetry,
		Profile:           profileExecutor,
		RuntimeTrace:      runtimeTrace,
		profileTelemetry:  profileTrace,
		taskDeps:          taskDeps,
		runConfig:         runCfg,
		combatProfileID:   combatProfileID,
		compatibility: d2rCompatibilityContract{
			supportedVersion: memory.DefaultOffsetSet().D2RVersion,
			expectedVersion:  expectedVersion,
			offsetVersion:    offsetSet.D2RVersion,
		},
		sessionSelection: tasks.RunSelection{Run: cfg.Session.Run},
		routePlayback:    routePlayback,
		townEgress:       townEgress,
		lootActions:      lootActions,
		townLayout:       townLayout,
		townTelemetry:    townTrace,
		townPreparation:  townPreparation,
		stashExecutor:    stashExecutor,
	}
	rt.sessionReset = sessionResetBarrier{
		components: []sessionNamedResetter{
			{name: "town_layout", resetter: townLayout},
			{name: "navigator", resetter: nav},
			{name: "waypoint", resetter: waypoints},
			{name: "town_preparation", resetter: townPreparation},
			{name: "town_start_layout", resetter: layoutTownWalker},
			{name: "town_portal", resetter: townPortals},
			{name: "personal_stash", resetter: personalStash},
			{name: "combat", resetter: combat},
			{name: "loot", resetter: lootActions},
			{name: "route", resetter: routePlayback},
			{name: "town_egress", resetter: townEgress},
			{name: "profile", resetter: profileActions},
		},
		resetWorld: func(at time.Time, reason string) { rt.World.Reset(at, reason) },
	}

	if err := rt.verifyEnvironment(); err != nil {
		return nil, err
	}

	rt.Memory.Bind(proc)

	rt.verifyComponents()
	return rt, nil
}

// SessionExecutionRequested reports whether the configured farm queue may own
// the process. Specialized CLI modes such as --pathing-test and --route must
// return false even when session.enabled is true.
func SessionExecutionRequested(opts Options) bool {
	return !opts.Desktop && !opts.SessionInspect && !opts.RunsInspect && !opts.WaypointTargetsInspect && !opts.Probe && opts.InputTest == "" && opts.Run == "" && opts.RunPhase == "" && opts.RuntimeTraceCapture == "" && opts.ReplayRuntimeTrace == "" && opts.PathingTest == "" && opts.OfflineDifficulty == "" && opts.OfflineCharacter == "" && !opts.OfflineExitTest && opts.UIStateProbe == "" && opts.ScreenAnchorCapture == "" && opts.MercenaryProbe == "" && opts.CowProbe == "" && opts.WeaponSetProbe == "" && opts.ObjectInspect == "" && opts.Route == "" && !opts.TownInspect && opts.TownTest == ""
}

// CharacterLoadoutRequired reports whether Runtime construction needs a frozen character loadout.
func CharacterLoadoutRequired(opts Options) bool {
	if opts.Desktop || opts.SessionInspect || opts.RunsInspect || opts.WaypointTargetsInspect || opts.Probe || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.WeaponSetProbe != "" || opts.ObjectInspect != "" || opts.TownInspect {
		return false
	}
	if SessionExecutionRequested(opts) {
		return true
	}
	return opts.Run != "" || opts.RunPhase != "" || opts.InputTest != "" || opts.PathingTest != "" || opts.TownTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.Route != ""
}

func resolveRuntimeBindings(opts Options) (configBindingSource, error) {
	if opts.Loadout != nil {
		return cloneConfigBindingSource(opts.Loadout.Bindings), nil
	}
	if CharacterLoadoutRequired(opts) {
		return configBindingSource{}, fmt.Errorf("character loadout required")
	}
	return configBindingSource{skills: make(map[uint16]input.SkillCast)}, nil
}

func loadRuntimePickitPolicy(opts Options, assignments *PickitAssignmentStore, character string, runID tasks.RunID) (*loot.Pickit, error) {
	if !CharacterLoadoutRequired(opts) {
		// Idle desktop and route recording must start without a dummy Countess
		// assignment. Productive runs and town-tests stay fail-closed below.
		empty, err := loot.CompilePickitRules("unassigned pickit", nil)
		return empty, err
	}
	return loadEffectivePickitPolicy(assignments, character, runID)
}

func loadEffectivePickitPolicy(assignments *PickitAssignmentStore, character string, runID tasks.RunID) (*loot.Pickit, error) {
	if strings.TrimSpace(character) == "" {
		// Passive Diagnosemodi besitzen noch keinen bestätigten Charakterkontext.
		// Eine leere Policy bleibt fail-closed und ist keine Legacy-Autorität.
		empty, err := loot.CompilePickitRules("unassigned pickit", nil)
		return empty, err
	}
	effective, err := assignments.Resolve(character, runID)
	if err != nil {
		return nil, fmt.Errorf("pickit assignment invalid for %s/%s: %w", character, runID, err)
	}
	return effective.All, nil
}

// CloseLog closes the runtime log file when file logging is active.
func (rt *Runtime) CloseLog() error {
	if rt == nil {
		return nil
	}
	var telemetryErr error
	if rt.Telemetry != nil {
		telemetryErr = rt.Telemetry.Close()
		rt.Telemetry = nil
		if rt.townTelemetry != nil {
			rt.townTelemetry.setTelemetry(nil)
		}
		if rt.profileTelemetry != nil {
			rt.profileTelemetry.setTelemetry(nil)
		}
	}
	var logErr error
	if rt.logFile != nil {
		logErr = rt.logFile.Close()
		rt.logFile = nil
	}
	return errors.Join(telemetryErr, logErr)
}

func (rt *Runtime) verifyEnvironment() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("unsupported OS %q: bot targets Windows/D2R", runtime.GOOS)
	}
	rt.Log.Info("runtime environment ok",
		"goos", runtime.GOOS,
		"goarch", runtime.GOARCH,
		"num_cpu", runtime.NumCPU(),
	)
	return nil
}

func (rt *Runtime) verifyComponents() {
	components := map[string]bool{
		"process": rt.Process.Ready(),
		"memory":  rt.Memory.Ready(),
		"world":   rt.World.Ready(),
		"input":   rt.Input.Ready(),
		"tasks":   rt.Tasks.Ready(),
		"pathing": rt.Pathing.Ready(),
		"loot":    rt.Loot.Ready(),
		"profile": rt.Profile.Ready(),
	}

	for name, ready := range components {
		rt.Log.Info("component ready", "name", name, "ready", ready)
	}
}

func (rt *Runtime) Run() error {
	rt.Log.Info("d2rbot started",
		"version", version.Version,
		"commit", version.Commit,
		"poll_interval_ms", rt.Config.Runtime.PollIntervalMs,
		"target_process", rt.Config.Process.ProcessName,
		"probe_enabled", rt.Options.Probe,
		"verbose", rt.Options.Verbose,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt.startShutdownSignals(ctx, cancel)

	defer func() {
		if err := rt.Process.Detach(); err != nil {
			rt.Log.Error("process detach failed", "error", err)
		}
	}()
	defer rt.Input.Unbind()

	hotkeyEvents, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)

	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	state := &runState{}

	for {
		select {
		case <-ctx.Done():
			rt.Log.Info("shutting down")
			result := rt.Tasks.Result()
			return rt.finalizeRuntimeTrace(replay.Terminal{Step: result.Step, Outcome: "failed", Reason: "operator_stop"})
		case ev := <-hotkeyEvents:
			rt.handleHotkeyEvent(ev, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil {
				if errors.Is(err, context.Canceled) {
					result := rt.Tasks.Result()
					return rt.finalizeRuntimeTrace(replay.Terminal{Step: result.Step, Outcome: "failed", Reason: "operator_stop"})
				}
				rt.Log.Error("run loop stopped", "error", err)
				result := rt.Tasks.Result()
				return errors.Join(err, rt.finalizeRuntimeTrace(replay.Terminal{Step: result.Step, Outcome: "failed", Reason: err.Error()}))
			}
			if done, err := rt.configuredTaskResult(); done {
				return err
			}
		}
	}
}

// configuredTaskResult ends the generic poll loop only for an explicitly
// selected run. Passive probe mode remains active until an operator stops it;
// session mode owns its separate multi-run lifecycle.
func (rt *Runtime) configuredTaskResult() (bool, error) {
	if rt.Tasks.ConfiguredRun() == "" || !rt.Tasks.Terminal() {
		return false, nil
	}

	result := rt.Tasks.Result()
	if result.Outcome == tasks.RunOutcomeSuccess {
		return true, rt.finalizeRuntimeTrace(replay.Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason})
	}

	runErr := fmt.Errorf(
		"task run %s phase %s failed at step %s: %s",
		rt.Tasks.ConfiguredRun(),
		rt.Tasks.ConfiguredPhase(),
		result.Step,
		result.Reason,
	)
	return true, errors.Join(runErr, rt.finalizeRuntimeTrace(replay.Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason}))
}

func (rt *Runtime) startShutdownSignals(ctx context.Context, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			rt.Log.Info("shutdown signal received")
			rt.Input.Stop("signal")
			cancel()
		case <-ctx.Done():
			signal.Stop(sigCh)
		}
	}()
}

func (rt *Runtime) startHotkeys(ctx context.Context) (<-chan input.HotkeyEvent, error) {
	if err := rt.waitForCompatibility(ctx); err != nil {
		return nil, err
	}
	hotkeyEvents := make(chan input.HotkeyEvent, 4)
	hotkeyReady := make(chan error, 1)
	rt.Input.ListenHotkeys(ctx, hotkeyEvents, hotkeyReady)

	if err := <-hotkeyReady; err != nil {
		return nil, fmt.Errorf("hotkey listener: %w", err)
	}
	return hotkeyEvents, nil
}

func (rt *Runtime) stopHotkeys(cancel context.CancelFunc) {
	cancel()
	if waiter, ok := rt.Input.(interface{ WaitHotkeys() }); ok {
		waiter.WaitHotkeys()
	}
}

func (rt *Runtime) logProcessStateChange(prev, next process.State) {
	if prev == next {
		return
	}

	st := rt.Process.Status()
	switch next {
	case process.StateAttached:
		rt.Log.Info("process attached",
			"pid", st.PID,
			"process", st.Process,
			"module_base", fmt.Sprintf("0x%X", st.ModuleBase),
		)
	case process.StateLost:
		rt.Log.Info("process lost",
			"pid", st.PID,
			"process", st.Process,
		)
	case process.StateDetached:
		rt.Log.Info("process detached", "process", st.Process)
	}
}

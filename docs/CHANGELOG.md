# Changelog

Alle wesentlichen Änderungen am **D2R Offline Farming Bot** werden hier dokumentiert.

Format basiert auf [Keep a Changelog](https://keepachangelog.com/),
Versionierung nach [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Add the Phase 8 implementation plan for generic character and encounter hooks with three focused manual acceptance runs
- Complete Phase 7.8 with three consecutive autonomous Nightmare Countess cycles covering start, navigation, kill, loot, stash, and Save & Exit
- Add terminal route segments so recorded navigation can end at the Countess room without a synthetic area transition
- Re-record and validate the active `MrBones` Nightmare town walk and six-segment Countess route after invalidating stale Countess layouts
- Validate one complete playback of the new Nightmare route from Black Marsh to Tower Cellar Level 5
- Add the route-independent Phase 7.8 finite multi-run core with cooldown, duration/run limits, unique game/run IDs, recovery decisions, and terminal summaries
- Cover three successful cycles, one budgeted hard-stuck restart, and terminal unknown failure without relying on invalidated route recordings
- Add Phase 7.7 exact sentinel-based recovery classification with hard consecutive-failure and total-restart budgets
- Add schema-v2 session lifecycle JSONL with correlated session, game, and run IDs plus terminal summaries
- Add structured hard-stuck route, segment, point, drift, target, and local-recovery context
- Add fail-closed hard-stuck ordering from `stuck_detected` through `run_aborted` to one recovery decision
- Add Phase 7.6 per-game verification with generation-bound fresh snapshots for character, version, town, and closed UI
- Add a mandatory cycle reset barrier for World, Navigator, Route, Loot, Combat, Stash, Waypoint, Portal, and Town-Walk state
- Rebuild and validate the authoritative route layout at every Black Marsh route start before RoutePlayer or navigation activation
- Add Phase 7.5 autonomous-session YAML with explicit opt-in, finite budgets, retry classes, and restrictive zero-value support
- Add mutually exclusive `--session-inspect` JSON plan resolution before Runtime creation, process attach, hotkeys, or input
- Add static session preflight for full-run registration, route binding, character anchors, difficulty, game version, and input opt-in
- Add the Phase 7.4 generic single-cycle session orchestrator with a fresh run executor per cycle
- Add fail-closed lifecycle action gates, reset-before-exit ordering, and canonical run outcome events
- Cover three successful fake cycles, run failure, hard-stuck reset, pause, stop, loading timeout, and telemetry failure
- Add the Phase 7.3 screen- and Memory-gated offline game start with explicit character verification
- Add versioned narrow character, Play, and difficulty-dialog anchors for the supported 1280x720 frontend
- Add read-only D2R client capture through `github.com/kbinani/screenshot`
- Validate three complete Phase 7.3 Nightmare starts with exactly one Play and one difficulty click each
- Add the Phase 7.2 isolated Memory-gated offline Save & Exit test with single-action invariants
- Add stable town, quit-menu, geometry, and menu-arrival gates for offline game exit
- Add a verified D2R foreground-focus guard for keyboard-sensitive lifecycle actions
- Validate three complete Phase 7.2 offline exits with exactly one Escape press and one Save & Exit click each
- Add and live-validate the Phase 7.1 read-only `QuitMenuOpen` flag at `UI-0xB`
- Validate the Phase 7.1 UI-state matrix and select Memory plus narrow screen anchors for offline frontend control
- Add the Phase 7.1 read-only UI-state capture CLI with stable and volatile byte classification
- Add atomically published local UI-buffer research artifacts with raw samples, known state, and SHA-256 fingerprint
- Define the Phase 7.0 session lifecycle contract with finite run, time, retry, and restart budgets
- Define fail-closed lifecycle states, hard-stuck abort semantics, correlated session telemetry, and UI input invariants
- Add Phase 6.7 Countess route adapter and `runs.countess.route_id` without a best-effort Explorer fallback
- Validate the Phase 6.7 Countess adapter live across all six recorded segments from Black Marsh to Tower Cellar Level 5
- Validate ten complete Phase 6 route playbacks in the bound Nightmare layout, including one Countess adapter run
- Add Phase 6.6 full route playback in one verified session with correlated route, segment, point, transition, stop, and failure telemetry
- Add Phase 6.5 strict route transitions with semantic entrance selection, runtime UnitID pinning, bounded recovery, and Area-only success
- Add Phase 6.4 isolated route segment playback with Memory-confirmed waypoints, drift limits, bounded corrections, and strict transitions
- Add Phase 6.3 read-only route recorder with World-coordinate sampling, confirmed area transitions, pause, and Stop-only publication
- Record and validate the first six-segment `MrBones` Nightmare route from Black Marsh to Tower Cellar Level 5
- Add Phase 6.2 generic Route Contract v1 types, YAML storage, registry, validator, compatibility precheck, and read-only route CLI
- Add Phase 6.1b three-snapshot character identity stabilization and confirmed World Model mapping
- Add Phase 6.1c isolated controlled offline difficulty selection for the prepared 1280×720 character screen
- Add Phase 6.1d deterministic layout fingerprints and read-only `--pathing-test inspect:layout` diagnostics
- Validate controlled Hell/Nightmare selection and stable cross-game Nightmare layout fingerprints against a distinct Hell layout
- Add Phase 6.1a read-only identity research probe for validated character name, class ID, and reconstructed offline map seed sources
- Define Phase 6.0 generic Route Contract v1 with stable route IDs, Memory-confirmed game-identity binding, segment invariants, sampling rules, failure classes, and a future CLI contract
- Plan Phase 6.1 read-only Game Identity for character name, class, and actual difficulty before route recording or playback
- Add Phase 5.10 fail-closed per-run JSONL telemetry for drop, Pickit, pickup, inventory-full, and stash events
- Add Phase 5.9 identification policy with stat-rule gating and explicit `identify_required` Keep/Stash decisions
- Add Phase 5.8 Personal Stash automation with Memory-gated town walking, protected Ctrl+LMB transfers, per-item verification, and clean UI close
- Add live-calibrated Inventory/Stash UI flags and an isolated `--phase stash-personal` E2E workflow
- Add generated inventory dimensions for every local item-catalog entry and enforce `1x1` for all local gem and skull rows
- Add Phase 5.7 inventory-full recovery with hover-confirmed Town Portal entry and verified Rogue Encampment arrival
- Add a versioned local `objects.txt` generator for synchronized Memory and World object IDs
- Add Phase 5.6 Countess loot phase with `wait_for_drops`, `scan_loot`, `pick_loot`, and isolated `--phase loot-countess`
- Add Phase 5.5 hover-confirmed item pickup with `loot.pickup` safety limits and `--pathing-test pickup:item`
- Add Phase 5.4 Loot Decision Pipeline with read-only stage decisions for Pickit matches, pickup candidates, Keep/Stash, and failure reasons
- Add Phase 5.3 Pickit MVP with a small NIP subset, `loot.pickit_file`, and Countess default rules
- Add Phase 5.2 read-only inventory model and `loot.inventory_lock` capacity guard
- Add read-only item enumeration from memory into the world model
- Add Phase 5.0 loot and recovery concept documentation covering Ground-Loot, Pickit, Inventory-Lock, Stash safety, and recovery slices

### Changed
- Document planned `internal/profile`, `internal/town`, `internal/api`, and `web` boundaries without pre-creating packages or moving the stable Phase 7 runtime
- Extend the Phase 8 plan with a profile-driven HP, mana, and rejuvenation resource policy while keeping manual acceptance at three combined live runs
- Insert act-aware Town Services as Phase 9 with service discovery, anchor-route graphs, validated waypoint act changes, demand-driven preparation, and fail-closed unknown-town handling; renumber later roadmap phases through Phase 15
- Update the central roadmap through Phase 15 with separate goals, scope, and acceptance gates for character profiles, Town Services, a second farm target, GUI, route management, Pickit editing, telemetry, and packaging
- Extend the Phase 6.1c difficulty selector into the canonical Phase 7.3 full offline-start flow without duplicating its click primitive
- Set the Phase 7 live acceptance baseline to three successful repetitions per isolated flow and three complete multi-run cycles
- Select the Act-1 town waypoint route by configured difficulty and fail safely when its recording is unavailable
- Rename the implemented stash decision reason from `stash_not_implemented` to `stash_candidate`
- Extend `loot-countess` and the full Countess run from verified town arrival through Personal Stash completion
- Expand read-only item probe diagnostics with generated item catalog type data and filtered verbose ground-item hints
- Regenerate the item catalog from local D2R `3.2.92777` data so current Countess drops resolve correctly

### Fixed
- Keep Windows global hotkey registration, message polling, and unregistration on one OS thread and wait for release between session stages
- Allow explicit diagnostic and route-recording modes while an enabled Phase-7 session remains execution-gated
- Block offline lifecycle input when D2R foreground activation is not confirmed
- Recover bounded route drift by returning to the last confirmed recorded point without widening the drift limit
- Prevent repeated entrance clicks while waiting for Loading or the expected Area transition
- Align navigator goals with per-route waypoint tolerance to prevent repeated arrival at a stricter playback waypoint
- Require three stable no-target loot scans so transient item reads cannot leave a second Pickit match behind
- Fix the Countess good-chest object ID by generating `PlaceUniqueChest` from local D2R `3.2.92777` data
- Increase the item snapshot cap so Countess ground drops are not hidden behind inventory/history item units

## [0.4.0] - 2026-07-04

### Added
- Add Countess run orchestration 4.7: full `--run countess` flow from Act-1 town through Countess kill, safety potion guard, and Town Portal completion
- Add Countess kill phase 4.6: `--run countess --phase kill-countess`, Necro Bone Spear combat, Good Chest position fallback, and defensive kill confirmation without loot pickup
- Add automatic runtime log files under `logs/` so verbose manual test runs remain inspectable after the terminal buffer scrolls away
- Add read-only `--pathing-test inspect:entrances` to capture player position, hover state, and visible entrance coordinates during manual calibration
- Add Countess tower traversal phase 4.5: `--run countess --phase travel-cellar5`, best-effort Forgotten Tower search, and hover-confirmed cellar transitions through Tower Cellar Level 5
- Add Act-1 town waypoint acquisition (Phase 4.5): Force Move town walking from Rogue Encampment stash/spawn toward the waypoint, optional route record/play pathing tests, and Countess `acquire_town_waypoint` step
- Add Countess travel phase 4.4: `--run countess --phase travel-marsh`, hover-confirmed Town-Waypoint click, fixed Black-Marsh waypoint UI selection, and loading-safe arrival wait
- Add teleport-based pathing (Phase 4.3): player-relative isometric projection, `Navigator` state machine with bearing explore and stuck detection, and `TeleportMover` using YAML teleport bindings
- Add memory hover read via d2go signature scan (`HoverState` in snapshot, `Hover` in world state, per-entity `IsHovered` matching)
- Add `EntityClicker` hover-feedback click loop: spiral search around the projected point, click only after confirmed hover match (fail-closed, no blind clicks)
- Add `--pathing-test` CLI mode (`teleport:TX,TY`, `hover:watch`, `move-area:<id|name>`, `click-entity:waypoint|entrance`) with `--pathing-test-timeout-ms`
- Add `pathing` config section (stuck, projection, click, explore tuning) with defaults and validation; warn when the window deviates from the recommended 1280×720
- Add `world.ParseAreaSpec` for resolving area names or numeric IDs from CLI specs
- Add YAML-driven skill, town portal, and belt bindings with `CastSkillAt`/`SelectSkill`/`CastBelt` and teleport precheck
- Add minimal unit enumeration (objects, entrances, monsters) and GamePhase to memory probe and world model (Phase 4.2)
- Add entity fingerprint world logging with object/entrance/monster counts; block task ticks during loading phase
- Add task runner framework with lazy run start, step timeouts, and Countess stub run (Phase 4.1)
- Add `--run` CLI flag and `runs` config section (`active`, `step_timeout_ms`)

### Changed
- Mark Countess phase 4.5 tower traversal as MVP-complete but intentionally best-effort until route recording/playback provides deterministic paths
- Replace memory-read keybindings with explicit `input.bindings` YAML for skills, town portal, and belt slots
- Remove keybinding offset scanning, cache validation, diagnostics, and hotkey calibration from the runtime path

### Fixed
- Ignore implausibly distant route entrances so stale cellar transition units from the previous area do not stop the next traversal step
- Clamp teleport casts to the playable client area and reject entity-click projections inside the bottom UI panel
- Fall back from a blocked visible-entrance approach to hover-clicking that entrance so Tower Cellar down exits are not ignored behind room geometry
- Enumerate unknown entrance units and make `enter_cellar_1` prefer the hidden Forgotten Tower antechamber entrance instead of known back-to-surface entrances
- Classify observed Tower Cellar entrance IDs `8`/`9` as up/down so cellar traversal approaches the visible down exit instead of continuing bearing exploration
- Allow `travel-cellar5` to resume from Black Marsh, Forgotten Tower, or Tower Cellar route areas during manual testing instead of requiring a town start every time
- Prioritize visible Countess-route entrances before bearing exploration and stop `find_tower` when it leaves Black Marsh for an unexpected area
- Fix bearing explore spinning in circles: judge a teleport cast as blocked only after a settle timeout (cast animation delays the memory position update), confirm progress immediately for fast cast chaining
- Fix entity enumeration: walk entrances and monsters before the large object segment; do not require `unitData` for entrances (aligned with d2go); treat unreadable unit-table segments as empty instead of discarding already-enumerated entities
- Fix unreliable offset pattern scan on bot restart: page-wise module image read for signatures, scan retries, persisted `configs/offsets.scanned.yaml` cache, and no permanent fallback lock-in on failed scans
- Improve Countess detection: enumerate super-unique monsters (flag 10) regardless of NPC id; use remaining visit budget for monster segment walk

## [0.3.0] - 2026-06-26

### Added
- Add manual CLI input-test mode for validating keyboard and mouse primitives in-game (Phase 3.5)
- Add input safety controls, global pause/stop hotkeys, and action logging (Phase 3.4)
- Add client-relative mouse movement and click primitives for the input controller (Phase 3.3)
- Add configurable keyboard primitives for the input controller using Windows SendInput (Phase 3.2)
- Add D2R window binding for the input controller using PID and client-area discovery (Phase 3.1)

### Fixed
- Avoid defaulting town portal to `t`, which opens the D2R skill tree.

### Removed
- Remove GitHub Actions release workflow; releases are built locally via `scripts/build-release.ps1`

## [0.2.0] - 2026-06-25

Erstes veröffentlichtes Release: Phase 1 (read-only Memory/Probe) und Phase 2 (World Model) abgeschlossen. Validiert mit D2R `3.2.92777`.

### Added
- Add world-state logging for named areas, player percentages, and position (Phase 2.3)
- Add snapshot-to-world-state mapper and current world model state storage (Phase 2.2)
- Add world domain types and embedded area catalog in `internal/world` (Phase 2.1)
- Add `memory` config section with `game_version` and `offsets_file` for optional YAML offset overrides
- Add `process.attach_timeout_ms` to limit initial wait for first D2R attach (`0` = unlimited)
- Add `configs/offsets.example.yaml` and `internal/memory/offsets_file.go` (hex YAML overlay on `DefaultOffsetSet`)
- Add `--probe` and `--verbose` CLI flags for Phase-1 state probing
- Add `memory.Snapshot` as the raw Phase-1 probe data model
- Add runtime d2go pattern scan for probe module offsets (UnitTable, UI, Expansion) to fix patch-specific static offset mismatches
- Add read-only state probe in `internal/memory` with versioned `OffsetSet`, `ProbeReader`, and minimal Life/Mana stat parsing via UnitTable (d2go reference `16d248a53591`)
- Add sparse CLI probe logging in `internal/app` with change detection, 5s heartbeat, and reset on process lost
- Add low-level read-only memory reader in `internal/memory` with `ReadBytes`, `ReadUint32`, `ReadUint64`, and pointer-chain resolution
- Add `process.Service.ReadAt` backed by `ReadProcessMemory` with typed read errors and mutex-protected handle access
- Add read-only D2R process detection in `internal/process` with attach/poll/detach lifecycle
- Add `golang.org/x/sys/windows` dependency for Windows process and module enumeration
- Add Go project scaffold with `cmd/d2rbot` and `internal/*` package layout
- Add YAML config loading and structured logging (`slog`)
- Add Cursor project rules and feature documentation structure

### Changed
- Update app loop to refresh world state on every attached poll and make `--probe` control world-state logging (Phase 2.3)
- Remove raw `StatsSource` from operator world logs; logs use semantic `world.State` fields instead
- Complete Phase 2: manual World Model validation for Countess route (Sessions A–C)
- Add release build script and embedded app version (`--version`)
- Complete Phase 1: config-driven offsets, attach timeout, startup logging for offset set and game version
- Prefer `BaseStats` for probe HP/Mana values; suppress position-only probe Info logs unless `--verbose` (Debug)

---

## [0.1.0] - 2026-06-25

### Added
- Initial repository scaffold (Phase 0)

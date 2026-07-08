# Changelog

Alle wesentlichen Änderungen am **D2R Offline Farming Bot** werden hier dokumentiert.

Format basiert auf [Keep a Changelog](https://keepachangelog.com/),
Versionierung nach [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Add Phase 5.4 Loot Decision Pipeline with read-only stage decisions for Pickit matches, pickup candidates, Keep/Stash, and failure reasons
- Add Phase 5.3 Pickit MVP with a small NIP subset, `loot.pickit_file`, and Countess default rules
- Add Phase 5.2 read-only inventory model and `loot.inventory_lock` capacity guard
- Add read-only item enumeration from memory into the world model
- Add Phase 5.0 loot and recovery concept documentation covering Ground-Loot, Pickit, Inventory-Lock, Stash safety, and recovery slices

### Changed
- Expand read-only item probe diagnostics with generated item catalog type data and filtered verbose ground-item hints
- Regenerate the item catalog from local D2R `3.2.92777` data so current Countess drops resolve correctly

### Fixed
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

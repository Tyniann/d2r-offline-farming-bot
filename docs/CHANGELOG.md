# Changelog

Alle wesentlichen Änderungen am **D2R Offline Farming Bot** werden hier dokumentiert.

Format basiert auf [Keep a Changelog](https://keepachangelog.com/),
Versionierung nach [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.3.0] - 2026-06-26

### Added
- Add manual CLI input-test mode for validating keyboard and mouse primitives in-game (Phase 3.5)
- Add input safety controls, global pause/stop hotkeys, and action logging (Phase 3.4)
- Add client-relative mouse movement and click primitives for the input controller (Phase 3.3)
- Add configurable keyboard primitives for the input controller using Windows SendInput (Phase 3.2)
- Add D2R window binding for the input controller using PID and client-area discovery (Phase 3.1)

### Fixed
- Avoid defaulting `input.town_portal` to `t`, which opens the D2R skill tree.

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

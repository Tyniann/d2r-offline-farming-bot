# Changelog

Alle wesentlichen Änderungen am **D2R Offline Farming Bot** werden hier dokumentiert.

Format basiert auf [Keep a Changelog](https://keepachangelog.com/),
Versionierung nach [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
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
- Complete Phase 1: config-driven offsets, attach timeout, startup logging for offset set and game version
- Make state probing opt-in via `--probe`; default run only monitors process attach/lost
- Prefer `BaseStats` for probe HP/Mana values; suppress position-only probe Info logs unless `--verbose` (Debug)

---

## [0.1.0] - 2026-06-25

### Added
- Initial repository scaffold (Phase 0)

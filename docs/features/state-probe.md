# State Probe

## Überblick

Phase-1 State-Probe über minimale D2R-Offsets. Der Bot findet den Main-Player über die UnitTable (d2go-Muster) und mappt HP, MaxHP, Mana, MaxMana, Area-ID sowie Position in `world.State`. Ab Phase 2.3 liest der App-Loop Memory-Snapshots nach jedem erfolgreichen `Poll()` im attached-Zustand; `--probe` steuert nur noch semantisches World-State-Logging. Keine Items, keine Monsterliste.

## Ort im Code

- **Paket:** `internal/memory/` (Reads), `internal/world/` (Mapping), `internal/app/` (Loop + Logging)
- **Einstieg:** [`internal/app/run_tick.go`](../../internal/app/run_tick.go) → `Probe.Snapshot()` und `World.Update()` nach `Process.Poll()` im attached-State
- **Wichtige Dateien:**
  - `offsets.go` — versioniertes `OffsetSet`, `DefaultOffsetSet()`
  - `offsets_file.go` — optionales YAML-Overlay (`LoadOffsetSetFile`, `ResolveOffsetSet`)
  - `scan.go` — Runtime Pattern-Scan für Modul-Offsets (`ensureOffsets`)
  - `probe.go` — `ProbeReader`, `Snapshot`, UnitTable-Scan
  - `stats.go` — minimaler Life/Mana-Stat-Parser
  - `world_log.go` (app) — sparsames CLI-Logging auf `world.State` mit Heartbeat und Verbose-Positionslogs
  - `run_tick.go` (app) — testbare Loop-Iteration
- **Config:** `configs/config.example.yaml` → `memory.game_version`, `memory.offsets_file`; Overrides in `configs/offsets.example.yaml` (lokal z. B. `configs/offsets.local.yaml`, gitignored)

## Funktionalität

### CLI / Runtime-Optionen

| Flag | Wirkung |
|------|---------|
| (default) | Prozess-Attach/Poll/Lost; Memory-Snapshot-Reads und `World.Update` nach jedem attached `Poll()`, **kein** World-State-Log |
| `--probe` | Zusätzlich semantisches World-State-Logging (`world state` / `world unavailable`) |
| `--verbose` | Global Debug-Logging (`app.log_level` wird überschrieben) |
| `--probe --verbose` | World-Logging aktiv; Positionsänderungen zusätzlich auf Debug |

Startup-Log enthält `probe_enabled` (World-State-Logging-Schalter), `verbose` sowie `offset configuration` (`game_version`, `offset_set`, `offsets_file`, `attach_timeout_ms`).

### Offset-Konfiguration

| Key | Bedeutung |
|-----|-----------|
| `memory.game_version` | Erwartete D2R-Version für den Offset-Stand; leer = Version aus aktivem Offset-Set |
| `memory.offsets_file` | Relativer/absoluter Pfad zu YAML-Overrides; leer = eingebautes `DefaultOffsetSet()` |
| `configs/offsets.example.yaml` | Versioniertes Beispiel (Hex-Strings oder Dezimal) |
| `configs/offsets.local.yaml` | Lokale Overrides (gitignored via `configs/*.local.yaml`) |

Beim Start lädt `app.New()` optional die Offset-Datei und legt sie über `DefaultOffsetSet()` drüber. Abweichung zwischen `memory.game_version` und `d2r_version` im Offset-Set erzeugt nur eine Warnung, keinen Abbruch.

### Offset-Recherche

Referenz (gepinnt, nicht als Runtime-Dependency):

| Feld | Wert |
|------|------|
| Quelle | `github.com/hectorgimenez/d2go` |
| Commit | `16d248a53591` (Modul `v0.0.0-20251023061335-16d248a53591`) |
| Koolo | nutzt dieselbe d2go-Abstraktion; Main-Player-Kriterium aus `GetRawPlayerUnits` |

d2go ermittelt Modul-Offsets per Pattern-Scan zur Laufzeit. Dieses Projekt nutzt dieselben Signaturen für `UnitTable` und `UI`; `Expansion` wird ebenfalls versucht, ist aber optional. `DefaultOffsetSet()` bleibt als Fallback und dokumentiert zusätzlich die **Struct-Feld-Offsets** aus d2go (`player.go`, `game_reader.go`). `ensureOffsets()` scannt das Modul seitenweise und überspringt unlesbare Bereiche, weil große `ReadProcessMemory`-Reads über das gesamte Image auf manchen D2R-Builds fehlschlagen.

### Main-Player-Erkennung

Wie d2go `GetRawPlayerUnits` / `GetMainPlayer`:

1. 128 Buckets ab `moduleBase + UnitTable`, je Bucket eine verkettete Unit-Liste (`NextUnit` bei `+0x158`)
2. Inventory-Pointer bei Unit `+0x90`
3. Main-Player-Flag: `inventory+0x30` (ohne Expansion) oder `inventory+0x70` (mit Expansion)
4. Expansion-Check über `moduleBase + Expansion` → Char-Flag `+0x5C`; falls der Offset nicht sicher lesbar ist, prüft die Probe beide Flags
5. Early-Exit nach erstem Main-Player-Treffer; Loop-Schutz pro Bucket und gesamt

### InGame-Gate

d2go `IsIngame`: Byte bei `moduleBase + UI - 0xA` ist der erste Hinweis. Weil dieser Gate in aktuellen D2R-Builds auch im Spiel `0` liefern kann, blockiert er die Probe nicht mehr hart: Der Bot versucht trotzdem den Main-Player zu lesen und meldet erst `not_in_game`, wenn Gate `0` ist und kein Player-Snapshot möglich ist.

### Vital-Stats

Stat-Liste am Unit `+0x88`: Die Probe bevorzugt `BaseStats` bei `statsListEx + 0x30` und fällt nur bei fehlenden Werten auf die aktive Liste bei `statsListEx + 0xA8` zurück. d2go verwendet `BaseStats` für `PlayerUnit.HPPercent()`, daher ist diese Reihenfolge für HP/Mana wichtig.

- Header: `[+0]` Array-Pointer, `[+8]` Eintragsanzahl
- Eintrag 8 Byte: Layer (uint16), ID (uint16), Wert (int32)
- IDs: Life=6, MaxLife=7, Mana=8, MaxMana=9 (d2go `pkg/data/stat`)
- Life/Mana-Skalierung: `value >> 8`
- `HP`/`Mana` sind die aktuellen Werte. `MaxHP`/`MaxMana` werden als effektiver beobachteter Max-Wert geführt, weil D2R `MaxLife`/`MaxMana` als unmodifizierte Basiswerte liefern kann, während Equipment/Boni bereits im aktuellen Wert sichtbar sind.
- Fehlt einer der vier Werte → `Valid=false`, `reason=stats_unavailable`

### memory.Snapshot

| Feld | Bedeutung |
|------|-----------|
| `Valid` | Alle Pflichtfelder lesbar |
| `Reason` | Technischer Grund bei `Valid=false` |
| `StatsSource` | `base` oder `active`, Quelle der Vitalwerte (Memory-only, nicht im World-Log) |
| `HP`/`MaxHP`/`Mana`/`MaxMana` | Dekodierte Anzeigewerte |
| `AreaID` | Rohe Area-ID aus Level-Struct |
| `PosX`/`PosY` | `uint32` in `memory.Snapshot` (aus uint16 Path-Reads erweitert; `world.Position` gleicher Typ) |

Reason-Konstanten: `not_attached`, `not_in_game`, `unit_table_unavailable`, `player_pointer_unavailable`, `stats_unavailable`, `read_failed`.

## Operator / CLI

```powershell
go run ./cmd/d2rbot
go run ./cmd/d2rbot --probe
go run ./cmd/d2rbot --probe --verbose
```

Mit `--probe` erscheinen nach `process attached` sparsame World-State-Logs:

```text
world state phase=in_game area_name=... area_id=... act=act1 area_kind=... hp=... max_hp=... hp_pct=... mana=... max_mana=... mana_pct=... pos_x=... pos_y=...
world unavailable reason=...
```

Regeln:

- Log sofort bei HP-/Mana-/Area-/Phase-Änderung; reine Positionsänderungen nur mit `--probe --verbose` (Debug)
- Heartbeat alle 5 s (fest, kein Config-Key)
- Ungültige Heartbeats auf Debug; neuer Reason einmalig auf Info
- Snapshot-Read nur im attached-Zustand nach `Poll()`; bei `process lost` kein Read, World-State wird auf `process_lost` zurückgesetzt
- Re-Attach erzwingt einmaliges Log (`force`)
- Bei `poll_interval_ms=100` kann `--probe --verbose` während des Laufens bis zu ca. 10 Debug-Zeilen/s erzeugen (Diagnose-Modus)
- `StatsSource` erscheint nicht mehr im Operator-Log (semantische World-Logs statt Roh-Probe)

### Manuelle Validierung (Offline/Singleplayer)

Phase-1 Low-Level-Validierung (Attach, Offset-Scan, rohe Snapshots):

**Validiert mit D2R-Version:** `3.2.92777` (`D2R.exe`, ProductVersion/FileVersion), `VerifiedAt: 2026-06-25`. Bestätigt: Attach, runtime Offset-Scan (`UnitTable=0x1EB9430`, `UI=0x1EC9134`), Main-Player-Snapshot, Area-ID, rohe Position X/Y und aktuelle HP/Mana mit beobachtetem effektivem Max-Wert.

Semantische World-State-Validierung (Countess-Route, Area-Namen, `hp_pct`, Log-Policy): siehe [World Model — Validierung](world-model.md#validierung-phase-24).

## Grenzen

- Modul-Offsets (`GameData`, `UnitTable`, `UI`, `Expansion`) sind patch-sensitiv; falsche Werte → invalid statt geratener Anzeige
- Positionen sind Rohwerte, keine normalisierten Weltkoordinaten
- Kein Battle.net / Online-Modus
- `ReadAt`-Mutex: Probe-Reads blockieren kurz `Poll`/`Detach` — deshalb immer zuerst `Poll()`, dann Probe

### Phase 1 vs. Phase 2

| Phase 1 | Phase 2 (2.3) |
|---------|---------------|
| `memory.Snapshot` mit rohen IDs und `uint32`-Positionen | `world.State` / `world.Model` mit benannten Areas |
| Opt-in `--probe` für Roh-Snapshot-Logging | Kontinuierliches World-Update im App-Loop; `--probe` nur für Logs |
| Offset-Set + optional YAML-Override | Interpretation und Validierung im World-Paket |
| Keine Input-Aktionen | Navigation, Kampf, Loot folgen später |

## Abhängigkeiten

- [Memory Reader](memory-reader.md) — Primitive Reads
- [Process Detection](process-detection.md) — Attach, Modulbasis
- [World Model](world-model.md) — semantischer State aus Snapshots
- Recherche: hectorgimenez/d2go @ `16d248a53591`, Koolo/dulingzhi-Mirror

## Verwandte Features

- [Memory Reader](memory-reader.md) — Low-Level-Reads (verweist hierher für Spiel-Semantik)
- [World Model](world-model.md) — Domain-Typen und kontinuierliches Update im App-Loop

---
*Zuletzt aktualisiert: 2026-06-25*

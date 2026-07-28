# Changelog

Alle wesentlichen Änderungen am **D2R Offline Farming Bot** werden hier dokumentiert.

Format basiert auf [Keep a Changelog](https://keepachangelog.com/),
Versionierung nach [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
- Recover town-portal entry once via TeleportToward after `too_far` or `hover_not_found` so Bone-Prison blockers cannot abort the return
- Close an open telemetry step on emergency cancel, map `emergency_stop_requested` to `run_aborted`, and isolate corrupt history runs in AnalyzeHistory so one bad file cannot blank the whole Historie
- Serialize cleared queue `entries` as an empty JSON array and null-guard the Dashboard Core-Queue render so F11 no longer crashes the UI

## [0.12.0] - 2026-07-28

### Added
- Add the Phase 16 core contract with a locally characterized bounded D2R v105 save prefix, canonical Warlock class, character-setup ownership, stable reason codes, Pickit defaults, and explicit non-goals
- Add a reparse-safe read-only D2R v105 character-save prefix reader and revisioned catalog with isolated failures, canonical class projection, and no run-profile fallback
- Add developer-approved combat-profile setup metadata, exact validated Pickit defaults, OperatorSettings schema 2 with protected character profile pairs, and idempotent missing-assignment creation
- Add the Core-owned character-setup preview/confirm/capture API, atomic selection-image capture, generated React contracts, inline onboarding flow, and exact catalog invalidation

### Changed
- Require freshly matched save class, persisted compatible combat profile, requested-run profile, and run-scoped Pickit assignment before desktop selection, queue start, or run input
- Raise the default run `step_timeout_ms` from 30s to 45s so longer Hell boss engages like Mephisto are less likely to abort at kill confirmation

### Fixed
- Refresh the Dashboard immediately after saved operator settings, project the new idle queue and budgets consistently, and replace raw run-availability codes with clear German guidance.
- Resolve Act 1 to Rogue Encampment when preparing isolated Countess candidate playback instead of passing it through the foreign-act-only egress registry.
- Capture the visibly selected D2R roster row instead of always storing the first row as character-specific selection evidence.
- Exclude character-specific selection evidence from fresh installed defaults so first-run setup always requires the explicit user-confirmed capture.
- Follow the currently navigated D2R roster row and require both its stable character name and selected gold border so level changes cannot stale the evidence and another character cannot be started.
- Keep an unchanged ready character catalog revision-stable so Core selection preview can proceed, and explain the next confirmation action inline
- Keep operator-settings fields disabled together with a clear lock notice whenever the Core is not fully inactive
- Retry one distance-ignoring item teleport after `hover_not_found` or `pickup_failed` so Bone-Prison and similar blockers cannot skip reachable loot
- Keep Act-1 town waypoint handoff reuse as a cold-start skip only, so mid-walk Force Move cannot open or select the waypoint while still sliding past it

## [0.11.0] - 2026-07-27

### Added
- Add the Phase 15 core contract with compile-tested desktop ownership, data-root boundaries, lifecycle states, version and settings gates, retention defaults, stable reason codes, pinned desktop dependencies, and a reproducible 10,000-run history performance baseline
- Add an explicit installed Core data root with hash-bound defaults, reparse-safe staging import, productive-loader validation, atomic publication, and a separate strict Electron desktop-settings store
- Add revisioned Core-owned operator settings with per-character queues and difficulty, finite budgets, input and hotkeys, retention defaults, atomic re-read, ten backups, restart projection, and guarded generated API clients
- Add a hardened single-instance Electron shell with a private PID-bound Core handshake, per-data-root ownership lock, minimal sender-validated IPC bridge, and bounded inactive crash recovery
- Add a PID- and image-bound Windows D2R version gate with exact build/expected/offset/actual compatibility, path-free API and SSE projection, and pre-hotkey input/workflow blocking
- Add a responsive single React app shell with five stable hash targets, an original portal mark, Ember/Gold/Crimson design tokens, accessible shared controls, and unchanged Core-driven route, Pickit, history, and session flows
- Add Core-revisioned settings UI and an eight-state native desktop lifecycle with persisted visible window bounds, opt-in autostart, guarded tray controls, unfocused notifications, and generation-bound quit and command safety
- Add a pre-Core provisioning mode in the same React app, one-shot Go-owned data import, a nine-step Core-driven first-run assistant, route prerequisite projection, safe skip semantics, and handoff to the existing recording workflow
- Add IANA-local Core day buckets with embedded timezone data, DST-safe boundaries, parity-preserving API/JSON transport, and four table-backed non-animated history charts
- Add daily idle-only retention for complete terminal session bundles and a token-, generation-, metadata-, and active-set-bound delete-all workflow for direct history JSONL
- Add a Core-owned local diagnostic ZIP with fixed allowlists, token and user-path redaction, explicit telemetry and route opt-ins, and a path-free reveal contract
- Add one packaged-start GitHub latest-release check with stable SemVer comparison, neutral network failures, manual retry, and one compiled release-page link without download or installation
- Add a per-user Windows x64 NSIS package with one release-version parameter, fixed App ID and icon, minimal Core/default resources, preserved uninstall data by default, and SHA-256 output
- Add a frozen local release pipeline with ASAR/content audits and temporary install, packaged App/Core/sidebar version, upgrade, and default-uninstall data-preservation smoke tests

### Changed
- Reduce the confirmed-boss-to-Bone-Prison lead-in from 750 ms to 250 ms and its post-cast settle from 1.5 seconds to 1 second while retaining target pinning
- Make the Dashboard project and start the persistent per-character queue while keeping all queue mutations in the revisioned Core settings editor

### Fixed
- Preserve a valid completed onboarding state across data-root import before starting the productive renderer without importing autostart or window bounds
- Recover post-kill loot positioning with actual-input-aware bounded retries and candidate-specific teleports before hover-confirmed pickup
- Use the standard D2R belt hotkeys `1` through `4` in fresh installed data roots instead of stale developer-specific punctuation bindings
- Persist the last Core-confirmed character together with its difficulty, and migrate one unambiguous lifecycle-confirmed context so an installed restart does not lock route recording and candidate testing
- Keep isolated route-candidate travel in the run's origin act and select its registered start waypoint directly instead of detouring Mephisto through Rogue Encampment
- Recompute and live-refresh run availability after route publication, and finish first-route onboarding once any run has a published runtime-validatable route instead of requiring every optional run
- Place unfinished first-route setup first on the Dashboard and preserve an explicit return from the routed recording, test, and publish workflow to the matching onboarding step
- Refresh route-recording prerequisites immediately after confirmed character selection, translate missing Pickit readiness for users, and explain that the recording button starts while F9 only finishes an active recording
- Preserve the current onboarding step across an input-triggered controlled Core restart, and resolve every installed character-selection anchor from the absolute loaded Core configuration directory
- Explain unsupported or unprepared local characters with user-facing class and setup reasons, avoid assigning an unconfigured save the current run class, require effective runtime input before character selection, and exclude stale renderer bundles from rebuilt installers
- Allow an anchored local character to be selected from a fresh empty-session onboarding context, order Safety and Input opt-in before controlled D2R selection, and isolate onboarding steppers and prose lists from the global three-column list layout
- Keep Electron's disposable Chromium runtime profile and pre-Core window state outside an unpublished data root so first-run provisioning can atomically publish a truly fresh target
- Allow the passive installed desktop UI to start against a freshly provisioned root before the first Farming route assignment exists, while concrete run and session paths remain fail-closed

### Removed
- Remove the public `--ui` browser product mode, OS browser launchers, browser fallback text, and the Dashboard-only runtime queue draft after Electron parity

## [0.10.0] - 2026-07-22

### Added
- Add the Phase 14 baseline and core contract with schema-3 fixture characterization, farming metrics, denominators, funnel, stages, filters, pagination, exports, and stable history reason codes
- Add globally unique run IDs and schema-3 session/run streams with immutable productive context, explicit diagnostic mode, exact cross-stream correlation, and duplicate-terminal/context-drift rejection
- Add memory-confirmed boss-kill and vendor-sale milestones, complete run-stage classification, and stable item/Pickit correlation across the loot funnel
- Add a bounded schema-3 history reader and race-safe rebuildable in-memory index with strict cross-stream correlation, content fingerprints, incomplete/active projection, and isolated file diagnostics
- Add the canonical history analyzer with UTC filters, terminal duration statistics, stage time, item funnels, failure loss, and boss/route yields per run, kill, and active hour
- Add the read-only history API with canonical filters, generation-bound pagination, German reason projections, parity-preserving JSON/CSV exports, generated TypeScript contracts, and bounded SSE invalidation
- Add the accessible responsive React run-history feature with visible filters, Core-sorted route comparisons, item and run pagination, semantic drill-downs, diagnostics, and filtered downloads
- Complete the Phase 14 live provenance and deterministic Countess/Mephisto product-parity gates across history, API, JSON, CSV, and React

### Fixed
- Remove deleted Phase 13 acceptance profiles from the example assignment and stale profile characterization so the repository baseline is reproducible
- Write the correlated terminal event to productive run telemetry before closing it so completed queue runs appear in strict history views
- Prevent stale post-kill Memory snapshots from triggering a second teleport away from the boss-drop position
- Isolate route-assignment test fixtures from the real ignored operator manifest so the test suite cannot restore an archived farming route
- Keep run IDs off session-level game boundary events so productive session files satisfy the strict history-reader contract
- Separate supervisor run definitions from globally unique execution IDs and remove derived aggregate arithmetic from the history UI
- Show the immutable Pickit profile, rule, action, revisions, failure reason code, and last step in the history run drill-down
- Publish `history_changed` only when the correlated terminal history population changes, not for each flushed in-progress event
- Reuse fully validated history projections when their SHA-256 content fingerprint is unchanged instead of reparsing every file on each refresh

## [0.9.0] - 2026-07-21

### Added
- Add the Phase 13 baseline and core contract with Countess/Mephisto policy characterization, strict parser non-goals, profile and assignment schemas, actions, revisions, ownership, reason codes, and a fallback-free migration matrix
- Add a deterministic D2R `3.2.92777` Set/Unique identity catalog with 140 Set rows, 433 Unique rows, English names, collision-safe stable keys, source validation, and Tal Rasha coverage
- Add fail-closed Set/Unique identity transport and World resolution with quality/base validation and read-only Ground/Inventory diagnostics
- Add exact `[setitem]`/`[uniqueitem]` Pickit fields, canonical escaped expressions, `keep`/`sell` rule metadata, and ordered First-Match evaluation traces
- Add atomic revisioned Pickit profile and assignment stores with CRUD, duplicate/delete guards, strict references, and migrated Countess/Mephisto policies
- Add the complete Pickit Core API, generated TypeScript client, serialized revision-safe mutations, queue preflight, and immutable per-run policy snapshots
- Add the accessible React Pickit profile library and guided catalog editor with set expansion, ethereal base rules, advanced import, conflict handling, and assignments
- Add Core-backed Pickit decision previews and enforce effective keep/sell actions with fail-closed identity rechecks and revision-correlated logs and JSONL telemetry
- Add an unassigned low-risk arrow-quiver acceptance profile for the isolated Phase 13 GUI-to-loot gate
- Complete the Phase 13 GUI-to-loot acceptance with revision-correlated match, hover-confirmed pickup, portal return, stash transfer, and successful JSONL completion

### Fixed
- Allow only the isolated `loot-and-return` phase to start when an unrelated Farming-route lifecycle entry is unavailable, while leaving every other run mode and all loot/input gates unchanged
- Store stable Excel codes rather than English display names when guided base-item rules are created

### Removed
- Remove run-level `pickup_file`/`sell_file` configuration and the three legacy NIP policy authorities after equivalent profile migration
- Remove the unused separately compiled Pickit sell subset and its dead Stash/Town wiring after all consumers switched to the single First-Match action policy
- Remove the temporary synthetic decision-preview controls and their UI-only state from the Pickit editor after the Phase 13 live acceptance

## [0.8.0] - 2026-07-18

### Added
- Add the Phase 12 baseline and core contract with typed recording metadata, assignment and candidate schemas, global system-egress boundaries, workflow transitions, lock ownership, recovery checkpoints, DTO shapes, and productive-route characterization
- Add an atomic revisioned route-assignment authority per character and run with idempotent legacy migration and lifecycle management metadata
- Add global walk-only system Egress contracts, setup commands, validation, and playback for Acts 2–5
- Add an exclusive guided recording coordinator with immutable hash-checked candidate storage, F9 finish semantics, terminal boss validation, and safe TP return states
- Add the global F9 recording-finish hotkey and separate it from F10 stop-after-run and F11 emergency stop
- Add isolated candidate-only playback and revision-bound publish, replace, archive, restore, and delete management with startup recovery
- Add a path-free versioned route API and React route library with character/archive views, guided recording, candidate review, system-Egress setup, accessible confirmations, and Core-backed hotkey help

### Changed
- Migrate the productive Act-3 Egress points to the global `portal-waypoint.yaml` format without character, difficulty, or map-seed bindings
- Reuse the existing Runtime recorder and navigation components for dashboard recording, API finish, candidate playback, and system-Egress setup instead of introducing a parallel input architecture
- Bind guided recording to the confirmed character and class, enforce finite recording timeouts, and project Core workflow area, segment, progress, and state through SSE
- Enforce the exact Phase-12 route API paths, workflow-wide selection/queue/session/mutation exclusion, and live character/difficulty revalidation before candidate test or publish
- Require portal proximity at system-Egress recording start and waypoint proximity at finish, and atomically replace malformed drafts without overwriting a valid ready route
- Guide farming recording, candidate playback, and route management with state-specific German instructions, workflow-wide action locks, and explicit predecessor and delete confirmations
- Complete the Phase 12 live route-management acceptance with global Act 2–5 Egress assets and an atomic Countess candidate replacement that preserves the previous route as an archive

### Fixed
- Fix system-Egress recording preflight to use the configured Memory-confirmed portal and waypoint interaction distances and keep the affected Act's waiting, recording, and failure state visible in the dashboard
- Fix explicit route CLI commands being preempted by the configured automatic farming session
- Fix guided farming recording starting from anywhere in the waypoint area by requiring configured Memory-confirmed proximity to the start waypoint
- Fix guided recording remaining in preflight because D2R retains the waypoint UI flag after a completed waypoint transfer
- Fix guided-recording safety return failing when living monsters occlude the cast Town Portal by widening only its bounded, Memory-confirmed hover search
- Keep same-character and same-difficulty lifecycle confirmation revision-idempotent so frozen route candidates survive a Core restart
- Fix the dashboard candidate review staying empty when the confirmed character arrives after the route feature's initial mount
- Mark candidates with failed post-freeze safety returns as non-testable while retaining their immutable diagnostic route
- Reject candidate playback outside a Memory-confirmed Town portal-arrival handoff without invalidating the retryable candidate
- Fix candidate playback rejecting a valid portal arrival when its variable landing position is outside the Town graph's narrower first-point tolerance, and log the exact playback failure
- Characterize the newly assigned Countess route alongside its unchanged archived predecessor and the existing Mephisto route

## [0.7.0] - 2026-07-18

### Added
- Add a configurable global F10 stop-after-run hotkey that preserves D2R focus and completes the active run before the supervisor-owned exit
- Correlate queue session, game, and run boundaries in status, SSE, and synchronously flushed session JSONL telemetry
- Add the Phase 11 core contract with immutable supervisor states, command transitions, queue semantics, stable reason codes, characterization coverage, and a one-shot runtime migration matrix
- Add a thread-safe long-lived session supervisor with immutable snapshots, monotonic generations, idempotent commands, between-run intents, panic containment, and immediate cancellation
- Add a versioned loopback-only Core API, fail-closed host/origin/token envelope, OpenAPI-generated TypeScript client, pinned React/Vite build, and embedded dashboard assets
- Add a read-only live dashboard with Core, D2R, input and World projections, bounded monotonic SSE events, replay, reconnect, deduplication, and slow-client isolation
- Add a filename-only offline character catalog and bounded screenshot-gated Home/Down selector with Play, difficulty-dialog, class, name, and in-game verification
- Add a recursive Farming RouteCatalog with an atomically persisted lifecycle manifest, bootstrap protection, precise difficulty/layout invalidation, and file-fingerprint correlation
- Add revision-bound selection previews, explicit route-impact confirmation, and post-Memory lifecycle commits for safe difficulty changes
- Add a cyclic runtime FarmQueue scheduler with full-queue preflight, unique per-game run entries, retry-same-index, between-run revalidation, and YAML-authoritative safety budgets
- Add an accessible runtime Queue Builder with unique-entry/reorder/remove/reset operations and Core-backed start, pause-after-run, resume, stop-after-run, and confirmed emergency-stop controls

### Changed
- Complete the Phase 11 live queue acceptance for same-game pause/resume, natural wrap, global stop-after-run, and one supervisor-owned exit
- Reposition every registered boss run at the last Memory-confirmed boss position before scanning and picking loot
- Run unique queued farms sequentially within one verified offline game and move Save & Exit to the supervisor-owned wrap, stop, budget, or recovery boundary
- Keep pause-after-run in the open game, revalidate the same game on resume, and reject duplicate queue runs before attach or input
- Route CLI autonomous queues through the same `RuntimeQueueRunner`, supervisor, game lifecycle, run executor, recovery budgets, and Countess/Mephisto plan validation as the dashboard
- Make `--ui` an explicit passive mode that never starts YAML session or run defaults and prints only the token-free loopback URL
- Replace the single `routes.directory` authority with `routes.farming_root`, `routes.lifecycle_file`, and context-derived resolver/recorder paths
- Show confirmed and draft selection contexts separately and require an accessible modal before a difficulty change can invalidate Farming routes
- Project queue index, cycle, retry counters and hard budgets through the versioned Core API
- Execute each dashboard queue entry with fresh run-specific state while retaining one verified game context until wrap, stop, budget, or safe recovery

### Fixed
- Remove the data race in the app probe mock used by world-delta observation tests
- Respect the validated `session.retry_classes` allowlist before restarting a failed queue entry
- Refresh run-scoped Bone Armor on every same-game queue entry without repeating the new-game five-second settle delay
- Reuse the confirmed Waypoint handoff when a fresh same-game queue run starts instead of incorrectly replaying the Stash-to-Waypoint edge
- Focus and verify the bound D2R window through the shared guarded input controller before dashboard queue start or resume continues
- Reuse a passively Memory-confirmed Rogue Encampment game when starting a dashboard queue after a Core restart instead of incorrectly waiting for the offline character screen
- Reposition at the last Memory-confirmed Countess position after kill confirmation before scanning and picking loot
- Route the dashboard queue Pause hotkey to pause-after-run without suspending active route input or requiring browser focus
- Retry visually unstable post-exit character and difficulty screens within a bounded settle window before starting the next queued game
- Classify the character screen by a decisive Play-versus-dialog anchor margin instead of misusing the tolerant positive-match threshold as an absence check
- Retry D2R foreground activation through bounded Windows GUI input-queue attachment when the dashboard owns the foreground lock
- Refresh the dashboard status projection after every serialized live delta so D2R attach, window binding, resolution and World changes no longer remain hidden behind the initial snapshot
- Restore the process-local control token through a same-origin custom-header bootstrap after browser refresh without persisting it in browser storage, cookies, files, history, or logs
- Focus D2R and wait for UI settle before the first character-screen capture, and keep selection failures visible across subsequent live status refreshes

### Removed
- Remove the superseded per-run `sessionCycleOrchestrator`, `sessionMultiRunner`, recovery stack, and duplicate CLI session lifecycle after the Phase 11 ownership audit

## [0.6.0] - 2026-07-15

### Added
- Add the productive Mephisto definition flow with its bound Durance route, two indexed boss actions, run-specific loot policies, and Act-3 return normalization
- Add isolated read-only inspect, walk recording, structural validation, and Kurast-to-Rogue playback commands for the Act-3 egress acceptance gate
- Add a read-only waypoint-target calibration report and registered 1280×720 actions for Black Marsh, Durance of Hate Level 2, and Rogue Encampment
- Add bound Act-3 egress playback from Kurast portal arrival through the local waypoint and registered Rogue Encampment transfer
- Add typed Countess and Mephisto run definitions, immutable registry metadata, capability validation, shared lifecycle contracts, and a fail-closed definition resolver
- Add a definition-driven run pipeline with indexed encounter actions, centralized generation resets, and fail-closed step telemetry
- Add deterministic read-only run availability, stable blocking reasons, generic session preflight, and `--runs-inspect` JSON output
- Complete the live Countess regression gate with one successful run, one indexed boss hook, and one Save & Exit
- Add version-bound Mephisto monster data, Durance semantics, generated equipment base tiers, and the read-only Pickit `[tier]` predicate
- Add run-specific Mephisto pickup/sell policies and a productive UnitID-pinned Cain-to-Akara item-service acceptance flow
- Complete the live Phase 10.6 item-service acceptance with verified Cain identification, Akara sale, shop close, and Town handoff

### Changed
- Bind Town handoff planning and telemetry to the selected Run ID instead of a hard-coded Countess target
- Move Bone Spear combat to interval-paced right-clicks and require Memory confirmation that F8 selected Bone Spear on the right skill slot before the first attack
- Replace the Black-Marsh-only waypoint selector with a Memory-gated generic executor that selects act tab and destination on separate ticks and never repeats the destination click
- Require a complete route contract and stable route ID for foreign-Town egress instead of accepting a non-playable format placeholder
- Replace the Countess-only run and global Pickit configuration with one ID-keyed run schema containing route, combat, pickup, and optional sell policies
- Replace Countess-specific CLI phase names and task-step identifiers with generic run phases and definition-neutral pipeline steps
- Keep explicit sell candidates out of personal-stash transfers and order Cain identification before combined Akara restock/sell work

### Fixed
- Reject left-mouse attack bindings before runtime input and fail closed without clicking when the configured hotkey does not produce the expected right-side skill selection
- Acquire Mephisto by his exact generated act-boss NPC ID instead of incorrectly requiring Countess's super-unique type flag
- Preserve the definition action index through the real profile executor so Mephisto casts both Bone Prisons while retries of either action remain idempotent
- Fail closed with `boss_pin_lost` when a boss disappears before its encounter sequence completes or reappears under a different UnitID
- Exit explicit CLI runs automatically after their terminal task result instead of requiring the F11 stop hotkey after successful playback
- Resolve registered Durance entrance kinds during route playback instead of truncating conversion at the former Act-1 enum boundary
- Keep waypoint travel pending across transient Area-0 loading snapshots instead of aborting before the confirmed destination appears
- Wait for the stable read-only game identity before starting isolated Act-3 egress playback
- Accept route name and difficulty options for the dedicated Act-3 egress recorder
- Force Act-3 egress playback through an area-bound walking executor instead of delegating a `walk` route to the teleport navigator
- Bind each session run recorder to shared pipeline telemetry before the first transition, preventing a false `telemetry_failed` abort in Town
- Select Cain's second dialog entry before confirming Identify Items instead of activating Talk
- Preserve the pending sell order across the Cain service boundary and reject isolated item-service tests without exactly one unidentified candidate before movement

### Removed
- Remove the legacy `SelectBlackMarsh` action and `pathing.waypoint_ui` Black-Marsh coordinate schema

## [0.5.0] - 2026-07-13

### Changed
- Wire the validated Akara restock planner, Town graph, NPC/shop gates, conservative table-derived prices, quantity verification, executor budgets, and run telemetry into the productive post-Countess handoff
- Replace and live-validate the invalidated Nightmare Countess route with the newly recorded seven-segment `560356…19e37` Black-Marsh layout
- Remove the obsolete difficulty-selected Act-1 Town walker, recorder, playback command, and runtime wiring; Town movement now accepts only layout-bound graph edges
- Bind the previously recorded Act-1 service edges to their confirmed right-Waypoint Town fingerprint and Stash origin

### Fixed
- Preserve the complete Town Portal hover budget until the newly cast portal has a stable Memory identity and position for its observed activation period
- Wait for a Memory-confirmed stationary period after the force-move stash approach before starting the hover-confirmed click
- Complete an already executed vendor purchase without recalculating a now-zero missing quantity, allowing the verified shop-close and Akara-to-Waypoint gates to run
- Expose and live-validate carried and private-stash gold from the player stat list instead of unconditionally aborting Town demand inspection with an unavailable gold source
- Preserve the game-scoped Act-1 Town layout pin while Stash and Waypoint units are regionally unloaded, wait for confirmed game identity before persisting or restoring it, revalidate it when anchors return, and safely bridge isolated graph-test processes
- Replace all remaining pre-fix south Town assets with continuous, endpoint-confirmed recordings
- Re-record the south-layout `akara-cain` edge with eight preserved samples and live-confirmed Cain ID `265`
- Use live hover-validated Rogue Encampment Cain ID `265` (`cain5`) and remove temporary multi-variant discovery
- Pin the declared endpoint entity throughout Town recording so a transiently missing NPC in the F11 snapshot can be validated against its last position from the same recording
- Buffer valid Town positions before the fingerprint becomes observable, clear that buffer across pre-pin invalid states, and bind it once the same Town layout anchors appear
- Log Town recording rejection reasons structurally instead of exposing them only on stderr
- Withdraw every unaccepted south-layout service variant recorded before the anchor-unloading fix; retain only separately live-validated edges
- Re-record the south-layout `stash-akara` edge with all nine detour samples preserved after fixing fingerprint-anchor unloading
- Continue sampling a pinned Town edge while Stash and Waypoint temporarily leave the enumerated region; revalidate the fingerprint whenever both anchors return and never skip Stop handling
- Reject Town edge recordings unless the final player position is Memory-confirmed within interaction distance of the declared NPC, Waypoint, or Stash endpoint
- Re-record and reactivate the south-layout `stash-akara` edge at the confirmed Akara boundary; retain isolated visual acceptance before combined use
- Withdraw the south-layout `stash-akara` variant after visual acceptance exposed an inconsistent fourteen-tile Akara boundary despite technical graph completion
- Replace the incompatible migrated `charsi-waypoint` path with a separately recorded edge sharing the confirmed Charsi boundary
- Disable the incompatible migrated `charsi-waypoint` variant after strict edge composition exposed an eleven-tile Charsi anchor mismatch
- Allow later Town graph edges to approach their own recorded boundary point after the preceding edge reaches the shared NPC anchor, while retaining strict start confirmation for the first edge
- Replace the obstructed right-layout `cain-charsi` path with a separately recorded and layout-bound route
- Disable the right-layout `cain-charsi` variant after live playback exposed an obstructed path; require a separate replacement recording before routing it again
- Correct Act-1 Town routing to account for the randomly rolled Town preset instead of treating character or difficulty as the route binding
- Remove unrecorded placeholder edges from the Act-1 service graph so production routing selects only existing route assets
- Fix dropped back-to-back vendor inputs with a bounded 500 ms settle interval between Phase 9.5 purchases
- Fix Phase 9.5 vendor purchases blocking on the world-entity hover buffer, which does not reliably expose shop UI items

### Added
- Complete and live-validate the Phase 9 central post-Countess flow through portal return, stash, demand-driven Akara restock, verified shop close, Waypoint handoff, and Save & Exit
- Add and live-validate the complete separately recorded west-exit `4ad7f3…33f30` Town graph variant set
- Add and live-validate the complete separately recorded north-exit `768769…17381` Town graph variant set
- Log the raw monster `npc_id` in read-only hover-watch mode for live catalog validation
- Complete the south-exit `5f6354…60f17` Town graph with separately recorded `stash-waypoint` and `portal-cain` variants
- Add five separately recorded service-edge variants for the newly observed `(-7,-30)` left-Waypoint Town fingerprint
- Migrate the read-only confirmed right-Waypoint Town preset to an exact `stash-waypoint` graph variant
- Migrate the first read-only confirmed left-Waypoint preset to a stash-relative direct `stash-waypoint` graph variant
- Add Town graph schema v2 with Memory-derived layout fingerprints, exact route variants, layout-bound recording/loading, and fail-closed runtime resolution
- Add a non-routable `stash-waypoint` recording draft so the direct edge can be captured before production graph activation
- Add the Phase 9.10 central post-Countess stash-to-waypoint handoff with Memory-confirmed endpoint and a one-run session override for manual acceptance
- Add Phase 9.9 finite Town plan execution, sticky telemetry safety, reset semantics, and detailed Town JSONL fields
- Add Phase 9.8 fail-closed repair evidence and one-shot, area-verified hub/Countess waypoint transfer contracts
- Add Phase 9.7 protected item-service planning with UnitID-pinned identify/sell actions and finite state/location verification
- Add Phase 9.6 threshold-triggered restock orders with gold gates, one-shot bulk mode, bounded single-buy fallback, and finite quantity verification
- Add fail-closed Phase 9.0 Town contracts, normalization phases, reason codes, and execution budgets
- Add read-only Town research, validated central-hub and egress configuration, demand planning, and selective edge-based Town graph routing for Phase 9.1–9.4
- Record and live-validate the central Act-1 service graph through Stash, Akara, Cain, Charsi, and Waypoint
- Add and live-validate Phase 9.5 UnitID-pinned Town NPC interaction, Memory-confirmed dialog/shop and vendor-item gates, and atomic bulk/single purchase primitives

- Pick up Rejuvenation Potions as Countess loot because Town vendors cannot replenish them
- Add the detailed Phase 9 implementation plan for act-aware, demand-driven Town Services with three combined manual gates
- Complete and live-validate Phase 8.7 with visible Bone Armor and Bone Prison casts in one autonomous Countess cycle
- Complete Phase 8.6 with run-scoped profile JSONL events, fail-closed telemetry errors, and reset coverage for hooks, potion verification, and cooldown state
- Complete Phase 8.5 with a UnitID-pinned Bone Prison boss hook before the first Countess attack and live-validate the ordered full run
- Add Phase 8.0–8.4 generic profile contracts, Bone Armor skill/config, resettable hook executor, prioritized verified resource policy, and Town-ready Countess integration
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
- Add Phase 5.6 Countess loot phase with `wait_for_drops`, `scan_loot`, `pick_loot`, and isolated `--phase loot-and-return`
- Add Phase 5.5 hover-confirmed item pickup with `loot.pickup` safety limits and `--pathing-test pickup:item`
- Add Phase 5.4 Loot Decision Pipeline with read-only stage decisions for Pickit matches, pickup candidates, Keep/Stash, and failure reasons
- Add Phase 5.3 Pickit MVP with a small NIP subset, `loot.pickit_file`, and Countess default rules
- Add Phase 5.2 read-only inventory model and `loot.inventory_lock` capacity guard
- Add read-only item enumeration from memory into the world model
- Add Phase 5.0 loot and recovery concept documentation covering Ground-Loot, Pickit, Inventory-Lock, Stash safety, and recovery slices

### Changed
- Treat the Act-1 spawn as a navigation-free stash alias in the Town service graph
- Use combat-profile belt assignments, configurable restock thresholds, Shift-right-click bulk refill, and single-buy fallback for incomplete layouts in the Phase 9 plan
- Exclude Rejuvenation buying, crafting, and 99-slot stash withdrawal from Phase 9 Town replenishment
- Simplify Phase 9 to one central Act-1 stash/service hub with minimal per-act waypoint egress adapters and next-run handoff
- Separate permanent Town route assets under `configs/routes/town/` from invalidatable character/difficulty Farming routes under `configs/routes/farming/`
- Document planned `internal/profile`, `internal/town`, `internal/api`, and `web` boundaries without pre-creating packages or moving the stable Phase 7 runtime
- Extend the Phase 8 plan with a profile-driven HP, mana, and rejuvenation resource policy while keeping manual acceptance at three combined live runs
- Insert act-aware Town Services as Phase 9 with service discovery, anchor-route graphs, validated waypoint act changes, demand-driven preparation, and fail-closed unknown-town handling; renumber later roadmap phases through Phase 15
- Update the central roadmap through Phase 15 with separate goals, scope, and acceptance gates for character profiles, Town Services, a second farm target, GUI, route management, Pickit editing, telemetry, and packaging
- Extend the Phase 6.1c difficulty selector into the canonical Phase 7.3 full offline-start flow without duplicating its click primitive
- Set the Phase 7 live acceptance baseline to three successful repetitions per isolated flow and three complete multi-run cycles
- Select the Act-1 town waypoint route by configured difficulty and fail safely when its recording is unavailable
- Rename the implemented stash decision reason from `stash_not_implemented` to `stash_candidate`
- Extend `loot-and-return` and the full Countess run from verified town arrival through Personal Stash completion
- Expand read-only item probe diagnostics with generated item catalog type data and filtered verbose ground-item hints
- Regenerate the item catalog from local D2R `3.2.92777` data so current Countess drops resolve correctly

### Fixed
- Wait five seconds of stable town state before the first profile input because Memory can report in-game before D2R is visibly input-ready
- Hold profile execution for 1.5 seconds after hook clicks to protect the complete in-game cast animation
- Cast self-targeted profile skills at the neutral client center instead of the player anchor that D2R can interpret as movement
- Delay profile hooks until the character is stable after game entry or route teleport before requesting the skill cast
- Start profile skill settle windows after the blocking click completes so immediate town or attack inputs cannot cancel the cast animation
- Omit misleading skill names from resource-only profile telemetry events
- Prevent healing and mana potion spam by honoring their gradual four-second effect window while retaining a shorter rejuvenation cooldown
- Prevent explicit `--run` phases and probes from falling through into an enabled autonomous session after their own runtime completes
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
- Add Countess kill phase 4.6: `--run countess --phase boss`, Necro Bone Spear combat, Good Chest position fallback, and defensive kill confirmation without loot pickup
- Add automatic runtime log files under `logs/` so verbose manual test runs remain inspectable after the terminal buffer scrolls away
- Add read-only `--pathing-test inspect:entrances` to capture player position, hover state, and visible entrance coordinates during manual calibration
- Add Countess tower traversal phase 4.5: `--run countess --phase play-route`, best-effort Forgotten Tower search, and hover-confirmed cellar transitions through Tower Cellar Level 5
- Add Act-1 town waypoint acquisition (Phase 4.5): Force Move town walking from Rogue Encampment stash/spawn toward the waypoint, optional route record/play pathing tests, and Countess `acquire_town_waypoint` step
- Add Countess travel phase 4.4: `--run countess --phase travel-entry`, hover-confirmed Town-Waypoint click, fixed Black-Marsh waypoint UI selection, and loading-safe arrival wait
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
- Allow `play-route` to resume from Black Marsh, Forgotten Tower, or Tower Cellar route areas during manual testing instead of requiring a town start every time
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

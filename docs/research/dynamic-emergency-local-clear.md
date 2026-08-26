# Dynamischer lokaler Notfall-Clear

Stand: 26. August 2026, Repository `3165bda`.

## Fragestellung

Kann der Bot bei lokaler Gefahr oder einer durch Gegner blockierten Interaktion den Standardkampf des aktiven Kampfprofils vorübergehend ausführen, die unmittelbare Umgebung räumen, den Kampf sauber beenden und danach Route oder Portal erneut versuchen?

## Kurzantwort

Ja, das ist mit der bestehenden Architektur machbar. Für Lower Kurast existiert bereits fast genau dieses Muster. Ein lokaler Clear hält die Route an, wählt Gegner in einem festen Radius, ruft den profilgebundenen Route-Clear auf, wartet mehrere gegnerfreie Snapshots ab, setzt den Combat-Zustand zurück und kehrt zur unterbrochenen Objektaktion zurück.

Für den beobachteten Countess-Fall ist diese Funktion heute aber nicht verfügbar. Zwei getrennte Lücken sind relevant:

1. Ein Portal-`hover_not_found` löst derzeit nur einen Teleport zum Portal und einen zweiten Hover-Versuch aus. Der lokale Portal-Clear beginnt erst, wenn ein bereits gesendeter Portal-Klick die Ziel-Area nicht erreicht hat.
2. Countess-Route und Countess-Profilstrategie sind nicht für Route-Clear verdrahtet. Ein `route_transition_failed` beendet den Run unmittelbar und übergibt an den kontrollierten Town-Retry.

Die richtige Erweiterungsstelle wäre eine begrenzte Recovery-State-Machine in `internal/tasks`, die `localThreatClear` und `RouteClearExecutor` wiederverwendet. Die Boss-Pipeline ist dafür ungeeignet. Sie enthält Boss-Pinning, Encounter-Hooks, Repositionierung und Kill-Bestätigung, die eine Portal- oder Routeninteraktion nicht braucht.

## Was bereits vorhanden ist

### Lokaler Clear mit Rückkehr zur unterbrochenen Aktion

[`localThreatClear`](../../internal/tasks/local_threat_clear.go#L11-L19) besitzt feste Grenzen von 12 Kacheln, 12 Kampfaktionen, sechs Sekunden Gesamtdauer, drei Sekunden ohne Aktion und drei frischen gegnerfreien Snapshots. Die State-Machine wählt bevorzugt die vom Hover gemeldete UnitID, sonst den nächsten lebenden Gegner am Anker, und ruft pro Tick den profilgebundenen [`TickRouteClear`](../../internal/tasks/local_threat_clear.go#L46-L84) auf. [`reset`](../../internal/tasks/local_threat_clear.go#L87-L91) beendet den Route-Clear-Zustand.

Lower Kurast nutzt diesen Baustein bereits als Unterbrechung einer Objektaktion. Ein bestätigter Monsterblocker startet den Clear, merkt sich die Rückkehrphase und setzt danach denselben Truhen- oder Pickup-Versuch fort ([`startChestBlockerClear`](../../internal/tasks/chest_sweep.go#L384-L420), [`tickChestBlockerClear`](../../internal/tasks/chest_sweep.go#L423-L452), [`finishChestBlockerClear`](../../internal/tasks/chest_sweep.go#L462-L485)). Währenddessen ruft die Pipeline `Route.Hold` auf und sendet keinen normalen Routen-Tick ([`tickChestWork`](../../internal/tasks/chest_sweep.go#L231-L275)).

Die Tests belegen den gewünschten Ablauf und seine Grenzen:

- Blocker räumen, Objekt erneut versuchen und öffnen: [`TestChestSweepClearsObservedMonsterBlockerThenRetriesObject`](../../internal/tasks/chest_sweep_test.go#L367-L407)
- Ohne lokalen Monsterbeleg keinen Kampf starten: [`TestChestSweepDoesNotClearWithoutLocalMonsterEvidence`](../../internal/tasks/chest_sweep_test.go#L409-L443)
- Persistenten Blocker auf 12 Aktionen begrenzen und nur einmal erneut versuchen: [`TestChestSweepPersistentBlockerClearIsBoundedAndRetriesOnce`](../../internal/tasks/chest_sweep_test.go#L674-L713)
- Route während des Clears halten: [`TestLowerKurastBlockerClearHoldsRoute`](../../internal/tasks/chest_sweep_test.go#L715-L743)

### Profilgebundener Standardangriff

Die Task-Schicht besitzt mit [`RouteClearExecutor`](../../internal/tasks/deps.go#L122-L126) bereits eine schmale Combat-Schnittstelle. Das Profil erhält genau ein von der Task-Schicht ausgewähltes Monster. Seine Eingaberechte beschränken sich auf Angriff und `StopAttack`; Bewegung oder Navigation gehören nicht zum Profil ([`RouteClearRequest` und `RouteCombatActions`](../../internal/profile/types.go#L156-L170)).

Der Executor wählt den konfigurierten Standardangriff. Beim Bone-Spear-Necro wirkt er für einen Threat einmal Amplify Damage und danach Bone Spear. Beim Hammerdin ist kein Opener konfiguriert, der Standardangriff ist Blessed Hammer ([`TickRouteClear`](../../internal/profile/executor.go#L497-L529), [Hammerdin-Summoner-Strategie](../../internal/profile/hammerdin/strategy.go#L88-L103), [Necro-Summoner-Strategie](../../internal/profile/necrobonespear/strategy.go#L105-L118)). [`ResetRouteClear`](../../internal/profile/executor.go#L532-L540) löscht den Opener- und CE-Zustand und löst einen gehaltenen Angriff.

Das ist der geeignete Vertrag für ein lokales Kampf-Intermezzo. Ein globales `combat_enabled` gibt es nicht und wäre auch zu grob. Aktivieren bedeutet, dass die Task-State-Machine für einige Ticks `TickRouteClear` besitzt und aufruft. Deaktivieren bedeutet, dass sie den Zustand verlässt, `StopAttack` fehlergeprüft aufruft und danach `ResetRouteClear` ausführt. `ResetRouteClear` allein versucht den Release nur best effort und verwirft dessen Fehler.

### Bestehende Portal-Recoveries

Vor einem bestätigten Portal-Klick behandelt [`tickEnterTownPortalWithDeps`](../../internal/tasks/pipeline_return.go#L172-L209) `too_far` und `hover_not_found` gleich. Es teleportiert höchstens einmal direkt zum Portal und probiert den normalen Hover-Klick erneut. [`tickPortalEntryRecovery`](../../internal/tasks/pipeline_return.go#L345-L406) enthält keinen Kampf.

Nach einem bestätigten Klick gibt es dagegen bereits einen lokalen Combat-Retry. Bleibt der Charakter mindestens eine Sekunde und drei frische Snapshots in der Route-Terminal-Area, startet [`tickPortalDestinationObservation`](../../internal/tasks/portal_destination_recovery.go#L67-L108) einen `localThreatClear` um das gepinnte Portal. Danach stoppt die Pipeline den Angriff, teleportiert zum Portal und wiederholt den Hover-Klick ([`tickPortalDestinationClear`](../../internal/tasks/portal_destination_recovery.go#L111-L130), [`tickPortalDestinationTeleport`](../../internal/tasks/portal_destination_recovery.go#L133-L152), [`tickPortalDestinationRetryClick`](../../internal/tasks/portal_destination_recovery.go#L167-L196)). Der ganze Ablauf bleibt im ursprünglichen `wait_origin_town`-Timeout ([Phasenvertrag](../../internal/tasks/portal_destination_recovery.go#L17-L30)).

Dieser Ablauf half beim untersuchten Run nicht. Der Portal-Klick wurde nie bestätigt, also wurde `wait_origin_town` und damit der vorhandene lokale Portal-Clear nicht erreicht.

Es gibt noch eine zweite Einschränkung: Auch nach einem bestätigten Klick könnte der produktive Countess-Executor den Clear derzeit nicht ausführen. `Deps.RouteClear` zeigt zwar auf den Profil-Executor, doch die Countess-Strategien konfigurieren dessen internen Route-Clear nicht. [`TickRouteClear`](../../internal/profile/executor.go#L503-L506) würde deshalb `route_clear_strategy_unavailable` liefern, das `localThreatClear` zu `combat_action_failed` reduziert ([Fehlerabbildung](../../internal/tasks/local_threat_clear.go#L77-L80)). Der gemeinsame Portal-Recovery-Code und seine produktive Profilmatrix passen an dieser Stelle noch nicht zusammen.

## Warum Countess heute nicht einfach umgeschaltet werden kann

### Route-Combat ist runbezogen freigeschaltet

Die Run-Definition erlaubt Route-Clear nur für Summoner und Cows. Countess hat weder `RunCapabilityRouteClear` noch eine Hostile-Allowlist ([Run-Registry](../../internal/tasks/registry.go#L120-L167), [Cow-Definition](../../internal/tasks/registry.go#L190-L217)). `tickTravel` bewertet Threats nur, wenn Config und Capability aktiv sind; andernfalls geht der Snapshot direkt an `Route.Tick` ([Travel-Pipeline](../../internal/tasks/pipeline_travel.go#L117-L155), [Route-Tick](../../internal/tasks/pipeline_travel.go#L277-L290)). Die Config-Abbildung lehnt aktiviertes Route-Combat für Runs ohne Capability ab ([`mapRunConfig`](../../internal/app/run_mode.go#L77-L93)).

Ein Fehler des RoutePlayers wird unmittelbar in `route_transition_failed` übersetzt ([`routePlaybackFailureReason`](../../internal/tasks/pipeline_travel.go#L664-L678)). Der Route-Adapter hält intern Segment und Transition, aber sein Task-Interface stellt nur `Start`, `Progress`, `Hold`, `Tick` und `Reset` bereit ([`RoutePlayback`](../../internal/tasks/deps.go#L135-L142)). Der aktive Transition-Handler hält die gepinnte Entrance-UnitID privat und liefert nach erschöpften Korrekturen nur den Fehler zurück ([`RouteTransitionHandler`](../../internal/pathing/route_transition.go#L18-L35), [Fehlerpfad](../../internal/pathing/route_transition.go#L85-L93)). Für eine saubere Recovery am selben Übergang fehlt deshalb ein task-sichtbarer Transition-Kontext mit Anker und Blockerbeleg.

### Nicht jedes Profil-/Run-Paar besitzt Local-Clear

Die Registry löst Combat-Strategien pro `(Profil, Run)` auf ([`CombatStrategyRegistry`](../../internal/app/combat_strategy_registry.go#L21-L42)). Der App-Adapter übergibt Combat-Aktionen nur an Strategien, die `SupportsRouteClear` implementieren ([`newProfileExecutor`](../../internal/app/profile.go#L137-L176)).

Aktuell gilt:

| Profil und Run | Route-Clear verdrahtet | Bedeutung für Notfall-Clear |
|---|---:|---|
| Necro Summoner, Cows | Ja | Vorhandener Route-Clear direkt nutzbar |
| Hammerdin Summoner, Cows | Ja | Vorhandener Route-Clear direkt nutzbar |
| Necro Lower Kurast | Ja, nur lokal | Vorbild für Objekt-/Portal-Recovery |
| Hammerdin Lower Kurast | Ja, nur lokal | Vorbild für Objekt-/Portal-Recovery |
| Necro Nihlathak | Ja, nur Post-Boss | Technisch lokal nutzbar, aber nicht als Travel-Clear freigegeben |
| Countess und Mephisto, beide Profile | Nein | Neue explizite Local-Clear-Verdrahtung nötig |
| Hammerdin Nihlathak | Nein | Neue explizite Local-Clear-Verdrahtung nötig |

Die Sonderfälle sind absichtlich modelliert. Lower Kurast und Necro-Nihlathak implementieren `SupportsRouteClear`, melden aber `RequiresRouteClear() == false`; so erhalten sie Combat nur für lokale beziehungsweise Post-Boss-Aktionen, ohne Travel-Combat zu aktivieren ([Necro-Strategien](../../internal/profile/necrobonespear/strategy.go#L20-L31), [Necro-Konfiguration](../../internal/profile/necrobonespear/strategy.go#L63-L71), [Registry-Test](../../internal/app/combat_strategy_registry_test.go#L24-L33)). Hammerdin-Countess und -Mephisto implementieren den Vertrag nicht ([Hammerdin-Bossstrategie](../../internal/profile/hammerdin/strategy.go#L49-L66), [Registry-Test](../../internal/app/combat_strategy_registry_test.go#L61-L72)).

Damit ist die Funktion für die zwei heutigen Profile implementierbar, aber nicht automatisch für jedes zukünftige Profil generisch. Jedes neue Profil-/Run-Paar muss ausdrücklich eine Local-Clear-Strategie konfigurieren und seine Skill-Abhängigkeiten deklarieren. Fehlt diese Strategie, muss der Bot vor Input abbrechen.

## Fehlende Fähigkeiten für den gewünschten Ablauf

### 1. Ein Trigger vor dem Portal-Klick

[`TownPortalActionResult`](../../internal/pathing/town_portal.go#L22-L41) meldet nur Status, Reason und Done. Der zugrunde liegende EntityClicker protokolliert zwar nach erschöpftem Hover-Budget die zuletzt gehoverte UnitID und deren Typ ([`EntityClicker.Tick`](../../internal/pathing/click.go#L108-L120)), gibt diesen Blocker aber nicht an die Task-Schicht zurück. Der Chest-Adapter besitzt dafür bereits das passendere Ergebnisfeld `BlockerUnitID` ([`ChestOperateResult`](../../internal/tasks/deps.go#L60-L70)).

Ein belastbarer Portal-Trigger sollte deshalb einen aktuellen Memory-bestätigten Monsterblocker transportieren. Nur `hover_not_found` plus irgendein Monster in der Nähe wäre schwächer und könnte unnötigen Kampf starten.

### 2. Ein Trigger und Rückkehrpunkt für Route-Transitions

Bei einer Transition ist `RouteProgress.Mode == transition`, enthält aber kein MovementTarget und keine Entrance-/Object-UnitID ([`RouteProgress`](../../internal/pathing/route_segment_player.go#L40-L70), [Transition-Projektion](../../internal/pathing/route_segment_player.go#L208-L239)). Für einen ankergebundenen Clear und einen Retry derselben Transition sollte `tasks` den aktiven Übergang eindeutig kennen. Der Besitzer dieses Zustands bleibt die Travel-Pipeline. `pathing` sollte nur den read-only Kontext liefern und nach dem Clear denselben begrenzten Transition-Versuch ausführen.

### 3. Eigener Recovery-Zustand statt Fehler-Retry

Sobald ein Step fehlschlägt, markiert der Runner ihn terminal und setzt alle zustandsbehafteten Adapter zurück ([`finishStepFailed`](../../internal/tasks/runner.go#L460-L487), [zentrale Reset-Barriere](../../internal/tasks/runner.go#L144-L195)). Der lokale Clear muss daher vor dem terminalen `stepResult{failed: true}` beginnen. Sinnvolle Zustände wären `observe_blocker -> clear -> stop_combat -> retry_interaction -> verify`, jeweils mit genau einem gespeicherten Rückkehrpunkt.

### 4. Sicherheitsressourcen innerhalb des Notfall-Clears

Außerhalb der opt-in Combat-Routen führt der Runner zwar die normale Resource Policy vor jeder Run-Aktion aus ([`Runner.Tick`](../../internal/tasks/runner.go#L228-L252)). Fehlende Tränke sind dort jedoch nicht terminal, weil `FailOnUnavailable` nicht gesetzt ist. Das Profil loggt die fehlende Ressource und lässt die Task weiterlaufen ([`TickResources`](../../internal/profile/executor.go#L373-L402)).

Auf Combat-Routen ist der Vertrag strenger. `FailOnUnavailable` stoppt zuerst den Angriff und liefert `combat_resource_exhausted` ([`TickResources`](../../internal/profile/executor.go#L386-L400)); die Travel-Pipeline setzt diesen Context vor dem Clear ([`tickTravel`](../../internal/tasks/pipeline_travel.go#L154-L187)). Ein echter Notfall-Clear muss dieselbe strenge Semantik besitzen. Sonst würde er gerade bei leerem Gürtel weiterkämpfen.

### 5. Gefahren- und Fortschrittsmodell

`world.Monster` enthält nur NPC-ID, UnitID, Position, Name, Typflag und Hoverstatus. Immunitäten, Resistenzen, Monster-HP und verursachter Schaden fehlen ([`Monster`](../../internal/world/npc_ids.go#L5-L13)). Der Bot kann einen physisch immunen Gegner daher nicht als physisch immun erkennen und kann auch nicht messen, ob ein Angriff dessen HP senkt. Er erkennt Fortschritt nur daran, dass lebende UnitIDs aus späteren Snapshots verschwinden.

Das hat zwei Folgen:

- Der Standardangriff des Profils kann funktionieren, wenn dessen Schadensart den Gegner trifft. Eine automatische Immunitätsreaktion oder Skillalternative ist mit dem heutigen World Model nicht möglich.
- Der bestehende `localThreatClear` meldet bei sechs Sekunden, zwölf Aktionen oder drei Sekunden ohne Aktion `done`, auch wenn noch Gegner am Anker stehen ([Budgetpfad](../../internal/tasks/local_threat_clear.go#L53-L57)). Das ist für eine Truhe vertretbar, weil der folgende Objektversuch begrenzt scheitert. Für eine Überlebensfunktion ist es die falsche Bedeutung. Budgetende bei verbleibendem Threat muss `unsafe/exhausted` sein und darf nicht als erfolgreicher Clear gelten.

Auch die freie Bedingung braucht einen Coverage-Guard. Der World-State weist ausdrücklich aus, ob die Monsterliste abgeschnitten wurde und bis zu welchem Radius sie vollständig ist ([`MonsterCoverage`](../../internal/world/phase17_contract.go#L3-L12)). Ein generalisierter Notfall-Clear sollte drei frische Snapshots nur dann als sicher zählen, wenn die Abdeckung den gesamten Clear-Radius umfasst. Der heutige lokale Clear prüft das nicht.

### 6. Todes- und Exit-Fallback

Der Spielerzustand enthält HP und MaxHP, aber kein eindeutiges Alive-/Dead-Feld ([`Player`](../../internal/world/player.go#L32-L54)). Ein HP-Wert von null sollte ohne zusätzlichen belegten Death-State nicht als alleinige Aktionsfreigabe dienen.

Wenn Clear oder Portal-Retry erschöpft sind, darf der Bot den Angriff nicht halten und das Spiel nicht einfach offen lassen. Die vorhandene Supervisor-Schnittstelle kennt mit `ExitRequired` bereits einen zentralen Save-&-Exit-Fallback für einen terminalen Fehler ohne bestätigte Town-Grenze ([`SupervisorRunResult`](../../internal/app/supervisor.go#L70-L82), [Supervisor-Exitentscheidung](../../internal/app/supervisor.go#L587-L594)). Der normale kontrollierte Retry setzt dieses Flag bei einer gescheiterten Rückkehr derzeit nicht; er endet mit `retry_return_failed`, `SafeToExit=false` und ohne Exit-Anforderung ([`controlledRetryResult`](../../internal/app/queue_runtime.go#L587-L600)). Für eine Überlebensfunktion muss diese letzte Entscheidung ausdrücklich festgelegt werden.

## Empfohlene Ownership

Die Zuständigkeiten sollten so bleiben:

| Paket | Verantwortung |
|---|---|
| `internal/pathing` | Blockierte Portal-/Transition-Interaktion samt gepinnter Ziel- und optionaler Blocker-UnitID melden; keine Kampfentscheidung treffen |
| `internal/tasks` | Trigger, Anker, Zielauswahl, Hold, Budgets, sichere Snapshots, Rückkehrpunkt und genau einen Retry besitzen |
| `internal/profile` | Für das aktive `(Profil, Run)` den Standardangriff und optionalen Opener ausführen; keine Route oder Portalsteuerung besitzen |
| `internal/app` | Die bereits eingefrorene Profil-/Run-Strategie verdrahten und das Supervisor-Ergebnis bestimmen |
| `internal/telemetry` | Trigger, Blocker, Kampfaktionen, Abschlussgrund, Retry und endgültigen Fallback korreliert aufzeichnen |

`localThreatClear` ist die beste Basis, braucht für den Survival-Einsatz aber ein Ergebnis, das `cleared`, `exhausted` und `failed` trennt. Die Boss-Pipeline sollte nicht wiederverwendet werden. Deren Standardkampf umfasst je nach Profil zusätzlich Bossdistanz, Teleport, gepinnte Boss-UnitID, Kill-Bestätigung und Hammerdin-Reposition ([Bosskampf](../../internal/tasks/pipeline_boss.go#L366-L430)). Das würde eine lokale Portal-Recovery unnötig an Bosssemantik koppeln.

## Möglicher Ablauf ohne neue Combat-Engine

Ein schmaler Ablauf wäre:

1. Portal- oder Transition-Interaktion stoppt mit einem aktuellen Monsterblocker oder eine klar definierte lokale Gefahr hält die Route vor dem nächsten Input.
2. `tasks` pinnt Interaktionsziel, Anker, Rückkehrzustand und eine einmalige Recovery-ID.
3. Vor jedem Clear-Tick laufen kritische HP-/Mana- und Trankregeln mit `FailOnUnavailable=true`.
4. Die Pipeline hält Route und Interaktion. `TickRouteClear` erhält genau ein lebendes Ziel im lokalen Radius.
5. Drei frische, für den Radius vollständige monsterfreie Snapshots bestätigen `cleared`.
6. Die Pipeline ruft `StopAttack` fehlergeprüft auf, setzt danach den Route-Clear-Zustand zurück und setzt Portal oder Transition genau einmal fort.
7. `exhausted`, fehlende Ressourcen, ungültiger Combat-Vertrag oder ein zweiter Interaktionsfehler führen zu einem ausdrücklich definierten terminalen Exit-Fallback. Sie gelten nicht als erfolgreicher Clear.

Das ist keine neue universelle Combat-AI. Es ist eine begrenzte Task-Recovery, die den vorhandenen Standardangriff des gewählten Profils nutzt.

## Bewertung

**Machbarkeit:** hoch für die beiden vorhandenen Profile. Die benötigten Eingabe-, Profil-, Zielauswahl-, Reset- und Route-Hold-Bausteine existieren bereits.

**Aufwandstreiber:** nicht der Kampf selbst, sondern die saubere Trigger-Evidenz, die Countess-/Mephisto-Strategieverdrahtung, der Transition-Rückkehrpunkt und das Verhalten bei erschöpftem Clear oder leerem Gürtel.

**Generik:** pro Profil ja, global nein. Der gemeinsame Task-Ablauf kann generisch sein. Jede Profil-/Run-Kombination muss ihren Standard-Clear ausdrücklich bereitstellen.

**Bezug zum untersuchten Tod:** Ein Pre-Click-Portal-Clear hätte den beobachteten Monster-Hover abfangen können. Ein Transition-Clear hätte schon am gescheiterten Kellerabgang ansetzen können. Beides fehlt im aktuellen Pfad. Ohne strengere Ressourcen- und Exit-Semantik würde eine bloße Aktivierung des vorhandenen lokalen Clears das Todesrisiko jedoch nicht zuverlässig lösen.

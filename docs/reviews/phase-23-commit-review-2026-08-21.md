# Review der Phase-23-Commits

Stand: 2026-08-21. Verglichen wurde `git diff cf355b6...HEAD`. Der feste Punkt `cf355b6` ist der direkte Eltern-Commit von `455e09a`. Der Review umfasst damit:

- `455e09a` Release v0.22.0 mit Lower-Kurast-Supertruhen
- `c7bb195` Korrektur des Chat-Settle-Tests
- `c26a570` Korrekturen für `golangci-lint`
- `6077c1d` Load-Fade-Wartezeit und virtuelle Tasten für `/players N`

Sollquelle ist `docs/plans/phase-23-implementation-plan.html` in `HEAD`. Die Prüfung war statisch und umfasste den Diff, die betroffenen Tests und die im Plan festgehaltenen Live-Belege. Ein eigener D2R-Live-Lauf war nicht Teil dieses Reviews.

## Urteil

Die Hauptfunktion ist breit umgesetzt. Katalog, Object-Mode, Quantity, Schlüssel-Restock, Wegpunkt, bosslose Registry, Operate-on-sight, Rest-Sweep, Pickit, UI und Telemetrie sind vorhanden. Eine vollständige Planerfüllung lässt sich trotzdem nicht bestätigen. Drei Lücken können Laufzeit oder Replay-Verhalten ändern. Dazu kommen widersprüchliche Planstellen und mehrere Verstöße gegen die Projektdokumentation.

## Beschlüsse und Umsetzung

Stand nach der Nacharbeit vom 21. August 2026:

- **S1–S4 behoben:** Die Chest-State-Machine dokumentiert Übergänge und Budgets; der Queue-Fade-Wait reagiert auf `ctx.Done()`; Object Inspect verwendet `memory.StatQuantity`; Windows-API-Namen sind nach Projektregel formatiert.
- **S5 entschieden:** `Uber` bleibt als etablierter D2R-Eigenname für Uber Tristram und dessen Schlüssel unübersetzt. Die betroffenen Feature-Dokumente und der Phase-23-Plan erklären den Begriff ausdrücklich.
- **S6 behoben:** `world.State.InventoryQuantityByCode` ist die gemeinsame read-only Zählfunktion für persönliche Inventarstapel. Tasks und Town-Planung verwenden denselben Vertrag.
- **S7 bewusst nicht zentralisiert:** Die fünf Restock-Ressourcen bleiben explizit. Kommentare an den Switches verlangen vor einer weiteren Ressource eine gemeinsame Beschreibung von Code, Preis, Kaufmodus und Zähler.
- **Mode-Fallback behoben:** Eine UnitID mit unbekanntem Mode wird genau einmal bedient, terminal markiert und durchläuft trotzdem Drop-Wait und Pickup.
- **Replay behoben:** `attempt` und `blocker_unit_id` gehören jetzt zum `chest.tick`-Ergebnis. Ein Capture→Replay-Test belegt Hover-Miss, lokalen Clear und anschließenden Retry.
- **Necro-Recovery ergänzt:** Eine eigene Lower-Kurast-Strategy bindet den lokalen Amplify-Damage-/Bone-Spear-Clear. Sie aktiviert kein reisebegleitendes Route-Clear.
- **Loot-Vertrag angeglichen:** Drop-Wait und Pickup laufen nach jedem bestätigten Operate; danach wird derselbe Hütten-Cluster fortgesetzt.
- **Gestell-Skip präzisiert:** Truhen und Gestelle teilen weiterhin dieselbe State-Machine und Blocker-Recovery. Nur das terminale Event unterscheidet `chest_skipped` und `rack_skipped`, damit keine Parallelarchitektur entsteht und die Auswertung dennoch korrekt bleibt.
- **Plan-Drift behoben:** Normative Combat-Stellen nennen beide lokalen Profilstrategien. 28/22 ist als verworfener erster Filter markiert; 34/32 ist der aktuelle Live-Wert.

## Standards

### Harte Verstöße

#### S1. Die Chest-State-Machine ist nicht vollständig dokumentiert

- **Referenz:** `AGENTS.md`, In-code-Dokumentation Regel 3. `internal/tasks/pipeline_state.go:80-121`, `internal/tasks/chest_sweep.go`.
- **Abweichung:** Die neue State-Machine benennt ihre Zustände und speichert Tick-übergreifende Felder. Erlaubte Übergänge, Erfolg, Abbruch sowie Retry- und Timeoutpfade stehen nicht am Code.
- **Begründung:** Bei `click`, `clear_blocker`, `settle`, `wait_drops` und `pickup` ist ohne Kontrollflussanalyse nicht erkennbar, welche Zustände terminal sind und welche Budgets einen Übergang auslösen.
- **Vorgeschlagener Fix:** Am `pipelineChestState` eine kurze Übergangstabelle ergänzen. Sie soll Startzustand, erlaubte Folgezustände, Retry-Budgets, Clear-Limits und Terminalbedingungen nennen.

#### S2. Die neue Drei-Sekunden-Wartezeit ignoriert Abbruch des Queue-Kontexts

- **Referenz:** `AGENTS.md`, Safety und Bot-Verhalten. `internal/app/queue_runtime.go:429-472`, besonders `time.Sleep` in Zeile 466.
- **Abweichung:** `StartOrVerifyGame(ctx, ...)` erhält einen abbrechbaren Kontext, reicht ihn aber nicht an `finishVerifiedQueueGame` weiter. Während der Load-Fade-Wartezeit reagieren Stop oder ein abgelaufener Kontext erst nach drei Sekunden.
- **Begründung:** Der Commit ergänzt eine feste Wartezeit genau an einer Queue-Grenze, an der der Aufrufer bereits Abbruchsignale bereitstellt.
- **Vorgeschlagener Fix:** `ctx` an `finishVerifiedQueueGame` übergeben und mit `time.NewTimer` plus `select` auf Timer oder `ctx.Done()` warten. Einen Test für Abbruch während des Fade-Waits ergänzen.

#### S3. Der Object-Inspect-Kommentar ist nach Gate 23.1 veraltet

- **Referenz:** `AGENTS.md`, In-code-Dokumentation Regel 6. `internal/app/object_inspect.go:24-26`.
- **Abweichung:** Der Kommentar sagt, die produktive `StatQuantity` gehöre erst in Gate 23.1. Derselbe Diff implementiert Gate 23.1 bereits.
- **Vorgeschlagener Fix:** Den Kommentar auf die Diagnoseaufgabe beschränken und für die ID `memory.StatQuantity` statt einer lokalen Doppeldefinition verwenden.

#### S4. Neue Kommentare formatieren Windows-APIs nicht nach Projektregel

- **Referenz:** `AGENTS.md`, In-code-Dokumentation Regel 6. `internal/input/chat.go:17-19`, `internal/input/keyboard_windows.go:77-79`.
- **Abweichung:** `KEYEVENTF_UNICODE` und `SendInput` stehen ohne Backticks im Godoc.
- **Vorgeschlagener Fix:** Beide API-Namen als Code formatieren.

#### S5. Deutsche Dokumentation verwendet einen uneinheitlichen Anglizismus

- **Referenz:** `AGENTS.md`, Dokumentation und UI-Strings in ordentlichem Deutsch. Unter anderem `docs/features/lower-kurast-run.md:19`.
- **Abweichung:** "Uber-Schlüssel" steht neben deutschen Bezeichnungen.
- **Vorgeschlagener Fix:** Projektweit einen Begriff festlegen, etwa "Über-Schlüssel", oder den D2R-Eigennamen ausdrücklich als Fachbegriff dokumentieren.

### Ermessensurteile

#### S6. Die Schlüsselzählung ist doppelt implementiert

- **Geruch:** Duplicated Code.
- **Referenz:** `internal/tasks/chest_select.go:30-41`, `internal/app/town_preparation_service.go:1020-1031`.
- **Abweichung:** Beide Funktionen summieren persönliche `key`-Items mit identischer Quantity-Logik.
- **Vorgeschlagener Fix:** Eine gemeinsame read-only World-Hilfsfunktion für die Stadtschlüsselzahl verwenden. Die Paketgrenze darf dabei nicht von `world` nach `town` zeigen. Der Item-Code kann als Parameter übergeben werden.

#### S7. `RestockKey` erzeugt mehrere Ressourcenswitches

- **Geruch:** Mögliche Repeated Switches und Shotgun Surgery.
- **Referenz:** `internal/app/town_preparation_service.go`, `internal/town/restock.go`, `internal/town/vendor_cost.go`.
- **Bewertung:** Bei fünf Ressourcen ist die jetzige Lösung noch nachvollziehbar und entspricht KISS. Erst bei einer weiteren Ressource lohnt sich eine zentrale Beschreibung von Code, Preis, Kaufmodus und Zählfunktion.

## Spec

### P1. Mode-unbekannte Objekte werden nie bedient, der Run kann trotzdem erfolgreich enden

- **Planreferenz:** `docs/plans/phase-23-implementation-plan.html:1419-1422`. Ohne lesbaren Mode gilt "UnitID-once plus Drop-Wait".
- **Codereferenz:** `internal/tasks/chest_select.go:22-27`, `internal/tasks/chest_select.go:148-169`, `internal/tasks/chest_sweep.go:136-139`, `internal/tasks/chest_sweep.go:199-202`.
- **Abweichung:** `objectIsClosed` akzeptiert nur `ModeKnown && Mode == 0`. Ein Objekt mit unbekanntem Mode fällt aus jeder Auswahl. Die reine Nähegruppe setzt trotzdem `seenEligible`, wodurch der Rest-Sweep Erfolg melden kann, obwohl kein Klick versucht wurde.
- **Folge:** Ein temporär fehlgeschlagener Mode-Read kann einen grünen Run ohne bearbeitete Supertruhe erzeugen.
- **Vorgeschlagener Fix:** Mode-unbekannte UnitIDs genau einmal auswählen. Nach dem Klick nur Drop-Evidenz auswerten und die UnitID unabhängig vom Ergebnis endgültig als bearbeitet markieren. Einen Test für "Mode unbekannt, ein Versuch, kein Re-Click" ergänzen.

### P1. Headless Replay verliert die Evidenz für Blocker-Recovery

- **Planreferenz:** `docs/plans/phase-23-implementation-plan.html:1182-1185` verlangt Replay-Unterstützung. Die Live-Anpassung in `docs/plans/phase-23-implementation-plan.html:1345-1350` bindet den lokalen Clear an einen Memory-bestätigten Monster-Hover.
- **Codereferenz:** `internal/replay/deps.go:496-500`, `internal/replay/replay_deps.go:347-350`, Felddefinition in `internal/tasks/deps.go:62-65`.
- **Abweichung:** `ChestOperateResult.BlockerUnitID` wird weder in `chest.tick` geschrieben noch beim Replay dekodiert.
- **Folge:** Live startet nach erschöpfter Objektsuche den lokalen Clear. Replay sieht keinen Blocker, überspringt das Objekt und verbraucht die nachfolgenden `route_clear`-Calls an der falschen Stelle.
- **Vorgeschlagener Fix:** `blocker_unit_id` und der Vollständigkeit halber `attempt` im Trace-Ergebnis speichern und dekodieren. Einen Capture-zu-Replay-Test für Hover-Miss, Local-Clear und Retry ergänzen.

### P1. Necro wird als unterstützt angeboten, kann bei derselben Blocker-Evidenz aber den ganzen Run abbrechen

- **Planreferenz:** `docs/plans/phase-23-implementation-plan.html:521`, `:661`, `:956-958` verlangt für beide Profile eine No-op-Strategy ohne Route-Clear. Die spätere Live-Anpassung `:1345-1350` beschreibt nur einen lokalen Hammerdin-Clear.
- **Codereferenz:** `internal/app/combat_strategy_registry.go:30-39`, `internal/profile/necrobonespear/strategy.go:42-55`, `internal/app/app.go:309-312`, `internal/tasks/chest_sweep.go:370-393`, `internal/tasks/chest_sweep.go:449-460`, `internal/profile/executor.go:499-505`.
- **Abweichung:** Der Necro nutzt `NewBossFactory("lower-kurast")`; dessen `Configure` bindet keinen Route-Clear. `tasks.Deps.RouteClear` enthält trotzdem immer den Profil-Executor. `startChestBlockerClear` prüft nur auf ein nicht-nil Interface und startet den Clear. Der Executor antwortet dann mit `route_clear_strategy_unavailable`; die Chest-Pipeline macht daraus `combat_action_failed` und bricht den Run ab.
- **Folge:** Dieselbe verdeckte Truhe wird beim Hammerdin lokal freigekämpft, beim als unterstützt ausgewiesenen Necro kann sie den gesamten Run beenden. Das widerspricht dem vorgesehenen Skip-Verhalten nach ausgeschöpfter Objektsuche.
- **Vorgeschlagener Fix:** Blocker-Recovery nur starten, wenn das konkrete Profil Route-Clear wirklich konfiguriert hat. Fehlt diese Fähigkeit, die UnitID mit `chest_hover_not_found` überspringen. Alternativ eine eigene Necro-LK-Strategy konfigurieren, falls lokaler Kampf auch für den Necro Produktvertrag werden soll. Dafür braucht es einen Profilintegrationstest mit nicht verfügbarem Route-Clear.

### P2. Drop-Wait und Pickup laufen pro Hütten-Cluster statt nach jeder Öffnung

- **Planreferenz:** `docs/plans/phase-23-implementation-plan.html:530`, `:914-916`, `:1182-1184` fordert `wait_for_drops` nach jeder Öffnung.
- **Codereferenz:** `internal/tasks/chest_sweep.go:124-133`, `internal/tasks/chest_sweep.go:247-254`, `internal/tasks/chest_select.go:205-210`.
- **Abweichung:** Nach bestätigtem Öffnen wechselt die State-Machine auf Idle und wählt bei gesetzter `clusterChest` erst die Gestelle. Drop-Wait und Pickup beginnen erst, wenn im Cluster kein weiteres Objekt übrig ist.
- **Begründung:** `docs/plans/phase-23-implementation-plan.html:883-885` beschreibt dagegen Truhe, danach Gestelle und erst dann Drop-Wait. Der Plan ist an dieser Stelle widersprüchlich. Die Feature-Dokumentation übernimmt das Cluster-Verhalten.
- **Vorgeschlagener Fix:** Produktentscheidung festziehen. Wenn das Gate gilt, nach jedem bestätigten Operate warten und looten, danach den Cluster fortsetzen. Wenn Cluster-Loot beabsichtigt ist, die früheren Muss-Formulierungen im Plan ändern und das Verhalten mit einem Drop-Stabilitäts-Test absichern.

### P2. `chest_skipped` umfasst auch fehlgeschlagene Gestelle

- **Planreferenz:** `docs/plans/phase-23-implementation-plan.html:973` und `:1234-1235` trennt geöffnete oder übersprungene Truhen von bedienten Gestellen.
- **Codereferenz:** `internal/tasks/chest_sweep.go:59-64`, `internal/tasks/chest_sweep.go:109-121`, `internal/telemetry/recorder.go:127-132`.
- **Abweichung:** `abandonChest` wird für Truhen und Gestelle verwendet. Beide erzeugen `ChestSkipped`. Der Kommentar am Event bestätigt diese Vermischung.
- **Folge:** Eine Auswertung nach Eventname kann fehlgeschlagene Rack-Klicks als übersprungene Supertruhen zählen.
- **Vorgeschlagener Fix:** `ChestSkipped` nur für `ObjectKindSuperChest` senden. Für Gestelle entweder kein Skip-Event schreiben oder ein eigenes `rack_skipped` einführen, falls diese Information gebraucht wird.

### P2. Der Plan widerspricht sich beim Combat-Vertrag und beim Nähefilter

- **Planreferenz Combat:** `docs/plans/phase-23-implementation-plan.html:521`, `:661`, `:956-958`, `:1169-1171` fordert No-op-Konfiguration. `:1345-1350` führt später lokalen Hammerdin-Kampf ein.
- **Planreferenz Nähefilter:** `docs/plans/phase-23-implementation-plan.html:1192-1204` und `:1458` nennt 28/22. `:1322-1325` sowie `internal/tasks/chest_select.go:9-19` verwenden 34/32.
- **Abweichung:** Die historischen Live-Erkenntnisse wurden ergänzt, die vorherigen Soll- und Abschlussstellen aber nicht aktualisiert.
- **Folge:** Der aktuelle HTML-Plan kann nicht als eindeutige Abnahmequelle dienen. Je nach Abschnitt ist dieselbe Implementierung konform oder abweichend.
- **Vorgeschlagener Fix:** Historische Werte deutlich als verworfen markieren. Die normativen Abschnitte auf lokalen, objektgebundenen Hammerdin-Clear und 34/32 aktualisieren. Außerdem festhalten, dass Necro ohne konfigurierte Recovery skippt, sofern kein eigener Necro-Clear beschlossen wird.

## Nachgewiesene Planerfüllung

Die folgenden Anforderungen sind im Diff mit Code und Tests nachvollziehbar:

- Objektkatalog 181, 183, 104 und 107 aus dem lokalen `objects.txt`-Generator
- `ObjectKindSuperChest`, `ObjectKindRack`, Object-Mode und `ModeKnown`
- Base-first `StatQuantity` 70 und persönliche Inventarsumme für `key`
- Akara-Restock nur vor `lower-kurast`, Schwelle 6, Ziel 12, Einzelkauf, Preis 45
- `WaypointTargetLowerKurast` und isolierter Town-Test für Area 79
- Bosslose Einzelroute mit `RunCapabilityChestSweep`, Endpoint in Area 79 und Act-3-Egress
- Operate-on-sight, UnitID-Pin, begrenzte Approaches, Rest-Sweep und `chest_sweep_empty`
- Pickit `r21` bis `r30` sowie Elite-Unique und Elite-Set
- Deutsche UI-Labels, Aufnahmevertrag, zwei Bilder und schließbares Overlay
- `boss_kills == 0` durch einen Pipelinepfad ohne Boss-Schritte

## Empfohlene Reihenfolge

1. Mode-Fallback und Replay-Feldverlust beheben. Beide unterlaufen die Fail-closed- und Replay-Verträge direkt.
2. Profilabhängige Blocker-Recovery absichern, damit Necro nicht aus einem optionalen Skip einen Run-Abbruch macht.
3. Drop-Wait-Vertrag und Telemetrie-Semantik entscheiden und Plan, Tests sowie Feature-Doc gemeinsam angleichen.
4. Plan-Drift und State-Machine-Dokumentation bereinigen.
5. Den abbrechbaren `/players`-Fade-Wait und die kleineren Godoc-Abweichungen korrigieren.

---
*Erstellt: 2026-08-21. Statischer Commit- und Spec-Review, kein D2R-Live-Playtest.*

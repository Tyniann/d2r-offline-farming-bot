# Storage Compaction

## Überblick

Ein bestätigter voller Materialslot löst automatisch die Verdichtung seiner Familie aus. Edelsteine und Schädel werden ab normaler Stufe bis perfekt verdichtet, El bis Ort genau zur nächsten Rune aufgewertet. Danach lagert der gemeinsame Return-Flow das ursprünglich blockierte Item ein und setzt die Session fort. Volle Ziele oder unbestätigte Aktionen beenden sie mit einem verständlichen Fehler. Charaktereinstellungen sind dafür nicht erforderlich.

Die Operator-Logs bestätigen beide Rubinrezepte und El → Eld mit einem Rechtsklick je Zutatensatz, Transmute und Rücklagerung. Pause/Fortsetzen sowie Tabnavigation aus Persönlich/Gemeinsam und von Gems zu Runen sind live belegt. Überlaufplanung, Grenzfälle und Wiederaufnahme aller Return-Flows sind automatisch geprüft. Ein natürlich auftretender Farming-Überlauf bleibt eine ergänzende Logbeobachtung; er wurde nicht als live abgenommen ausgegeben. Details und Nachweise stehen im [Phase-25-Plan](../plans/phase-25-implementation-plan.html).

## Ort im Code

- **Paket:** `internal/crafting/`, Diagnose in `internal/app/`.
- **Einstieg:** gemeinsamer Task-Return → `internal/app/storage_compaction.go` → `crafting.Executor`; CLI-Diagnosen über `cmd/d2rbot/main.go`.
- **Wichtige Dateien:** `internal/crafting/catalog.go`, `catalog_data.go`, `planner.go`, `executor.go`, `tools/generate-crafting-catalog/main.go`, `internal/app/storage_inspect.go`, `internal/memory/collection.go`, `internal/world/collection.go`, `internal/replay/collection.go`, `internal/loot/collection_actions.go`, `internal/input/positioned_click.go`.
- **Config:** keine neuen YAML-Keys oder Charaktereinstellungen.
- **Lokale Artefakte:** `diagnostics/storage/*.json`, gitignored.

## Funktionalität

### Rezeptkatalog

Der Generator liest ausschließlich die lokalen CASC-Extrakten `cubemain.txt` und `misc.txt`. Er wählt genau 14 Gem-/Skull-Rezepte, normal → makellos und makellos → perfekt, sowie neun Runenrezepte von El → Eld bis Ort → Thul. Lädiert, fehlerhaft und Thul als Zutat sind ausgeschlossen.

Jede ausgewählte Zeile muss aktiviert sein, drei identische Zutaten und genau einen Output ohne weitere Bedingungen oder Modifikatoren enthalten. Alle anderen Verhaltensfelder müssen leer sein. Die beiden Itemcodes stammen aus den eindeutigen `misc.name`-Zeilen; Typ und 1×1-Inventarmaße werden geprüft. Numerische Item-IDs werden nicht erzeugt. `version=0/100` ist ein Rezeptfeld und kein Prozessversionsgate. `misc.maxstack` wird nicht als Materialkapazität verwendet.

`crafting.Recipes()` liefert eine Kopie. `LookupRecipe` findet das einzige erlaubte Rezept eines Quellcodes. Jede Zeile trägt `cubemain.txt`, den stabilen `description`-Schlüssel sowie Zutaten- und Outputcode. Die kleinen Originalzeilen unter `tools/generate-crafting-catalog/testdata/` machen Tests unabhängig von D2R und `.tmp`.

```powershell
go generate ./internal/crafting
```

### Read-only Materialtruhe-Diagnose

Der Operator bereitet den Zustand vor. Der Modus wartet höchstens 30 Sekunden auf zwölf gültige In-Game-Samples und veröffentlicht sie atomar. Run-Auswahl, autonome Queue und erforderliches Charakterloadout sind deaktiviert. Der eigene Loop ruft weder den Task-Loop noch Input-Controller, Fensterbindung oder Hotkeys auf. Auch eine Konfiguration mit aktiviertem Input erzeugt keine Spielaktion.

Jedes Sample enthält Snapshot-Zeit und Generation, Spieler-UnitID und rohe Identität, bereits bekannte UI-Flags, rohe und gemappte Items sowie gesonderte Active-/Base-Statlisten. Vollständige UI-Captures klammern die Zusatzreads ein. Dazu kommt das vorhandene begrenzte Forschungsfenster von `UI-0x8000` bis `UI+0x7fff`, insgesamt 64 KiB. Die Daten werden vor dem nächsten Poll kopiert.

Die zusätzlichen Diagnosereads erfolgen nacheinander. `ui_unchanged=true` bedeutet nur, dass die beiden UI-Buffer gleich sind. Es beweist keine atomare Erfassung von Collection-Zählern und Cube-Units. Seit 25.2 enthalten `collection` und `world_collection` die gleichzeitig mit dem normalen Snapshot gelesene Collection-Evidenz. Leere Itemlisten und gewöhnliche Item-Quantity sind kein Ersatz dafür.

### Belegte Materialbestände und Tabwahl

`memory/collection.go` entdeckt das Collection-Inventar über die Spieler-Owner-Strukturen. Die Gate-25.1-Aufnahme belegt den Proxy-Zähler `ItemData+0x9C`, seine Parent-/Vorgänger-/Nachfolgerverweise und den Inventarheader. Der Read prüft die tatsächliche gebundene Prozessversion `3.2.92777`, Signatur, Owner-Rückverweis, alle Listenlinks und Endpunkte. Höchstens 512 Nodes werden akzeptiert. Zwei vollständige Reads müssen identische Counts und denselben Header liefern; ein inzwischen geänderter UI-Kontext widerruft die Quelle.

`CollectionSnapshot` enthält rohe Txt-Referenzen und ein Hash-basiertes Scope-Kennzeichen der aktuellen Inventarinstanz. Es ist kein dauerhaftes Charakter- oder Speicherbereichskennzeichen. Jeder Snapshot liest neu, ohne Cache über Spiel- oder Prozesswechsel. World löst die Referenzen über den vorhandenen lokalen CASC-Katalog auf. Die Runenaufnahme aus 25.4 bestätigt dieselbe Quelle für El = 7 und Eld = 6; Gem-, Skull- und Runen-Counts sind freigegeben. Der CASC-Typ für Smaragde lautet `geme`.

`memory/collection_tab.go` liest ausschließlich auf Client `3.2.92777` den aktuellen `PanelManager`, dessen begrenzte Kinderliste und genau ein `BankExpansionLayout`. VTable, Namen, Parent-Verweise und die vier Truhenbereiche werden geprüft. Genau ein sichtbarer und aktivierter Bereich liefert den aktiven Tab: `basic` (Persönlich/Gemeinsam), `materials`, `gems` oder `runes`. Basic und Materials erlauben ausschließlich die Navigation zum benötigten Materialtab. Beide Tab-Reads klammern die beiden Bestandsreads ein und müssen dieselbe Instanz und denselben Tab ergeben. Fehler, Mehrdeutigkeit oder ein Wechsel lassen `TabKnown=false`; unabhängig belegte Counts bleiben verfügbar. Es gibt keinen Laufzeit-Heapscan, Screenshotread oder angenommenen Tab aus einem Klick. Die Fixtures `internal/memory/testdata/collection-tabs*.json` erhalten die benötigten Originalbytes und ihre Herkunft. Die Basic-/Materials-Testzustände schalten nur die bekannten Aktivierungsflags dieser Fixtures um und gelten nicht als neue Live-Captures.

`State.CollectionCount(code)` verlangt einen gültigen In-Game-State, offenen Stash und exakt passende Zeit/Generation. Fehlende oder doppelte Slots und Werte außerhalb 0…99 bleiben unknown. Eine gelesene 0 ist bekannt. Das World Model kopiert die Counts und löscht sie bei Reset.

Runtime-Trace-Schema 2 normalisiert dieselben Werte und Known-Flags ohne Prozessadressen. Der Decoder bindet sie an den Replay-Frame. Alte Schema-1-Traces bleiben ohne Collection-Evidenz lesbar und erfinden keinen Bestand. Die automatische Fixture-Matrix übernimmt den konkreten Live-Wert und prüft Grenzwerte, beschädigte Daten, Scope-Wechsel und stale Frames.

### Collection-Eingabeaktionen

Sobald der Charakter einen Horadrimwürfel im persönlichen Inventar trägt, enthält die Truhe unter ihren Items ein eingebettetes 3×4-Würfelraster samt Transmute-Button. Strg+Shift+Linksklick transferiert zwischen Materialslot und diesem Raster. Der Ablauf verwendet diese Truhenansicht vollständig. Es gibt keine eigene Öffnen-/Schließen-Aktion und keinen zusätzlichen Cube-Open-Zustand. Das bestehende `UI.CubeOpen` bleibt für den separat geöffneten Cube des Cow-Ablaufs zuständig.

`loot.CollectionActions` bietet Tabwahl, einzelne Entnahme, Rückgabe eines gebundenen Cube-Outputs und Transmute. Es besitzt keine Rezepte oder Batch-Regeln. Jeder Aufruf prüft gültigen aktuellen Scope, bekannten Truhenbereich, einen eigenen Cube im persönlichen Inventar, Rogue Encampment, offenen Stash und Inventar, leeren Cursor sowie 1280×720. Basic und Materials dürfen einen Gems-/Runen-Tabklick starten. Material- und Cube-Aktionen benötigen anschließend den frisch bestätigten Zieltab und das belegte Settle; unbekannte UI bleibt gesperrt. Der Code `box` stammt aus dem vorhandenen CASC-Katalog. Pro Generation ist höchstens ein gesendeter Klick zulässig, auch bei einem Senderfehler. Reset löscht nur Softwarebindungen.

`DefaultCollectionLayout()` enthält die Ziele der unveränderten 1280×720-Operator-Screenshots aus 25.4: unterstützte Materialzellen, Tabbuttons, eingebettetes Cube-Raster und Transmute-Button. Das produktive Settle beträgt 300 ms, aufgerundet aus den beiden erfolgreichen Transfer-Logs mit 299 ms und 298 ms. Die Wartezeit ersetzt keine frische Memory-Bestätigung des aktiven Tabs. Ein unbekanntes Settle bleibt gesperrt. Beide Transfers nutzten die tatsächlich gelesene Cube-Position `2,3`; Rückgaben dürfen keine feste erste Zelle annehmen. Die drei Operator-Rezeptlogs aus 25.8 bestätigen die reale Transmute-Aktion. Automatische Aktionstests verwenden synthetische Layouts und prüfen zusätzlich den Zeitvertrag des kalibrierten Defaults. Der App-Adapter verdrahtet diese Aktionen mit Crafting; der gemeinsame Return-Flow ruft ihn ausschließlich bei bestätigtem Überlauf auf.

`input.Controller.ClickAtWithCtrlShift` hält beide Modifier, Cursorbewegung und genau einen Klick unter derselben Gameplay-Lease. Geometrie, Fokus und Safety werden innerhalb der Transaktion geprüft. Teilfehler geben Maus und beide versuchten Tasten frei; ein Releasefehler stoppt weitere Eingaben. `ClickAt` bietet denselben positionierten Vertrag für einfache Tab-/Button-Klicks. Einzelne Input-Primitives werden nicht über mehrere separate Tastenaufrufe zusammengesetzt.

`WithdrawIngredients` sendet anhand des Operator-Hinweises einen Strg+Shift+Rechtsklick pro Zutatensatz in den leeren Cube. Er ersetzt drei einzelne Entnahmen. Der Executor bestätigt Quellzahl −3 und genau drei neue passende eigene Cube-Units; eine unvollständige Übertragung wird nicht durch weitere Klicks aufgefüllt. Die Rücklagerung des Outputs bleibt beim Strg+Shift+Linksklick. Der vorhandene Controller akzeptiert bereits den Mausbutton; eine zweite Modifier-Implementierung ist unnötig. Die drei Operator-Rezeptlogs aus 25.8 bestätigen den realen Rechtsklick und die erwartete Zutatenmenge. Höhere Runen bleiben ausgeschlossen, unabhängig davon, ob der Spielbefehl dafür automatisch zwei Zutaten wählt.

### Batch-Planer und Executor

`BuildPlan` liest bekannte aktuelle Collection-Counts und simuliert ausschließlich die Triggerfamilie. Bei makellosen Gems/Skulls und niedrigen Runen gilt das kleinere Limit aus Quellmenge geteilt durch drei und Zielkapazität. Normale Gems/Skulls dürfen den makellosen Zwischenslot zunächst über ein perfektes Rezept freimachen und anschließend beide unterstützten Stufen derselben Familie verarbeiten. Ein bereits voller perfekter Endslot wird vor jedem Klick abgelehnt. Der Plan muss den Quellslot tatsächlich entlasten. Jeder Schritt trägt die erwarteten Vorher-/Nachherzahlen der ganzen beteiligten Familie; höchstens 77 beziehungsweise 33 Rezepte sind möglich.

`Executor.Tick` bindet ein zuvor gescheitertes ungelocktes Keep-Item, Run-Generation, Scope und das persönliche Inventar. Der Aufrufer autorisiert Pickit und Lock-Grid; der Executor verlangt zusätzlich den bekannten vollen Quellslot, den getragenen Cube, dessen leeren Inhalt und einen leeren Cursor. Er verbraucht nur Collection-Zutaten. Die Zustände wählen und bestätigen den Tab, laden und bestätigen den Zutatensatz, transmutieren und bestätigen genau einen neuen Output, lagern ihn zurück und bestätigen den leeren Cube. Ein Fortschrittsereignis entsteht erst nach vollständiger Rücklagerung. Erfolg verlangt unverändertes Inventar und freien Platz im ursprünglichen Slot; der normale Stash-Transfer bleibt Aufgabe des Return-Flows.

Jede Bestätigung benötigt einen neueren gültigen Frame. Unvollständige Zutaten- oder verzögerte Output-Evidenz wartet ohne erneuten Input; fremde Cube-Items, partieller Rezeptverbrauch, bekannte Bestandsdrift oder geänderte Bindungen stoppen. Pro Tick wird höchstens eine Aktion gesendet. Einmalige Aktionszustände werden vor dem Senderaufruf gesetzt, damit auch Teilfehler keine Wiederholung erlauben. Jede Bestätigung hat drei Sekunden aktive Zeit, der Auftrag 240 Sekunden. `Tick` muss auch bei Pause mit dem Pause-Flag aufgerufen werden: beide Budgets stehen still, nach Resume werden Bindungen frisch geprüft. Stop und Reset senden keine Bereinigungsklicks. Terminale Ergebnisse sind bis Reset gespeichert und erzeugen kein zweites Fortschrittsereignis.

### Gemeinsamer Stash-Return

`StashResult` erhält die tatsächliche Fehlerursache und Triggerposition. Nur `verify_timeout` nach ausgeschöpften normalen Stash-Versuchen kann ein Kandidat sein. `AuthorizeCompaction` prüft das unveränderte eigene Keep-Item, Lock-Grid, konsistentes Inventar und dieselbe Pickit-Zuordnung samt Revision erneut. Der App-Adapter friert bereits beim Stash-Fehler den Collection-Scope ein; ein Wechsel vor dem ersten Executor-Tick widerruft die Freigabe. Input-, Policy- und Telemetriefehler verdichten nie.

Der gemeinsame Return-State hält Pending-Request, erwarteten Trigger-Retry und die pro Truhenbesuch bereits behandelten UnitIDs. Bei einem unterstützten Kandidaten verlangt er einen bekannten aktuellen Quellbestand von 99. Dann gilt `stash_items → compact_storage → stash_items`. Erst der bestätigte normale Stash-Transfer des Originalitems erlaubt den weiteren Abschluss. Eine zweite gescheiterte Übertragung derselben Unit bleibt terminal; andere volle Kandidaten dürfen anschließend separat behandelt werden. Ein neuer Truhenbesuch und die zentrale Run-/Session-Resetbarriere löschen diese Softwarebindungen.

Cow delegiert an denselben Return-State wie Countess, Summoner, Mephisto, Nihlathak und Lower Kurast; `stash-personal` und `loot-and-return` verwenden ihn ebenfalls. `compact_storage` bleibt in der Return-Town-History und im bestehenden Stash-Fortschritt. Der Step verwendet den endlichen aktiven Executor-Zeitvertrag statt des gewöhnlichen Wall-Clock-Step-Timeouts. Während der Rezeptaktion bleiben automatische Profil-/Potion-Aktionen gesperrt. Der Runtime-Poll beobachtet Pause auch dann, wenn Tasks keinen Inputtick bekommen, damit die Budgets stehen bleiben. Compaction-Fehler bleiben auch bei einem benutzerdefinierten Retry-Eintrag terminal: kein Recovery-Portal, Save & Exit oder Queue-Neustart.

### Telemetrie, Replay und Fehleranzeige

Der App-Adapter schreibt `storage_compaction_started`, `storage_compaction_recipe`, `storage_compaction_completed` und `storage_compaction_failed` synchron über den bestehenden Run-Recorder. Ein Rezept gilt erst nach bestätigter Rücklagerung als Fortschritt. Sein eigener Mengenblock enthält CASC-Rezeptschlüssel, Quell-/Zielcode und Bestände. Diese Ereignisse erhöhen den Farming-Loot-Funnel nicht. Nur der anschließend normal eingelagerte ursprüngliche Trigger erhält `stash_success`. Ein fehlgeschlagener Emit verhindert den nächsten Input.

Die Fehlercodes `storage_gem_full`, `storage_rune_full`, `storage_state_unavailable`, `storage_cube_not_empty`, `storage_compaction_unconfirmed` und `storage_compaction_timeout` bleiben terminal. Volle perfekte Gems/Skulls und Runen ohne freigegebenes Rezept melden ihr volles Material, ohne einen Auftrag anzulegen. `ReasonParams` erhält `item_code` des Triggers und `material_code` des vollen Ziels durch Task, Supervisor, Session-/Run-Telemetrie, History und API. Bestehende DE-/EN-Presenter lokalisieren diese Codes über den Game-Katalog; auch `stash_failed` bekommt einen verständlichen Text. Die bestehende Session-Übersicht zeigt die eigene aufklappbare Kategorie „Verdichtet“. Sie zählt bestätigte, rückgelagerte Rezeptresultate pro Zielcode; später weiterverdichtete Zwischenstufen bleiben als geleistete Arbeit gezählt. Auch bei einem späteren Abbruch bestätigte Rezepte bleiben sichtbar. Die Kategorie misst keinen abschließenden Speicherbestand und erhöht keine Loot-Ausbeute. Keine Charaktersettings oder zusätzliche UI-Seite.

Runtime-Traces protokollieren den Stash-Kandidaten samt Ursache/Position und die `compaction.tick`-Dependency mit Request, Pending, Fortschritt oder terminalem Ergebnis. Task-Replay prüft den Übergang zur Verdichtung und zurück zum Originaltransfer. Es führt die internen Crafting-Aktionen nicht erneut aus; deren Reihenfolge prüfen die Executor-Fixtures. Die bestehenden Schema-2-Dependency-Maps benötigen dafür keinen neuen Top-Level-Tracevertrag.

## Datenmodell

- `crafting.Recipe`: Quellenreferenz, Quellcode, Menge drei, Zielcode und CASC-Rezeptversion.
- `crafting.Plan` / `PlannedRecipe`: Materialtab und begrenzte Rezeptliste samt simulierten Beständen.
- `crafting.Request`: UnitID, Code und Inventarposition des autorisierten Triggers sowie Run-Generation.
- `crafting.Result` / `Progress` / `Failure`: Pending beziehungsweise verifizierter Rezeptfortschritt, terminaler Erfolg oder stabiler Fehler mit Trigger- und Materialkontext.
- Diagnose-Schema `2`: Label, PID, tatsächliche `game_version` aus der Prozessbindung, getrennte konfigurierte Version, Collection-Rohdaten und World-Projektion. Die erste Gate-25.1-Aufnahme verwendet noch Schema 1.
- `completion`: `complete` bei zwölf Samples, andernfalls `partial_timeout` oder `partial_cancelled`, wenn bereits Samples vorliegen.
- Fenster enthalten Zeit, Ankerindex, Länge und Hexdaten oder expliziten Fehler. Nicht lesbar wird nicht als leer interpretiert.
- Die bestehende Item-Enumeration überspringt unlesbare Units und hat eigene Limits. Dieses Artefakt erklärt daher keine Collection für vollständig.

## Operator / CLI

Den lokalen Build verwenden und die Desktop-App vorher beenden. Beispiel mit installiertem Datenroot:

```powershell
.\.tmp\phase25\d2rbot.exe --data-root "$env:LOCALAPPDATA\D2ROfflineFarmingBot" --storage-inspect runes-current-stock
```

Labels bestehen aus 1–64 Kleinbuchstaben, Ziffern und Bindestrichen und beginnen mit Buchstabe oder Ziffer. Optional begrenzt `--storage-inspect-timeout-ms` die Gesamtdauer. `Ctrl+C` beendet die read-only Aufnahme; F11/F12 sind in diesem Modus nicht registriert. Andere Lauf-, Inspect- und Testmodi sind ausgeschlossen. Ohne In-Game-Sample oder bei Prozessverlust gibt es einen Fehler; partielle Artefakte gelten nicht als vollständige Abnahme.

Das Log `storage inspect written` nennt den Pfad. Er wird relativ zum Verzeichnis der geladenen Konfigurationsdatei aufgelöst. Beim installierten Datenroot liegt er unter `%LOCALAPPDATA%\D2ROfflineFarmingBot\configs\diagnostics\storage\`. Die Rohdaten bleiben lokal. Der Bestandsabgleich steht im [Gate 25.1 des Plans](../plans/phase-25-implementation-plan.html#gate-25-1), die abgeschlossene UI-Kalibrierung im [Gate 25.4](../plans/phase-25-implementation-plan.html#gate-25-4).

Für Gate 25.1 genügt eine Aufnahme des vorhandenen Gems-Tabs. Der konkrete Vergleichswert aus dem Operator-Screenshot ist normaler Rubin (`gsr`), Menge **22**. Die zwölf internen Samples stammen aus diesem einen unveränderten Zustand. Bei inzwischen geändertem Bestand gilt die tatsächlich sichtbare Menge mit entsprechendem Bild. Grenzwerte und volle Slots werden automatisch mit Fixtures geprüft. Gate 25.4 verwendet diese Aufnahme weiter und ergänzt einmal den vorhandenen Runentab mit vollständiger Spielansicht. Danach folgt je ein Transfer und Rücktransfer mit einem bestehenden Gem und einer bestehenden niedrigen Rune.

### Isolierter Transfer-Test aus 25.4

Mit geschlossenem Desktop-Bot, vorbereitetem Offline-Spiel in Rogue Encampment, 1280×720, getragenem Cube und leerem Truhen-Würfelraster:

```powershell
# Zuerst Runen geöffnet lassen; danach ist Gems geöffnet.
& .\.tmp\phase25\d2rbot.exe --data-root "$env:LOCALAPPDATA\D2ROfflineFarmingBot" --input-test storage-transfer:gsr
# Erst nach erfolgreichem Rubin-Test; Gems geöffnet lassen.
& .\.tmp\phase25\d2rbot.exe --data-root "$env:LOCALAPPDATA\D2ROfflineFarmingBot" --input-test storage-transfer:r01
```

Die Specs sind alleinstehende Aktionen im bestehenden Input-Testmodus und benötigen dessen aktivierten Input und installierten Charakter-Loadout. Der Modus startet keine Farming-Tasks. Er sendet einen Tabklick, verlangt zwei frische übereinstimmende Tab-/Bestandsframes und verwendet die gemessene Zeit als lokales Settle. Erst nach erneutem bestätigtem Settle sendet er genau eine Entnahme und die Rückgabe derselben verifizierten Cube-Unit. Er bestätigt Bestand −1/+1, unverändertes persönliches Inventar, leeren Cursor, offene Truhe und zum Abschluss leeren Cube. Transmute wird nie aufgerufen.

`storage transfer tab calibrated` enthält `settle_ms`; `storage transfer test completed` bestätigt die abgeschlossene Rückgabe. Jede Bestätigung hat höchstens drei Sekunden aktive Zeit. Die konfigurierten Pause-/Stop-Hotkeys bleiben aktiv (auf diesem Host Pause-Taste und F11); Stop, Kontextwechsel oder fehlende Bestätigung beenden den Test ohne Klickwiederholung oder automatische Bereinigung eines Teiltransfers. Die Logs liegen im installierten Datenroot unter `logs/`. Die erfolgreichen Abnahmen vom 1. Oktober 2026 stehen in `d2rbot-20261001-031742.log` und `d2rbot-20261001-032213.log`; eine Wiederholung ist nicht erforderlich. Das daraus abgeleitete produktive Settle beträgt 300 ms.

### Isolierte Rezeptabnahme aus 25.8

`--input-test storage-recipe:gsr:pause` verarbeitet genau einmal normaler Rubin → makelloser Rubin. `storage-recipe:glr` verarbeitet makelloser Rubin → perfekter Rubin, `storage-recipe:r01` El → Eld. Die Specs stehen jeweils allein. Der Test verwendet `NewRecipeTestExecutor` mit demselben Zustandsautomaten und den echten aktuellen Counts, maximal ein Rezept. Nur der volle persönliche Overflow-Trigger wird durch den ausdrücklichen CLI-Testauftrag ersetzt. Der produktive Executor akzeptiert keine UnitID-0-Testfreigabe und behält seine volle Quellslot-/Keep-Bindung.

Vorbereitung und Sperren entsprechen den Collection-Aktionen: Offline-Spiel in Rogue Encampment, 1280×720, offene Truhe/Inventar, getragener Cube sowie leerer Cursor und Truhen-Cube. Je drei vorhandene Zutaten und ein freier Zielplatz genügen. `:pause` hält einmal nach bestätigtem Rechtsklick-Zutatensatz vor Transmute an; die konfigurierte Pause-Taste setzt fort. Hotkeys, aktive Zeitbudgets und alle Bestätigungen bleiben wirksam. Ein Fehler führt ohne Nachklicken oder Bereinigung zum Abbruch.

`storage recipe verified` protokolliert −3/+1 nach bestätigter Rücklagerung, `storage recipe test completed` den abgeschlossenen Einzelzyklus mit leerem Cube/Cursor und unverändertem Inventar. Die isolierte CLI erzeugt keine produktive Session und keine Farming-Ausbeute. Die Logs `d2rbot-20261001-045320.log`, `045413.log` und `045431.log` bestätigen die drei Rezepte. Bestände danach: normaler Rubin 19, makelloser Rubin 95, perfekter Rubin 6, El 4 und Eld 7. Der erste Versuch (`045237.log`) stoppte ohne Klick mit `storage_state_unavailable`, weil Basic/Materials noch als unbekannt verworfen wurden. Diese Tabs bleiben jetzt als konsistent gelesene Navigationsstarts erhalten, auch in World und Replay. Die gezielte Regression prüft Mapping → echten Executor → echte Collection-Aktionen sowie die Sperre während unbestätigter Tabwechsel. Der abschließende Live-Test aus Persönlich/Gemeinsam (`d2rbot-20261001-051126.log`) bestätigt Gems-Auswahl und Rubin 19 → 18 → 19 mit derselben Cube-Unit, leerem Cube/Cursor und unverändertem Inventar. Die Abnahme ist abgeschlossen; die Befehle müssen nicht erneut ausgeführt werden. Alle Nachweise stehen im [Gate 25.8](../plans/phase-25-implementation-plan.html#gate-25-8).

## Abhängigkeiten

Die Diagnose verwendet die bestehende read-only Prozessbindung, `memory.ProbeReader`, UI-Capture und Item-Statlisten-Diagnose. Das Crafting-Paket konsumiert ausschließlich World-Evidenz und injizierte Aktionen; es importiert weder Memory noch Loot, App oder Tasks. D2R-Installations- und Savegame-Dateien werden nicht verändert.

## Verwandte Features

- [Personal-Stash MVP](personal-stash-mvp.md)
- [Inventory Model und Lock Grid](inventory-lock-grid.md)
- [Read-only UI-State-Probe](ui-state-probe.md)
- [Item Enumeration Read-Only](item-enumeration.md)
- [Cow Level](cow-level-run.md)

---
*Zuletzt aktualisiert: 2026-10-01*

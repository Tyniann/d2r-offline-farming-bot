# Town Services

## Überblick

Phase 9 definiert den fail-closed Vertrag für die bedarfsorientierte Run-Vorbereitung. Phase 10.6 bindet die vorbereiteten Identify-/Sell-Verträge produktiv an run-spezifische Loot-Policies, den geordneten Servicegraphen und Memory-verifizierten App-Input.

## Ort im Code

- **Paket:** `internal/town/`
- **Einstieg:** `internal/town/types.go`
- **Wichtige Typen:** `OriginAct`, `Egress`, `HubTransfer`, `Demand`, `PlanStep` und `Budgets`

## Funktionalität

`Plan.Validate` erzwingt die Reihenfolge: Ein Fremdakt beginnt mit Egress und bestätigtem Hub-Transfer nach Act 1. Erst danach dürfen Stash- und Service-Schritte folgen; ein Act-1-Start benötigt keine Normalisierung. Unbekannte Herkunft stoppt mit `town_origin_unknown`. Budgets begrenzen schon im Vertrag Input-, Verify-, Retry- und Gesamtversuche.

## Read-only Research (9.1)

`--town-inspect` schreibt nach einem gültigen World-Snapshot einen JSON-Bericht nach `diagnostics/town/`. Er klassifiziert Act, NPCs, Belt, Scroll-Zähler, `Identified`, Vendor-Kandidaten, Gold und Haltbarkeit in `reliable`, `optional`, `shifted` oder `unavailable`. Nicht validierte Quellen werden nie geschätzt.

Die Item-Enumeration belegt Belt-Items samt Typ und Position sowie den `Identified`-Status. Tome-Quantitäten und Haltbarkeits-Stats bleiben `shifted`; Vendor-Kandidaten bleiben ohne explizite Loot-Entscheidung `unavailable`. Der Player-Stat-Read bildet `gold`/ID `14` und `goldbank`/ID `15` ohne Scaling ab. `itemstatcost.txt` und ein Live-Vergleich am 13. Juli 2026 bestätigten exakt `50938` mitgeführtes und `2401390` privates Stash-Gold. Shared-Stash-Gold ist keine belegte Händlerquelle und wird nicht eingerechnet.

## Datenmodell

`Plan` besteht aus explizit phasierten Normalisierungs-, Service- und Handoff-Schritten. `Demand` bleibt bis zum Inspector aus 9.3 ein reiner, unveränderlicher Vertragswert.

Abschnitt 9.2 ergänzt `town` in der YAML-Datei. Rogue Encampment muss alle Anker sowie die festen Anbieter Akara (Potions/Scrolls/Sell), Cain (Identify) und Charsi (Repair) enthalten. Ein Egress darf nur `portal_arrival → waypoint` und sein Routenverzeichnis beschreiben; Dienste darin werden verworfen. `EgressFor` liefert bei fehlender Definition `town_egress_missing`.

Abschnitt 9.3 führt `SupplySnapshot`, `Thresholds` und `DemandSnapshot` ein. Die Mindestmengen sind ausschließlich Auslöseschwellen: Gleichstand löst keinen Service aus. Der Planner erzeugt bei Bedarf genau `egress → hub_transfer → stash → services → act1_waypoint → next_run_handoff`; fehlende Bedarfe verschwinden aus dem Plan. Ein unvollständiges Belt-Layout bleibt planbar, entscheidet aber noch keinen Kaufmodus.

Abschnitt 9.4 definiert den zentralen, kantenbasierten `ServiceGraph` unter `configs/routes/town/act1/graph/`. Jeder Edge besitzt eine stabile ID, `from`, `to`, eine eigene Aufnahme, positive Kosten und eine explizite `reversible`-Freigabe. Der Router erhält Start, eine ungeordnete Menge benötigter Serviceanker und das Endziel; er wählt den günstigsten Weg, der alle Bedarfe genau abdeckt. `spawn` wird für die Navigation zu `stash` normalisiert, weil der Charakter direkt am Stash erscheint; zwischen beiden entstehen weder Route noch Input. Damit kann beispielsweise `portal_arrival → cain → akara → waypoint` geplant werden, ohne Stash oder Charsi einzubeziehen. Rückwärtswiedergabe ist nur für explizit reversible Kanten erlaubt.

`EgressRoute` bleibt davon getrennt und erlaubt für Fremdakte ausschließlich `portal_arrival → waypoint`. Beide Manifestformate werden strikt geladen; unbekannte Felder und Pfade außerhalb des Graphverzeichnisses werden verworfen.

### Globaler System-Egress (Phase 12.2)

Die gemeinsame Run-Pipeline normalisiert `OriginAct2` bis `OriginAct5` nach der bestätigten Portalankunft über dieselben vier Schritte: globaler Egress bis zum lokalen Waypoint, hover-bestätigtes Öffnen, registrierte Auswahl von Rogue Encampment und Memory-bestätigte Act-1-Ankunft. Erst danach dürfen Personal Stash und zentrale Act-1-Dienste beginnen.

`town.egress.act2` bis `act5` benennen nur Area, Anker und Verzeichnis. Der Adapter lädt die feste Datei `portal-waypoint.yaml` und prüft Akt, Town-Area, Game-Version, Layout-Bindung sowie Memory-bestätigte Portalnähe. Character, Klasse, Difficulty und Map Seed sind weder persistiert noch Gates. Playback verwendet ausschließlich Force Move; fehlende Aufnahmen melden `town_egress_missing`.

Der Act-1-Graph bestätigt den externen Startanker (`portal_arrival`, `stash`, `waypoint`) über sichtbare Memory-Objekte in Klickreichweite. Der erste aufgezeichnete Walk-Punkt allein ist kein Abbruchgrund, wenn der Charakter nach Portal- oder Stash-Interaktion nur wenige Tiles daneben steht.

> **Korrigierte Live-Erkenntnis:** Der Act-1-Ausgang wird beim Charakter-/Difficulty-Wechsel neu auf Nord, Ost, Süd oder West gewürfelt; die Waypoint-Position hängt vom Preset ab. Difficulty und Charakter sind keine autoritativen Town-Route-Bindings. Die abgeschlossene Migration bindet alle produktiven Town-Aufnahmen an einen read-only `TownLayoutFingerprint`; ungebundene Aufnahmen werden nicht abgespielt.

Graphschema v2 trennt die semantische Kante von ihren Assets. `variants` bindet jede Routendatei an den SHA-256-Fingerprint der exakten Waypoint-Position relativ zum Stash. Diese beiden stabilen Objects sind autoritativ; Akara-, Cain- und Charsi-Deltas bleiben optionale Diagnosewerte, weil NPC-Units abhängig von der geladenen Town-Region zeitweise fehlen können. Legacy-`route` bleibt nur als Migrationsquelle lesbar und wird von `RouteForLayout` niemals ausgewählt. Recording schreibt den Fingerprint in die Routendatei; Loader, Graphplayer, initialer Countess-Town-Weg und Post-Run-Handoff verlangen eine exakte Übereinstimmung. Ein fehlendes oder wechselndes Layout endet mit `town_layout_unavailable`, `town_layout_route_missing` oder `town_layout_mismatch` vor Bewegungsinput.

Ein spielzyklusgebundener Layout-Pin hält Fingerprint und aktuellen Stash-Ursprung fest, sobald Stash und Waypoint gemeinsam sichtbar sind. Regionales Unit-Unloading löscht diesen Pin nicht; sobald beide Anker wieder erscheinen, müssen der erneut berechnete Hash und der absolute Stash-Ursprung weiterhin übereinstimmen. Charakter-, Klassen- oder Map-Seed-Wechsel, Session-Reset und ein abweichender beobachteter Hash oder Ursprung verwerfen beziehungsweise sperren ihn. Der produktive Executor verwendet ausschließlich den In-Memory-Pin. Für zwei getrennte `--pathing-test`-Prozesse existiert zusätzlich eine ignorierte Datei unter `diagnostics/town/layout-pin.json`: Sie wird durch eine direkte Graphbeobachtung oder den read-only Befehl `--town-inspect` geschrieben, ist an D2R-PID und Spielidentität gebunden, läuft nach zehn Minuten ab und wird während des Portalpfads erneut gegen sichtbare Layoutanker geprüft. Sie wird niemals vom produktiven Run geladen.

Layoutgebundene Routendateien speichern außerdem den Stash-Ursprung der Aufnahme. Beim Playback werden alle Punkte um die Differenz zum aktuellen Stash verschoben. Damit ist ein Preset-Asset unabhängig vom absoluten Koordinatenraum einer Difficulty oder eines Charakters. Live wurden vier exakte Fingerprints bestätigt: der als Nordausgang verwendete Preset `768769…17381` mit Waypoint-Vektor `(-12,-30)`, der als Ostausgang bestätigte Preset `911703…5a53` mit `(33,-20)`, der als Südausgang bestätigte Preset `5f6354…60f17` mit `(-7,-30)` und der als Westausgang bestätigte Preset `4ad7f3…33f30` mit `(28,-25)`. Damit liegen jeweils zwei Presets optisch links beziehungsweise rechts, sind aber nicht dieselbe exakte Geometrie. Für alle vier Presets liegt ein vollständiger Satz aus direkter Stash→Waypoint-, Portal→Cain- und fünf Servicekanten vor; alle vier wurden über den vollständigen und den selektiven kombinierten Graphpfad live abgenommen.

## Operator / CLI

`--town-inspect` ist read-only und beendet sich nach einem Report. Produktiver Town-Input läuft ausschließlich innerhalb eines validierten Plans über layoutgebundene Graphkanten und die getrennten NPC-/UI-/Bestands-Gates.

### Manuelles Gate 1 (abgeschlossen)

Eine einzelne Kante wird read-only aufgenommen mit:

`d2rbot.exe --config configs/config.yaml --pathing-test record-town-edge:<edge-id>`

Der Charakter wird vor dem Start manuell an `from` gestellt, anschließend zu `to` geführt und dort mit dem Stop-Hotkey beendet. Benötigt werden für das kombinierte Gate `stash-akara`, `akara-cain`, `cain-charsi` und `charsi-waypoint`. Eine Aufnahme `spawn-stash` existiert bewusst nicht. Für den zusätzlichen selektiven Beweis werden `portal-cain` und `akara-waypoint` aufgenommen.

Der kombinierte Graph-Walk wird anschließend mit `play-town-graph:spawn,stash,akara,cain,charsi,waypoint` abgespielt. Der selektive Kontrollfall verwendet `play-town-graph:portal_arrival,cain,akara,waypoint`; der Router muss dabei exakt `portal-cain`, die reversible Kante `akara-cain` rückwärts und `akara-waypoint` wählen.

Bestanden ist das Gate, wenn der kombinierte Walk jeden Serviceanker genau einmal besucht, der selektive Kontrollfall weder Stash noch Charsi anfährt und beide am Waypoint enden. Es gibt keine NPC-, Dialog- oder Shop-Interaktion.

Die Live-Abnahme am 12. Juli 2026 spielte `stash-akara → akara-cain → cain-charsi → charsi-waypoint` in dieser Reihenfolge jeweils genau einmal ab. `town graph plan completed` meldete `start=spawn`, die Bedarfsanker `stash akara cain charsi`, `end=waypoint` und vier Kanten; anschließend endete der Pathing-Test erfolgreich. Die Eingabelogs enthalten ausschließlich Mausbewegungen und Force-Move `E`. Die für den automatisiert abgesicherten Teilbedarf `portal_arrival → cain → akara → waypoint` benötigten Aufnahmen `portal-cain` und `akara-waypoint` liegen ebenfalls vor.

Die Nord-Abnahme am 13. Juli 2026 bestätigte für `768769…17381` ebenfalls beide kombinierten Pfade. Der vollständige Lauf schloss `stash-akara → akara-cain → cain-charsi → charsi-waypoint` mit vier Kanten ab. Der selektive Lauf schloss `portal-cain → akara-cain` rückwärts `→ akara-waypoint` mit drei Kanten ab. Beide meldeten `town graph plan completed`, endeten am Waypoint und enthielten keine Fehler-, Stuck- oder Recovery-Ereignisse.

Die West-Abnahme am 13. Juli 2026 bestätigte für `4ad7f3…33f30` zunächst den vollständigen Vier-Kanten-Pfad. Nach der Layout-Pin-Korrektur stellte der selektive Lauf den zuvor Memory-bestätigten Pin an der Portal-Ankunft wieder her, spielte `portal-cain → akara-cain` rückwärts `→ akara-waypoint` vollständig ab und endete ohne Fehler-, Stuck- oder Recovery-Ereignis am Waypoint.

## NPC-, Dialog- und Shop-Gates (9.5)

Die Monster-Enumeration führt Akara (`148`), Charsi (`154`) und Deckard Cain (`265`, `cain5`) explizit. Cain wurde über den read-only Hover-Buffer live im Rogue Encampment bestätigt; die übrigen Cain-Zeilen der lokal extrahierten `monstats.txt` werden nicht produktiv enumeriert. Eine Interaktion pinnt NPC-ID und Runtime-UnitID, verlangt höchstens 15 Tiles Distanz, bestätigt den Monster-Hover und klickt genau einmal. Ein verlorener Pin, ungeeignete Distanz, fehlender Hover oder ein bereits offenes fremdes UI stoppt ohne Ersatzklick.

Der UI-Buffer liefert getrennte Flags für `NPCInteractOpen` und `NPCShopOpen`. Erst der bestätigte Dialog erlaubt die begrenzte Akara-Sequenz Home, Down, Enter; pro Tick wird höchstens eine Taste gesendet. Erst der bestätigte Shop erlaubt Vendor-Aktionen. Vendor-Items werden nur bei `ItemLocationVendor`, passendem Typ beziehungsweise Code und gepinnter UnitID verwendet. Nach der Mausbewegung müssen dieselbe UnitID und dieselbe Shop-Rasterposition erneut im Memory-Snapshot vorhanden sein. Der globale Entity-Hover wird nicht als Vendor-Gate verwendet, weil D2R Shop-UI-Items dort nicht zuverlässig meldet. Die feste Vendor-Geometrie `109,147` mit 33-Pixel-Zellen gilt ausschließlich nach bestätigtem Shop und exakt 1280×720.

`VendorBuyer` besitzt zwei atomare Pfade: genau ein `Shift+RMB` für Bulk oder genau ein RMB für den begrenzten Einzelkauf. Der isolierte Akara-Test verlangt ein vollständig typisiertes Combat-Profil, Slot 4 als Rejuvenation-Spalte und ein TP-Tome im persönlichen Inventar. Er führt je eine Bulk-Aktion für Healing, Mana und `tsc` aus. Zwischen zwei Kaufaktionen liegen mindestens 500 ms UI-Settle-Zeit, damit D2R keine ansonsten gültige Eingabe zwischen zwei Shop-Aktualisierungen verwirft. Danach wird der Shop verifiziert geschlossen.

### Manuelles Gate 2 (abgeschlossen)

Vor dem Lauf werden Lücken in Healing- und Mana-Spalten geschaffen, mindestens eine Rejuvenation Potion in Slot 4 belassen und das TP-Tome nicht vollständig gefüllt. Danach:

1. `d2rbot.exe --config configs/config.yaml --pathing-test play-town-graph:spawn,akara`
2. `d2rbot.exe --config configs/config.yaml --town-test akara-shop`

Bestanden ist das Gate, wenn Akara genau einmal geklickt wird, Dialog und Shop bestätigt werden, genau je eine Bulk-Aktion für Healing, Mana und TP-Scroll erfolgt, Slot 4 unverändert bleibt und der Shop am Ende geschlossen ist.

Die Live-Abnahme am 12. Juli 2026 erfüllte diese Bedingungen. Das Log `d2rbot-20260712-221017.log` belegt den bestätigten Akara-Dialog und Shop, genau drei atomare Shift+RMB-Aktionen für die getrennten Vendor-UnitIDs `272`, `273` und `269`, rund 800 ms realen Abstand zwischen den Kaufklicks, das bestätigte Schließen des Shops und `outcome=success`. Die Sichtprüfung bestätigte aufgefüllte Healing-, Mana- und TP-Bestände sowie die unveränderte Rejuvenation-Spalte.

## Restock-Planung und Verifikation (9.6)

`PlanRestock` erhält pro kaufbarer Ressource Ist-Menge, Auslöseschwelle und Füllziel. Gleichstand erzeugt keinen Auftrag. Ein vollständig typisiertes Belt-Profil erlaubt genau einen Bulk-Auftrag; bei einer unzugewiesenen Belt-Spalte werden Healing und Mana ausschließlich über eine aus der Fehlmenge abgeleitete, endliche Zahl Einzelkäufe aufgefüllt. Scrolls bleiben Bulk-fähig. `MaximumRestockCost` verwendet vor jedem Town-Input die höchsten ungekürzten Akara-Kosten aus `misc.txt`. Akara besitzt laut `npc.txt` den Sell-Multiplikator `1024`; ein möglicher Quest-Rabatt wird konservativ ignoriert. Im Shop wird danach der tatsächlich angebotene Code erneut bepreist. Unbekanntes oder unzureichendes mitgeführtes Gold stoppt fail-closed.

`RestockVerifier` erlaubt höchstens die geplante Klickzahl und beendet erst bei bestätigter Zielmenge. Bleibt die Menge unverändert, endet der Auftrag nach einem festen Verify-Budget mit `town_restock_verify_timeout`; er wiederholt einen Bulk-Klick niemals. Rejuvenation gehört nicht zu `RestockResource` und kann daher weder Bedarf noch Vendor-Input erzeugen.

## Identify und Sell (9.7)

`PlanItemServices` ist die enge Übergabe aus der Loot-Klassifikation. Nur `IdentifyRequired` und explizite `VendorCandidate`-Markierungen erzeugen Aufträge. Keep-, Stash- und Inventory-Lock-Markierungen haben Vorrang und erzeugen keinen Input; eine gleichzeitig gesetzte Identify-/Sell-Klassifikation ist ungültig.

Jeder `ItemServiceExecutor` pinnt Code und Runtime-UnitID eines persönlichen Inventory-Items. Identify gilt erst mit unveränderter UnitID und `Identified=true` als abgeschlossen. Sell gilt erst dann als abgeschlossen, wenn die UnitID verschwunden ist oder das Item das persönliche Inventar verlassen hat. Nach einer Aktion wird nicht erneut geklickt; ein unveränderter Zustand endet mit `town_item_verify_timeout`.

### Produktive Item-Services (10.6)

Die effektive Assignment-Policy wird einmal geordnet kompiliert; das erste Match und dessen Aktion sind für Pickup, Stash und Item-Services autoritativ. `gems` schützt makellose/perfekte Gems und Schädel; `mephisto-standard` nimmt Exceptional-/Elite-Set/Unique mit Aktion `sell` auf. Ein Sell-Match wird vor dem Personal-Stash-Transfer ausgeschlossen. Normale Set-/Unique-Basen und Gems erzeugen keinen Sell-Auftrag.

Der App-Adapter klassifiziert nur persönliche, ungelockte Inventory-Items. Ein unidentifizierter Sell-Kandidat erzeugt für dieselbe UnitID zuerst `identify`, danach `sell`; ein bereits identifizierter Kandidat erzeugt nur `sell`. Vor der Erstellung jedes Executors wird das Live-Item erneut gegen denselben unveränderlichen Policy-Snapshot geprüft. No-Match, geänderte Regel/Aktion oder Identitätsdrift überspringen den Auftrag ohne Input. Keep-, Stash- oder Inventory-Lock-Konflikte enden mit `town_item_classification_invalid` vor jedem Input. Cain-Identifikation wird erst bei bestätigtem Dialog über die explizite Menüfolge `Home → Down → Enter` ausgelöst, weil `Talk` der erste und `Identify Items` der zweite Eintrag ist; abgeschlossen wird sie erst per unveränderter UnitID plus `Identified=true`. Akara-Verkauf verlangt bestätigten Shop, exakt 1280×720 und denselben Inventory-Footprint; Erfolg ist ausschließlich das Verschwinden der UnitID aus dem persönlichen Inventory.

Der Planner hält für Item-Services die Reihenfolge Cain → Akara fest. Potion-Restock und Sell dürfen denselben bereits bestätigten Akara-Shop geordnet nutzen. Vor jeder anschließenden Navigation müssen Dialog und Shop per Memory als geschlossen bestätigt sein. Der isolierte Operatorpfad lautet `--town-test item-services:mephisto`; er nutzt dieselbe produktive Planung und dieselben Input-Gates.

Der isolierte Pfad besitzt zusätzlich einen strikten Vorabcheck: Er bewegt die Figur nur bei exakt einem ungelockten Kandidaten. Ein unidentifizierter Kandidat muss `identify` und unmittelbar anschließend `sell` für dieselbe UnitID erzeugen. Ein durch einen früheren fehlgeschlagenen Versuch bereits identifizierter Kandidat darf direkt bei Akara fortgesetzt werden, damit der Operator keinen Ersatz-Drop farmen muss. Kein Kandidat, mehrere Kandidaten oder ein Pin-Konflikt enden vor Bewegungsinput. Der gemeinsame Auftragscursor bleibt beim Wechsel vom Cain- zum Akara-Schritt auf dem Sell-Auftrag stehen; ein Integrationstest führt Identifikation, Memory-Bestätigung, Servicegrenze, Verkauf und Verschwinden derselben UnitID vollständig aus.

### Manuelle Abnahme 10.6 (abgeschlossen)

Die Live-Abnahme am 14. Juli 2026 bestätigte die korrigierte Cain-Auswahl `Home → Down → Enter`, die Memory-bestätigte Identifikation sowie anschließend einen produktiven Akara-Resume. Dieser verkaufte UnitID `224` mit genau einem `item_sell`, bestätigte das Verschwinden aus dem Inventory, schloss den Shop, erreichte den Waypoint und endete mit `outcome=success`. Der vollständige kombinierte Identify→Sell-Auftrag über dieselbe UnitID ist zusätzlich durch den Regressionstest des korrigierten Service-Cursors belegt. Diese kombinierte Evidenz erfüllt Gate 10.6; ein weiterer seltener Testgegenstand ist nicht erforderlich.

## Repair und Waypoint-Transfers (9.8)

`PlanRepair` verlangt bei Reparaturbedarf gemeinsam belastbare Haltbarkeit, Kosten und Repair-UI. Fehlt eine Quelle, entsteht ausschließlich `repair_state_unavailable`; es gibt keinen Klickfallback.

`WaypointTransferExecutor` kennt nur registrierte Übergänge. Aktuell sind Act 2–5 → Rogue Encampment zur Hub-Normalisierung und Rogue Encampment → Black Marsh für Countess zugelassen. Die Quelle muss eine Town im erwarteten Act sein. Nach genau einer Zielauswahl wird bis zur bestätigten Ziel-Area gewartet; Timeout wiederholt die Auswahl nicht. Andere Run-Ziele enden mit `next_target_unsupported`.

## Endlicher Executor und Telemetrie (9.9)

Der zentrale `Executor` konsumiert ausschließlich einen validierten `Plan`. Globale Budgets begrenzen Planlänge, Inputs, Verify-Ticks und Voraktions-Retries. Nach der ersten realen Aktion eines Schritts ist Retry gesperrt. Pause ruft keinen Handler auf; Stop und `Reset` verwerfen Fortschritt und alle untergeordneten Pin-/UI-Zustände. Ein synchroner Telemetriefehler wird sticky und verhindert jeden weiteren Handleraufruf.

`town_action` und `town_step_completed` werden auf JSONL abgebildet. Das Schema kann Ist-Menge, Auslöseschwelle, Belt-Spalten, `bulk`/`single`, Vendor-UnitID, Anbieter, Kosten und verifizierten Endbestand tragen. Dadurch bleibt die Kaufentscheidung getrennt von der realen Aktion und ihrer Bestätigung nachvollziehbar. Ein `item_sell` bleibt ausschließlich eine Input-Aktion. Erst nachdem ein späterer World-Snapshot das Verlassen des persönlichen Inventory durch dieselbe gepinnte Unit bestätigt, schreibt der Handler genau ein terminales `sell_success` mit stabilem Itemkey und dem beim Sell erneut bestätigten Pickit-Profil-, Regel- und Revisionskontext. Unveränderte Inventory-Zustände, Timeout oder Telemetriefehler erzeugen keinen Verkaufserfolg.

## Zentraler Post-Run-Flow (9.10)

Der gemeinsame Full-Run wechselt nach `close_personal_stash` in `prepare_town_handoff`. Der App-Adapter bewertet den belastbaren Belt-Bestand. Ohne Bedarf bleibt der direkte layoutgebundene Stash→Waypoint-Pfad erhalten. Bei Potion-Bedarf erstellt der produktive Planner ausschließlich den benötigten Akara-Service, prüft den konservativen Maximalpreis gegen Carried Gold und führt `stash → akara → waypoint → run_handoff` über den zentralen Executor aus. NPC, Dialog, Shop, konkreter Vendor-Code, genau begrenzter Kauf, 500-ms-Settle und erneut gelesener Belt-Zielbestand bleiben getrennte Gates. Tome-Zähler bleiben als `unavailable_skip` sichtbar und erzeugen keinen erfundenen Scrollbedarf.

Ohne sicher belegten Servicebedarf spielt der Adapter ausgehend vom Stash nur vorhandene Kanten des zentralen Graphen bis zum Waypoint. Nicht aufgezeichnete Platzhalterkanten wurden aus `graph.yaml` entfernt. Abschluss verlangt einen per Memory gefundenen Waypoint innerhalb der konfigurierten Klickdistanz und protokolliert `central town preparation completed`, `anchor=waypoint` und die ausgewählte Run-ID als `next_run`. Session- und Run-Reset verwerfen die aktive Kante und den Handoff.

### Manuelles Gate 3 (abgeschlossen)

Für alle vier Act-1-Presets ist die direkte Kante `stash-waypoint` im produktiven `graph.yaml` registriert und separat abgenommen. Der Session-Reset leert den Layout-Pin; der initiale Town-Handoff pinnt das aktuelle Preset erneut und der Post-Run-Handoff darf ausschließlich dieselbe layoutgebundene Variante verwenden. Ein No-Service-Plan umgeht Akara vollständig.

Für die kombinierte Abnahme wurde die lokale Mana-Auslöseschwelle vorübergehend auf 8 erhöht, damit derselbe Lauf zwingend auch den produktiven Kauf- und Shop-Close-Pfad prüft. D2R stand auf dem Offline-Charakterbildschirm mit `MrBones`, Auflösung 1280×720. Der exakt einmalige Lauf startete mit:

`d2rbot.exe --config configs/config.yaml --session-max-runs 1`

Die Live-Abnahme am 13. Juli 2026 erfüllte das Gate vollständig. Der autonome Nightmare-Lauf betrat das Portal nach dem 500-ms-Aktivierungsgate mit zwei Hover-Versuchen, öffnete den persönlichen Stash und verstaute acht Items jeweils im ersten Versuch. Der erzeugte Bedarf führte ausschließlich über `stash → Akara → waypoint`: Eine Bulk-Aktion erhöhte Mana von 2 auf 12, danach wurden Shop-Close und Waypoint-Ankunft separat bestätigt. Run und Session endeten mit `outcome=success`, genau einem Save-&-Exit-Klick und `runs_successful=1/1`. Anschließend wurde die lokale Mana-Auslöseschwelle wieder auf den regulären Wert 4 zurückgestellt.

## Verwandte Features

- [Route Recording & Playback](route-recording-playback.md)
- [Personal Stash MVP](personal-stash-mvp.md)
- [Character & Encounter Profiles](character-encounter-profiles.md)

---
*Zuletzt aktualisiert: 22. Juli 2026*

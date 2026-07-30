# Pickit-Profilbibliothek und Editor

## Überblick

Das eingebettete React-Feature verwaltet globale Pickit-Profile und ihre geordnete Zuordnung pro Charakter und Run. Geführte Katalogaktionen erzeugen verständliche Regeln; Parser-, Katalog- und Persistenzentscheidungen bleiben vollständig im Core.

## Ort im Code

- **Feature:** `web/src/features/pickit/PickitFeature.tsx`
- **Tests:** `web/src/features/pickit/PickitFeature.test.tsx`
- **API:** `web/src/api/generated.ts`, `web/src/api/client.ts`
- **Styles:** `web/src/app/app.css`

## Funktionalität

Die Profilbibliothek zeigt ID und Revision, unterstützt Neu, Duplizieren und das Core-geschützte Löschen über In-App-Dialoge (kein `window.prompt`/`confirm`, damit Electron denselben Pfad nutzt). Ein Entwurf markiert ungespeicherte Änderungen und schützt vor versehentlichem Profilwechsel sowie Seitenwechsel.

Die Katalogsuche findet vollständige Sets, einzelne Set-/Unique-Items und Basisitems. Ein vollständiges Set wird sichtbar in einzelne Identitätsregeln expandiert. Basisitems zeigen ihren englischen Namen, speichern aber den stabilen Excel-Code; „Ätherischer Thresher“ wird daher als `[name] == "7s8" && [ethereal] == true` erzeugt. Die Aktionen `Behalten` und `Identifizieren / verkaufen`, First-Match-Reihenfolge, Entfernen und Umordnen sind direkt bedienbar.

Das erweiterte Ausdrucksfeld und NIP-Paste/-Dateiimport senden Entwürfe zur Core-Validierung; React enthält keinen zweiten Parser. Profil- und Assignment-Saves sind revisionsgebunden. Stale Writes bieten das Laden des aktuellen Stands an, zugeordnete Profile bleiben durch den Core vor Löschung geschützt und eine leere Zuordnung wird bereits im UI verworfen.

Der vollständige unterstützte Ausdrucksvertrag steht in der [Pickit Engine](pickit-engine.md). Import ist all-or-nothing und verlangt eine explizite Aktion. Nicht unterstützte Felder, unbekannte Katalogreferenzen, doppelte Regel-IDs, leere Assignments, stale Revisionen und Mutationen während eines aktiven Workflows werden sichtbar abgelehnt. Sockelbedingungen sind bewusst noch nicht spezifiziert; insbesondere entsteht in Phase 13 keine halbfertige Regel für „Elite Polearm mit vier Sockeln“.

## Bedienung und Accessibility

Labels, Fieldsets, Live-Status, Fehlerrollen, semantische Listen und Buttons unterstützen Tastatur und Screenreader. Bei aktiven Core-Workflows sind persistierende Aktionen gesperrt. Unter 700 Pixeln wechseln Bibliothek, Editor, Regelgenerator und Auswahllayout auf eine Spalte ohne horizontalen Überlauf.

## Grenzen

Der Editor führt keine Live-Loot-Aktion aus und greift nicht auf D2R-Memory zu. Die nach der Phase-13-Abnahme nicht mehr benötigte synthetische Entscheidungsvorschau ist aus dem Editor entfernt. Die Core-API behält ihren kontrollierten Preview-Endpunkt für automatisierte Diagnose- und Vertragstests; er besitzt keine Bedienoberfläche und löst keinen Gameplay-Input aus.

## Isoliertes Live-Gate

Das unzugeordnete Profil `phase13-live-acceptance` hält genau einen Pfeilköcher (`Arrows`, Code `aqv`) mit Aktion `keep`. Es ist bewusst nicht Teil der versionierten Beispielzuordnung und verändert daher keinen produktiven Run. Keine bestehende Countess-Regel matcht diesen Gegenstand, daher konnte das Profil ohne Umsortieren an die Zuordnung angehängt werden und gewann dennoch eindeutig. Für das inzwischen abgeschlossene Gate B wurde dieses Profil im Dashboard bearbeitet, über den damals temporär sichtbaren Preview-Pfad geprüft und dem bestätigten Charakter für Countess zugeordnet. Profil- und lokaler Assignment-Save erhöhten ihre Revisionen jeweils atomisch.

Der bestehende isolierte Run `--run countess --phase loot-and-return` beginnt direkt im vorbereiteten Tower Cellar Level 5, enthält keinen Travel- oder Boss-Schritt und verwendet danach die produktiven Pickup-, Portal- und Personal-Stash-Gates. Ein bei Charsi verfügbarer Pfeilköcher ist billig wiederbeschaffbar und vermeidet den riskanteren Vendorpfad.

Gate B wurde am 21. Juli 2026 bestanden: UnitID `225` wurde als `Arrows`/`aqv` von `phase13-live-acceptance/arrows-live-gate` mit `keep`, Profilrevision `2` und Assignment-Revision `2` klassifiziert. Hover und Pickup gelangen im ersten Versuch; Portalrückkehr, Personal-Stash-Transfer und UI-Schließen wurden Memory-basiert bestätigt. Core-Log und JSONL endeten ohne Warnung oder Fehler mit `outcome=success`.

## Verwandte Features

- [Pickit-API und sichere Run-Grenze](pickit-api.md)
- [Pickit-Profile und Assignments](pickit-profiles.md)

---
*Zuletzt aktualisiert: 30. Juli 2026*

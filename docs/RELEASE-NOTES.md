# Installationshinweise

## System und Einsatzbereich

- Windows 10 oder Windows 11, ausschließlich x64.
- Nur Diablo II: Resurrected Offline/Singleplayer; Battle.net und Online-Modi sind nicht unterstützt.
- D2R und Farming-Sessions werden niemals automatisch gestartet. D2R wird manuell durch den Operator geöffnet.
- Die App ist derzeit nicht code-signiert. Windows SmartScreen kann deshalb eine Warnung anzeigen. Es gibt keine Umgehungsautomatik und keine Behauptung, der Build sei verifiziert.

## Daten und Deinstallation

Veränderliche Daten liegen standardmäßig unter `%LOCALAPPDATA%\D2ROfflineFarmingBot\` und nicht im Installationsordner. Installation und Upgrade führen diesen Datenroot nicht zusammen und überschreiben ihn nicht. Die Deinstallation erhält ihn standardmäßig. Nur die separate, standardmäßig abgelehnte und doppelt bestätigte Löschoption entfernt exakt diesen festen Datenroot.

Electron verwendet zusätzlich ein entbehrliches Chromium-Laufzeitprofil im benutzereigenen Windows-Tempbereich. Dieses Profil enthält keine Bot-Konfiguration, Routen oder Telemetrie und kann vom Betriebssystem bereinigt werden. Der kanonische Datenroot bleibt dadurch bis zur abgeschlossenen Core-Provisionierung tatsächlich unveröffentlicht.

## First Run

Ein vorbereiteter lokaler Charakter kann auch dann im Onboarding gewählt werden, wenn der frische Datenroot noch keine Startvorauswahl enthält. Alle gefundenen Saves bleiben sichtbar. Nicht unterstützte oder noch nicht vorbereitete Charaktere sind deaktiviert und zeigen verständlich an, ob ein Kampfprofil fehlt, welche Klasse derzeit unterstützt wird oder ob die automatische Auswahl in D2R noch nicht eingerichtet ist. Die App liest keine Save-Inhalte und behauptet deshalb bei einem nicht eingerichteten Charakter keine automatisch erkannte Klasse.

Safety und explizites Input-Opt-in stehen vor der kontrollierten D2R-Charakterbestätigung. Gespeicherte Freigabe und effektiver Core-Zustand werden getrennt angezeigt; erst die Bestätigung des laufenden Controllers schaltet die Auswahl frei. Der Schritt-Fortschritt und einfache Hinweistexte verwenden ein eigenes kompaktes Layout ohne horizontalen Seitenüberlauf.

## Route-Threat-Combat

Der Summoner-Run hält seine gebundene Route vor bekannten Specter-/Ghost-, Hell-Clan- und Ghoul-Lord-Gegnern im unmittelbaren Umfeld, Korridor oder am nächsten Landepunkt. Der Charakter wirkt auf den ersten hover-bestätigten Blocker einmal Amplify Damage (`F1` im Default) und räumt danach stationär mit Bone Spear. Ist derselbe Blocker drei frische Snapshots lang nicht projizierbar, nähert er sich begrenzt per Force Move zum bereits validierten nächsten Routenpunkt. Nach 500 ms muss Memory mindestens ein Tile Distanzgewinn bestätigen; erst drei wirkungslose Versuche dürfen den Run mit `route_threat_out_of_range` abbrechen. Es gibt dabei keinen Blind-Teleport.

Mana-Hysterese und Recovery-Guard enden bei ausbleibender Erholung oder wirkungslosem identischem Teleport fail-closed. Ein reiner Mana-Notfall verwendet zuerst die in Town nachkaufbaren Mana-Tränke; Rejuvenation bleibt der Fallback und behält bei kritischen HP Vorrang. Nach einer kontrollierten Rückkehr nach Akt 1 verwendet der Retry exakt denselben Save-&-Exit-Pfad wie das normale Queue-Ende. Dieser wartet vor Escape mindestens drei Sekunden auf eine unveränderte Spielerposition bei geschlossenem Town-UI und wiederholt ein von D2R ignoriertes Escape nach 1,5 Sekunden genau einmal, solange Memory das Quit-Menü weiterhin als geschlossen bestätigt. Unter **Einstellungen → Effektive Route-Combat-Werte** zeigt die App die vom Core nach Defaults und Validierung verwendeten Werte read-only. `runs.definitions.summoner.route_combat.enabled: false` deaktiviert das neue Interleave, ohne Routen- oder Save-Dateien zu verändern.

Nihlathaks Bosskampf verwendet die aufgezeichnete Routenendposition als bevorzugten Kampfanker. Nur wenn sein sichtbarer Körper von dort nicht projizierbar ist, darf genau ein minimaler Annäherungsteleport folgen. Bone Spear wird anschließend kompromisslos durch jedes Memory-bestätigte lebende Monster unter dem Cursor gewirkt; die Kill-Bestätigung bleibt weiterhin ausschließlich an Nihlathaks gepinnte UnitID gebunden.

Nach bestätigtem Bosskill wirkt der begrenzte Post-Clear einmal Amplify Damage und räumt anschließend alle erfassten lebenden Halls-of-Vaught-Gegner innerhalb von 30 Tiles mit hover-bestätigtem Bone Spear. Ein nicht mehr projizierbarer Restgegner wird für diesen Cleanup übersprungen, damit der nächste Gegner gewählt werden kann. Drei Sekunden ohne tatsächlich gesendeten Kampfinput beenden den best-effort Cleanup, bevor Loot und Town Portal fortgesetzt werden.

## Release-Artefakte

Das lokale Release besteht aus einem per-user NSIS-Installer für x64 und seiner SHA-256-Datei. Für GitHub werden beide Dateien gemeinsam als `D2R-Offline-Farming-Bot-<Version>-Windows-x64.zip` veröffentlicht. GitHub stellt zum Tag zusätzlich seine normalen Sourcearchive bereit.

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

## Release-Artefakte

Das lokale Release besteht aus einem per-user NSIS-Installer für x64 und seiner SHA-256-Datei. GitHub stellt zu einem veröffentlichten Tag automatisch die Sourcearchive bereit; sie werden nicht nochmals als eigene Binärassets erzeugt.

# Installationshinweise

## System und Einsatzbereich

- Windows 10 oder Windows 11, ausschließlich x64.
- Nur Diablo II: Resurrected Offline/Singleplayer. Battle.net und Online-Modi sind nicht unterstützt.
- D2R und Farming-Sessions startet die App nicht selbst. D2R wird manuell geöffnet.
- Die App ist nicht code-signiert. Windows SmartScreen kann eine Warnung zeigen. Es gibt keine Umgehungsautomatik und keine Behauptung, der Build sei verifiziert.

Lizenz und Nutzungsgrenzen stehen in [`LICENSE`](../LICENSE) im Repository. Kurz: Quelltext ansehen ja, Online-Automation und Weitervertrieb als Produkt nein.

## Daten und Deinstallation

Veränderliche Daten liegen standardmäßig unter `%LOCALAPPDATA%\D2ROfflineFarmingBot\` und nicht im Installationsordner. Installation und Upgrade führen diesen Datenroot nicht zusammen und überschreiben ihn nicht. Die Deinstallation erhält ihn standardmäßig. Nur die separate, standardmäßig abgelehnte und doppelt bestätigte Löschoption entfernt genau diesen Datenroot.

Electron legt zusätzlich ein entbehrliches Chromium-Laufzeitprofil im benutzereigenen Tempordner an. Darin liegen keine Bot-Konfiguration, Routen oder Telemetrie. Der kanonische Datenroot bleibt `%LOCALAPPDATA%\D2ROfflineFarmingBot\`.

## First Run

Nach der Installation wählt der First-Run-Dialog einen frischen Datenroot oder den Import eines bestehenden. Ein vorbereiteter lokaler Charakter kann auch dann gewählt werden, wenn der frische Datenroot noch keine Vorauswahl enthält. Nicht unterstützte oder noch nicht eingerichtete Charaktere bleiben deaktiviert und sagen, woran es liegt. Die App liest keine Save-Inhalte.

Safety und explizites Input-Opt-in stehen vor der D2R-Charakterbestätigung. Gespeicherte Freigabe und effektiver Core-Zustand werden getrennt angezeigt. Erst die Bestätigung des laufenden Controllers schaltet die Auswahl frei.

## Release-Artefakte

Das lokale Release besteht aus einem per-user NSIS-Installer für x64 und seiner SHA-256-Datei. Für GitHub werden beide Dateien gemeinsam als `D2R-Offline-Farming-Bot-<Version>-Windows-x64.zip` veröffentlicht. GitHub stellt zum Tag zusätzlich die normalen Sourcearchive bereit.

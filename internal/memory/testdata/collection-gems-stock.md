# Collection-Bestandsfixture

Quelle ist die read-only Gate-25.1-Aufnahme vom 1. Oktober 2026 für D2R `3.2.92777`. Der unveränderte Gems-Screenshot zeigt 22 normale Rubine. Die Rohaufnahme liegt lokal unter `.tmp/phase25/storage-raw-research.json`; ihr SHA-256 steht im JSON-Fixture und im HTML-Nachweis.

Der Originalzähler bei `ItemData+0x9C`, die Page, der Owner-Sentinel und die Inventarsignatur sind unverändert. Die ursprüngliche 75-Node-Liste wurde auf den gebundenen Rubin reduziert. Listenlänge und Endpunkte wurden entsprechend angepasst. Verwendete Pointer wurden auf Fixture-Adressen versetzt, unbenutzte Pointer entfernt. Keine Charakter-/Savegame-Daten sind enthalten.

`TxtFileNo` ist die tatsächlich beobachtete Rubin-Referenz. World-Tests prüfen ihre Auflösung über den aus lokalen CASC-Dateien generierten Katalog und dessen stabilen Code `gsr`; sie schreiben keine zusätzlichen Spiel-IDs fest. Die Mengenmatrix verändert ausschließlich das belegte Zählerfeld. Fehlerfälle verändern Eigentümer, Verknüpfung oder Lesbarkeit derselben Struktur.

# CASC-Fixtures für Phase 25.0

Diese Dateien enthalten den unveränderten TSV-Header und ausgewählte Originalzeilen der lokalen D2R-3.2.92777-Extrakten unter `.tmp/d2r-excel/`. Es wurden keine Zellwerte ergänzt oder umgeschrieben.

- `cubemain.txt`: genau die 23 in Phase 25 freigegebenen `description`-Schlüssel.
- `misc.txt`: die 31 Zutaten- und Outputcodes dieser Zeilen. Stabile Schlüssel sind `name` und `code`.
- SHA-256 des vollständigen `cubemain.txt`: `C2AF6EF0C6EBF9EF5923CCF94CBD0CD1449AA46BDF469C07C8F170FC10E19785`.
- SHA-256 des vollständigen `misc.txt`: `4B7259D1DB5FA0A9DA1D9530A019066CD40806EDE91A4C0F4C69F81AEBFFDB7F`.

Die Tests lesen ausschließlich diese kleinen Fixtures. Der Generator liest die vollständigen lokalen Quellen und schreibt nur den Go-Katalog im Repository. `misc.maxstack` liefert keine Kapazität für die Materialtruhe.

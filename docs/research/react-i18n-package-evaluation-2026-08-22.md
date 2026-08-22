# Paketbewertung für React-Internationalisierung

Stand: 22. August 2026. Berücksichtigt wurden nur offizielle Dokumentation und die offiziellen Repositories der Projekte.

## Ergebnis

Für dieses Repository ist `i18next` zusammen mit `react-i18next` die passendste Wahl. Die Kombination braucht keine Änderung an Vite und keine Übersetzungs-Compile-Stufe. Sie kann alle Ressourcen in den Renderer-Bundle aufnehmen, stellt mit `useTranslation()` einen React-Hook bereit und wechselt die aktive Sprache zur Laufzeit über `i18n.changeLanguage(...)`. Der Hook reagiert auf den Sprachwechsel und rendert betroffene Komponenten neu. Das ist in der [offiziellen React-Integration](https://github.com/i18next/react-i18next-gitbook/blob/master/latest/usetranslation-hook.md) beschrieben.

Die aktuellen offiziellen Paketdateien weisen `i18next` 26.4.0 und `react-i18next` 17.0.12 aus. `react-i18next` 17.0.12 akzeptiert React ab 16.8, TypeScript 5, 6 oder 7 und verlangt i18next ab 26.2.0. Damit passt die Kombination ohne Kompatibilitätsausnahme zu React 19.2 und TypeScript 7 im vorhandenen `web/package.json`. Quellen: [i18next-Paketdatei](https://github.com/i18next/i18next/blob/master/package.json), [react-i18next-Paketdatei](https://github.com/i18next/react-i18next/blob/master/package.json).

Empfohlene Produktionsabhängigkeiten:

```json
{
  "i18next": "26.4.0",
  "react-i18next": "17.0.12"
}
```

Ein drittes Paket ist für den geplanten Funktionsumfang nicht nötig. Insbesondere würde ich `i18next-browser-languagedetector` hier nicht als Persistenzschicht einsetzen. Es bleibt eine mögliche spätere Ergänzung, falls Browser-Erkennung wirklich mehrere Quellen wie Query-Parameter, Browser-Sprache und `localStorage` priorisieren soll.

## Warum es zum Electron-Renderer passt

`react-i18next` läuft im React-Renderer und hängt nicht von Node-Integration oder Electron-Main-APIs ab. Das offizielle Paket veröffentlicht ESM und CommonJS und bezeichnet React ab 16.8 als Peer-Abhängigkeit. Die bestehende sichere Electron-Konfiguration mit `sandbox: true`, `contextIsolation: true` und deaktivierter Node-Integration steht dem daher nicht entgegen. Quelle für die Paketgrenzen: [offizielle Paketdatei](https://github.com/i18next/react-i18next/blob/master/package.json).

Für den ersten Render können beide Sprachressourcen statisch importiert werden. Es gibt dann keinen asynchronen Übersetzungs-Request, kein zusätzliches Backend-Paket und keinen neuen CSP-Freigabebedarf. `react-i18next` verwendet Suspense nur, wenn benötigte Ressourcen noch nicht bereit sind. Bei statisch gebündelten Ressourcen ist dieser Ladefall vermeidbar. Die Hook-Dokumentation erklärt das Verhalten bei noch nicht geladenen Namespaces und die Option `useSuspense: false`: [useTranslation](https://github.com/i18next/react-i18next-gitbook/blob/master/latest/usetranslation-hook.md).

## Eine zentrale Datei pro Sprache ist realistisch

Für den React-Renderer allein ist folgende Ablage ein guter Start:

```text
web/src/i18n/
  i18n.ts
  i18next.d.ts
  locales/
    de.ts
    en.ts
```

`de.ts` und `en.ts` können jeweils genau ein verschachteltes Objekt exportieren. Die oberste Ebene sollte die bestehenden Produktbereiche spiegeln, etwa `common`, `sidebar`, `dashboard`, `settings`, `routes`, `history`, `onboarding`, `errors` und `desktop`. `i18n.ts` registriert beide Objekte unter einem einzigen Namespace, zum Beispiel `translation`. Das offizielle Einstiegsmuster zeigt gebündelte Ressourcen direkt in `init({ resources: ... })` und empfiehlt, sie bei größerem Umfang in importierte Dateien zu verschieben: [react-i18next Getting started](https://github.com/i18next/react-i18next-gitbook/blob/master/getting-started.md).

i18next selbst hält eine einzelne Datei bei kleineren Projekten ausdrücklich für angemessen. Die offizielle Dokumentation schlägt Namespaces erst vor, wenn eine Datei unübersichtlich wird, Übersetzungen nicht alle beim Start geladen werden sollen oder eine fachliche Trennung sinnvoll ist. Sie nennt ungefähr 300 Segmente als praktische Schwelle, nicht als technische Grenze. Quelle: [i18next Namespaces](https://github.com/i18next/i18next-gitbook/blob/master/principles/namespaces.md).

Für dieses Produkt würde ich deshalb zunächst bei einer Datei je Sprache bleiben und die Objekte bereits nach Features gliedern. Wenn die Dateien später stören, kann dieselbe Schlüsselstruktur ohne Wechsel der Bibliothek in Namespaces wie `common`, `dashboard` und `routes` zerlegt werden. `useTranslation('dashboard')` bindet den Hook an einen Namespace; mehrere Namespaces können gemeinsam geladen werden. Quellen: [i18next Namespaces](https://github.com/i18next/i18next-gitbook/blob/master/principles/namespaces.md), [useTranslation](https://github.com/i18next/react-i18next-gitbook/blob/master/latest/usetranslation-hook.md).

## TypeScript-Typisierung

i18next liefert eigene TypeScript-Typen. Übersetzungsschlüssel lassen sich über Module Augmentation in `i18next.d.ts` aus den tatsächlichen Ressourcen ableiten. Die offizielle Empfehlung exportiert `resources` und `defaultNS` aus der i18n-Initialisierung und setzt in `CustomTypeOptions` `resources: typeof resources["en"]`. Damit prüft TypeScript Schlüssel und Rückgabewerte. Quelle: [offizielle TypeScript-Anleitung](https://github.com/i18next/i18next-gitbook/blob/master/overview/typescript.md).

Für dieses Repo sind `.ts`-Ressourcen mit `as const` besser als untypisierte JSON-Dateien. Die i18next-Dokumentation weist darauf hin, dass TypeScript Interpolationsvariablen nur aus `as const`-Ressourcen oder einer passenden generierten Deklaration ableiten kann. Die vorhandene Konfiguration hat bereits `strict: true`; i18next setzt `strict` oder mindestens `strictNullChecks` für die vollständige Typisierung voraus. Quelle: [offizielle TypeScript-Anleitung](https://github.com/i18next/i18next-gitbook/blob/master/overview/typescript.md).

Die optionale Selector-API sollte nicht Voraussetzung der ersten Migration sein. i18next 26.4 führt sie über `enableSelector` weiterhin als opt-in und unterstützt daneben typisierte String-Schlüssel. Bei sehr großen Katalogen kann `enableSelector: "optimize"` später die TypeScript-Last begrenzen. Quelle: [i18next TypeScript-Optionen](https://github.com/i18next/i18next/blob/master/typescript/options.d.ts).

Zusätzlich zur i18next-Typisierung sollte ein kleiner Test die Schlüsselbäume von `de` und `en` rekursiv auf Gleichheit prüfen. Die Typdefinition prüft primär die Form der Referenzressource; ein expliziter Paritätstest macht fehlende oder zusätzliche Schlüssel in der zweiten Sprache zu einem klaren Testfehler.

## Dynamischer Sprachwechsel und Persistenz

Der Sprachschalter ruft `await i18n.changeLanguage('de')` oder `await i18n.changeLanguage('en')` auf. `useTranslation()` liefert die dafür nötige i18n-Instanz. `i18next.resolvedLanguage` eignet sich für den aktiven Zustand, weil es den tatsächlich aufgelösten Sprachcode enthält. Der Sprachwechsel über die Hook-Instanz ist Teil der [offiziellen useTranslation-API](https://github.com/i18next/react-i18next-gitbook/blob/master/latest/usetranslation-hook.md).

Die Persistenz sollte in diesem Repo über den vorhandenen Electron-Settings-Vertrag laufen:

- `language?: "de" | "en"` in `web/electron/desktop-settings.ts`;
- Auslieferung über die vorhandene Preload-Bridge;
- Laden vor der i18n-Initialisierung oder vor dem ersten sichtbaren App-Render;
- Speichern nach erfolgreichem Sprachwechsel;
- Default nur beim ersten Start aus `navigator.language`, danach ausschließlich aus der gespeicherten Auswahl.

Der Grund ist projektspezifisch. `desktop-settings.json` ist bereits die atomare, streng validierte Desktop-Persistenz. Das Chromium-Profil liegt dagegen unter einem bewusst entbehrlichen Temp-Pfad. Hinzu kommt, dass `internal/api/server.go` den Core an `127.0.0.1:0` bindet und `web/electron/main.ts` die UI von der jeweils zurückgegebenen Bootstrap-URL lädt. Der freie Port und damit der Web-Origin können nach jedem Core-Start wechseln. Da `localStorage` an den Origin gebunden ist, wäre es hier selbst bei erhaltenem Chromium-Profil keine verlässliche Autorität für die Produktpräferenz.

`i18next-browser-languagedetector` 8.2.1 kann Browser-Sprache, Querystring, Cookie, `localStorage`, `sessionStorage`, HTML-Attribut, Pfad, Subdomain und Hash auswerten. Es kann die Auswahl in `localStorage` oder Cookie cachen und erlaubt einen eigenen Detector mit `lookup` und `cacheUserLanguage`. Quellen: [offizielles README](https://github.com/i18next/i18next-browser-languageDetector), [offizielle Paketdatei](https://github.com/i18next/i18next-browser-languageDetector/blob/master/package.json). Technisch wäre also ein eigener Detector für die Electron-Bridge möglich. Er bringt hier aber keinen Vorteil gegenüber einem kleinen expliziten Startablauf und würde asynchrone Settings-Persistenz in eine primär browserorientierte Detection-Abstraktion drücken.

Empfohlene Sprachauflösung:

1. Gespeichertes `de` oder `en` verwenden.
2. Ohne gespeicherten Wert `navigator.language` auf `de` oder `en` reduzieren.
3. Für jede andere Systemsprache Deutsch als bestehende Produktsprache wählen, alternativ Englisch, falls das Produkt Englisch zum künftigen Fallback erklärt.
4. `supportedLngs: ['de', 'en']`, `load: 'languageOnly'` und einen expliziten `fallbackLng` setzen, damit etwa `de-AT` sauber auf `de` fällt. Die zugehörigen Initialisierungsoptionen sind im [offiziellen i18next-Typvertrag](https://github.com/i18next/i18next/blob/master/typescript/options.d.ts) dokumentiert.

## Pluralisierung, Interpolation und formatierte Werte

i18next verwendet die `Intl.PluralRules`-Kategorien. Übersetzungen erhalten Suffixe wie `_one` und `_other`, und der Aufruf muss die Variable `count` enthalten. Eine optionale `_zero`-Variante kann für natürlichere Nulltexte genutzt werden. Das deckt Deutsch und Englisch ohne Zusatzpaket ab. Quelle: [offizielle Plural-Dokumentation](https://github.com/i18next/i18next-gitbook/blob/master/translation-function/plurals.md).

Dynamische Werte gehören als benannte Platzhalter in vollständige Sätze, zum Beispiel `"{{count}} Runs abgeschlossen"`. i18next interpoliert `{{name}}`-Werte und escaped sie standardmäßig. Die React-Einstiegsanleitung setzt oft `escapeValue: false`, weil React Textwerte bereits escaped. Falls das Projekt diese Option übernimmt, dürfen übersetzte oder interpolierte Werte weiterhin niemals über `dangerouslySetInnerHTML` gerendert werden. Quellen: [i18next Interpolation](https://github.com/i18next/i18next-gitbook/blob/master/translation-function/interpolation.md), [react-i18next Getting started](https://github.com/i18next/react-i18next-gitbook/blob/master/getting-started.md).

Für Texte mit React-Elementen, etwa Links oder Hervorhebungen innerhalb eines Satzes, gibt es die `Trans`-Komponente. Sie sollte sparsam für solche Rich-Text-Sätze eingesetzt werden; normale Labels, ARIA-Texte, Tooltips und Fehlermeldungen bleiben bei `t(...)`. Die offizielle Anleitung zeigt beide Fälle: [react-i18next Getting started](https://github.com/i18next/react-i18next-gitbook/blob/master/getting-started.md).

Datum, Uhrzeit und Zahlen sollten mit dem aktiven Sprachcode über `Intl.DateTimeFormat` und `Intl.NumberFormat` formatiert werden. Dafür ist kein weiteres i18n-Paket nötig. Die Übersetzungsbibliothek ersetzt nicht die lokale Formatierung bereits berechneter Werte.

## Grenzen bei Core- und API-Meldungen

Keine React-i18n-Bibliothek kann freie deutsche Sätze, die der Go-Core über HTTP oder SSE liefert, zuverlässig nachträglich übersetzen. Die UI braucht dort einen maschinenlesbaren Vertrag:

```text
reason_code + strukturierte Parameter -> UI-Übersetzungsschlüssel + Interpolation
```

Bereits vorhandene stabile Codes können direkt auf Schlüssel wie `errors.queueContextMismatch` abgebildet werden. Für API-Stellen, die nur freie Texte liefern, ist eine schrittweise Vertragsänderung nötig. Der Core sollte einen stabilen Code und Werte liefern, die UI formuliert den benutzer sichtbaren Satz. Ein roher Core-Text kann während der Migration nur als letzter Fallback dienen. Er bleibt dann definitionsgemäß nicht internationalisiert. Das ist eine Architekturgrenze, keine Lücke von i18next.

Electron-Main-Texte in Tray, nativen Dialogen, Benachrichtigungen, Provisionierungs- und Recovery-Seiten liegen außerhalb des React-Baums. Sie können dieselben Sprachcodes und Schlüsselkonzepte verwenden, aber nicht einfach `useTranslation()`. Sinnvoll ist eine kleine frameworkfreie Übersetzungsfunktion für den Main-Prozess mit einem gezielt freigegebenen Teil derselben Ressourcen oder ein eigener kleiner `desktop`-Katalog. i18next selbst ist frameworkunabhängig; `react-i18next` ist nur der React-Adapter. Quelle: [i18next-Repository](https://github.com/i18next/i18next), [react-i18next-Repository](https://github.com/i18next/react-i18next).

### Packaging-Grenze des Electron-Main-Prozesses

Der jetzige Electron-Build ist keine JavaScript-Bündelung. `web/tsconfig.electron.build.json` kompiliert mit `rootDir: "electron"` nach `dist-electron`, und `web/package.json` schließt `node_modules` ausdrücklich aus dem Installer aus. Zwei Folgen sind wichtig:

- Electron-Main darf `i18next` nicht einfach als Runtime-Import verwenden. Das Paket wäre im installierten Produkt nicht vorhanden.
- Eine gemeinsame TypeScript-Ressource unter `web/src/i18n/` liegt außerhalb des Electron-`rootDir` und kann nicht ohne Build-Anpassung direkt in den Main-Compile gezogen werden.

Für eine echte zentrale Datei je Sprache gibt es deshalb zwei saubere Wege:

1. Die kanonischen Kataloge bleiben als `de.ts` und `en.ts` im Renderer-Bereich. Ein kleiner Build-Schritt projiziert den frameworkfreien `desktop`-Teil als generierte JSON- oder TypeScript-Artefakte unter `web/electron/`. Electron-Main benutzt eine lokale, mitkompilierte Übersetzungsfunktion ohne npm-Runtime-Abhängigkeit. Ein Check stellt sicher, dass die Projektion aktuell ist.
2. Die kanonischen Kataloge ziehen nach `web/shared/i18n/`. Dann müssen der Electron-`rootDir`, der ausgegebene Main-Pfad in `web/package.json` und die Ziele in `web/electron/copy-assets.mjs` gemeinsam angepasst werden. Das ist konzeptionell sauberer, aber ein größerer Packaging-Umbau.

Der erste Weg hält den aktuellen Installervertrag stabil. Der zweite Weg lohnt sich, wenn Renderer, Tray, native Dialoge, Recovery und Provisionierung dauerhaft dieselbe umfangreiche Katalogbibliothek importieren sollen. In beiden Fällen bleiben `i18next` und `react-i18next` reine Renderer-Abhängigkeiten; Main und statische Seiten erhalten fertig gebündelte beziehungsweise kopierte Daten.

## Vergleich mit ernsthaften Alternativen

| Paket | Stärken | Mehrarbeit oder Nachteil in diesem Repo | Urteil |
|---|---|---|---|
| `react-intl` aus FormatJS | React-Provider, ICU Message Syntax, starke eingebaute Datums-, Zahlen-, Relative-Time- und Pluralformatierung, TypeScript-Typisierung für Message-IDs. React 19 wird offiziell unterstützt. | Sprachzustand und Persistenz bleiben eigene App-Logik. Beim Locale-Wechsel müssen Provider-Konfiguration und Nachrichten ersetzt werden. Der typische FormatJS-Workflow bringt Message-Deskriptoren sowie optional CLI- oder Build-Extraktion mit. | Gute Wahl für ein Produkt, dessen Schwerpunkt auf komplexen ICU-Nachrichten und automatischer Extraktion liegt. Für zwei zentrale Sprachdateien und eine bestehende UI-Migration ist i18next direkter. Quellen: [React Intl](https://formatjs.github.io/docs/react-intl/), [FormatJS Message Extraction](https://formatjs.github.io/docs/getting-started/message-extraction/), [FormatJS Intl-Lebenszyklus](https://formatjs.github.io/docs/intl/). |
| Lingui | ICU MessageFormat, React-Hooks und Komponenten, dynamische Katalogaktivierung, ein Katalog pro Sprache sowie gute Extraktions- und Compile-Werkzeuge. | Der empfohlene Ablauf installiert mindestens `@lingui/core` und `@lingui/react`, nutzt Compile-Time-Makros, extrahiert PO-Kataloge und kompiliert sie für die Laufzeit. Mit Vite kommt üblicherweise ein Lingui-Plugin hinzu. Das ist mehr Tooling und ein anderer Autorenworkflow als die gewünschten direkt gepflegten zentralen TypeScript-Dateien. | Technisch stark, aber für den derzeitigen KISS-Ansatz unnötig schwer. Quellen: [Lingui React tutorial](https://lingui.dev/tutorials/react), [Lingui Vite plugin](https://lingui.dev/ref/vite-plugin), [Lingui introduction](https://lingui.dev/introduction). |

## Konkrete Paketentscheidung

1. `i18next` und `react-i18next` als einzige neue Runtime-Abhängigkeiten einplanen.
2. `i18next-browser-languagedetector` zunächst weglassen. Systemsprache einmal selbst lesen und die Nutzerwahl im vorhandenen Electron-Settings-Store speichern.
3. Mit je einer `de.ts` und `en.ts` beginnen. Top-Level-Gruppen folgen den Features, der Runtime-Namespace bleibt zunächst einer.
4. TypeScript über `CustomTypeOptions` an den deutschen oder englischen Referenzbaum binden und zusätzlich die Baumparität beider Sprachen testen.
5. API-Texte nicht anhand englischer oder deutscher Satzfragmente erkennen. Stabile Core-Codes und strukturierte Parameter sind die notwendige Grenze.
6. React-Renderer, Electron-Main, statische Recovery-/Provisionierungsseiten und Installertexte als getrennte Ausgabekanäle inventarisieren. Ein React-Paket deckt nur den ersten davon automatisch ab.

Diese Lösung hält die anfängliche Abhängigkeit klein, erlaubt den sofortigen Sprachwechsel und lässt eine spätere Aufteilung in Namespaces offen, ohne die Bibliothek oder die Schlüssel wechseln zu müssen.

## Kurzer Hinweis zum Sprachschalter

Die Bibliothek schreibt die Darstellung nicht vor. Zwei direkt sichtbare Schalter im unteren Menübereich sind für zwei Sprachen ausreichend. Text wie `Deutsch` und `English`, bei Platzmangel `DE` und `EN`, ist eindeutiger als Landesflaggen. Die W3C-Hinweise raten von Flaggen als Sprachsymbol ab, weil eine Sprache nicht eindeutig einem Land entspricht. Quelle: [W3C Internationalization Authoring Techniques](https://www.w3.org/International/techniques/authoring-html.en?open=language&open=langvalues).

Der aktive Schalter sollte eine echte Auswahlsemantik wie `aria-pressed` erhalten. Nach jedem Wechsel muss außerdem `document.documentElement.lang` auf `de` oder `en` gesetzt werden, damit Hilfstechnologien und Browser die Dokumentsprache korrekt erkennen. Quelle: [W3C Language declarations in HTML](https://www.w3.org/International/questions/qa-html-language-declarations).

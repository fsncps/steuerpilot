# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Projektübersicht

**SteuerPilot SG** – KI-gestützte Web-App zur Vorbereitung der Steuererklärung für Privatpersonen im Kanton St. Gallen. Claude Vision wird für Dokumenten-Extraktion (Lohnausweis, Kontoauszug, Säule 3a) verwendet; die Steuerberechnung selbst läuft **immer lokal**.

**MVP-Zielgruppe:** Unselbständig erwerbstätige natürliche Personen im Kanton SG (keine Liegenschaften, keine Selbständigkeit).

**Sprache:** Deutsch (Schweizer Hochdeutsch – kein ß, CHF statt €, Apostroph als Tausendertrenner: `CHF 1'234.56`).

---

## Repository-Struktur

```
steuern/
├── CLAUDE.md                  # Diese Datei
├── setup.sh                   # Einmalig: Next.js-Projekt aufsetzen
├── src-lib-*.ts               # Entwurfsdateien (NICHT Teil der App)
├── src-store-taxStore.ts      # Entwurfsdatei (NICHT Teil der App)
├── steuer-app-recherche.md    # Steuer-Recherche-Notizen
├── tarif2025.{pdf,txt}        # Offizielle SG-Tarifdokumente
├── wegleitung2025.pdf         # Wegleitung Steuererklärung
└── steuerpilot-sg-bu/         # ← HIER LEBT DIE APP (Next.js)
    ├── src/
    │   ├── app/               # Next.js App Router
    │   │   ├── api/extract/   # POST: Dokument → Claude Vision → typisiertes JSON
    │   │   ├── api/optimize/  # POST: Steuerfall → Optimierungsvorschläge
    │   │   ├── upload/        # Upload-Seite
    │   │   ├── wizard/        # Wizard-Seite + steps/ (5 Steps)
    │   │   └── ergebnis/      # Ergebnisseite
    │   ├── components/        # React-Komponenten (layout/, results/, upload/, wizard/, ui/)
    │   ├── lib/               # Business-Logik
    │   │   ├── tax/           # Steuerberechnung (calculator, types, parameters)
    │   │   ├── extraction/    # Claude Vision Parsing (noch nicht angelegt)
    │   │   ├── export/        # PDF-Export (pdf.tsx)
    │   │   └── anthropic.ts   # Anthropic API Client
    │   └── store/taxStore.ts  # Zustand Store (globaler App-State)
    ├── docs/
    │   └── steuerparameter.json  # Tarife, Steuerfüsse, Abzugsparameter (Source of Truth)
    ├── scripts/               # Playwright-basierte ETax-API-Analyse (nicht Teil der App)
    └── db/                    # (leer; für künftige SQLite-Nutzung reserviert)
```

---

## Entwicklungs-Befehle

Alle Befehle müssen in `steuerpilot-sg-bu/` ausgeführt werden:

```bash
cd steuerpilot-sg-bu

npm run dev        # Dev-Server starten (http://localhost:3000)
npm run build      # Produktions-Build
npm run lint       # ESLint
npm test           # Alle Tests (Jest)
npm run test:watch # Tests im Watch-Modus
```

**Einzelnen Test ausführen:**
```bash
npm test -- --testPathPattern="calculator"
```

**Umgebungsvariablen:** `.env.local` in `steuerpilot-sg-bu/` anlegen:
```
ANTHROPIC_API_KEY=sk-ant-xxxxx
NEXT_PUBLIC_APP_NAME=SteuerPilot SG
NEXT_PUBLIC_STEUERPERIODE=2024
```

---

## Tech Stack

| Komponente | Technologie |
|---|---|
| Framework | Next.js 16 (App Router) |
| Sprache | TypeScript (strict) |
| Styling | Tailwind CSS v4 |
| UI-Komponenten | shadcn/ui (radix-ui) |
| State | Zustand v5 |
| Formulare | react-hook-form + zod |
| KI | @anthropic-ai/sdk (Claude Sonnet) |
| PDF | @react-pdf/renderer |
| Tests | Jest + ts-jest |

**Aktives Modell:** `claude-sonnet-4-5-20250929` (in `src/lib/anthropic.ts`)

---

## Architektur-Kernpunkte

### Steuerparameter
Alle Tarife und Abzugslimits kommen aus `docs/steuerparameter.json`. Laden via `src/lib/tax/parameters.ts` (`getSteuerparameter()`, `getAlleGemeinden()`). Gemeindesteuerfüsse und Kirchensteuerfüsse sind dort konfiguriert – **niemals hardcoden**.

### Berechnungsflow (`src/lib/tax/calculator.ts`)
`berechneSteuern(steuerfall, parameter)` → `Steuerergebnis`
1. Gesamteinkommen (Bruttolöhne + Kapitalerträge + übrige Einkünfte)
2. Abzüge separat für Kanton und Bund (abweichende Limits, z.B. Fahrkosten: Kanton 4'595, Bund 3'000)
3. Einfache Steuer via progressivem Tarif, mit Splitting für Verheiratete (Satz auf ½ Einkommen, dann × 2)
4. Kantonssteuer = Einfache Steuer × Steuerfuss%; Gemeindesteuer, Kirchensteuer analog
5. Vermögenssteuer linear (1.7‰ auf steuerbares Vermögen nach Freibetrag)
6. Bundessteuer separat (eigener Tarif + eigene Abzugslimits)

### Anthropic API (`src/lib/anthropic.ts`)
- `extractDocument(base64, mediaType, type)` – Vision-Extraktion, gibt typisiertes JSON zurück
- `getOptimierungen(steuerfall)` – Optimierungsvorschläge als `Optimierung[]`
- Alle API-Calls **serverseitig** (API Routes) – `ANTHROPIC_API_KEY` nie im Frontend

### Zustand Store (`src/store/taxStore.ts`)
Wizard-Steps: `upload → personalien → einkommen → abzuege → vermoegen → zusammenfassung`
Zugriff via `useTaxStore()`. `getSteuerfall()` baut den vollständigen `Steuerfall`-Objekt zusammen.

### Beträge
Alle Berechnungen intern in **CHF** (nicht Rappen), Rundung auf 1 Franken am Ende via `Math.round()`. Anzeige via `formatCHF()` / `formatCHFRund()` aus `calculator.ts`.

---

## Bekannte TODOs / offene Punkte

- **Kinderabzug auf Steuerbetrag** (SG-spezifisch): Reduktion des Rechnungsbetrags pro Kind ist noch nicht implementiert (`calculator.ts`, Zeile ~71)
- **Bundessteuer Verheirateten-Tarif**: Aktuell als Approximation (`alleinstehend × 0.85`). Muss durch echten dBSt-Verheirateten-Tarif ersetzt werden (`calculator.ts`, Zeile ~349)
- **API Routes**: `/api/calculate` und `/api/export` noch nicht implementiert (vorhanden: `/api/extract`, `/api/optimize`)
- **Dokumenten-Extraktion**: `src/lib/extraction/` (lohnausweis.ts, bankstatement.ts, pillar3a.ts) noch nicht angelegt

---

## Kritische Entwicklungsregeln

1. **Steuerberechnung IMMER lokal** – Claude nur für OCR und Optimierungstext
2. **Jeder Abzug gegen sein Maximum validieren** – keine ungeprüften Werte durchlassen
3. **Bundessteuer separat berechnen** – eigener Tarif, eigene Limiten
4. **Kein Dokument dauerhaft speichern** – nach Extraktion sofort aus Speicher entfernen
5. **Splitting korrekt**: Satz auf halbes Einkommen berechnen, dann auf das ganze Einkommen anwenden

---

## User Flow

```
Landing → Upload (Lohnausweis Pflicht, 3a/Konto optional)
→ Extraktion & Prüfung (Claude Vision, User korrigiert)
→ Wizard (5 Steps: Personalien, Einkommen, Abzüge, Vermögen, Zusammenfassung)
→ Ergebnis (Steuerberechnung, Optimierungsvorschläge, PDF-Export)
```

---

## Rechtlicher Disclaimer (muss prominent angezeigt werden)

```
SteuerPilot SG ist ein Hilfstool zur Vorbereitung Ihrer Steuererklärung.
Die App ersetzt keine professionelle Steuerberatung und stellt keine
rechtsverbindliche Steuerberatung dar. Die Verantwortung für die
Richtigkeit der eingereichten Steuererklärung liegt ausschliesslich
bei der steuerpflichtigen Person. Für die definitive Einreichung
verwenden Sie bitte das offizielle E-Tax SG Portal des Kantons
St. Gallen. Alle hochgeladenen Dokumente werden verschlüsselt
übertragen und nach der Verarbeitung nicht dauerhaft gespeichert.
```

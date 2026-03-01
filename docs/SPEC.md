# SteuerPilot SG — Complete Technical Specification

> Intended audience: A developer recreating this app in Go + HTMX + Fiber.
> Everything needed to reimplement is documented here — no original source code required.

---

## Table of Contents

1. [Project Purpose](#1-project-purpose)
2. [Data Models](#2-data-models)
3. [Tax Parameters Database](#3-tax-parameters-database)
4. [Session & Application State](#4-session--application-state)
5. [HTTP Routes](#5-http-routes)
6. [Page-by-Page Specification](#6-page-by-page-specification)
7. [Tax Calculation Engine](#7-tax-calculation-engine)
8. [Claude API Integration](#8-claude-api-integration)
9. [PDF Export](#9-pdf-export)
10. [Validation Rules](#10-validation-rules)
11. [UI Behaviour & HTMX Patterns](#11-ui-behaviour--htmx-patterns)
12. [Design System](#12-design-system)

---

## 1. Project Purpose

SteuerPilot SG is a tax-preparation assistant for private individuals in the **Canton of St. Gallen (SG), Switzerland**. It does **not** submit tax declarations — it helps users fill them in by:

1. Reading uploaded documents (Lohnausweis / payslip, bank statements, pillar-3a certificates) via Claude Vision.
2. Walking the user through a 5-step wizard to verify and complete all tax-relevant data.
3. Computing the estimated tax burden locally using official SG parameters.
4. Offering AI-generated optimisation suggestions via Claude.
5. Exporting a structured PDF summary the user can transfer to the official E-Tax SG portal.

**Target group (MVP):** Employed (non-self-employed) individuals in canton SG; no real estate, no foreign income.

**Language:** Swiss High German throughout. No ß. Currency: CHF. Thousand separator: apostrophe (`CHF 1'234.56`). Decimal separator: full stop.

---

## 2. Data Models

These are the core data structures. Translate directly to Go structs with JSON tags.

### 2.1 Top-Level: `Steuerfall`

The complete tax case. This is the single object that flows through the entire application.

```
Steuerfall {
    Steuerperiode  int          // e.g. 2024
    Personalien    Personalien
    Einkommen      Einkommen
    Abzuege        Abzuege
    Vermoegen      Vermoegen
    Ergebnis       *Steuerergebnis  // null until calculated
    Optimierungen  []Optimierung    // null until fetched from Claude
}
```

### 2.2 `Personalien`

```
Personalien {
    Vorname         string
    Nachname        string
    Geburtsdatum    string   // ISO date, e.g. "1980-01-01"
    Zivilstand      string   // enum: see below
    Konfession      string   // enum: see below
    Gemeinde        string   // SG municipality name, e.g. "St. Gallen"
    Kinder          []Kind
    Partner         *Personalien  // only when married/registered partnership
}

Zivilstand values:
    "alleinstehend"             Single
    "verheiratet"               Married
    "geschieden"                Divorced
    "verwitwet"                 Widowed
    "getrennt"                  Legally separated
    "eingetragene_partnerschaft" Registered partnership

Konfession values:
    "evangelisch"
    "katholisch"
    "christkatholisch"
    "andere"
    "keine"              → church tax = 0

Kind {
    Vorname          string
    Geburtsdatum     string
    InAusbildung     bool    // true = in education/training
    Fremdbetreuung   bool    // external childcare
    Betreuungskosten float64 // annual childcare cost (if Fremdbetreuung)
}
```

### 2.3 `Einkommen`

```
Einkommen {
    Haupterwerb           Erwerbseinkommen
    Nebenerwerb           []Erwerbseinkommen
    WertschriftenErtraege float64   // securities income (Ziff. 4)
    Bankzinsen            float64   // bank interest (Ziff. 4.1)
    BeteiligungsErtraege  float64   // dividend income (Ziff. 4.3)
    LiegenschaftenEinkuenfte float64 // rental income (Ziff. 5) — not in MVP
    UebrigeEinkuenfte     float64   // other income (Ziff. 6)
    Renten                float64   // pension income (Ziff. 7)
    Kinderzulagen         float64   // child allowances (Ziff. 5 of Lohnausweis)
}

Erwerbseinkommen {
    Arbeitgeber          string
    Bruttolohn           float64  // gross salary (Lohnausweis Ziff. 1)
    Nettolohn            float64  // net salary (Lohnausweis Ziff. 11)
    AhvIvEoAlvNbuv       float64  // social contributions (Lohnausweis Ziff. 9)
    BvgOrdentlich        float64  // mandatory pension contributions (Ziff. 10.1)
    BvgEinkauf           float64  // voluntary pension buy-in (Ziff. 10.2)
    Quellensteuer        float64  // withholding tax (Ziff. 12)
    SpesenEffektiv       float64  // actual expenses (Ziff. 13.1)
    SpesenPauschal       float64  // flat expense allowance (Ziff. 13.2)
    AussendienstProzent  float64  // % of time in field sales (Ziff. 15)
    HatGeschaeftsauto    bool     // company car (field F)
    HatGA                bool     // employer-paid GA travelcard (field F)
    HatKantine           bool     // subsidised canteen (field G)
}
```

### 2.4 `Abzuege`

```
Abzuege {
    Berufskosten          Berufskosten
    Sozialabgaben         float64  // AHV/IV/EO/ALV/NBUV (direct from Lohnausweis)
    BvgBeitraege          float64  // BVG ordinary + buy-in
    Saeule3a              float64  // pillar 3a contributions
    Versicherungspraemien float64  // insurance premiums (Form 6)
    Krankheitskosten      float64  // out-of-pocket medical costs (before threshold)
    Schuldzinsen          float64  // interest on debt
    Unterhaltsbeitraege   float64  // alimony paid
    Spenden               float64  // charitable donations
    Weiterbildung         float64  // further education costs
    Liegenschaftsunterhalt float64 // property maintenance — not in MVP
}

Berufskosten {
    Fahrkosten            Fahrkosten
    Verpflegung           Verpflegung
    UebrigeBerufskosten   float64  // if 0, computed automatically (3% rule)
    Weiterbildungskosten  float64
}

Fahrkosten {
    Art          string   // "oev" | "auto" | "motorrad" | "velo" | "keine"
    DistanzKm    float64  // one-way distance to workplace in km
    Arbeitstage  int      // default 220
    OevKosten    float64  // actual public transport cost (if Art == "oev")
}

Verpflegung {
    Auswaertig  bool
    Kantine     bool
    Arbeitstage int  // default 220
}
```

### 2.5 `Vermoegen`

```
Vermoegen {
    Bankguthaben              []Bankkonto
    Wertschriften             float64  // securities (tax value at 31.12.)
    Fahrzeuge                 float64
    LebensversicherungRueckkauf float64 // surrender value of life insurance
    UebrigesVermoegen         float64
    Schulden                  float64  // all debt (mortgages, loans)
}

Bankkonto {
    Bank               string
    Bezeichnung        string   // account label
    IBAN               string   // optional
    Saldo              float64  // balance at 31.12.
    Waehrung           string   // "CHF"
    Zinsertrag         float64  // interest earned
    Verrechnungssteuer float64  // withholding tax on interest
}
```

### 2.6 `Steuerergebnis`

Output of the local tax calculation engine.

```
Steuerergebnis {
    // Income
    TotalEinkommen          float64
    TotalAbzuege            float64
    SteuerbaresEinkommen    float64  // canton
    SteuerbaresEinkommenBund float64 // federal (may differ due to different limits)

    // Wealth
    TotalVermoegen          float64
    TotalSchulden           float64
    SteuerbaresVermoegen    float64

    // Canton tax
    EinfacheSteuer          float64  // "unit tax" before multipliers
    Kantonssteuer           float64
    Gemeindesteuer          float64
    Kirchensteuer           float64

    // Federal tax
    Bundessteuer            float64

    // Wealth tax
    VermoegensSteuerKanton  float64
    VermoegensSteuerGemeinde float64

    // Totals
    TotalSteuer             float64

    // Meta
    Gemeinde                string
    SteuerfussGemeinde      int
    SteuerfussKanton        int
    SteuerfussKirche        int
    Steuerperiode           int
}
```

### 2.7 `Optimierung`

AI-generated tax optimisation suggestion.

```
Optimierung {
    Titel                string
    Beschreibung         string
    SparpotenzialMin     *float64  // nullable CHF savings estimate (lower bound)
    SparpotenzialMax     *float64  // nullable CHF savings estimate (upper bound)
    Aufwand              string    // "gering" | "mittel" | "hoch"
    Zeitrahmen           string    // "sofort" | "naechstes_jahr" | "langfristig"
    Kategorie            string    // "vorsorge" | "berufskosten" | "versicherung" | "timing" | "spenden" | "sonstiges"
    GesetzlicheGrundlage string   // legal basis reference
}
```

### 2.8 Document Extraction Results

Returned by Claude Vision after analysing an uploaded document.

```
ExtraktionsergebnisLohnausweis {
    ArbeitgeberName      string
    ArbeitgeberOrt       string
    ArbeitnehmerName     string
    AhvNummer            *string
    Bruttolohn           float64
    Nettolohn            float64
    Sozialabgaben        float64
    BvgOrdentlich        *float64
    BvgEinkauf           *float64
    SpesenEffektiv       *float64
    SpesenPauschal       *float64
    AussendienstProzent  *float64
    HatGeschaeftsauto    bool
    HatGA                bool
    HatKantine           bool
    Kinderzulagen        *float64
    Konfidenz {
        Gesamt           string    // "hoch" | "mittel" | "tief"
        UnsichereFelder  []string  // field names where Claude was uncertain
    }
}

ExtraktionsergebnisKonto {
    Stichtag  string  // ISO date
    Konten    []KontoExtraktion
    Konfidenz { ... }
}

KontoExtraktion {
    Bank               string
    Kontonummer        *string
    IBAN               *string
    Bezeichnung        string
    Waehrung           string
    Saldo              float64
    Zinsertrag         *float64
    Verrechnungssteuer *float64
}

Extraktion3a {
    Institut       string
    Steuerjahr     int
    Einzahlung     float64
    Art            string    // "bankkonto" | "versicherung" | "wertschriften"
    SaldoJahresende *float64
    Konfidenz { ... }
}
```

---

## 3. Tax Parameters Database

All hard numbers come from `docs/steuerparameter.json`. This is the single source of truth — load it at startup, never hardcode values. It must be reloadable without restart when tax years change.

### 3.1 Top-Level Structure

```json
{
  "steuerperiode": 2024,
  "kanton": "SG",
  "tarif": { ... },
  "steuerfuesse": { ... },
  "abzuege": { ... },
  "bundessteuer": { ... }
}
```

### 3.2 `tarif.einkommenssteuer`

Progressive tariff for canton-level income tax ("einfache Steuer"). The tariff is expressed as a lookup table with 10 brackets. Each bracket gives: lower bound, upper bound, base tax at lower bound, and marginal rate within the bracket.

```json
{
  "stufen": [
    { "von": 0,      "bis": 10000,  "basisSteuer": 0,       "rate": 0.0 },
    { "von": 10001,  "bis": 20000,  "basisSteuer": 0,       "rate": 0.015 },
    { "von": 20001,  "bis": 30000,  "basisSteuer": 150,     "rate": 0.02 },
    { "von": 30001,  "bis": 50000,  "basisSteuer": 350,     "rate": 0.03 },
    { "von": 50001,  "bis": 75000,  "basisSteuer": 950,     "rate": 0.04 },
    { "von": 75001,  "bis": 100000, "basisSteuer": 1950,    "rate": 0.05 },
    { "von": 100001, "bis": 150000, "basisSteuer": 3200,    "rate": 0.06 },
    { "von": 150001, "bis": 250000, "basisSteuer": 6200,    "rate": 0.07 },
    { "von": 250001, "bis": 521600, "basisSteuer": 13200,   "rate": 0.08 },
    { "von": 521601, "bis": 999999999, "basisSteuer": 34928, "rate": 0.085 }
  ],
  "maxRate": 0.085,
  "maxEinkommenGemeinsam": 521600,
  "maxEinkommenAlleinstehend": 260800
}
```

### 3.3 `tarif.vermoegenssteuer`

Linear wealth tax.

```json
{
  "rate": 0.0017,
  "freibetragPerson": 75000,
  "freibetragKind": 20000
}
```

### 3.4 `steuerfuesse`

Tax multipliers as integers (percentage points). The canton sets the base; municipalities and churches apply their own multipliers on top of the same "einfache Steuer".

```json
{
  "kanton": 105,
  "gemeinden": {
    "Gommiswald": 103,
    "St. Gallen": 138,
    "Rapperswil-Jona": 80,
    "Goldach": 104,
    "Gossau": 107,
    "Rorschach": 123,
    "Arbon": 112,
    "Wattwil": 108,
    "Buchs": 105,
    "Wil": 108
    // ... 121 municipalities total
  },
  "kirche": {
    "evangelisch": { "typisch": 24 },
    "katholisch":  { "typisch": 24 },
    "christkatholisch": 24
  }
}
```

> Note: Church tax is typically 20–28%. The JSON stores a representative value per confession. For the MVP a fixed typical value per confession is used.

### 3.5 `abzuege`

All deduction rules (canton-level unless noted).

```json
{
  "berufskosten": {
    "fahrkostenMax": 4595,
    "fahrkostenMaxBund": 3000,
    "kmAuto": 0.70,
    "kmMotorrad": 0.40,
    "veloPauschale": 700,
    "verpflegungTag": 15.00,
    "verpflegungMax": 3200,
    "verpflegungKantineTag": 7.50,
    "verpflegungKantineMax": 1600,
    "uebrigeMin": 2000,
    "uebrigeMax": 4000,
    "uebrigeProzent": 0.03
  },
  "vorsorge": {
    "saeule3aMitPk": 7056,
    "saeule3aOhnePk": 35280,
    "saeule3aOhnePkProzent": 0.20
  },
  "versicherungen": {
    "alleinstehend": 3400,
    "alleinstehendOhneVorsorge": 3900,
    "gemeinsam": 6700,
    "gemeinsamOhneVorsorge": 7700,
    "proKind": 1100
  },
  "krankheitskosten": {
    "selbstbehaltProzent": 0.05
  },
  "weiterbildungMax": 12000,
  "schuldzinsenMaxBasis": 50000,
  "spendenMaxProzent": 0.20,
  "sozialabzuege": {
    "kinderabzugVorschule": 7500,
    "kinderabzugAusbildung": 10600
  }
}
```

### 3.6 `bundessteuer`

Federal direct tax uses a completely separate tariff and separate deduction limits.

```json
{
  "stufen_alleinstehend": [
    { "von": 0,      "bis": 14500,  "basisSteuer": 0,       "rate": 0.0 },
    { "von": 14501,  "bis": 31600,  "basisSteuer": 0,       "rate": 0.0077 },
    { "von": 31601,  "bis": 41400,  "basisSteuer": 131.65,  "rate": 0.0088 },
    { "von": 41401,  "bis": 55200,  "basisSteuer": 217.90,  "rate": 0.0264 },
    { "von": 55201,  "bis": 72500,  "basisSteuer": 582.15,  "rate": 0.0297 },
    { "von": 72501,  "bis": 78100,  "basisSteuer": 1096.05, "rate": 0.0561 },
    { "von": 78101,  "bis": 103600, "basisSteuer": 1410.00, "rate": 0.0825 },
    { "von": 103601, "bis": 134600, "basisSteuer": 3514.50, "rate": 0.0990 },
    { "von": 134601, "bis": 176000, "basisSteuer": 6583.50, "rate": 0.1080 },
    { "von": 176001, "bis": 755200, "basisSteuer": 11055,   "rate": 0.1190 },
    { "von": 755201, "bis": 999999999, "basisSteuer": 79970, "rate": 0.1150 }
  ],
  "fahrkostenMax": 3000,
  "versicherungenAlleinstehend": 1800,
  "versicherungenGemeinsam": 3700,
  "versicherungenProKind": 700,
  "kinderabzug": 6700,
  "weiterbildungMax": 12000
}
```

> **Important:** The federal married tariff is a completely separate table (lower rates). As a known simplification in the MVP, `married federal tax = single federal tax × 0.85`. This should be replaced with the proper Tarif B table.

---

## 4. Session & Application State

Since HTMX does not use a JavaScript global store, all state must live **server-side in a session**.

### 4.1 Session Data Structure

Store the entire `Steuerfall` object in the server session, keyed by a session cookie. Additionally store:

```
Session {
    SessionID           string
    Steuerfall          Steuerfall
    CurrentStep         string        // wizard step name
    UploadedFiles       []UploadedFile
    ExtractionResult    *ExtractionResult  // latest OCR result (discarded after accepted)
    IsLoading           bool
}

UploadedFile {
    Name       string
    Type       string   // "lohnausweis" | "kontoauszug" | "3a"
    Extracted  bool
}

ExtractionResult {
    Type   string      // document type
    Result interface{} // one of the Extraktion* structs
}
```

### 4.2 Wizard Step Order

```
upload → personalien → einkommen → abzuege → vermoegen → zusammenfassung
```

Steps map to routes: `GET /wizard/{step}`. Navigating forward: `POST /wizard/{step}/next`. Navigating back: `GET /wizard/{prev-step}`.

### 4.3 Step Validation for "Next"

| Step | Can proceed when |
|---|---|
| `upload` | Always (Lohnausweis upload is recommended but not blocking) |
| `personalien` | Vorname, Nachname, and Gemeinde are non-empty |
| `einkommen` | Always |
| `abzuege` | Always |
| `vermoegen` | Always |
| `zusammenfassung` | Never (last step, navigates to `/ergebnis`) |

---

## 5. HTTP Routes

### Public Routes

| Method | Path | Handler | Description |
|---|---|---|---|
| GET | `/` | LandingPage | Landing page with hero, workflow cards, disclaimer |
| GET | `/upload` | UploadPage | Document upload interface |
| POST | `/api/upload` | HandleUpload | Accepts file, calls Claude Vision, returns extraction JSON |
| GET | `/wizard/{step}` | WizardPage | Renders current wizard step |
| POST | `/wizard/{step}/submit` | WizardSubmit | Saves form data, advances to next step |
| GET | `/wizard/{step}/back` | WizardBack | Returns to previous step |
| GET | `/ergebnis` | ErgebnisPage | Results page |
| POST | `/api/optimize` | HandleOptimize | Calls Claude for optimisation suggestions |
| GET | `/api/export/pdf` | ExportPDF | Generates and streams PDF |
| POST | `/api/extraction/accept` | AcceptExtraction | Transfers OCR results to session Steuerfall |

### HTMX Partial Routes (return HTML fragments)

| Method | Path | Description |
|---|---|---|
| POST | `/htmx/wizard/personalien` | Partial: update personalien in session, return form |
| POST | `/htmx/wizard/kind/add` | Add a child entry, return updated Kinder list |
| DELETE | `/htmx/wizard/kind/{index}` | Remove child, return updated list |
| POST | `/htmx/wizard/konto/add` | Add bank account row |
| DELETE | `/htmx/wizard/konto/{index}` | Remove bank account row |
| GET | `/htmx/tax/calculate` | Run tax calculation, return result fragment |
| POST | `/htmx/extraction/preview` | Upload document, return extraction preview fragment |

---

## 6. Page-by-Page Specification

### 6.1 Landing Page (`/`)

**Purpose:** Introduce the app, build trust, direct user to start.

**Content:**
- **Hero section**: Blue gradient background. Title "SteuerPilot SG", subtitle explaining the app saves time by reading documents and computing taxes.
- **Three feature cards** (horizontal row, icons):
  1. 📄 "Dokumente einlesen" — Upload Lohnausweis and documents, AI reads them
  2. 🧮 "Abzüge optimieren" — Wizard with intelligent deduction suggestions
  3. 📊 "Übersicht exportieren" — PDF summary for E-Tax SG transfer
- **Disclaimer alert** (amber/yellow background, AlertTriangle icon): Legal disclaimer text (see Section 12). Always visible.
- **CTA button**: "Jetzt starten" → `/upload`

**Data flow:** No session data needed. Stateless page.

---

### 6.2 Upload Page (`/upload`)

**Purpose:** Let user upload one or more tax documents; show Claude's extraction results for review.

**Layout:**

Three document upload zones stacked vertically:

| # | Label | Required | Accepted types |
|---|---|---|---|
| 1 | Lohnausweis | Required | JPG, PNG, WEBP, PDF |
| 2 | Kontoauszug / Bankbestätigung | Optional | JPG, PNG, WEBP, PDF |
| 3 | Säule-3a-Bescheinigung | Optional | JPG, PNG, WEBP, PDF |

**Each upload zone ("DropZone"):**
- Dashed border, centered content
- Icon (Upload cloud)
- Text: "Datei hier ablegen oder klicken"
- Subtext: "JPG, PNG, PDF · max. 10 MB"
- On hover: blue border + light blue background
- On drag-over: same highlight
- On file selected: show filename + file size
- Disabled state: grayed out with cursor-not-allowed

**Upload flow (HTMX):**
1. User drops/selects file → `hx-post="/htmx/extraction/preview"` with file + type
2. Server processes file (see Section 8.1), returns extraction preview HTML fragment
3. Preview replaces the drop zone area (`hx-target="#extraction-preview-{type}"`)
4. Show loading spinner while Claude processes

**Extraction Preview Component (for Lohnausweis):**

After extraction, display a review panel:
- **Konfidenz badge** top-right:
  - `hoch` → green badge "Hohe Konfidenz"
  - `mittel` → amber badge "Mittlere Konfidenz"
  - `tief` → red badge "Geringe Konfidenz — bitte prüfen"
- **Employer block**: "Arbeitgeber: {ArbeitgeberName}, {ArbeitgeberOrt}"
- **Editable field table** for each extracted value:

| Field label | JSON field | Notes |
|---|---|---|
| Bruttolohn (Ziff. 1) | Bruttolohn | |
| Nettolohn (Ziff. 11) | Nettolohn | |
| AHV/IV/EO/ALV/NBUV (Ziff. 9) | Sozialabgaben | |
| BVG ordentlich (Ziff. 10.1) | BvgOrdentlich | nullable |
| BVG Einkauf (Ziff. 10.2) | BvgEinkauf | nullable |
| Kinderzulagen (Ziff. 5) | Kinderzulagen | nullable |

Each editable field:
- Display mode: CHF-formatted value, "Korrigieren" link
- Fields in `UnsichereFelder` list: amber background + ⚠ icon
- Edit mode: plain number input (click "Korrigieren" to toggle)
- On blur/Enter: update display, return to display mode

Checkboxes (non-editable, but correctable):
- Geschäftsauto / GA / Kantine — shown as toggle badges

**"Übernehmen" button:**
- Calls `POST /api/extraction/accept` with document type
- Server merges extraction into session `Steuerfall`:
  - `Haupterwerb.Bruttolohn`, `.Nettolohn`, `.AhvIvEoAlvNbuv` ← from Lohnausweis
  - `Abzuege.Sozialabgaben` ← AHV/IV/EO/ALV
  - `Abzuege.BvgBeitraege` ← BVG ordentlich
  - `Einkommen.Kinderzulagen` ← Kinderzulagen
  - `HatGeschaeftsauto`, `HatGA`, `HatKantine` ← flags
- Marks file as `Extracted: true` in session
- On success: redirect to `/wizard/personalien` (if Lohnausweis accepted)

---

### 6.3 Wizard Page (`/wizard/{step}`)

**Purpose:** 5-step form to collect/verify all tax data.

**Layout:**
- **StepIndicator** at top: 5 circles with connecting lines
  - Completed steps: filled blue circle with ✓
  - Current step: blue border, white fill
  - Future steps: gray border
  - Label under each circle (hidden on mobile)
  - Labels: Upload | Personalien | Einkommen | Abzüge | Vermögen | Zusammenfassung
- **Content area**: step-specific form (see 6.3.1–6.3.5)
- **Navigation buttons** at bottom:
  - Left: "Zurück" button (gray) → `GET /wizard/{prev-step}` — hidden on first step
  - Right: "Weiter" / "Berechnung starten" (blue) → `POST /wizard/{step}/submit`
  - Last step button: "Ergebnis anzeigen" → `GET /ergebnis`

#### 6.3.1 Step: `personalien`

**Purpose:** Collect personal information.

**Fields:**

| Label | Input type | Binding | Notes |
|---|---|---|---|
| Vorname | text | Personalien.Vorname | Required |
| Nachname | text | Personalien.Nachname | Required |
| Geburtsdatum | date | Personalien.Geburtsdatum | ISO date |
| Gemeinde | select | Personalien.Gemeinde | Dropdown of all 121 SG municipalities (alphabetically sorted) |
| Zivilstand | select | Personalien.Zivilstand | 6 options |
| Konfession | select | Personalien.Konfession | 5 options |

**Children section:**
- List of current children (empty by default)
- For each child:
  - Name, Geburtsdatum, InAusbildung checkbox, Fremdbetreuung checkbox (+ Betreuungskosten field if checked)
  - ✕ button to remove
- "Kind hinzufügen" button → HTMX inserts new empty child row
- Each add/remove: `hx-post="/htmx/wizard/kind/add"` or `hx-delete="/htmx/wizard/kind/{i}"`

**Help tooltips:**
- Gemeinde: "Wählen Sie die Gemeinde, in der Sie am 31. Dezember des Steuerjahres wohnen."
- Konfession: "Massgebend für die Kirchensteuer. Bei 'keine' entfällt die Kirchensteuer."

#### 6.3.2 Step: `einkommen`

**Purpose:** Review and complete income data (pre-filled from Lohnausweis extraction).

**Section 1 — Haupterwerb:**

| Label | Input | Notes |
|---|---|---|
| Arbeitgeber | text | Pre-filled from extraction |
| Bruttolohn (Ziff. 1) | number (CHF) | Pre-filled |
| Nettolohn (Ziff. 11) | number (CHF) | Pre-filled |
| AHV/IV/EO/ALV/NBUV (Ziff. 9) | number (CHF) | Pre-filled |
| BVG ordentlich (Ziff. 10.1) | number (CHF) | Pre-filled |
| BVG Einkauf (Ziff. 10.2) | number (CHF) | Optional |
| Geschäftsauto? | checkbox | Pre-filled |
| GA/ÖV bezahlt vom AG? | checkbox | Pre-filled |
| Verbilligte Kantine? | checkbox | Pre-filled |

**Section 2 — Weitere Einkünfte:**

| Label | Input | Tooltip |
|---|---|---|
| Wertschriftenerträge (Ziff. 4) | number (CHF) | "Dividenden, Fonds-Ausschüttungen. Steht auf der Vermögensaufstellung der Bank." |
| Bankzinsen (Ziff. 4.1) | number (CHF) | "Zinserträge auf Spar- und Kontokorrentkonten." |
| Kinderzulagen | number (CHF) | "Familienzulagen vom Arbeitgeber. Meist auf dem Lohnausweis Ziff. 5." |
| Renten | number (CHF) | "AHV, IV, Pensionskassenrenten, etc." |
| Übrige Einkünfte | number (CHF) | "Sonstige steuerbare Einkünfte, z.B. Stipendien über CHF 2'400." |

**Live total display:** Below the form, show "Gesamteinkommen: CHF {sum}" updating via HTMX on blur.

#### 6.3.3 Step: `abzuege`

**Purpose:** Enter deductions — the most complex step. Show computed limits next to each field.

**Section 1 — Berufskosten (Formular 4):**

Sub-section: Fahrkosten
- Radio group: `oev` | `auto` | `motorrad` | `velo` | `keine`
- If `oev`: show field "Effektive ÖV-Kosten (CHF)" with tooltip "Jahreskarte/Monatsabos zum Arbeitsort."
- If `auto` or `motorrad`: show fields:
  - "Distanz einfacher Weg (km)"
  - "Arbeitstage" (default 220)
  - Computed cost displayed: `{Distanz × 2 × Arbeitstage × Ansatz}` (auto: 0.70, motorrad: 0.40)
- If `velo`: show flat CHF 700, no fields
- Limit shown: "Max. CHF 4'595 (Kanton), CHF 3'000 (Bund)"

Sub-section: Verpflegung
- Checkbox: "Auswärts verpflegt" → if checked show:
  - Checkbox: "Verbilligte Kantine vorhanden"
  - "Arbeitstage" field (default 220)
  - Computed amount shown: if Kantine: `Tage × 7.50`, max CHF 1'600; else: `Tage × 15.00`, max CHF 3'200

Sub-section: Übrige Berufskosten
- Input (optional): "Effektive übrige Berufskosten (CHF)"
- If left at 0: auto-computed as `max(2'000, min(Nettolohn × 3%, 4'000))`
- Show computed amount: "Automatisch berechnet: CHF {X}"

**Section 2 — Vorsorge:**

| Label | Notes |
|---|---|
| Säule 3a | Show max: CHF 7'056 (mit PK) or CHF 35'280 / 20% of net (ohne PK). PK status auto-detected from BvgOrdentlich > 0 |
| BVG-Einkauf | Pre-filled from Lohnausweis Ziff. 10.2 |

**Section 3 — Versicherungsprämien:**
- Input: total paid for health/life/accident insurance
- Show max: CHF 3'400 alleinstehend (CHF 3'900 without PK), + CHF 1'100 per child; CHF 6'700 verheiratet

**Section 4 — Krankheits- und Unfallkosten:**
- Input: total out-of-pocket medical costs
- Tooltip: "Nur der Teil über 5% des Nettoeinkommens ist abzugsfähig."
- Show threshold: "5%-Schwelle: CHF {0.05 × Nettoeinkommen}"

**Section 5 — Weitere Abzüge:**

| Label | Notes |
|---|---|
| Schuldzinsen | Max: CHF 50'000 + Vermögenserträge |
| Weiterbildungskosten | Max: CHF 12'000 |
| Unterhaltsbeiträge | No maximum |
| Spenden | Max: 20% of net income |

#### 6.3.4 Step: `vermoegen`

**Purpose:** Capture wealth at 31 December of the tax year.

**Section 1 — Bankkonten (dynamic list):**
For each account:
- Bank (text)
- Bezeichnung (text, e.g. "Privatkonto", "Sparkonto")
- Saldo per 31.12. (number CHF)
- Zinsertrag (number CHF)
- ✕ to remove

Button: "Konto hinzufügen" → HTMX appends new row.

**Section 2 — Wertschriften:**
- Input: "Steuerwert Wertschriften per 31.12. (CHF)"
- Tooltip: "Steht auf der Vermögensaufstellung Ihrer Bank. Massgebend ist der Kurs-/Steuerwert, nicht der Marktwert."

**Section 3 — Übrige Vermögenswerte:**

| Label | Notes |
|---|---|
| Fahrzeuge | Verkehrswert (market value) |
| Rückkaufswert Lebensversicherung | Surrender value |
| Übriges Vermögen | Antiquitäten, Sammlungen, etc. |

**Section 4 — Schulden:**
- Input: "Total Schulden per 31.12. (CHF)"
- Tooltip: "Hypotheken, Privatkredite, Leasingverbindlichkeiten."

**Live calculation — Reinvermögen:**
`Reinvermögen = (Summe Bankguthaben + Wertschriften + Fahrzeuge + Lebensversicherung + Übriges) − Schulden`
Display below: "Reinvermögen: CHF {X}"

#### 6.3.5 Step: `zusammenfassung`

**Purpose:** Show full calculation summary; trigger `berechneSteuern()` on the server.

**Trigger:** When this step is rendered, the server calls `berechneSteuern(steuerfall, parameter)` and stores `Steuerergebnis` in session.

**Layout — three cards:**

**Card 1: Einkommen**

| Line | Value |
|---|---|
| Gesamteinkommen | CHF {TotalEinkommen} |
| ./. Abzüge | CHF {TotalAbzuege} |
| = Steuerbares Einkommen (Kanton) | **CHF {SteuerbaresEinkommen}** |
| = Steuerbares Einkommen (Bund) | CHF {SteuerbaresEinkommenBund} |

**Card 2: Vermögen**

| Line | Value |
|---|---|
| Bruttovermögen | CHF {TotalVermoegen} |
| ./. Schulden | CHF {TotalSchulden} |
| ./. Freibetrag | CHF {Freibetrag} |
| = Steuerbares Vermögen | **CHF {SteuerbaresVermoegen}** |

**Card 3: Steuerberechnung**

| Line | Value |
|---|---|
| Einfache Steuer | CHF {EinfacheSteuer} |
| Kantonssteuer ({SteuerfussKanton}%) | CHF {Kantonssteuer} |
| Gemeindesteuer ({SteuerfussGemeinde}%) | CHF {Gemeindesteuer} |
| Kirchensteuer ({SteuerfussKirche}%) | CHF {Kirchensteuer} |
| Bundessteuer | CHF {Bundessteuer} |
| Vermögenssteuer Kanton | CHF {VermoegensSteuerKanton} |
| Vermögenssteuer Gemeinde | CHF {VermoegensSteuerGemeinde} |
| **Total Steuer** | **CHF {TotalSteuer}** |

Button: "Ergebnis anzeigen" → `GET /ergebnis`

---

### 6.4 Results Page (`/ergebnis`)

**Purpose:** Full results with visualisation, optimisation suggestions, and PDF export.

**Section 1 — 3 KPI cards (top row):**
- "Steuerbares Einkommen: CHF {X}"
- "Steuerbares Vermögen: CHF {X}"
- "Geschätzte Gesamtsteuer: CHF {X}"

**Section 2 — DeductionBreakdown:**
A horizontal bar visualising:
- Blue segment: Steuerbares Einkommen (proportion of TotalEinkommen)
- Green segment: Abzüge (proportion of TotalEinkommen)
- Percentages labelled inside segments
- Below the bar: table of individual deductions with labels and CHF amounts

**Section 3 — Steuerberechnung detail table:**
Full tax breakdown (same as Zusammenfassung step card 3).

**Section 4 — Optimierungsvorschläge:**
Load trigger: when the page first loads, fire `POST /api/optimize` in background (HTMX `hx-trigger="load"`). While loading, show spinner.

Display up to 5 `OptimizationCard` components. Each card:
- **Left column**: Rank circle (1–5, blue)
- **Main content**:
  - Title (bold)
  - Kategorie badge (green outline): e.g. "Vorsorge", "Berufskosten"
  - Aufwand: "Geringer Aufwand" (Zap icon) / "Mittlerer Aufwand" / "Hoher Aufwand" (Clock icon)
  - Zeitrahmen: "Sofort möglich" / "Nächstes Jahr" / "Langfristig"
  - Description paragraph
  - Gesetzliche Grundlage (small gray text at bottom)
- **Right column**: Sparpotenzial (TrendingDown icon, green text) — "CHF {min} – CHF {max}/Jahr" or "–" if null

**Section 5 — Export & navigation:**
- Button: "PDF exportieren" → `GET /api/export/pdf` — streams PDF download
- Link: "E-Tax SG öffnen" → external link to official portal
- Button: "Neue Berechnung" → clears session, redirect to `/`

---

## 7. Tax Calculation Engine

This is the core of the application. Implement as a pure function:

```
berechneSteuern(steuerfall Steuerfall, params SteuerparameterDB) → Steuerergebnis
```

**All monetary values in CHF (float64). Round to nearest franc at the end using `math.Round()`.**

### 7.1 Step 1: Total Income

```
totalEinkommen =
    Haupterwerb.Bruttolohn
    + sum(Nebenerwerb[i].Bruttolohn)
    + WertschriftenErtraege
    + Bankzinsen
    + BeteiligungsErtraege
    + LiegenschaftenEinkuenfte
    + UebrigeEinkuenfte
    + Renten
    + Kinderzulagen
```

### 7.2 Step 2: Total Deductions

Call `berechneTotalAbzuege()` **twice**: once with `isBund=false` for canton, once with `isBund=true` for federal. The two values can differ (e.g. Fahrkosten limit 4'595 vs 3'000).

#### 7.2.1 Berufskosten

**Fahrkosten:**
```
fahrkostenBrutto =
    if Art == "oev":   OevKosten
    if Art == "auto":  DistanzKm × 2 × Arbeitstage × 0.70
    if Art == "motorrad": DistanzKm × 2 × Arbeitstage × 0.40
    if Art == "velo":  700
    if Art == "keine": 0

fahrkostenAbzug = min(fahrkostenBrutto, fahrkostenMax)
    where fahrkostenMax = 4595 (canton) or 3000 (federal)
```

**Verpflegung:**
```
if Auswaertig:
    if Kantine:
        verpflegung = min(Arbeitstage × 7.50, 1600)
    else:
        verpflegung = min(Arbeitstage × 15.00, 3200)
else:
    verpflegung = 0
```

**Übrige Berufskosten:**
```
if UebrigeBerufskosten > 0:
    uebrige = UebrigeBerufskosten
else:
    computed = Nettolohn × 0.03
    uebrige = max(2000, min(computed, 4000))
```

**Total Berufskosten:**
```
berufskosten = fahrkostenAbzug + verpflegung + uebrige
    + min(Weiterbildungskosten, weiterbildungMax)
```

#### 7.2.2 Sozialabgaben

No limit — take directly from `Abzuege.Sozialabgaben` (sum of AHV/IV/EO/ALV/NBUV from Lohnausweis Ziff. 9).

#### 7.2.3 BVG

No limit — take `Abzuege.BvgBeitraege` (sum of BvgOrdentlich + BvgEinkauf from Lohnausweis).

#### 7.2.4 Säule 3a

```
hatPK = BvgBeitraege > 0

if hatPK:
    max3a = params.Vorsorge.Saeule3aMitPk  // CHF 7'056
else:
    max3a = min(
        params.Vorsorge.Saeule3aOhnePk,    // CHF 35'280
        Haupterwerb.Nettolohn × 0.20
    )

saeule3aAbzug = min(Saeule3a, max3a)
```

#### 7.2.5 Versicherungsprämien

```
istVerheiratet = Zivilstand in ["verheiratet", "eingetragene_partnerschaft"]
anzahlKinder = len(Kinder)

// Federal
if isBund:
    versMax = istVerheiratet ? 3700 : 1800
    versMax += anzahlKinder × 700

// Canton
else:
    hatVorsorge = BvgBeitraege > 0 || Saeule3a > 0
    if istVerheiratet:
        versMax = hatVorsorge ? 6700 : 7700
    else:
        versMax = hatVorsorge ? 3400 : 3900
    versMax += anzahlKinder × 1100

versAbzug = min(Versicherungspraemien, versMax)
```

#### 7.2.6 Krankheitskosten

```
nettoeinkommen = totalEinkommen - Sozialabgaben - BvgBeitraege
schwelle = nettoeinkommen × 0.05
krankheitAbzug = max(0, Krankheitskosten - schwelle)
```

#### 7.2.7 Schuldzinsen

```
schuldzinsenMax = 50000 + Bankzinsen + WertschriftenErtraege
schuldzinsenAbzug = min(Schuldzinsen, schuldzinsenMax)
```

#### 7.2.8 Spenden

```
spendenMax = nettoeinkommen × 0.20
spendenAbzug = min(Spenden, spendenMax)
```

#### 7.2.9 Weiterbildung

```
wbMax = isBund ? 12000 : 12000  // same for both (federal also 12'000)
weiterbildungAbzug = min(Weiterbildung, wbMax)
```

#### 7.2.10 Unterhaltsbeiträge

```
unterhaltAbzug = Unterhaltsbeitraege  // no limit
```

#### 7.2.11 Sozialabzüge (Kinder)

```
for each Kind:
    if isBund:
        abzug += 6700  // federal child deduction, fixed
    else if Kind.InAusbildung:
        abzug += 10600
    else:
        abzug += 7500
```

#### 7.2.12 Total Abzüge

```
totalAbzuege =
    berufskosten
    + sozialabgaben
    + bvgBeitraege
    + saeule3aAbzug
    + versAbzug
    + krankheitAbzug
    + schuldzinsenAbzug
    + spendenAbzug
    + weiterbildungAbzug
    + unterhaltAbzug
    + liegenschaftsunterhalt
    + kinderabzuege
```

### 7.3 Step 3: Steuerbares Einkommen

```
steuerbaresEinkommen     = max(0, round(totalEinkommen - totalAbzuege_kanton))
steuerbaresEinkommenBund = max(0, round(totalEinkommen - totalAbzuege_bund))
```

### 7.4 Step 4: Steuerbares Vermögen

```
totalVermoegen = sum(Bankguthaben[i].Saldo)
    + Wertschriften + Fahrzeuge
    + LebensversicherungRueckkauf + UebrigesVermoegen

reinvermoegen = max(0, totalVermoegen - Schulden)

// Freibetrag
freibetragPersonen = len + (istVerheiratet ? 1 : 0) × 75000
// Note: married = 2 × freibetragPerson
freibetragKinder   = len(Kinder) × 20000
freibetragTotal    = freibetragPersonen + freibetragKinder

steuerbaresVermoegen = max(0, reinvermoegen - freibetragTotal)
```

### 7.5 Step 5: Einfache Steuer (Progressive Tariff)

```
func berechneEinfacheSteuer(steuerbaresEinkommen float64, splitting bool, params) float64:

    // Splitting: use half income to find rate, then apply that rate to full income
    satzEinkommen = if splitting: floor(steuerbaresEinkommen / 2)
                    else: steuerbaresEinkommen

    // Check max rate cap
    maxEinkommen = if splitting: params.MaxEinkommenGemeinsam
                   else: params.MaxEinkommenAlleinstehend
    if steuerbaresEinkommen > maxEinkommen:
        return round(steuerbaresEinkommen × params.MaxRate)

    // Find bracket
    for each Stufe in params.Tarif.Stufen:
        if satzEinkommen >= Stufe.Von && satzEinkommen <= Stufe.Bis:
            steuerBetrag = Stufe.BasisSteuer + (satzEinkommen - Stufe.Von) × Stufe.Rate
            break

    // Apply splitting
    if splitting && satzEinkommen > 0:
        effektiverSatz = steuerBetrag / satzEinkommen
        return round(steuerbaresEinkommen × effektiverSatz)

    return round(steuerBetrag)
```

**Splitting rule:** Married taxpayers use the "Vollsplitting" method: look up the tax rate for half the income, then apply that effective rate to the full income. This gives a significantly lower result than a single person with the same income.

### 7.6 Step 6: Tax Amounts

```
steuerfussGemeinde = params.Steuerfuesse.Gemeinden[Personalien.Gemeinde]
steuerfussKanton   = params.Steuerfuesse.Kanton  // 105
steuerfussKirche   = lookupKirchensteuerfuss(Konfession, params)
    // evangelisch → 24, katholisch → 24, christkatholisch → 24, andere/keine → 0

kantonssteuer   = round(einfacheSteuer × steuerfussKanton   / 100)
gemeindesteuer  = round(einfacheSteuer × steuerfussGemeinde / 100)
kirchensteuer   = round(einfacheSteuer × steuerfussKirche   / 100)
```

### 7.7 Step 7: Bundessteuer

```
func berechneBundessteuer(steuerbaresEinkommenBund float64, istVerheiratet bool, params) float64:
    stufen = params.Bundessteuer.StufenAlleinstehend

    for each Stufe in stufen:
        if steuerbaresEinkommenBund >= Stufe.Von && steuerbaresEinkommenBund <= Stufe.Bis:
            steuer = Stufe.BasisSteuer + (steuerbaresEinkommenBund - Stufe.Von) × Stufe.Rate
            break

    // TODO: replace with proper married tariff (Tarif B)
    // Current approximation:
    if istVerheiratet:
        steuer = steuer × 0.85

    return round(steuer)
```

### 7.8 Step 8: Vermögenssteuer

```
vermoegensSteuerRate = params.Vermoegenssteuer.Rate  // 0.0017

einfacheVermoegensSteuer = round(steuerbaresVermoegen × vermoegensSteuerRate)

vermoegensSteuerKanton  = round(einfacheVermoegensSteuer × steuerfussKanton   / 100)
vermoegensSteuerGemeinde = round(einfacheVermoegensSteuer × steuerfussGemeinde / 100)
```

### 7.9 Step 9: Total

```
totalSteuer =
    kantonssteuer
    + gemeindesteuer
    + kirchensteuer
    + bundessteuer
    + vermoegensSteuerKanton
    + vermoegensSteuerGemeinde
```

---

## 8. Claude API Integration

All Claude calls are **server-side only**. The API key is never exposed to the client. Model: `claude-sonnet-4-5-20250929` for all calls.

### 8.1 Document Extraction

**Endpoint:** `POST /api/upload` (called by the server when user uploads a file)

**Request to Claude:**

```
POST to Anthropic Messages API:
  model: "claude-sonnet-4-5-20250929"
  max_tokens: 2000
  messages: [{
    role: "user",
    content: [
      {
        type: "image" (or "document" for PDF),
        source: {
          type: "base64",
          media_type: <mimetype>,
          data: <base64 encoded file>
        }
      },
      {
        type: "text",
        text: <prompt — see below>
      }
    ]
  }]
```

**File validation before calling Claude:**
- Max size: 10 MB
- Allowed MIME types: `image/jpeg`, `image/png`, `image/webp`, `application/pdf`
- For PDF: use `type: "document"` in the content block
- For images: use `type: "image"`

**Prompt for Lohnausweis:**

```
Du analysierst einen Schweizer Lohnausweis (Formular 11 / Neues Lohnausweisformular NLA).
Extrahiere alle steuerlich relevanten Felder.

Antworte NUR mit einem JSON-Objekt, kein Fliesstext:

{
  "arbeitgeber_name": string,
  "arbeitgeber_ort": string,
  "arbeitnehmer_name": string,
  "ahv_nummer": string | null,
  "ziff8_bruttolohn": number,
  "ziff9_sozialabgaben": number,
  "ziff10_1_bvg_ordentlich": number | null,
  "ziff10_2_bvg_einkauf": number | null,
  "ziff11_nettolohn": number,
  "ziff12_quellensteuer": number | null,
  "ziff13_1_spesen_effektiv": number | null,
  "ziff13_2_spesen_pauschal": number | null,
  "ziff15_aussendienst_prozent": number | null,
  "ziff5_kinderzulagen": number | null,
  "feld_f_ga_oder_geschaeftsauto": boolean,
  "feld_g_kantine": boolean,
  "konfidenz": {
    "gesamt": "hoch" | "mittel" | "tief",
    "unsichere_felder": string[]
  }
}

Regeln:
- Beträge in CHF als Zahl (z.B. 85000.00), kein Tausendertrenner
- null für fehlende/unleserliche Felder
- Nicht raten – lieber null als falscher Wert
- Wenn das Bild kein Lohnausweis ist: {"error": "Kein Lohnausweis erkannt"}
```

**Prompt for Kontoauszug:**

```
Du analysierst einen Schweizer Bankauszug oder eine Kontoübersicht.
Extrahiere die steuerlich relevanten Daten.

Antworte NUR mit JSON:

{
  "stichtag": string,
  "konten": [
    {
      "bank": string,
      "kontonummer": string | null,
      "iban": string | null,
      "bezeichnung": string,
      "waehrung": string,
      "saldo": number,
      "zinsertrag": number | null,
      "verrechnungssteuer": number | null
    }
  ],
  "konfidenz": {
    "gesamt": "hoch" | "mittel" | "tief",
    "unsichere_felder": string[]
  }
}
```

**Prompt for Säule 3a:**

```
Du analysierst eine Schweizer Säule-3a-Einzahlungsbescheinigung.

Antworte NUR mit JSON:

{
  "institut": string,
  "steuerjahr": number,
  "einzahlung": number,
  "art": "bankkonto" | "versicherung" | "wertschriften",
  "saldo_jahresende": number | null,
  "konfidenz": {
    "gesamt": "hoch" | "mittel" | "tief",
    "unsichere_felder": string[]
  }
}
```

**Response parsing:**
- Extract text from `response.content[].text` blocks
- If response contains ` ```json ... ``` ` — strip those markers first
- Parse as JSON
- Map snake_case JSON keys to your struct fields

**Error response from Claude:** If Claude returns `{"error": "..."}`, show an error message to the user and allow re-upload.

### 8.2 Optimisation Suggestions

**Endpoint:** `POST /api/optimize`

**Request body:** Complete `Steuerfall` JSON

**Request to Claude:**

```
POST to Anthropic Messages API:
  model: "claude-sonnet-4-5-20250929"
  max_tokens: 2000
  system: <system prompt — see below>
  messages: [{
    role: "user",
    content: "Steuerdaten (Kanton SG, Steuerperiode {Jahr}):\n{steuerfall as JSON}"
  }]
```

**System prompt:**

```
Du bist ein erfahrener Schweizer Steuerberater, spezialisiert auf den Kanton St. Gallen.
Analysiere die Steuersituation und gib konkrete, legale Optimierungsvorschläge.

Regeln:
1. Nur LEGALE Optimierungen
2. Konkrete CHF-Beträge wo möglich
3. Max. 5 Vorschläge, nach Sparpotenzial sortiert
4. Schweizer Hochdeutsch (kein ß)
5. Einfach verständlich

Antworte NUR mit einem JSON-Array:
[
  {
    "titel": string,
    "beschreibung": string,
    "sparpotenzial_min": number | null,
    "sparpotenzial_max": number | null,
    "aufwand": "gering" | "mittel" | "hoch",
    "zeitrahmen": "sofort" | "naechstes_jahr" | "langfristig",
    "kategorie": "vorsorge" | "berufskosten" | "versicherung" | "timing" | "spenden" | "sonstiges",
    "gesetzliche_grundlage": string
  }
]
```

**Response:** Parse JSON array → list of `Optimierung` structs. Map `sparpotenzial_min`/`sparpotenzial_max` → nullable floats.

---

## 9. PDF Export

**Route:** `GET /api/export/pdf`

**Response:** `Content-Type: application/pdf`, `Content-Disposition: attachment; filename="SteuerPilot-SG-{Steuerperiode}-{Nachname}.pdf"`

**PDF content (A4, portrait):**

**Header block:**
- Title "SteuerPilot SG" in blue (#1E40AF)
- Name: `{Vorname} {Nachname}`
- Gemeinde: `{Gemeinde} · Steuerperiode {Jahr}`
- Date generated: "Erstellt am {date}"

**Section: Einkommen** (table)
- Gesamteinkommen | CHF {X}
- Abzüge | CHF {X}
- Steuerbares Einkommen (Kanton) | CHF {X}
- Steuerbares Einkommen (Bund) | CHF {X}

**Section: Vermögen** (table)
- Bruttovermögen | CHF {X}
- Schulden | CHF {X}
- Steuerbares Vermögen | CHF {X}

**Section: Steuerberechnung** (table)
- Each tax component with amount
- Total Steuer highlighted in blue

**Section: Optimierungsvorschläge** (list, up to 5)
For each suggestion:
- Title (bold)
- Category tag
- Savings potential in green
- Description paragraph

**Footer:**
- Disclaimer text (1 small font)
- "Für die offizielle Einreichung: E-Tax SG, www.steuerverwaltung.sg.ch"

Recommended library for Go: `github.com/jung-kurt/gofpdf` or `github.com/go-pdf/fpdf`.

---

## 10. Validation Rules

### 10.1 File Upload

| Rule | Value |
|---|---|
| Max file size | 10 MB (10 × 1024 × 1024 bytes) |
| Allowed types | image/jpeg, image/png, image/webp, application/pdf |
| Required document | Lohnausweis (app warns if missing, does not block) |

### 10.2 Tax Calculation Limits

| Deduction | Canton Max | Federal Max |
|---|---|---|
| Fahrkosten ÖV/Auto | CHF 4'595 | CHF 3'000 |
| Verpflegung (normal) | CHF 15/Tag, max CHF 3'200 | same |
| Verpflegung (Kantine) | CHF 7.50/Tag, max CHF 1'600 | same |
| Übrige Berufskosten | 3% Nettolohn, min 2'000, max 4'000 | same |
| Säule 3a (mit PK) | CHF 7'056 | same |
| Säule 3a (ohne PK) | min(CHF 35'280, 20% Nettolohn) | same |
| Versicherungen alleinstehend (mit PK) | CHF 3'400 | CHF 1'800 |
| Versicherungen alleinstehend (ohne PK) | CHF 3'900 | CHF 1'800 |
| Versicherungen verheiratet (mit PK) | CHF 6'700 | CHF 3'700 |
| Versicherungen verheiratet (ohne PK) | CHF 7'700 | CHF 3'700 |
| Zusatz pro Kind Versicherungen | CHF 1'100 | CHF 700 |
| Krankheitskosten | nur über 5% Nettoeinkommen | same |
| Schuldzinsen | CHF 50'000 + Vermögenserträge | same |
| Spenden | 20% Nettoeinkommen | same |
| Weiterbildung | CHF 12'000 | CHF 12'000 |
| Kinderabzug (Vorschule) | CHF 7'500 | CHF 6'700 |
| Kinderabzug (Ausbildung) | CHF 10'600 | CHF 6'700 |

### 10.3 Wizard Form Validation

| Step | Field | Rule |
|---|---|---|
| personalien | Vorname | Non-empty |
| personalien | Nachname | Non-empty |
| personalien | Gemeinde | Must be in municipalities list |
| einkommen | Bruttolohn | ≥ 0 |
| einkommen | Nettolohn | ≥ 0, ≤ Bruttolohn |
| upload | File size | ≤ 10 MB |

---

## 11. UI Behaviour & HTMX Patterns

### 11.1 Core HTMX Interactions

**File upload with progress:**
```html
<input type="file" name="file"
  hx-post="/htmx/extraction/preview"
  hx-target="#extraction-preview"
  hx-indicator="#upload-spinner"
  hx-encoding="multipart/form-data"
  hx-trigger="change">
<div id="upload-spinner" class="htmx-indicator">Wird verarbeitet...</div>
<div id="extraction-preview"></div>
```

**Dynamic child list:**
```html
<div id="kinder-liste">
  <!-- rendered list items -->
</div>
<button hx-post="/htmx/wizard/kind/add"
        hx-target="#kinder-liste"
        hx-swap="innerHTML">
  Kind hinzufügen
</button>
```

**Remove child:**
```html
<button hx-delete="/htmx/wizard/kind/2"
        hx-target="#kinder-liste"
        hx-swap="innerHTML"
        hx-confirm="Kind entfernen?">
  ✕
</button>
```

**Auto-calculating deduction limits (on-blur):**
```html
<input type="number" name="saeule3a"
  hx-post="/htmx/wizard/abzuege/calculate"
  hx-target="#saeule3a-limit"
  hx-trigger="blur">
<span id="saeule3a-limit">Max. CHF 7'056</span>
```

**Load optimisations after page load:**
```html
<div id="optimierungen"
  hx-post="/api/optimize"
  hx-trigger="load"
  hx-indicator="#opt-spinner">
  <div id="opt-spinner">Optimierungen werden geladen...</div>
</div>
```

### 11.2 Session Management

Use Fiber's built-in session middleware with cookie-based sessions. Store the entire `Steuerfall` struct serialised as JSON in the session. Session lifetime: 4 hours. On `reset`: delete session and redirect to `/`.

### 11.3 Inline Field Editing (Extraction Preview)

For editable extraction fields, use HTMX out-of-band swaps or a simple click-toggle pattern:

```html
<!-- Display mode -->
<div id="field-bruttolohn" class="editable-field">
  <span class="value">CHF 80'000</span>
  <button hx-get="/htmx/field/bruttolohn/edit"
          hx-target="#field-bruttolohn"
          hx-swap="outerHTML">
    Korrigieren
  </button>
</div>

<!-- Edit mode (returned by server) -->
<div id="field-bruttolohn">
  <input type="number" name="bruttolohn" value="80000"
    hx-post="/htmx/field/bruttolohn/save"
    hx-target="#field-bruttolohn"
    hx-swap="outerHTML"
    hx-trigger="blur, keyup[key=='Enter']">
</div>
```

### 11.4 CHF Formatting

Format all monetary values for display as:
- `CHF 1'234.56` (with apostrophe as thousand separator)
- For rounded integers: `CHF 1'234`

Go implementation:
```go
func FormatCHF(amount float64) string {
    // Format with 2 decimal places
    s := fmt.Sprintf("%.2f", amount)
    // Insert apostrophes as thousand separators in integer part
    // ... standard grouping logic
    return "CHF " + grouped
}
```

---

## 12. Design System

### 12.1 Colors

| Token | Hex | Usage |
|---|---|---|
| `primary` | `#1E40AF` | Headers, primary buttons, active state |
| `primary-light` | `#3B82F6` | Hover states, badges |
| `success` | `#059669` | Savings indicators, positive values |
| `warning` | `#D97706` | Medium confidence, warnings |
| `danger` | `#DC2626` | Low confidence, errors |
| `gray-100` | `#F3F4F6` | Page background |
| `gray-600` | `#4B5563` | Body text |
| `gray-900` | `#111827` | Headings |

### 12.2 Typography

Font: Inter (system sans-serif fallback acceptable). Body: 14px/1.5. Headings: 18–24px bold.

### 12.3 Component Patterns

**Card:** white background, 1px border `#E5E7EB`, `border-radius: 8px`, `padding: 24px`, subtle box-shadow.

**Button — primary:** `background: #1E40AF`, white text, `padding: 10px 20px`, `border-radius: 6px`, hover `#1D3FA0`.

**Button — secondary:** white background, `border: 1px solid #D1D5DB`, gray text, same sizing.

**Badge:** small pill, `padding: 2px 8px`, `font-size: 12px`, `border-radius: 9999px`. Colors match token set.

**Form field:** label above, `margin-bottom: 4px`, `font-weight: 500`. Input: `border: 1px solid #D1D5DB`, `border-radius: 6px`, `padding: 8px 12px`, focus `border-color: #3B82F6` with ring.

**Alert/Disclaimer:** amber background `#FFFBEB`, amber border `#FDE68A`, `padding: 16px`, with AlertTriangle icon.

**Step indicator:** 5 circles, 32×32px. Connected by 2px lines. Colors: completed = `#1E40AF` fill + white ✓; current = white fill + `#1E40AF` 2px border; future = `#E5E7EB` fill.

**Progress bar (DeductionBreakdown):** full-width, 32px height, `border-radius: 4px`. Left segment blue (`#3B82F6`), right segment green (`#059669`). Percentage labels in white inside segments.

**OptimizationCard:** white card, rank circle (40×40px, `#1E40AF` fill, white number). Kategorie badge: green outline. Sparpotenzial: green text with TrendingDown icon.

### 12.4 Mobile Responsiveness

- Step indicator: hide text labels below 640px, show only circles + lines
- Wizard forms: single column on mobile, two-column grid on desktop (≥768px) where appropriate
- Cards: full width on mobile, grid on desktop
- Navigation buttons: full width stacked on mobile, side-by-side on desktop

### 12.5 Legal Disclaimer Text

Must appear on Landing Page and in PDF footer:

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

---

## Appendix: Municipality List (Steuerfüsse 2024)

The complete list of all 121 SG municipalities with their Steuerfuss values is in `docs/steuerparameter.json` under `steuerfuesse.gemeinden`. Abbreviated sample:

| Gemeinde | Steuerfuss |
|---|---|
| Rapperswil-Jona | 80 |
| Wollerau (nicht SG) | — |
| Gommiswald | 103 |
| Buchs | 105 |
| Goldach | 104 |
| Gossau | 107 |
| Wil | 108 |
| Wattwil | 108 |
| Arbon | 112 |
| Rorschach | 123 |
| St. Gallen | 138 |

Use the full JSON as the definitive source.

---

## Appendix: Known Simplifications & Future Work

| Item | Current State | Proper Solution |
|---|---|---|
| Bundessteuer Verheiratete | `single_tax × 0.85` approximation | Load official Tarif B (Verheiratete) table |
| Kinderabzug auf Steuerbetrag | Not implemented | SG reduces Steuerbetrag by a fixed amount per child (check current Steuergesetz SG) |
| Kirchensteuerfuss | Fixed typical value per confession | Per-municipality church rate (varies 20–28%) |
| Nebenwerb | Supported in data model | No UI step yet for additional employers |
| Auth / multi-user | No auth in MVP | NextAuth.js stub or Fiber JWT middleware |
| Session persistence | Memory / cookie only | Redis or SQLite-backed sessions for production |

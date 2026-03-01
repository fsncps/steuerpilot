# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Projektübersicht

**SteuerPilot SG** – KI-gestützte Web-App zur Vorbereitung der Steuererklärung für Privatpersonen im Kanton St. Gallen. Claude Vision wird für Dokumenten-Extraktion verwendet; die Steuerberechnung läuft **immer lokal**.

**MVP-Zielgruppe:** Unselbständig erwerbstätige natürliche Personen im Kanton SG.

**Sprache:** Deutsch (Schweizer Hochdeutsch – kein ß, CHF statt €, `CHF 1'234.56`).

---

## Entwicklungs-Befehle

Alle Befehle im **Repo-Root** ausführen:

```bash
make tools       # Einmalig: templ + air installieren, go mod tidy
make dev         # Dev: templ watch + air hot-reload (parallel)
make build       # templ generate + go build → ./steuerpilot
make build-windows  # Cross-compile → steuerpilot.exe (Windows AMD64)
make run         # build + ausführen
make test        # alle Tests (requires generate)
make test-calc   # nur tax/calculator Tests, schnell, kein Netzwerk
make generate    # nur templ kompilieren
make clean       # generierte Dateien + Binary entfernen
```

**Umgebungsvariablen:** `.env` (aus `.env.example`):
```
ANTHROPIC_API_KEY=sk-ant-xxxxx   # im Dev/Server-Modus required
SESSION_SECRET=                  # required in production (openssl rand -base64 32)
PORT=3000
ENV=development
```

Im Windows-Exe-Modus wird kein `.env` benötigt — der Schlüssel wird beim ersten Start über
die Setup-Seite eingegeben und in `%APPDATA%\SteuerPilot\config.json` gespeichert.

---

## Tech Stack

| Komponente | Technologie |
|---|---|
| Framework | Go 1.23 + Fiber v2 |
| Templates | a-h/templ (compiled, type-safe) |
| State | Server-side Sessions (Fiber session, in-memory) |
| KI | anthropic-sdk-go (Claude Sonnet) |
| PDF | export/pdf.go (`internal/export`) |
| Tests | Go test + testify |
| Dev | air (hot reload) + templ --watch |

---

## Repository-Struktur

```
steuerpilot/                    # ← Repo-Root = Go-Module-Root
├── CLAUDE.md
├── main.go                     # Entry point, alle Routen
├── Makefile
├── go.mod / go.sum
├── config/config.go            # Config struct + env loading (NeedsSetup bool)
├── middleware/
│   ├── session.go              # Session-Store-Middleware
│   └── requiresetup.go         # Redirect → /setup wenn kein API-Key
├── handlers/
│   ├── pages.go                # Landing, Upload, Ergebnis, Setup
│   ├── wizard.go               # WizardStep, WizardSubmit, WizardBack
│   ├── htmx.go                 # HTMX-Partials (Kind, Konto, TaxCalculate)
│   └── api.go                  # /api/upload, /api/optimize, /api/export/pdf, SaveSetup
├── internal/
│   ├── models/
│   │   ├── steuerfall.go       # Steuerfall, Personalien, Einkommen, Abzuege, Vermoegen, Steuerergebnis
│   │   ├── parameter.go        # SteuerparameterDB
│   │   └── extraktion.go       # Extractions-Modelle für Claude Vision
│   ├── tax/
│   │   ├── calculator.go       # BerechneSteuern() – reine Funktion
│   │   ├── parameters.go       # LoadSteuerparameterFromBytes(), GetGemeinden()
│   │   └── calculator_test.go
│   ├── claude/
│   │   ├── client.go           # Init(apiKey), IsInitialized(), globaler Anthropic-Client
│   │   ├── extract.go          # ExtractDocument() – Vision OCR
│   │   └── optimize.go         # GetOptimierungen()
│   ├── userconfig/
│   │   └── config.go           # Load/Save API-Key → APPDATA/~/.config/steuerpilot/
│   ├── session/session.go      # GetSteuerfall/SaveSteuerfall etc. (Fiber ctx)
│   ├── export/pdf.go           # PDF-Generierung
│   └── util/format.go          # FormatCHF(), FormatCHFRund()
├── templates/                  # .templ Quelldateien + *_templ.go (generiert)
│   ├── layout/                 # base.templ, header.templ, footer.templ
│   ├── pages/                  # landing, upload, wizard, ergebnis, setup
│   ├── wizard/                 # personalien, einkommen, abzuege, vermoegen, zusammenfassung
│   ├── components/             # dropzone, formfield, stepindicator, etc.
│   └── partials/               # HTMX-Partials (kindrow, kontorow, taxresult, etc.)
├── docs/
│   ├── steuerparameter.json    # Source of Truth für alle Tarife (go:embed in Binary)
│   └── SPEC.md                 # Fachliche Spezifikation
├── static/                     # Statische Assets
└── archive/                    # Legacy-Daten (nicht Teil der App)
```

---

## Architektur-Kernpunkte

### Routing & Handlers (`main.go`, `handlers/`)
- Fiber v2, alle Routen in `main.go`
- Wizard-Steps via `/wizard/:step` (GET render, POST submit, GET back)
- HTMX-Endpunkte unter `/htmx/...` für dynamische Partials
- `handlers.New(&cfg, params)` – Handler hält `*config.Config` (Pointer, damit SaveSetup den Key setzen kann)

### First-Run Setup-Flow
Wenn kein `ANTHROPIC_API_KEY` gesetzt ist (z.B. Windows-Exe-Modus): `cfg.NeedsSetup = true`, Claude wird nicht initialisiert. Die `RequireSetup`-Middleware leitet alle Requests auf `/setup` um. Nach Eingabe des Keys: `userconfig.Save()` → `claude.Init(key)` → Redirect auf `/`.

### Steuerparameter
Source of Truth: `docs/steuerparameter.json`. Via `//go:embed` in das Binary eingebettet → kein externer Dateizugriff zur Laufzeit. Gemeindesteuerfüsse, Kirchensteuerfüsse, Tarife – **niemals hardcoden**.

### Steuerberechnung (`internal/tax/calculator.go`)
`BerechneSteuern(steuerfall, parameter)` → `Steuerergebnis` – reine Funktion ohne Seiteneffekte.
Ablauf: Einkommen → Abzüge Kanton → Abzüge Bund → Vermögen → Einfache Steuer → Kantons-/Gemeinde-/Kirchensteuer → Bundessteuer → Vermögenssteuer.
Splitting (halbes Einkommen, Satz × 2) für Verheiratete und Geschiedene mit Kindern.

### Session-State (`internal/session/session.go`)
Gesamter Wizard-State (Steuerfall, CurrentStep, ExtractionResult) in Server-seitiger Fiber-Session. Zugriff über `session.GetSteuerfall(c)` / `session.SaveSteuerfall(c, sf)` etc. Kein Client-State.

### Templates (`templates/`)
`.templ`-Dateien müssen vor dem Build mit `templ generate` kompiliert werden → `*_templ.go`. Die generierten Dateien werden committet. `make dev` übernimmt das im Watch-Modus.

### Claude Integration (`internal/claude/`)
- `claude.Init(apiKey)` beim Start (oder nach Setup), globaler Client
- `claude.IsInitialized()` – prüft ob Client bereit ist
- `ExtractDocument()` – Vision OCR, gibt typisiertes Extraktions-JSON zurück
- `GetOptimierungen()` – gibt `[]Optimierung` zurück
- Alle API-Calls nur serverseitig

---

## Bekannte TODOs / offene Punkte

- **Kinderabzug auf Steuerbetrag** (SG-spezifisch): Reduktion des Rechnungsbetrags pro Kind fehlt noch (`calculator.go` TODO-Kommentar)
- **Bundessteuer Verheirateten-Tarif**: Approximation (`alleinstehend × 0.85`), echter dBSt-Tarif ausstehend
- **PDF-Export** (`/api/export/pdf`): Implementierung prüfen

---

## Kritische Regeln

1. **Steuerberechnung IMMER lokal** – Claude nur für OCR + Optimierungstext
2. **Jeden Abzug gegen sein Maximum validieren** – in `calculator.go`
3. **Bundessteuer separat** – eigener Tarif, eigene Limits
4. **Kein Dokument dauerhaft speichern** – nach Extraktion sofort verwerfen
5. **`*_templ.go` nie manuell bearbeiten** – werden von `templ generate` überschrieben

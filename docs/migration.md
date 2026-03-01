# metamorphosis.md

> **SteuerPilot SG — Transformation Roadmap: Next.js → Go + Fiber + templ + HTMX**
>
> This document is the definitive implementation guide for rewriting SteuerPilot SG in Go.
> It covers every architectural decision, the mapping from React/Node concepts to Go equivalents,
> a phased build sequence, and all known gotchas surfaced during the dialectic review.
> The canonical domain spec lives in `SPEC.md`. This document answers *how* to build it in Go.

---

## Table of Contents

1. [Architectural Transformation Map](#1-architectural-transformation-map)
2. [Key Design Decisions](#2-key-design-decisions)
3. [Dependency Manifest](#3-dependency-manifest)
4. [Go Package Structure](#4-go-package-structure)
5. [Implementation Phases](#5-implementation-phases)
6. [Critical Implementation Details](#6-critical-implementation-details)
7. [Known TODOs from the Original](#7-known-todos-from-the-original)
8. [Security Checklist](#8-security-checklist)
9. [Development Commands](#9-development-commands)
10. [Environment Variables](#10-environment-variables)

---

## 1. Architectural Transformation Map

Every major concern in the Next.js app maps to a Go/Fiber/templ equivalent. This section is the core of the metamorphosis.

### 1.1 Runtime & Server Model

| Next.js / Node | Go equivalent | Notes |
|---|---|---|
| Next.js App Router | Fiber v2 router | Fiber uses Express-like `app.Get/Post/Delete`. Groups for `/wizard`, `/htmx`, `/api` |
| Serverless API Routes (`/app/api/*/route.ts`) | Fiber handlers in `handlers/api.go` | Long-running Go process, not serverless. No cold start. |
| `npm run dev` | `make dev` (`air` + `templ generate --watch`) | `air` provides hot reload equivalent |
| Node.js runtime | Single self-contained binary | `go build` → one binary, no runtime deps |
| `next build` / `.next/` | `go build -o steuerpilot .` | No build cache folder; binary is the artefact |

### 1.2 State Management

This is the most fundamental structural change. React's client-side store disappears entirely.

| Next.js / Zustand | Go / Fiber Sessions | Notes |
|---|---|---|
| `useTaxStore()` (client-side Zustand) | `session.GetSteuerfall(c)` (server-side) | `Steuerfall` lives in the server session, keyed by cookie |
| `taxStore.setHaupterwerb(...)` | `session.SaveSteuerfall(c, sf)` | Every form POST reads, mutates, then saves the session struct |
| `getSteuerfall()` builds the object | `Steuerfall` is always the complete struct | No assembly step; the struct is always complete |
| Client persists across tab switches | Session re-renders from server on every GET | **See §6.1 for save-on-blur pattern to avoid data loss** |
| Zustand slice actions (`setAbzuege`, etc.) | Direct field mutation on the struct | Go: `sf.Abzuege.Saeule3a = v` |
| WizardStep type in store | `session.CurrentStep` string field | Stored alongside `Steuerfall` in session |

**Session storage:** The full `Steuerfall` is JSON-encoded and stored server-side (in-memory store). A signed session cookie holds only the session ID. This avoids the 4 KB cookie limit. See §6.2.

### 1.3 Templating & Components

| Next.js / React | Go / templ | Notes |
|---|---|---|
| `.tsx` files with JSX | `.templ` files with templ syntax | Compile-time type-checked; errors are build errors, not runtime panics |
| `export default function Page()` | `templ PageName(data Model)` | Each templ component is a typed Go function |
| `export function ComponentName()` | `templ ComponentName(args...)` | Reusable; import like any Go package |
| `shadcn/ui` (Radix + Tailwind) | Hand-written templ components + Tailwind CDN | No component library dep; simpler, fully owned |
| `{condition && <Element/>}` | `if condition { @Element() }` | templ uses Go control flow directly |
| `{list.map(item => <Row item={item}/>)}` | `for _, item := range list { @Row(item) }` | Same idea, Go syntax |
| `className="..."` | `class="..."` | Standard HTML in templ |
| `style={{ width: pct + '%' }}` | `style={ fmt.Sprintf("width:%.1f%%", pct) }` | Inline style for dynamic values (DeductionBreakdown bar) |
| React `useState` for edit toggle | HTMX swap of display/edit HTML fragments | Server renders both modes; "Korrigieren" fetches the edit fragment |
| `formatCHF()` imported in components | `util.FormatCHF()` imported in templ files | Shared helper in `internal/util/format.go` |

### 1.4 Forms & Validation

| Next.js | Go | Notes |
|---|---|---|
| `react-hook-form` + `zod` schema | `c.FormValue()` / `c.MultipartForm()` + Go validation | Server parses POST body; validation errors returned as HTML fragment |
| `zod.parse()` at form submit | Manual validation in handler, inline error messages | Keep simple: just required field checks + range checks per SPEC §10 |
| Client-side real-time validation | On-blur HTMX call returns computed value/limit display | e.g. Säule-3a limit recomputed server-side, injected into `#saeule3a-limit` |
| Form state retained on back-nav | GET `/wizard/{prev}` re-renders from session | Data already saved; no stale form state |

### 1.5 API Routes & Claude Integration

| Next.js | Go | Notes |
|---|---|---|
| `app/api/extract/route.ts` | `handlers/api.go` → `POST /htmx/extraction/preview` | Returns HTML fragment (extraction preview) directly |
| `app/api/optimize/route.ts` | `handlers/api.go` → `POST /api/optimize` | Returns HTML fragment (list of OptimizationCards) |
| `@anthropic-ai/sdk` (Node) | `github.com/anthropics/anthropic-sdk-go` | Official Go SDK; same API surface |
| `extractDocument(base64, mediaType, type)` | `claude.ExtractDocument(ctx, b64, mime, docType)` | In `claude/extract.go` |
| `getOptimierungen(steuerfall)` | `claude.GetOptimierungen(ctx, steuerfall)` | In `claude/optimize.go` |
| PDF: `image` type in API content | PDF: must use `"document"` type, not `"image"` | **Critical distinction — see §6.3** |

### 1.6 Client-Side Reactivity → HTMX

| React pattern | HTMX pattern | Notes |
|---|---|---|
| `onChange` handler updates state | `hx-trigger="change"` posts to server | Server recalculates, returns fragment |
| Conditional render based on state | HTMX swap of server-rendered fragment | e.g. Fahrkosten radio → server returns relevant sub-form |
| Dynamic list (add/remove) | `hx-post="/htmx/wizard/kind/add"` + `hx-swap="innerHTML"` | Server returns full updated list |
| Loading spinner during async | `hx-indicator="#spinner"` + CSS `.htmx-indicator` class | Shown during in-flight requests |
| Lazy load on mount (`useEffect`) | `hx-trigger="load"` | Optimierungen panel on `/ergebnis` |
| SPA navigation | Standard full-page navigation | Each wizard step is a full page GET |
| Toast / error notification | `HX-Trigger` response header with OOB swap | Or simple inline error div |

### 1.7 PDF Generation

| Next.js | Go | Notes |
|---|---|---|
| `@react-pdf/renderer` (React tree → PDF) | `github.com/go-pdf/fpdf` | Pure Go; no CGO; no headless browser |
| Rendered in browser / edge function | Streamed from server on `GET /api/export/pdf` | `c.Set("Content-Disposition", "attachment; ...")` |
| JSX-based layout | Imperative `fpdf` calls (CellFormat, MultiCell, etc.) | More verbose but deterministic |

### 1.8 Static Assets

| Next.js | Go / Fiber | Notes |
|---|---|---|
| `public/` folder | `static/` served via `app.Static("/", "./static")` | Favicon, any images |
| Tailwind PostCSS pipeline | Tailwind CDN `<script>` in base layout | No build step required |
| `next/font` | Standard `<link rel="preconnect">` + Google Fonts or system font | Inter via CDN or system sans-serif |

---

## 2. Key Design Decisions

### 2.1 templ over `html/template`

**Decision:** Use `github.com/a-h/templ`.

**Why:** `html/template` errors surface at runtime when the template is rendered — a mistyped field name compiles fine but panics in production. `templ` generates Go code from `.templ` files; any type error (wrong field name, wrong argument type) is a compile error. This is the single biggest quality-of-life improvement for a project with deeply nested domain structs like `Steuerfall`.

**Trade-off:** Requires `templ generate` before `go build`. Add to Makefile and CI. The `templ` binary must be installed (`go install github.com/a-h/templ/cmd/templ@latest`).

### 2.2 Fiber v2 over net/http or chi

**Decision:** Use `github.com/gofiber/fiber/v2`.

**Why:** Built-in session middleware, multipart form handling, middleware ecosystem (CSRF, rate limiter, logger) all configured with minimal code. The Express-like API reduces cognitive overhead. Fiber v3 is still in development and has breaking changes; v2 is stable and production-proven.

**Note:** Fiber uses `fasthttp` under the hood, not `net/http`. This means `http.Handler` adapters are needed if using net/http-based middleware. Prefer Fiber-native middleware.

### 2.3 Server-side Sessions (in-memory store, not cookies)

**Decision:** Use Fiber session middleware with **in-memory server-side store**, not cookie store.

**Why:** A full `Steuerfall` with multiple Nebenerwerb, Kinder, and Bankkonto serialises to well over the 4 KB browser cookie limit. Fiber's default `CookieStore` will silently fail or error. Instead:

```go
import "github.com/gofiber/fiber/v2/middleware/session"
import "github.com/gofiber/storage/memory"

store := session.New(session.Config{
    Storage:    memory.New(),
    Expiration: 4 * time.Hour,
    KeyLookup:  "cookie:session_id",
    CookieSecure:   true,
    CookieHTTPOnly: true,
    CookieSameSite: "Strict",
})
```

The session cookie holds only the session ID (a UUID). The `Steuerfall` JSON lives server-side in memory. For a single-instance deployment, this is perfectly sufficient.

**Single-instance note:** This design requires sticky sessions if ever scaled horizontally. For the MVP (local or single-server), this is not a concern.

### 2.4 Tailwind via CDN

**Decision:** Load Tailwind from the official CDN in the base layout template.

**Why:** Zero build tooling. No `npm`, no `postcss.config.js`, no build step. For a focused tool used by one or a few users, the CDN overhead (~100 KB) is irrelevant. The CDN version supports all Tailwind utilities dynamically.

```html
<script src="https://cdn.tailwindcss.com"></script>
```

Add Tailwind config inline in the script tag if custom colors are needed (matching the design system from SPEC §12).

### 2.5 go-pdf/fpdf for PDF

**Decision:** Use `github.com/go-pdf/fpdf` (the maintained fork of the archived `jung-kurt/gofpdf`).

**Why:** Pure Go, no CGO, no external dependencies (no headless browser, no Chromium). Produces A4 PDFs imperatively. The output is a streaming `io.Writer` which Fiber can pipe directly to the response.

**Note:** Do NOT use `github.com/jung-kurt/gofpdf` — it is archived and unmaintained. The correct import path is `github.com/go-pdf/fpdf`.

### 2.6 anthropic-sdk-go

**Decision:** Use `github.com/anthropics/anthropic-sdk-go` (official SDK, v1.x as of 2025).

**Why:** Official, maintained by Anthropic. Handles authentication, retries, and streaming. Wraps the same REST API as the Node.js SDK.

---

## 3. Dependency Manifest

```
github.com/gofiber/fiber/v2          v2.52.x   // HTTP framework
github.com/gofiber/storage/memory    v1.x      // In-memory session store
github.com/a-h/templ                 v0.3.x    // Type-safe HTML templates
github.com/anthropics/anthropic-sdk-go v1.x    // Claude API client
github.com/go-pdf/fpdf               v2.x      // PDF generation
github.com/joho/godotenv             v1.x      // .env loading in dev
github.com/stretchr/testify          v1.x      // Test assertions
```

**Dev tooling (installed globally, not in go.mod):**
```
github.com/a-h/templ/cmd/templ      // templ generate
github.com/air-verse/air            // hot reload
```

---

## 4. Go Package Structure

```
steuerpilot-go/
│
├── main.go                        # Entry: load config, init session store, mount routes, start Fiber
├── go.mod
├── go.sum
├── Makefile
├── .env                           # Dev only (gitignored)
│
├── config/
│   └── config.go                  # Load env vars: PORT, ANTHROPIC_API_KEY, SESSION_SECRET, STEUERPARAMETER_PATH
│
├── internal/
│   │
│   ├── models/                    # All domain structs — the heart of the app
│   │   ├── steuerfall.go          # Steuerfall, Personalien, Kind, Einkommen, Erwerbseinkommen,
│   │   │                          #   Abzuege, Berufskosten, Fahrkosten, Verpflegung,
│   │   │                          #   Vermoegen, Bankkonto, Steuerergebnis, Optimierung
│   │   ├── extraktion.go          # ExtraktionsergebnisLohnausweis, ExtraktionsergebnisKonto,
│   │   │                          #   Extraktion3a, Konfidenz, SessionExtractionResult
│   │   └── parameter.go           # SteuerparameterDB, TarifStufe, Steuerfuesse, Abzugsparameter
│   │
│   ├── tax/                       # Pure calculation logic — no side effects, no HTTP
│   │   ├── calculator.go          # BerechneSteuern(Steuerfall, SteuerparameterDB) → Steuerergebnis
│   │   ├── calculator_test.go     # Unit tests — port the TypeScript test cases directly
│   │   └── parameters.go          # LoadSteuerparameter(path string), GetAlleGemeinden()
│   │
│   ├── claude/                    # Anthropic SDK wrappers — all server-side only
│   │   ├── client.go              # Init anthropic.NewClient(apiKey), shared client instance
│   │   ├── extract.go             # ExtractDocument(ctx, b64 string, mime string, docType string)
│   │   │                          #   → (ExtraktionsergebnisLohnausweis | ..., error)
│   │   └── optimize.go            # GetOptimierungen(ctx, Steuerfall) → ([]Optimierung, error)
│   │
│   ├── session/
│   │   └── session.go             # GetSteuerfall(c), SaveSteuerfall(c, sf), ClearSession(c)
│   │                              # GetCurrentStep(c), SetCurrentStep(c, step)
│   │                              # GetExtractionResult(c), SetExtractionResult(c, r), ClearExtractionResult(c)
│   │
│   ├── util/
│   │   └── format.go              # FormatCHF(float64) string, FormatCHFRound(float64) string
│   │                              # Apostrophe thousand separator: "CHF 1'234.56"
│   │
│   └── export/
│       └── pdf.go                 # GeneratePDF(Steuerfall, Steuerergebnis, []Optimierung) ([]byte, error)
│                                  # Uses go-pdf/fpdf; returns bytes streamed by handler
│
├── handlers/                      # HTTP handlers — thin layer over internal packages
│   ├── pages.go                   # GET /  GET /upload  GET /ergebnis
│   ├── wizard.go                  # GET /wizard/:step  POST /wizard/:step/submit  GET /wizard/:step/back
│   ├── htmx.go                    # All /htmx/* partial handlers (return HTML fragments)
│   └── api.go                     # POST /api/optimize  GET /api/export/pdf
│                                  # POST /api/extraction/accept  POST /api/upload (Claude Vision)
│
├── middleware/
│   └── session.go                 # Fiber middleware: init session store, attach to context
│
├── templates/                     # All .templ files (compiled to _templ.go by `templ generate`)
│   │
│   ├── layout/
│   │   ├── base.templ             # HTML shell: <html>, <head> with Tailwind CDN + HTMX CDN, <body>
│   │   ├── header.templ           # App header with logo
│   │   └── footer.templ           # Disclaimer always visible here
│   │
│   ├── pages/
│   │   ├── landing.templ          # Hero, 3 feature cards, disclaimer, CTA "Jetzt starten"
│   │   ├── upload.templ           # 3 DropZone instances, extraction preview area
│   │   ├── wizard.templ           # Wizard shell: StepIndicator + step content slot + nav buttons
│   │   └── ergebnis.templ         # 3 KPI cards, DeductionBreakdown, tax table, optimizations div, PDF button
│   │
│   ├── wizard/                    # One templ per step (rendered inside wizard shell)
│   │   ├── personalien.templ      # Name, Geburtsdatum, Gemeinde select (121 municipalities), Zivilstand,
│   │   │                          #   Konfession, Kinder list with HTMX add/remove
│   │   ├── einkommen.templ        # Haupterwerb pre-filled, Nebenerwerb section, Weitere Einkünfte,
│   │   │                          #   live Gesamteinkommen total
│   │   ├── abzuege.templ          # Berufskosten (Fahrkosten radio, Verpflegung, Übrige),
│   │   │                          #   Vorsorge, Versicherungen, Krankheitskosten, Weitere Abzüge
│   │   │                          #   — each section shows computed limits inline
│   │   ├── vermoegen.templ        # Bankkonten list (HTMX add/remove), Wertschriften, Übriges,
│   │   │                          #   Schulden, live Reinvermögen display
│   │   └── zusammenfassung.templ  # 3 summary cards (triggers berechneSteuern on render)
│   │
│   ├── components/                # Reusable UI atoms
│   │   ├── stepindicator.templ    # 5-circle step progress bar
│   │   ├── dropzone.templ         # File drop zone with HTMX upload trigger
│   │   ├── extractionpreview.templ # Lohnausweis review panel with editable fields + Konfidenz badge
│   │   ├── optimizationcard.templ # Single AI optimisation suggestion card
│   │   ├── deductionbreakdown.templ # Horizontal bar + deduction table
│   │   └── formfield.templ        # Label + input + optional help tooltip wrapper
│   │
│   └── partials/                  # HTMX swap targets (return HTML fragments, not full pages)
│       ├── kindrow.templ          # Single child row (for add/remove list)
│       ├── kontorow.templ         # Single bank account row
│       ├── fieldeditor.templ      # Display mode vs edit mode for extraction field (Korrigieren pattern)
│       ├── taxresult.templ        # Tax result fragment (swapped by GET /htmx/tax/calculate)
│       └── optimierungen.templ    # Full optimizations list (swapped after HTMX lazy load)
│
├── static/                        # Public static files served by Fiber
│   └── favicon.ico
│
└── docs/
    └── steuerparameter.json       # Copied from Next.js project — source of truth for all tax parameters
```

**Key principle:** `internal/` enforces that `tax/`, `claude/`, `session/`, and `util/` cannot be imported by external packages. `handlers/` is the only place that touches `*fiber.Ctx`. Templates only receive data structs; they never call business logic.

---

## 5. Implementation Phases

Each phase ends with a runnable, testable state.

### Phase 0 — Scaffold & Infrastructure (1–2 days)

**Goal:** Empty app that boots, serves a page, and handles sessions.

- `go mod init steuerpilot-go`
- Install Fiber v2, templ, godotenv
- `config/config.go` — read env vars, panic on missing `ANTHROPIC_API_KEY`
- `internal/models/parameter.go` + `LoadSteuerparameter()` — load JSON at startup, fail fast if missing
- `middleware/session.go` — in-memory session store, attach to all routes
- `templates/layout/base.templ` — HTML shell with Tailwind CDN + HTMX CDN
- `GET /health` → `{"ok": true}` — smoke test
- `GET /` → minimal landing page rendered with templ
- `Makefile` with `generate`, `build`, `run`, `dev`, `test` targets

**Validation:** `make dev` → `curl localhost:3000/health` → `{"ok":true}`

---

### Phase 1 — Data Models & Calculator (2–3 days)

**Goal:** The tax engine works correctly, verified by tests.

- All structs in `internal/models/steuerfall.go` (translate TypeScript types directly — see SPEC §2)
- `internal/models/extraktion.go` — Claude Vision result structs
- `internal/util/format.go` — `FormatCHF()` with apostrophe thousand separator
- `internal/tax/calculator.go` — `BerechneSteuern()` pure function (port from `src-lib-tax-calculator.ts`)
- `internal/tax/calculator_test.go` — port the existing TypeScript test cases:
  - Grundfall: Alleinstehend St. Gallen, CHF 80'000 Bruttolohn
  - Fahrkosten cap at CHF 4'595
  - Säule 3a cap at CHF 7'056 (mit PK)
  - Splitting: Verheiratete pay less than Alleinstehende at same income
  - Vermögenssteuer: CHF 200'000 → steuerbares Vermögen CHF 125'000
  - Gemeindesteuer ratio: St. Gallen (138%) vs Kanton (105%)

**Validation:** `make test-calc` → all tests pass

---

### Phase 2 — Wizard Skeleton (2–3 days)

**Goal:** Navigation through all 6 steps works end-to-end, session state persists.

- `internal/session/session.go` — `GetSteuerfall`, `SaveSteuerfall`, `GetCurrentStep`, `SetCurrentStep`, `ClearSession`
- `handlers/wizard.go` — GET + POST for each of: `personalien`, `einkommen`, `abzuege`, `vermoegen`, `zusammenfassung`
- `templates/components/stepindicator.templ` — 5-circle progress indicator
- `templates/pages/wizard.templ` — shell template; renders StepIndicator + step content + nav buttons
- `templates/wizard/*.templ` — empty scaffolds for all 5 steps (just headings, no real fields yet)
- Navigation: POST `/wizard/:step/submit` saves minimal valid data, redirects to next step
- Back navigation: GET `/wizard/:step/back` re-renders previous step from session

**HTMX redirect pattern:** Step submit returns `c.Redirect(nextURL)` for full-page navigation. For HTMX-triggered POSTs that need to navigate to a new page, use:
```go
if c.Get("HX-Request") == "true" {
    c.Set("HX-Redirect", nextURL)
    return c.SendStatus(200)
}
return c.Redirect(nextURL)
```

**Validation:** Manual walkthrough: fill nothing, click "Weiter" 6 times, arrive at `/ergebnis`.

---

### Phase 3 — Upload & Claude Extraction (2–3 days)

**Goal:** User can upload a Lohnausweis, see extracted values, correct them, and accept into session.

- `handlers/api.go` → `POST /htmx/extraction/preview`:
  1. Parse multipart file from `c.FormFile("file")`
  2. Validate MIME type and size (≤ 10 MB)
  3. Read file bytes → `base64.StdEncoding.EncodeToString(bytes)`
  4. **Drop bytes immediately after encoding** — no file storage
  5. Call `claude.ExtractDocument(ctx, b64, mime, docType)`
  6. Store raw extraction result in session (not yet in Steuerfall)
  7. Return `templates/components/extractionpreview.templ` rendered as HTML fragment

- `internal/claude/client.go` — init Anthropic client once at startup
- `internal/claude/extract.go` — `ExtractDocument()`:
  - Detect MIME: `image/jpeg`, `image/png`, `image/webp` → use `"image"` content block type
  - `application/pdf` → use `"document"` content block type (**not** `"image"`)
  - Use prompts exactly as specified in SPEC §8.1
  - Strip ` ```json ... ``` ` markers from response before JSON parsing

- `templates/components/extractionpreview.templ` — Konfidenz badge, employer block, editable fields
- `templates/partials/fieldeditor.templ` — display mode / edit mode toggle via HTMX outerHTML swap

- `POST /api/extraction/accept` — reads edited values from form POST body, merges into session `Steuerfall.Haupterwerb`, sets `Extracted: true`, redirects to `/wizard/personalien`

- `templates/components/dropzone.templ` — dashed border drop zone, file input with HTMX attributes:
  ```html
  hx-post="/htmx/extraction/preview"
  hx-target="#extraction-preview-lohnausweis"
  hx-encoding="multipart/form-data"
  hx-indicator="#upload-spinner-lohnausweis"
  hx-trigger="change"
  ```

**Validation:** Upload a real Lohnausweis PNG → see extracted values → correct Bruttolohn → click "Übernehmen" → redirected to wizard, session contains correct Haupterwerb data.

---

### Phase 4 — Wizard Forms (3–4 days)

**Goal:** All 5 wizard steps fully functional with live limit calculations.

**Personalien step:**
- All fields with pre-fill from session
- Gemeinde `<select>` populated from `tax.GetAlleGemeinden()` (121 municipalities, alphabetically sorted)
- HTMX Kinder list: `POST /htmx/wizard/kind/add` → append empty `kindrow.templ`; `DELETE /htmx/wizard/kind/:i` → remove and return updated list
- Validation: Vorname + Nachname + Gemeinde required (return 422 + error fragment if missing)

**Einkommen step:**
- Pre-filled from session (Lohnausweis extraction data)
- On-blur HTMX call updates live "Gesamteinkommen: CHF X" total
- Nebenerwerb: HTMX add/remove pattern same as Kinder

**Abzüge step (most complex):**
- Fahrkosten radio → on-change HTMX swap shows relevant sub-form (ÖV cost input / km + days / Velo flat)
- On-blur calls `GET /htmx/tax/calculate` to refresh limit display:
  - Säule 3a: checks `BvgBeitraege > 0` in session → shows CHF 7'056 or CHF 35'280
  - Versicherungen: depends on Zivilstand + PK status
  - Krankheitskosten 5% threshold: shown dynamically
- All limits shown next to their input field as `<span class="text-xs text-gray-500">Max. CHF X</span>`

**Vermögen step:**
- Bankkonten: HTMX add/remove list (`kontorow.templ`)
- Live Reinvermögen: on-blur updates `#reinvermoegen-display`

**Zusammenfassung step:**
- Server calls `BerechneSteuern(sf, params)` when this step is rendered (on GET)
- Stores `Steuerergebnis` in session
- Shows 3 summary cards (see SPEC §6.3.5)
- "Ergebnis anzeigen" → `GET /ergebnis`

**Validation:** Full end-to-end with real data: upload → wizard → Zusammenfassung shows correct totals matching SPEC test case.

---

### Phase 5 — Results & Optimisierungen (2–3 days)

**Goal:** Complete `/ergebnis` page with all sections.

- `handlers/pages.go` → `GET /ergebnis` — reads `Steuerergebnis` from session; if missing, redirect to `/wizard/zusammenfassung`
- `templates/pages/ergebnis.templ`:
  - 3 KPI cards
  - `DeductionBreakdown` component (bar + table)
  - Tax breakdown table
  - Optimierungen div with `hx-trigger="load"` and `hx-post="/api/optimize"`:
    ```html
    <div id="optimierungen"
         hx-post="/api/optimize"
         hx-trigger="load"
         hx-target="#optimierungen"
         hx-swap="innerHTML"
         hx-indicator="#opt-spinner">
    ```
- `internal/claude/optimize.go` → `GetOptimierungen(ctx, Steuerfall)`:
  - System prompt + user message exactly as in SPEC §8.2
  - Parse response JSON array → `[]Optimierung`
- `POST /api/optimize` → calls `GetOptimierungen`, returns `templates/partials/optimierungen.templ`
- `templates/components/optimizationcard.templ` — ranked card with badge, Sparpotenzial, etc.
- `templates/components/deductionbreakdown.templ` — bar with `style={fmt.Sprintf("width:%.1f%%", pct)}` for dynamic widths
- "Neue Berechnung" button → `POST /api/reset` → `session.ClearSession(c)` → `HX-Redirect: /`

**Validation:** Full flow → `/ergebnis` → Optimierungen load after page → 5 cards visible.

---

### Phase 6 — PDF Export (1–2 days)

**Goal:** "PDF exportieren" button triggers file download.

- `internal/export/pdf.go` — `GeneratePDF(sf Steuerfall, ergebnis Steuerergebnis, opt []Optimierung) ([]byte, error)`:
  - Uses `github.com/go-pdf/fpdf`
  - A4 portrait, margins 20mm
  - Header block: "SteuerPilot SG" in `#1E40AF`, name, Gemeinde, date
  - Einkommen table, Vermögen table, Steuerberechnung table (Total in blue, bold)
  - Optimierungen list (up to 5)
  - Footer: disclaimer text + "Für die offizielle Einreichung: E-Tax SG"

- `handlers/api.go` → `GET /api/export/pdf`:
  - Reads `Steuerfall` + `Steuerergebnis` + `Optimierungen` from session
  - Calls `export.GeneratePDF(...)`
  - `c.Set("Content-Type", "application/pdf")`
  - `c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"SteuerPilot-SG-%d-%s.pdf\"", year, nachname))`
  - `c.Send(pdfBytes)`

**Validation:** Click PDF export → browser downloads a valid, readable PDF with correct data.

---

### Phase 7 — Polish, TODOs & Hardening (1–2 days)

**Goal:** Production-ready for personal use. Fix known shortcomings.

- **Kinderabzug auf Steuerbetrag** (see §7.1): implement the SG-specific per-child reduction on `Kantonssteuer` + `Gemeindesteuer` (not on income)
- **Bundessteuer married tariff** (see §7.2): replace ×0.85 approximation with proper Tarif B table; label the approximation clearly in UI until done
- **CSRF middleware**: add `csrf.New()` to all state-mutating routes with HTMX-compatible header lookup (see §6.4)
- **Error pages**: `404.templ`, `500.templ`, session-expired redirect to `/` with explanatory message
- **Rate limiting on Claude routes**: max 10 Claude calls per session per hour (see §6.5)
- **Disclaimer**: ensure it appears on every page (footer component, always rendered in base layout)
- **Mobile responsiveness**: test wizard forms on narrow viewport; single-column layout below 640px
- **Session expired mid-wizard**: if `GetSteuerfall` returns empty session, redirect to `/` with `?expired=1` query param → show banner

---

## 6. Critical Implementation Details

### 6.1 Save-on-Blur vs Submit — Avoiding Data Loss

In React/Zustand, typing in a field updates state immediately. In HTMX, data only reaches the server when the form is submitted (or when an explicit HTMX trigger fires).

**Problem:** User fills in Einkommen step, clicks browser back without clicking "Weiter" → data lost.

**Solution options (pick one):**

A. **Warn on unload** (simplest): Add `onbeforeunload` JS warning: "Nicht gespeicherte Änderungen. Trotzdem verlassen?" This requires a small inline `<script>` on each wizard step that sets a dirty flag on any input change and clears it on form submit.

B. **Auto-save on blur** (better UX): Key numeric fields post their value on blur: `hx-trigger="blur" hx-post="/htmx/wizard/:step/autosave"` — server updates just that field in session and returns empty 200. No visible feedback needed. Implement for the Abzüge step (most fields) as it's the step users most likely interrupt.

**Recommendation:** Implement B for numeric fields, A as a safety net for the entire wizard page.

### 6.2 Session Architecture Detail

```go
// internal/session/session.go

type SessionData struct {
    Steuerfall       models.Steuerfall
    CurrentStep      string
    ExtractionResult *models.SessionExtractionResult
    UploadedFiles    []models.UploadedFile
}

func GetSteuerfall(c *fiber.Ctx) (models.Steuerfall, error) {
    sess, _ := store.Get(c)
    raw := sess.Get("data")
    if raw == nil {
        return models.NewDefaultSteuerfall(), nil
    }
    var sd SessionData
    json.Unmarshal(raw.([]byte), &sd)
    return sd.Steuerfall, nil
}
```

The session stores a single `SessionData` struct as JSON bytes. The whole struct is read and written atomically — no partial updates.

### 6.3 Claude API: PDF vs Image Content Type

The Anthropic Messages API requires different content block types depending on the file:

```go
// claude/extract.go

func contentBlockForFile(b64 string, mime string) anthropic.ContentBlockParam {
    if mime == "application/pdf" {
        // PDF: use "document" type
        return anthropic.NewDocumentBlockParam(
            anthropic.Base64PDFSourceParam{
                Type:      "base64",
                MediaType: "application/pdf",
                Data:      b64,
            },
        )
    }
    // Images: use "image" type
    return anthropic.NewImageBlockParam(
        anthropic.Base64ImageSourceParam{
            Type:      "base64",
            MediaType: anthropic.ImageBlockParamSourceBase64MediaType(mime),
            Data:      b64,
        },
    )
}
```

Mixing these up is a silent failure — Claude receives a malformed request and returns an error. Always branch on MIME type before constructing the content block.

### 6.4 CSRF Protection with HTMX

Fiber's CSRF middleware works with HTMX if configured to look for the token in a request header that HTMX sends automatically:

```go
// main.go
app.Use(csrf.New(csrf.Config{
    KeyLookup:      "header:X-CSRF-Token",
    CookieName:     "csrf_",
    CookieSecure:   true,
    CookieHTTPOnly: true,
}))
```

In the base layout, inject the CSRF token into HTMX's default header configuration:

```html
<script>
document.addEventListener('htmx:configRequest', (e) => {
    e.detail.headers['X-CSRF-Token'] =
        document.cookie.match(/csrf_=([^;]+)/)?.[1] ?? '';
});
</script>
```

This way every HTMX POST/DELETE automatically includes the CSRF token. Static GET routes need no token.

### 6.5 Rate Limiting Claude Calls

Claude calls are expensive. Add a per-session counter stored in session:

```go
// handlers/api.go
const MaxClaudeCallsPerSession = 10

func checkRateLimit(c *fiber.Ctx, store *session.Store) error {
    sess, _ := store.Get(c)
    count, _ := sess.Get("claude_calls").(int)
    if count >= MaxClaudeCallsPerSession {
        return c.Status(429).SendString("Zu viele Anfragen. Bitte laden Sie die Seite neu.")
    }
    sess.Set("claude_calls", count+1)
    sess.Save()
    return nil
}
```

Apply before every call to `claude.ExtractDocument()` and `claude.GetOptimierungen()`.

### 6.6 HTMX Error Handling

When a server handler returns an error, HTMX by default does nothing with non-2xx responses. To show errors to the user, either:

A. Return HTTP 200 with an error HTML fragment (simplest):
```go
if err != nil {
    return c.Render("partials/error", fiber.Map{"Message": err.Error()})
}
```

B. Use HTMX response events (`HX-Trigger`) to show an out-of-band error toast.

**Recommendation:** Use approach A. Return an error div that replaces the target element. Keep it simple.

### 6.7 CHF Formatting in templ

`util.FormatCHF()` must be importable from templ files. In `base.templ` or each file that needs it:

```go
// templates/components/deductionbreakdown.templ
package components

import "steuerpilot-go/internal/util"

templ DeductionBreakdown(totalEinkommen float64, abzuege float64) {
    <span>{ util.FormatCHF(totalEinkommen) }</span>
}
```

The `util` package must be in `internal/util/` and all templ files that need formatting must import it. Do not duplicate the formatting logic.

### 6.8 Dynamic CSS Widths in DeductionBreakdown

The DeductionBreakdown bar calculates widths as percentages. In templ, inline styles require `fmt.Sprintf`:

```go
// templates/components/deductionbreakdown.templ
package components

import "fmt"

templ DeductionBar(steuerbaresEinkommen float64, abzuege float64, total float64) {
    <div class="h-5 rounded-full overflow-hidden flex bg-gray-100">
        <div class="bg-blue-600 h-full"
             style={ fmt.Sprintf("width:%.1f%%", steuerbaresEinkommen/total*100) }>
        </div>
        <div class="bg-green-500 h-full"
             style={ fmt.Sprintf("width:%.1f%%", abzuege/total*100) }>
        </div>
    </div>
}
```

Guard against division by zero: if `total == 0`, render an empty bar.

---

## 7. Known TODOs from the Original

These issues existed in the Next.js app and must be resolved (not carried over) in the Go rebuild.

### 7.1 Kinderabzug auf Steuerbetrag

**What:** Canton SG applies a per-child reduction directly on the final `Kantonssteuer` and `Gemeindesteuer` amounts — this is separate from the child income deduction (`Sozialabzug`).

**Where to implement:** In `tax/calculator.go`, after computing `Kantonssteuer` and `Gemeindesteuer`:

```go
// TODO: Confirm exact CHF amount per child from official SG tariff tables
// Placeholder: CHF 150 per child
for range steuerfall.Personalien.Kinder {
    kantonssteuer  = math.Max(0, kantonssteuer  - 150)
    gemeindesteuer = math.Max(0, gemeindesteuer - 150)
}
```

**Action required:** Verify the exact per-child reduction amount from the official SG Steuergesetz and `tarif2025.pdf` before implementing.

### 7.2 Bundessteuer Verheirateten-Tarif

**What:** The married federal tax rate uses a completely separate Tarif B (different brackets). The current approximation `alleinstehend × 0.85` underestimates the benefit for high incomes.

**Short-term:** Label clearly in UI: "Bundessteuer (Näherungswert, ×0.85)" with an amber warning badge.

**Long-term:** Add `stufen_verheiratet` to `steuerparameter.json` with the official Tarif B brackets, and implement `berechneBundessteuerVerheiratet()`.

### 7.3 Extraction Layer (Built from Scratch)

The Next.js app never implemented `src/lib/extraction/` (lohnausweis.ts, bankstatement.ts, pillar3a.ts). The Go implementation builds this for the first time in `internal/claude/extract.go`.

The exact prompts are in SPEC §8.1. The JSON key mapping (snake_case from Claude → Go struct fields) must be explicit:

```go
type lohnausweisRaw struct {
    ArbeitgeberName      string   `json:"arbeitgeber_name"`
    ZiffBruttolohn       float64  `json:"ziff8_bruttolohn"`
    ZiffSozialabgaben    float64  `json:"ziff9_sozialabgaben"`
    ZiffBvgOrdentlich    *float64 `json:"ziff10_1_bvg_ordentlich"`
    // ... etc
}
```

Then map to the canonical `ExtraktionsergebnisLohnausweis` struct.

---

## 8. Security Checklist

| Concern | Requirement | How enforced |
|---|---|---|
| API key exposure | Never in client code or logs | Server-side only; loaded from env; never rendered in templates |
| File storage | No uploaded files persisted to disk | Base64 encode in-memory → call Claude → discard bytes. No `os.WriteFile` calls in upload handler. |
| Session secret | Must be cryptographically random | Generate with `openssl rand -base64 32`; fail to start if `SESSION_SECRET` is empty |
| CSRF | All state-mutating routes protected | Fiber CSRF middleware + HTMX header injection (§6.4) |
| Session cookie | Secure + HTTPOnly + SameSite=Strict | Set in session store config |
| Claude call rate | Prevent accidental cost explosion | Per-session counter (§6.5) |
| Input validation | All deductions validated against limits | `BerechneSteuern()` enforces all caps; server also validates form inputs before saving |
| Error messages | No stack traces or internal details to client | Use generic error messages; log details server-side only |

---

## 9. Development Commands

```makefile
# Makefile

.PHONY: generate build run test test-calc dev clean

# Install tools (run once)
tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/air-verse/air@latest

# Generate templ files → *_templ.go
generate:
	templ generate ./templates/...

# Build binary (runs generate first)
build: generate
	go build -o steuerpilot .

# Run built binary
run: build
	./steuerpilot

# Development: watch-mode templ + air hot reload
dev:
	templ generate --watch &
	air

# All tests
test: generate
	go test ./...

# Calculator unit tests only (fast, no network)
test-calc:
	go test ./internal/tax/... -v -run .

# Clean generated files and binary
clean:
	find . -name "*_templ.go" -delete
	rm -f steuerpilot
```

**air config** (`.air.toml`):
```toml
[build]
  cmd = "go build -o ./tmp/steuerpilot ."
  bin = "./tmp/steuerpilot"
  exclude_dir = ["static", "docs", "tmp"]
  include_ext = ["go"]   # templ --watch handles .templ files separately
```

---

## 10. Environment Variables

```bash
# .env (dev only — gitignored)

# Required
ANTHROPIC_API_KEY=sk-ant-xxxxx

# Optional (defaults shown)
PORT=3000
SESSION_SECRET=   # MUST set in prod: openssl rand -base64 32
STEUERPARAMETER_PATH=./docs/steuerparameter.json
ENV=development   # or "production" — disables .env loading in prod
```

In `config/config.go`:
```go
type Config struct {
    Port                  string
    AnthropicAPIKey       string
    SessionSecret         string
    SteuerparameterPath   string
    IsDev                 bool
}

func Load() Config {
    if os.Getenv("ENV") != "production" {
        godotenv.Load()
    }
    key := os.Getenv("ANTHROPIC_API_KEY")
    if key == "" {
        log.Fatal("ANTHROPIC_API_KEY is required")
    }
    secret := os.Getenv("SESSION_SECRET")
    if secret == "" && os.Getenv("ENV") == "production" {
        log.Fatal("SESSION_SECRET is required in production")
    }
    return Config{
        Port:                os.Getenv("PORT"),
        AnthropicAPIKey:     key,
        SessionSecret:       secret,
        SteuerparameterPath: getEnvOrDefault("STEUERPARAMETER_PATH", "./docs/steuerparameter.json"),
        IsDev:               os.Getenv("ENV") != "production",
    }
}
```

---

## Appendix: HTMX CDN

```html
<script src="https://unpkg.com/htmx.org@2.0.4"
        integrity="sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+"
        crossorigin="anonymous"></script>
```

Use HTMX 2.x (not 1.x). The main breaking change from 1.x: `hx-on:*` event syntax. Use HTMX 2 from the start to avoid a migration later.

---

*Document status: Initial draft — synthesised from SPEC.md, existing Next.js implementation, and dialectic review.*
*Last updated: 2026-03-01*

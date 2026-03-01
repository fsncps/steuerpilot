# Repo Restructure + Windows Exe Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move Go app to repo root (canonical layout), embed all assets into the binary, add first-run API key setup, and produce a single `steuerpilot.exe` Robin can double-click.

**Architecture:** Flat repo root = Go module root. `steuerparameter.json` embedded via `go:embed` — zero external file dependencies at runtime. API key persisted to `%APPDATA%\SteuerPilot\config.json` (Windows) or `~/.config/steuerpilot/config.json` (others). Browser opens automatically on startup. No Docker needed.

**Tech Stack:** Go 1.23, Fiber v2, a-h/templ, `go:embed`, `os/exec` for browser launch.

---

### Task 1: Add .gitignore

**Files:**
- Create: `.gitignore`

**Step 1: Create**

```
steuerpilot
steuerpilot.exe
tmp/
.env
.DS_Store
*.swp
.serena/
src/.serena/
```

**Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add .gitignore"
```

---

### Task 2: Move Go app from src/ to repo root

All commands from repo root.

**Step 1: Move Go source**

```bash
git mv src/go.mod .
git mv src/go.sum .
git mv src/main.go .
git mv src/Makefile .
git mv src/.env.example .
git mv src/.air.toml .
git mv src/config .
git mv src/middleware .
git mv src/handlers .
git mv src/internal .
git mv src/templates .
git mv src/static .
```

**Step 2: Move docs**

```bash
git mv src/docs/steuerparameter.json docs/steuerparameter.json
git mv src/docs/SPEC.md docs/SPEC.md
git mv src/docs/migration.md docs/migration.md
git mv src/docs/wegleitung2025.pdf docs/wegleitung2025.pdf
```

**Step 3: Remove src/ debris**

```bash
rm -rf src/.serena src/tmp
rmdir src
git rm setup.sh
```

**Step 4: Verify**

```bash
ls go.mod main.go Makefile .air.toml
ls docs/steuerparameter.json docs/SPEC.md
```

**Step 5: Commit**

```bash
git add -A
git commit -m "chore: move Go app from src/ to repo root"
```

---

### Task 3: Consolidate legacy data into archive/

**Step 1:**

```bash
git mv data archive
git mv legacy_node.tar.xz archive/
```

**Step 2: Verify**

```bash
ls archive/
# Expected: drafts/  etax/  legacy_node.tar.xz  tarif2025.pdf  tarif2025.txt
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: consolidate legacy data into archive/"
```

---

### Task 4: Rename Go module steuerpilot-go → steuerpilot

**Step 1: Update go.mod line 1**

```
module steuerpilot
```

**Step 2: Rename all import paths**

```bash
find . -type f \( -name "*.go" -o -name "*.templ" \) \
  ! -path "./archive/*" \
  | xargs sed -i 's|steuerpilot-go|steuerpilot|g'
```

**Step 3: Regenerate templates**

```bash
make generate
```

Expected: templ outputs one `(✓)` line per template file, no errors.

**Step 4: Verify no old name remains**

```bash
grep -r "steuerpilot-go" --include="*.go" --include="*.templ" .
# Expected: (no output)
head -1 go.mod
# Expected: module steuerpilot
```

**Step 5: Build + test — checkpoint before feature work**

```bash
make build && make test-calc
```

Expected: binary produced, all tests PASS.

**Step 6: Commit**

```bash
git add -A
git commit -m "chore: rename module steuerpilot-go → steuerpilot"
```

---

### Task 5: Embed steuerparameter.json into binary

**Files:**
- Modify: `internal/tax/parameters.go`
- Modify: `main.go`

**Step 1: Add `LoadSteuerparameterFromBytes` to parameters.go**

Add after the existing `LoadSteuerparameter` function:

```go
// LoadSteuerparameterFromBytes parses a JSON byte slice — used with go:embed.
func LoadSteuerparameterFromBytes(data []byte) (models.SteuerparameterDB, error) {
	var params models.SteuerparameterDB
	if err := json.Unmarshal(data, &params); err != nil {
		return models.SteuerparameterDB{}, err
	}
	return params, nil
}
```

**Step 2: Add embed directive and switch to embedded load in main.go**

Add after the `import` block in `main.go`:

```go
import "embed"

//go:embed docs/steuerparameter.json
var steuerparameterData []byte
```

Replace the `LoadSteuerparameter` call in `main()`:

```go
// Before:
params, err := tax.LoadSteuerparameter(cfg.SteuerparameterPath)

// After:
params, err := tax.LoadSteuerparameterFromBytes(steuerparameterData)
```

Remove `cfg.SteuerparameterPath` from the call (the env var can stay in Config for dev overrides but is no longer used in main).

**Step 3: Build to verify embed works**

```bash
make build
./steuerpilot &
sleep 1 && curl -s http://localhost:3000/health
kill %1
```

Expected: server starts without errors, health returns `{"ok":true}`.

**Step 4: Commit**

```bash
git add internal/tax/parameters.go main.go
git commit -m "feat: embed steuerparameter.json into binary"
```

---

### Task 6: User config persistence package

**Files:**
- Create: `internal/userconfig/config.go`
- Create: `internal/userconfig/config_test.go`

**Step 1: Write the package**

```go
package userconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type UserConfig struct {
	AnthropicAPIKey string `json:"anthropic_api_key"`
}

func configPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "SteuerPilot", "config.json")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "steuerpilot", "config.json")
	}
}

// Load reads the persisted user config. Returns empty config (not error) if file absent.
func Load() (UserConfig, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return UserConfig{}, nil
	}
	if err != nil {
		return UserConfig{}, err
	}
	var cfg UserConfig
	return cfg, json.Unmarshal(data, &cfg)
}

// Save writes the user config, creating the directory if needed.
func Save(cfg UserConfig) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
```

**Step 2: Write tests**

```go
package userconfig

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoad(t *testing.T) {
	// Override config path to temp dir
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	cfg := UserConfig{AnthropicAPIKey: "sk-ant-test123"}
	require.NoError(t, Save(cfg))

	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-test123", loaded.AnthropicAPIKey)
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Empty(t, cfg.AnthropicAPIKey)
}
```

**Step 3: Run tests**

```bash
go test ./internal/userconfig/... -v
```

Expected: both PASS.

**Step 4: Commit**

```bash
git add internal/userconfig/
git commit -m "feat: add user config persistence (APPDATA/~/.config)"
```

---

### Task 7: Graceful startup — no fatal on missing API key

**Files:**
- Modify: `config/config.go`
- Modify: `internal/claude/client.go`
- Modify: `main.go`

**Step 1: Add `NeedsSetup` to Config and soft-load API key in config.go**

```go
type Config struct {
	Port            string
	AnthropicAPIKey string
	SessionSecret   string
	IsDev           bool
	NeedsSetup      bool  // true if no API key found at startup
}
```

Replace the fatal on missing key with a soft check:

```go
key := os.Getenv("ANTHROPIC_API_KEY")
// ...existing code...
return Config{
	Port:            port,
	AnthropicAPIKey: key,
	SessionSecret:   secret,
	IsDev:           os.Getenv("ENV") != "production",
	NeedsSetup:      key == "",
}
```

Remove the `log.Fatal` for missing key entirely — the setup flow handles it.

**Step 2: Add `IsInitialized()` to claude/client.go**

```go
// IsInitialized reports whether the Anthropic client has been configured.
func IsInitialized() bool { return client != nil }
```

**Step 3: Update main.go startup — conditional Init**

Replace:
```go
claude.Init(cfg.AnthropicAPIKey)
```

With:
```go
if !cfg.NeedsSetup {
	claude.Init(cfg.AnthropicAPIKey)
}
```

**Step 4: Pass `cfg` to handler so setup handler can re-init**

`handlers.New(cfg, params)` already receives `cfg` — confirm `Handler` stores `cfg` as a pointer or value. If value, change to pointer so the setup handler can mutate it after saving the key:

In `handlers/api.go` (or wherever `Handler` is defined), ensure:
```go
type Handler struct {
	cfg    *config.Config   // pointer so setup can update AnthropicAPIKey
	params models.SteuerparameterDB
}

func New(cfg *config.Config, params models.SteuerparameterDB) *Handler {
	return &Handler{cfg: cfg, params: params}
}
```

Update `main.go` to pass `&cfg`.

**Step 5: Build to verify**

```bash
# Simulate missing key
ANTHROPIC_API_KEY="" make build
./steuerpilot &
sleep 1 && curl -s http://localhost:3000/health
kill %1
```

Expected: server starts (no fatal), health responds.

**Step 6: Commit**

```bash
git add config/config.go internal/claude/client.go main.go handlers/
git commit -m "feat: graceful startup without API key (NeedsSetup mode)"
```

---

### Task 8: Setup page template + handler

**Files:**
- Create: `templates/pages/setup.templ`
- Modify: `handlers/pages.go` (add Setup handler)
- Modify: `handlers/api.go` (add SaveSetup handler)
- Modify: `main.go` (register routes)

**Step 1: Write setup.templ**

```templ
package pages

templ Setup(errMsg string) {
	<div class="min-h-screen flex items-center justify-center bg-gray-50">
		<div class="max-w-md w-full bg-white rounded-xl shadow p-8">
			<h1 class="text-2xl font-bold text-gray-900 mb-2">Willkommen bei SteuerPilot SG</h1>
			<p class="text-gray-600 mb-6">
				Für die KI-gestützte Dokumenten-Erkennung wird ein Anthropic API-Schlüssel benötigt.
				Sie erhalten einen Schlüssel unter
				<a href="https://console.anthropic.com" class="text-blue-600 underline" target="_blank">
					console.anthropic.com
				</a>.
			</p>
			if errMsg != "" {
				<p class="text-red-600 text-sm mb-4">{ errMsg }</p>
			}
			<form method="POST" action="/setup/save">
				<label class="block text-sm font-medium text-gray-700 mb-1">
					API-Schlüssel <span class="text-gray-400">(beginnt mit sk-ant-…)</span>
				</label>
				<input
					type="password"
					name="api_key"
					placeholder="sk-ant-..."
					required
					class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm mb-4 focus:outline-none focus:ring-2 focus:ring-blue-500"
				/>
				<button
					type="submit"
					class="w-full bg-blue-600 text-white rounded-lg py-2 font-medium hover:bg-blue-700"
				>
					Schlüssel speichern und starten
				</button>
			</form>
		</div>
	</div>
}
```

**Step 2: Add GET /setup handler to handlers/pages.go**

```go
func (h *Handler) Setup(c *fiber.Ctx) error {
	if claude.IsInitialized() {
		return c.Redirect("/")
	}
	return render(c, pages.Setup(""))
}
```

**Step 3: Add POST /setup/save handler to handlers/api.go**

```go
func (h *Handler) SaveSetup(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.FormValue("api_key"))
	if !strings.HasPrefix(key, "sk-ant-") || len(key) < 20 {
		return render(c, pages.Setup("Ungültiger Schlüssel. Er muss mit sk-ant- beginnen."))
	}
	if err := userconfig.Save(userconfig.UserConfig{AnthropicAPIKey: key}); err != nil {
		return render(c, pages.Setup("Fehler beim Speichern: "+err.Error()))
	}
	h.cfg.AnthropicAPIKey = key
	claude.Init(key)
	return c.Redirect("/")
}
```

**Step 4: Register routes in main.go**

```go
app.Get("/setup", h.Setup)
app.Post("/setup/save", h.SaveSetup)
```

**Step 5: Generate templates + build**

```bash
make generate && make build
```

Expected: no errors.

**Step 6: Commit**

```bash
git add templates/pages/setup.templ templates/pages/setup_templ.go \
        handlers/pages.go handlers/api.go main.go
git commit -m "feat: add first-run API key setup page"
```

---

### Task 9: Setup redirect middleware

Redirect all non-setup requests to `/setup` if Claude is not yet initialised.

**Files:**
- Modify: `middleware/session.go` OR create `middleware/requiresetup.go`

**Step 1: Create middleware/requiresetup.go**

```go
package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"steuerpilot/internal/claude"
)

// RequireSetup redirects to /setup if the Anthropic client is not yet initialised.
// Passes through /setup itself and static assets.
func RequireSetup() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if claude.IsInitialized() {
			return c.Next()
		}
		path := c.Path()
		if strings.HasPrefix(path, "/setup") || strings.HasPrefix(path, "/health") {
			return c.Next()
		}
		return c.Redirect("/setup")
	}
}
```

**Step 2: Register in main.go after session middleware**

```go
app.Use(appmiddleware.RequireSetup())
```

**Step 3: Manual smoke test**

```bash
make build
ANTHROPIC_API_KEY="" ./steuerpilot &
sleep 1
curl -si http://localhost:3000/ | head -5
# Expected: HTTP/1.1 302  Location: /setup
curl -si http://localhost:3000/health | head -5
# Expected: HTTP/1.1 200
kill %1
```

**Step 4: Commit**

```bash
git add middleware/requiresetup.go main.go
git commit -m "feat: redirect to /setup when API key not configured"
```

---

### Task 10: Auto-open browser on startup

**Files:**
- Modify: `main.go`

**Step 1: Add openBrowser helper to main.go**

```go
import (
	"os/exec"
	"runtime"
)

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
```

**Step 2: Call it after server binds (in a goroutine)**

```go
url := "http://localhost:" + cfg.Port
go func() {
	time.Sleep(600 * time.Millisecond)
	openBrowser(url)
}()
log.Printf("SteuerPilot SG läuft auf %s", url)
log.Fatal(app.Listen(":" + cfg.Port))
```

**Step 3: Build and verify on Linux**

```bash
make build
./steuerpilot &
sleep 2
# xdg-open will attempt to open browser (may fail in headless env — that's fine)
kill %1
```

Expected: server starts cleanly, no crash.

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat: auto-open browser on startup"
```

---

### Task 11: Version ldflags + Windows build target

**Files:**
- Modify: `Makefile`
- Modify: `main.go`

**Step 1: Add VERSION and ldflags to Makefile, add build-windows target**

```makefile
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

build: generate
	go build $(LDFLAGS) -o steuerpilot .

build-windows: generate
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o steuerpilot.exe .
```

**Step 2: Declare version var and expose on /health in main.go**

```go
var version = "dev"

// in health handler:
return c.JSON(fiber.Map{"ok": true, "version": version})
```

**Step 3: Tag and test**

```bash
git tag v0.1.0
make build
./steuerpilot &
sleep 1 && curl -s http://localhost:3000/health
# Expected: {"ok":true,"version":"v0.1.0"}
kill %1
```

**Step 4: Cross-compile for Windows**

```bash
make build-windows
file steuerpilot.exe
# Expected: PE32+ executable (console) x86-64, for MS Windows
```

**Step 5: Commit**

```bash
git add Makefile main.go
git commit -m "feat: version ldflags + make build-windows"
```

---

### Task 12: README + CLAUDE.md

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

**Step 1: Write README.md**

```markdown
# SteuerPilot SG

KI-gestützte Web-App zur Vorbereitung der Steuererklärung für Privatpersonen im Kanton St. Gallen.

Lohnausweis, Kontoauszug und Säule-3a-Belege hochladen → Claude Vision extrahiert die Daten →
5-Schritt-Wizard → lokale Steuerberechnung → PDF-Export für E-Tax SG.

---

## Für Robin (Windows)

1. `steuerpilot.exe` herunterladen und starten
2. Browser öffnet sich automatisch
3. Anthropic API-Schlüssel eingeben (einmalig)
4. Fertig

---

## Entwicklung

```bash
make tools        # einmalig: templ + air installieren
cp .env.example .env && vi .env   # ANTHROPIC_API_KEY eintragen
make dev          # templ watch + air hot-reload → http://localhost:3000
```

## Build

```bash
make build          # Linux-Binary
make build-windows  # steuerpilot.exe (Windows AMD64)
make test-calc      # Steuerrechner-Unit-Tests
make test           # alle Tests
```

## Umgebungsvariablen (.env)

| Variable | Pflicht | Default |
|---|---|---|
| `ANTHROPIC_API_KEY` | Dev/Server | — |
| `SESSION_SECRET` | Prod | — |
| `PORT` | nein | `3000` |
| `ENV` | nein | `development` |

Im Windows-Exe-Modus wird kein `.env` benötigt — der Schlüssel wird beim ersten Start über
die Setup-Seite eingegeben und in `%APPDATA%\SteuerPilot\config.json` gespeichert.
```

**Step 2: Update CLAUDE.md**

The structure section already reflects the flat layout. Update only the dev commands block — remove any `cd src/` references. Confirm the commands are:

```bash
make tools
make dev
make build
make build-windows
make test-calc
```

Add a note about the first-run setup flow and `NeedsSetup` in the architecture section.

**Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "docs: README and CLAUDE.md for flat structure + Windows exe"
```

---

### Task 13: Final verification

**Step 1: Clean build**

```bash
make clean && make build
```

**Step 2: All tests**

```bash
make test
```

Expected: all PASS.

**Step 3: Windows cross-compile**

```bash
make build-windows
ls -lh steuerpilot.exe
```

Expected: ~20MB single binary, no errors.

**Step 4: Simulate first-run (no key)**

```bash
ANTHROPIC_API_KEY="" ./steuerpilot &
sleep 1
curl -si http://localhost:3000/ | grep Location
# Expected: Location: /setup
curl -si http://localhost:3000/setup | grep "200 OK"
# Expected: HTTP/1.1 200 OK
kill %1
```

**Step 5: Final commit if any loose ends**

```bash
git status
# Should be clean.
```

---

## Summary of commits

1. `chore: add .gitignore`
2. `chore: move Go app from src/ to repo root`
3. `chore: consolidate legacy data into archive/`
4. `chore: rename module steuerpilot-go → steuerpilot`
5. `feat: embed steuerparameter.json into binary`
6. `feat: add user config persistence (APPDATA/~/.config)`
7. `feat: graceful startup without API key (NeedsSetup mode)`
8. `feat: add first-run API key setup page`
9. `feat: redirect to /setup when API key not configured`
10. `feat: auto-open browser on startup`
11. `feat: version ldflags + make build-windows`
12. `docs: README and CLAUDE.md for flat structure + Windows exe`

#!/usr/bin/env bash
# setup.sh — run as root
# Creates the full steuerpilot-go project structure under go_version/,
# seeds every Go and templ file with the correct package stub,
# and chowns everything to fsncps:users.
# Idempotent: existing non-empty files are never overwritten.

set -euo pipefail

OWNER="fsncps:users"
BASE="$(cd "$(dirname "$0")" && pwd)/steuerpilot-go"

# ── helpers ──────────────────────────────────────────────────────────────────

mkd() { mkdir -p "$BASE/$1"; }

# Write content to a file only if it is currently empty (or newly created).
# Usage: stub <relative-path-from-BASE> <content>
stub() {
  local file="$BASE/$1"
  mkdir -p "$(dirname "$file")"
  touch "$file"
  if [ ! -s "$file" ]; then
    printf '%s\n' "$2" > "$file"
  fi
}

echo "→ Creating directories..."

mkd config
mkd internal/models
mkd internal/tax
mkd internal/claude
mkd internal/session
mkd internal/util
mkd internal/export
mkd handlers
mkd middleware
mkd templates/layout
mkd templates/pages
mkd templates/wizard
mkd templates/components
mkd templates/partials
mkd static
mkd docs
mkd tmp

# ── root files ────────────────────────────────────────────────────────────────

echo "→ Writing root files..."

stub go.mod 'module steuerpilot-go

go 1.23

require (
	github.com/a-h/templ                   v0.3.833
	github.com/anthropics/anthropic-sdk-go v1.2.0
	github.com/go-pdf/fpdf                 v2.7.0+incompatible
	github.com/gofiber/fiber/v2            v2.52.6
	github.com/gofiber/storage/memory      v1.3.4
	github.com/joho/godotenv               v1.5.1
	github.com/stretchr/testify            v1.10.0
)'

stub .env.example '# Required
ANTHROPIC_API_KEY=sk-ant-xxxxx

# Optional (defaults shown)
PORT=3000
ENV=development

# Generate with: openssl rand -base64 32
# Required in production (app refuses to start if empty)
SESSION_SECRET=

STEUERPARAMETER_PATH=./docs/steuerparameter.json'

stub .air.toml '[build]
  cmd = "go build -o ./tmp/steuerpilot ."
  bin = "./tmp/steuerpilot"
  exclude_dir = ["static", "docs", "tmp", "templates"]
  include_ext = ["go"]
  delay = 500

[log]
  time = false

[color]
  app = "cyan"'

stub Makefile '.PHONY: tools generate build run test test-calc dev clean

tools:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/air-verse/air@latest

generate:
	templ generate ./templates/...

build: generate
	go build -o steuerpilot .

run: build
	./steuerpilot

dev:
	templ generate --watch &
	air

test: generate
	go test ./...

test-calc:
	go test ./internal/tax/... -v -run .

clean:
	find . -name "*_templ.go" -delete
	rm -f steuerpilot
	rm -rf tmp/'

stub main.go 'package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"steuerpilot-go/config"
	"steuerpilot-go/handlers"
	"steuerpilot-go/internal/tax"
	appmiddleware "steuerpilot-go/middleware"
)

func main() {
	cfg := config.Load()

	params, err := tax.LoadSteuerparameter(cfg.SteuerparameterPath)
	if err != nil {
		log.Fatalf("Steuerparameter konnten nicht geladen werden: %v", err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: handlers.ErrorHandler,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(appmiddleware.Session(cfg))

	app.Static("/", "./static")

	h := handlers.New(cfg, params)

	app.Get("/", h.Landing)
	app.Get("/upload", h.Upload)
	app.Post("/api/upload", h.HandleUpload)
	app.Post("/api/extraction/accept", h.AcceptExtraction)
	app.Post("/api/optimize", h.HandleOptimize)
	app.Get("/api/export/pdf", h.ExportPDF)
	app.Post("/api/reset", h.Reset)

	wizard := app.Group("/wizard")
	wizard.Get("/:step", h.WizardStep)
	wizard.Post("/:step/submit", h.WizardSubmit)
	wizard.Get("/:step/back", h.WizardBack)

	htmx := app.Group("/htmx")
	htmx.Post("/extraction/preview", h.ExtractionPreview)
	htmx.Post("/wizard/kind/add", h.KindAdd)
	htmx.Delete("/wizard/kind/:i", h.KindRemove)
	htmx.Post("/wizard/konto/add", h.KontoAdd)
	htmx.Delete("/wizard/konto/:i", h.KontoRemove)
	htmx.Get("/tax/calculate", h.TaxCalculate)

	app.Get("/ergebnis", h.Ergebnis)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	log.Printf("SteuerPilot SG läuft auf :%s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
}'

# ── Go source stubs ───────────────────────────────────────────────────────────

echo "→ Writing Go package stubs..."

stub config/config.go 'package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	AnthropicAPIKey     string
	SessionSecret       string
	SteuerparameterPath string
	IsDev               bool
}

func Load() Config {
	if os.Getenv("ENV") != "production" {
		_ = godotenv.Load()
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" && os.Getenv("ENV") == "production" {
		log.Fatal("SESSION_SECRET is required in production")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	path := os.Getenv("STEUERPARAMETER_PATH")
	if path == "" {
		path = "./docs/steuerparameter.json"
	}
	return Config{
		Port:                port,
		AnthropicAPIKey:     key,
		SessionSecret:       secret,
		SteuerparameterPath: path,
		IsDev:               os.Getenv("ENV") != "production",
	}
}'

stub internal/models/steuerfall.go 'package models

// ── Enumerations ──────────────────────────────────────────────────────────────

type Zivilstand string

const (
	ZivilstandAlleinstehend           Zivilstand = "alleinstehend"
	ZivilstandVerheiratet             Zivilstand = "verheiratet"
	ZivilstandGeschieden              Zivilstand = "geschieden"
	ZivilstandVerwitwet               Zivilstand = "verwitwet"
	ZivilstandGetrennt                Zivilstand = "getrennt"
	ZivilstandEingetragenePartnerschaft Zivilstand = "eingetragene_partnerschaft"
)

type Konfession string

const (
	KonfessionEvangelisch     Konfession = "evangelisch"
	KonfessionKatholisch      Konfession = "katholisch"
	KonfessionChristkatholisch Konfession = "christkatholisch"
	KonfessionAndere          Konfession = "andere"
	KonfessionKeine           Konfession = "keine"
)

type FahrkostenArt string

const (
	FahrkostenOev       FahrkostenArt = "oev"
	FahrkostenAuto      FahrkostenArt = "auto"
	FahrkostenMotorrad  FahrkostenArt = "motorrad"
	FahrkostenVelo      FahrkostenArt = "velo"
	FahrkostenKeine     FahrkostenArt = "keine"
)

// ── Top-level ─────────────────────────────────────────────────────────────────

type Steuerfall struct {
	Steuerperiode int             `json:"steuerperiode"`
	Personalien   Personalien     `json:"personalien"`
	Einkommen     Einkommen       `json:"einkommen"`
	Abzuege       Abzuege         `json:"abzuege"`
	Vermoegen     Vermoegen       `json:"vermoegen"`
	Ergebnis      *Steuerergebnis `json:"ergebnis,omitempty"`
	Optimierungen []Optimierung   `json:"optimierungen,omitempty"`
}

type Personalien struct {
	Vorname     string     `json:"vorname"`
	Nachname    string     `json:"nachname"`
	Geburtsdatum string    `json:"geburtsdatum"`
	Zivilstand  Zivilstand `json:"zivilstand"`
	Konfession  Konfession `json:"konfession"`
	Gemeinde    string     `json:"gemeinde"`
	Kinder      []Kind     `json:"kinder"`
	Partner     *Personalien `json:"partner,omitempty"`
}

type Kind struct {
	Vorname         string  `json:"vorname"`
	Geburtsdatum    string  `json:"geburtsdatum"`
	InAusbildung    bool    `json:"inAusbildung"`
	Fremdbetreuung  bool    `json:"fremdbetreuung"`
	Betreuungskosten float64 `json:"betreuungskosten,omitempty"`
}

type Einkommen struct {
	Haupterwerb          Erwerbseinkommen   `json:"haupterwerb"`
	Nebenerwerb          []Erwerbseinkommen `json:"nebenerwerb"`
	WertschriftenErtraege float64           `json:"wertschriftenErtraege"`
	Bankzinsen           float64            `json:"bankzinsen"`
	BeteiligungsErtraege float64            `json:"beteiligungsErtraege"`
	LiegenschaftenEinkuenfte float64        `json:"liegenschaftenEinkuenfte"`
	UebrigeEinkuenfte    float64            `json:"uebrigeEinkuenfte"`
	Renten               float64            `json:"renten"`
	Kinderzulagen        float64            `json:"kinderzulagen"`
}

type Erwerbseinkommen struct {
	Arbeitgeber        string        `json:"arbeitgeber"`
	Bruttolohn         float64       `json:"bruttolohn"`
	Nettolohn          float64       `json:"nettolohn"`
	AhvIvEoAlvNbuv     float64       `json:"ahvIvEoAlvNbuv"`
	BvgOrdentlich      float64       `json:"bvgOrdentlich"`
	BvgEinkauf         float64       `json:"bvgEinkauf"`
	Quellensteuer      float64       `json:"quellensteuer"`
	SpesenEffektiv     float64       `json:"spesenEffektiv"`
	SpesenPauschal     float64       `json:"spesenPauschal"`
	AussendienstProzent float64      `json:"aussendienstProzent"`
	HatGeschaeftsauto  bool          `json:"hatGeschaeftsauto"`
	HatGA              bool          `json:"hatGA"`
	HatKantine         bool          `json:"hatKantine"`
}

type Abzuege struct {
	Berufskosten         Berufskosten `json:"berufskosten"`
	Sozialabgaben        float64      `json:"sozialabgaben"`
	BvgBeitraege         float64      `json:"bvgBeitraege"`
	Saeule3a             float64      `json:"saeule3a"`
	Versicherungspraemien float64     `json:"versicherungspraemien"`
	Krankheitskosten     float64      `json:"krankheitskosten"`
	Schuldzinsen         float64      `json:"schuldzinsen"`
	Unterhaltsbeitraege  float64      `json:"unterhaltsbeitraege"`
	Spenden              float64      `json:"spenden"`
	Weiterbildung        float64      `json:"weiterbildung"`
	Liegenschaftsunterhalt float64    `json:"liegenschaftsunterhalt"`
}

type Berufskosten struct {
	Fahrkosten          Fahrkosten  `json:"fahrkosten"`
	Verpflegung         Verpflegung `json:"verpflegung"`
	UebrigeBerufskosten float64     `json:"uebrigeBerufskosten"`
	Weiterbildungskosten float64    `json:"weiterbildungskosten"`
}

type Fahrkosten struct {
	Art        FahrkostenArt `json:"art"`
	DistanzKm  float64       `json:"distanzKm"`
	Arbeitstage int          `json:"arbeitstage"`
	OevKosten  float64       `json:"oevKosten"`
}

type Verpflegung struct {
	Auswaertig  bool `json:"auswaertig"`
	Kantine     bool `json:"kantine"`
	Arbeitstage int  `json:"arbeitstage"`
}

type Vermoegen struct {
	Bankguthaben              []Bankkonto `json:"bankguthaben"`
	Wertschriften             float64     `json:"wertschriften"`
	Fahrzeuge                 float64     `json:"fahrzeuge"`
	LebensversicherungRueckkauf float64   `json:"lebensversicherungRueckkauf"`
	UebrigesVermoegen         float64     `json:"uebrigesVermoegen"`
	Schulden                  float64     `json:"schulden"`
}

type Bankkonto struct {
	Bank              string  `json:"bank"`
	Bezeichnung       string  `json:"bezeichnung"`
	IBAN              string  `json:"iban,omitempty"`
	Saldo             float64 `json:"saldo"`
	Waehrung          string  `json:"waehrung"`
	Zinsertrag        float64 `json:"zinsertrag"`
	Verrechnungssteuer float64 `json:"verrechnungssteuer"`
}

type Steuerergebnis struct {
	TotalEinkommen          float64 `json:"totalEinkommen"`
	TotalAbzuege            float64 `json:"totalAbzuege"`
	SteuerbaresEinkommen    float64 `json:"steuerbaresEinkommen"`
	SteuerbaresEinkommenBund float64 `json:"steuerbaresEinkommenBund"`
	TotalVermoegen          float64 `json:"totalVermoegen"`
	TotalSchulden           float64 `json:"totalSchulden"`
	SteuerbaresVermoegen    float64 `json:"steuerbaresVermoegen"`
	EinfacheSteuer          float64 `json:"einfacheSteuer"`
	Kantonssteuer           float64 `json:"kantonssteuer"`
	Gemeindesteuer          float64 `json:"gemeindesteuer"`
	Kirchensteuer           float64 `json:"kirchensteuer"`
	Bundessteuer            float64 `json:"bundessteuer"`
	VermoegensSteuerKanton  float64 `json:"vermoegensSteuerKanton"`
	VermoegensSteuerGemeinde float64 `json:"vermoegensSteuerGemeinde"`
	TotalSteuer             float64 `json:"totalSteuer"`
	Gemeinde                string  `json:"gemeinde"`
	SteuerfussGemeinde      int     `json:"steuerfussGemeinde"`
	SteuerfussKanton        int     `json:"steuerfussKanton"`
	SteuerfussKirche        int     `json:"steuerfussKirche"`
	Steuerperiode           int     `json:"steuerperiode"`
}

type Optimierung struct {
	Titel               string   `json:"titel"`
	Beschreibung        string   `json:"beschreibung"`
	SparpotenzialMin    *float64 `json:"sparpotenzial_min"`
	SparpotenzialMax    *float64 `json:"sparpotenzial_max"`
	Aufwand             string   `json:"aufwand"`
	Zeitrahmen          string   `json:"zeitrahmen"`
	Kategorie           string   `json:"kategorie"`
	GesetzlicheGrundlage string  `json:"gesetzliche_grundlage"`
}

// NewDefaultSteuerfall returns a zero-value Steuerfall with sensible defaults.
func NewDefaultSteuerfall() Steuerfall {
	return Steuerfall{
		Steuerperiode: 2024,
		Abzuege: Abzuege{
			Berufskosten: Berufskosten{
				Fahrkosten:  Fahrkosten{Art: FahrkostenOev, Arbeitstage: 220},
				Verpflegung: Verpflegung{Arbeitstage: 220},
			},
		},
	}
}'

stub internal/models/extraktion.go 'package models

// Extraction result structs — returned by Claude Vision, stored temporarily in session.

type Konfidenz struct {
	Gesamt         string   `json:"gesamt"`          // "hoch" | "mittel" | "tief"
	UnsichereFelder []string `json:"unsichere_felder"`
}

// Raw JSON shape returned by Claude for Lohnausweis
type LohnausweisRaw struct {
	ArbeitgeberName       string   `json:"arbeitgeber_name"`
	ArbeitgeberOrt        string   `json:"arbeitgeber_ort"`
	ArbeitnehmerName      string   `json:"arbeitnehmer_name"`
	AhvNummer             *string  `json:"ahv_nummer"`
	Bruttolohn            float64  `json:"ziff8_bruttolohn"`
	Sozialabgaben         float64  `json:"ziff9_sozialabgaben"`
	BvgOrdentlich         *float64 `json:"ziff10_1_bvg_ordentlich"`
	BvgEinkauf            *float64 `json:"ziff10_2_bvg_einkauf"`
	Nettolohn             float64  `json:"ziff11_nettolohn"`
	Quellensteuer         *float64 `json:"ziff12_quellensteuer"`
	SpesenEffektiv        *float64 `json:"ziff13_1_spesen_effektiv"`
	SpesenPauschal        *float64 `json:"ziff13_2_spesen_pauschal"`
	AussendienstProzent   *float64 `json:"ziff15_aussendienst_prozent"`
	Kinderzulagen         *float64 `json:"ziff5_kinderzulagen"`
	FeldFGAOderGeschaeftsauto bool `json:"feld_f_ga_oder_geschaeftsauto"`
	FeldGKantine          bool     `json:"feld_g_kantine"`
	Konfidenz             Konfidenz `json:"konfidenz"`
}

type KontoauszugRaw struct {
	Stichtag  string           `json:"stichtag"`
	Konten    []KontoRaw       `json:"konten"`
	Konfidenz Konfidenz        `json:"konfidenz"`
}

type KontoRaw struct {
	Bank              string   `json:"bank"`
	Kontonummer       *string  `json:"kontonummer"`
	IBAN              *string  `json:"iban"`
	Bezeichnung       string   `json:"bezeichnung"`
	Waehrung          string   `json:"waehrung"`
	Saldo             float64  `json:"saldo"`
	Zinsertrag        *float64 `json:"zinsertrag"`
	Verrechnungssteuer *float64 `json:"verrechnungssteuer"`
}

type Saeule3aRaw struct {
	Institut       string    `json:"institut"`
	Steuerjahr     int       `json:"steuerjahr"`
	Einzahlung     float64   `json:"einzahlung"`
	Art            string    `json:"art"`
	SaldoJahresende *float64 `json:"saldo_jahresende"`
	Konfidenz      Konfidenz `json:"konfidenz"`
}

// SessionExtractionResult holds the latest OCR result before the user accepts it.
type SessionExtractionResult struct {
	Type    string `json:"type"` // "lohnausweis" | "kontoauszug" | "3a"
	// Raw JSON bytes — one of the *Raw types above, stored as JSON for session serialization
	Payload []byte `json:"payload"`
}

type UploadedFile struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Extracted bool   `json:"extracted"`
}'

stub internal/models/parameter.go 'package models

// SteuerparameterDB is loaded once at startup from docs/steuerparameter.json.
type SteuerparameterDB struct {
	Steuerperiode int           `json:"steuerperiode"`
	Kanton        string        `json:"kanton"`
	Tarif         Tarif         `json:"tarif"`
	Steuerfuesse  Steuerfuesse  `json:"steuerfuesse"`
	Abzuege       AbzugsConfig  `json:"abzuege"`
	Bundessteuer  BundessteuerConfig `json:"bundessteuer"`
}

type Tarif struct {
	Einkommenssteuer EinkommenssteuerTarif `json:"einkommenssteuer"`
	Vermoegenssteuer VermoegenssteuerTarif `json:"vermoegenssteuer"`
}

type EinkommenssteuerTarif struct {
	Stufen                   []TarifStufe `json:"stufen"`
	MaxRate                  float64      `json:"maxRate"`
	MaxEinkommenGemeinsam    float64      `json:"maxEinkommenGemeinsam"`
	MaxEinkommenAlleinstehend float64     `json:"maxEinkommenAlleinstehend"`
}

type TarifStufe struct {
	Von        float64 `json:"von"`
	Bis        float64 `json:"bis"`
	BasisSteuer float64 `json:"basisSteuer"`
	Rate       float64 `json:"rate"`
}

type VermoegenssteuerTarif struct {
	Rate             float64 `json:"rate"`
	FreibetragPerson float64 `json:"freibetragPerson"`
	FreibetragKind   float64 `json:"freibetragKind"`
}

type Steuerfuesse struct {
	Kanton   int            `json:"kanton"`
	Gemeinden map[string]int `json:"gemeinden"`
	Kirche   map[string]KircheConfig `json:"kirche"`
}

type KircheConfig struct {
	Typisch int `json:"typisch"`
}

type AbzugsConfig struct {
	Berufskosten   BerufskostenConfig   `json:"berufskosten"`
	Vorsorge       VorsorgeConfig       `json:"vorsorge"`
	Versicherungen VersicherungenConfig `json:"versicherungen"`
	Krankheitskosten KrankheitskostenConfig `json:"krankheitskosten"`
	WeiterbildungMax float64            `json:"weiterbildungMax"`
	SchuldzinsenMaxBasis float64        `json:"schuldzinsenMaxBasis"`
	SpendenMaxProzent float64           `json:"spendenMaxProzent"`
	Sozialabzuege    SozialabzuegeConfig `json:"sozialabzuege"`
}

type BerufskostenConfig struct {
	FahrkostenMax      float64 `json:"fahrkostenMax"`
	FahrkostenMaxBund  float64 `json:"fahrkostenMaxBund"`
	KmAuto             float64 `json:"kmAuto"`
	KmMotorrad         float64 `json:"kmMotorrad"`
	VeloPauschale      float64 `json:"veloPauschale"`
	VerpflegungTag     float64 `json:"verpflegungTag"`
	VerpflegungMax     float64 `json:"verpflegungMax"`
	VerpflegungKantineTag float64 `json:"verpflegungKantineTag"`
	VerpflegungKantineMax float64 `json:"verpflegungKantineMax"`
	UebrigeMin         float64 `json:"uebrigeMin"`
	UebrigeMax         float64 `json:"uebrigeMax"`
	UebrigeProzent     float64 `json:"uebrigeProzent"`
}

type VorsorgeConfig struct {
	Saeule3aMitPk        float64 `json:"saeule3aMitPk"`
	Saeule3aOhnePk       float64 `json:"saeule3aOhnePk"`
	Saeule3aOhnePkProzent float64 `json:"saeule3aOhnePkProzent"`
}

type VersicherungenConfig struct {
	Alleinstehend           float64 `json:"alleinstehend"`
	AlleinstehendOhneVorsorge float64 `json:"alleinstehendOhneVorsorge"`
	Gemeinsam               float64 `json:"gemeinsam"`
	GemeinsamOhneVorsorge   float64 `json:"gemeinsamOhneVorsorge"`
	ProKind                 float64 `json:"proKind"`
}

type KrankheitskostenConfig struct {
	SelbstbehaltProzent float64 `json:"selbstbehaltProzent"`
}

type SozialabzuegeConfig struct {
	KinderabzugVorschule  float64 `json:"kinderabzugVorschule"`
	KinderabzugAusbildung float64 `json:"kinderabzugAusbildung"`
}

type BundessteuerConfig struct {
	StufenAlleinstehend     []TarifStufe `json:"stufen_alleinstehend"`
	FahrkostenMax           float64      `json:"fahrkostenMax"`
	VersicherungenAlleinstehend float64  `json:"versicherungenAlleinstehend"`
	VersicherungenGemeinsam float64      `json:"versicherungenGemeinsam"`
	VersicherungenProKind   float64      `json:"versicherungenProKind"`
	Kinderabzug             float64      `json:"kinderabzug"`
	WeiterbildungMax        float64      `json:"weiterbildungMax"`
}'

stub internal/tax/parameters.go 'package tax

import (
	"encoding/json"
	"os"

	"steuerpilot-go/internal/models"
)

// LoadSteuerparameter reads and parses docs/steuerparameter.json.
// Called once at startup; panics if the file is missing or malformed.
func LoadSteuerparameter(path string) (models.SteuerparameterDB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return models.SteuerparameterDB{}, err
	}
	var params models.SteuerparameterDB
	if err := json.Unmarshal(data, &params); err != nil {
		return models.SteuerparameterDB{}, err
	}
	return params, nil
}

// GetAlleGemeinden returns all municipality names from the Steuerfuesse map,
// sorted alphabetically — used to populate the Gemeinde <select>.
func GetAlleGemeinden(params models.SteuerparameterDB) []string {
	names := make([]string, 0, len(params.Steuerfuesse.Gemeinden))
	for name := range params.Steuerfuesse.Gemeinden {
		names = append(names, name)
	}
	// sort inline — import "sort" if needed
	return names
}'

stub internal/tax/calculator.go 'package tax

// BerechneSteuern is the core pure function: Steuerfall → Steuerergebnis.
// See metamorphosis.md §5 Phase 1 and SPEC.md §7 for the full algorithm.
// TODO: implement — port from drafts/src-lib-tax-calculator.ts'

stub internal/tax/calculator_test.go 'package tax

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"steuerpilot-go/internal/models"
)

func basisSteuerfall() models.Steuerfall {
	sf := models.NewDefaultSteuerfall()
	sf.Personalien = models.Personalien{
		Vorname:      "Hans",
		Nachname:     "Muster",
		Geburtsdatum: "1980-01-01",
		Zivilstand:   models.ZivilstandAlleinstehend,
		Konfession:   models.KonfessionEvangelisch,
		Gemeinde:     "St. Gallen",
		Kinder:       []models.Kind{},
	}
	sf.Einkommen.Haupterwerb = models.Erwerbseinkommen{
		Arbeitgeber:   "Muster AG",
		Bruttolohn:    80000,
		Nettolohn:     73000,
		AhvIvEoAlvNbuv: 5800,
		BvgOrdentlich: 4000,
	}
	sf.Einkommen.WertschriftenErtraege = 500
	sf.Einkommen.Bankzinsen = 100
	sf.Abzuege.Sozialabgaben = 5800
	sf.Abzuege.BvgBeitraege = 4000
	sf.Abzuege.Saeule3a = 7056
	sf.Abzuege.Versicherungspraemien = 3400
	sf.Abzuege.Berufskosten.Fahrkosten = models.Fahrkosten{
		Art: models.FahrkostenOev, Arbeitstage: 220, OevKosten: 1800,
	}
	sf.Abzuege.Berufskosten.Verpflegung = models.Verpflegung{Auswaertig: true, Arbeitstage: 220}
	sf.Vermoegen.Bankguthaben = []models.Bankkonto{
		{Bank: "UBS", Bezeichnung: "Konto", Saldo: 20000, Waehrung: "CHF", Zinsertrag: 100},
	}
	return sf
}

func TestBerechneSteuern_Grundfall(t *testing.T) {
	params, err := LoadSteuerparameter("../../docs/steuerparameter.json")
	require.NoError(t, err)

	sf := basisSteuerfall()
	ergebnis := BerechneSteuern(sf, params)

	assert.Equal(t, 80600.0, ergebnis.TotalEinkommen, "Gesamteinkommen")
	assert.Greater(t, ergebnis.Kantonssteuer, 0.0)
	assert.Greater(t, ergebnis.Gemeindesteuer, 0.0)
	assert.Greater(t, ergebnis.Bundessteuer, 0.0)

	// St. Gallen (138%) / Kanton (105%) ratio
	ratio := ergebnis.Gemeindesteuer / ergebnis.Kantonssteuer
	assert.InDelta(t, 138.0/105.0, ratio, 0.05)
}

func TestBerechneSteuern_TiefereGemeindesteuerRapperswilJona(t *testing.T) {
	params, _ := LoadSteuerparameter("../../docs/steuerparameter.json")
	sg := basisSteuerfall()
	rj := basisSteuerfall()
	rj.Personalien.Gemeinde = "Rapperswil-Jona"

	sgE := BerechneSteuern(sg, params)
	rjE := BerechneSteuern(rj, params)
	assert.Less(t, rjE.Gemeindesteuer, sgE.Gemeindesteuer)
}

func TestBerechneSteuern_Splitting(t *testing.T) {
	params, _ := LoadSteuerparameter("../../docs/steuerparameter.json")
	single := basisSteuerfall()
	single.Einkommen.Haupterwerb.Bruttolohn = 120000

	married := basisSteuerfall()
	married.Personalien.Zivilstand = models.ZivilstandVerheiratet
	married.Einkommen.Haupterwerb.Bruttolohn = 120000

	singleE := BerechneSteuern(single, params)
	marriedE := BerechneSteuern(married, params)
	assert.Less(t, marriedE.Kantonssteuer, singleE.Kantonssteuer, "Splitting muss Steuer senken")
}'

stub internal/util/format.go 'package util

import (
	"fmt"
	"math"
	"strings"
)

// FormatCHF formats a float64 as a Swiss franc amount with apostrophe thousand separator.
// Example: 1234567.89 → "CHF 1'"'"'234'"'"'567.89"
func FormatCHF(amount float64) string {
	return "CHF " + formatNumber(amount, 2)
}

// FormatCHFRound formats without decimal places, rounding to nearest franc.
// Example: 1234.6 → "CHF 1'"'"'235"
func FormatCHFRound(amount float64) string {
	return "CHF " + formatNumber(math.Round(amount), 0)
}

func formatNumber(amount float64, decimals int) string {
	format := fmt.Sprintf("%%.%df", decimals)
	s := fmt.Sprintf(format, amount)

	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]

	// Insert apostrophes as thousand separators
	var grouped strings.Builder
	for i, ch := range intPart {
		pos := len(intPart) - i
		if i > 0 && pos%3 == 0 {
			grouped.WriteRune('\x27') // apostrophe
		}
		grouped.WriteRune(ch)
	}

	if decimals > 0 && len(parts) == 2 {
		return grouped.String() + "." + parts[1]
	}
	return grouped.String()
}'

stub internal/session/session.go 'package session

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"

	"steuerpilot-go/internal/models"
)

const (
	keyData = "session_data"
)

// SessionData is everything stored server-side per user session.
type SessionData struct {
	Steuerfall       models.Steuerfall                `json:"steuerfall"`
	CurrentStep      string                           `json:"currentStep"`
	ExtractionResult *models.SessionExtractionResult  `json:"extractionResult,omitempty"`
	UploadedFiles    []models.UploadedFile             `json:"uploadedFiles,omitempty"`
	ClaudeCalls      int                              `json:"claudeCalls"`
}

func getStore(c *fiber.Ctx) *session.Session {
	store := c.Locals("session_store").(*session.Store)
	sess, _ := store.Get(c)
	return sess
}

func load(c *fiber.Ctx) SessionData {
	sess := getStore(c)
	raw, ok := sess.Get(keyData).([]byte)
	if !ok || raw == nil {
		return SessionData{
			Steuerfall:  models.NewDefaultSteuerfall(),
			CurrentStep: "upload",
		}
	}
	var sd SessionData
	_ = json.Unmarshal(raw, &sd)
	return sd
}

func save(c *fiber.Ctx, sd SessionData) {
	sess := getStore(c)
	raw, _ := json.Marshal(sd)
	sess.Set(keyData, raw)
	_ = sess.Save()
}

func GetSteuerfall(c *fiber.Ctx) models.Steuerfall    { return load(c).Steuerfall }
func GetCurrentStep(c *fiber.Ctx) string              { return load(c).CurrentStep }
func GetClaudeCalls(c *fiber.Ctx) int                 { return load(c).ClaudeCalls }

func SaveSteuerfall(c *fiber.Ctx, sf models.Steuerfall) {
	sd := load(c)
	sd.Steuerfall = sf
	save(c, sd)
}

func SetCurrentStep(c *fiber.Ctx, step string) {
	sd := load(c)
	sd.CurrentStep = step
	save(c, sd)
}

func SetExtractionResult(c *fiber.Ctx, r *models.SessionExtractionResult) {
	sd := load(c)
	sd.ExtractionResult = r
	save(c, sd)
}

func ClearExtractionResult(c *fiber.Ctx) {
	sd := load(c)
	sd.ExtractionResult = nil
	save(c, sd)
}

func IncrementClaudeCalls(c *fiber.Ctx) int {
	sd := load(c)
	sd.ClaudeCalls++
	save(c, sd)
	return sd.ClaudeCalls
}

func ClearSession(c *fiber.Ctx) {
	sess := getStore(c)
	_ = sess.Destroy()
}'

stub internal/claude/client.go 'package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

var client *anthropic.Client

// Init initialises the shared Anthropic client. Call once at startup.
func Init(apiKey string) {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	client = &c
}'

stub internal/claude/extract.go 'package claude

// ExtractDocument calls Claude Vision on a base64-encoded file.
// mime must be one of: image/jpeg, image/png, image/webp, application/pdf
// docType is one of: "lohnausweis", "kontoauszug", "3a"
// Returns raw JSON bytes of the extraction result.
// TODO: implement — see SPEC.md §8.1 and metamorphosis.md §6.3'

stub internal/claude/optimize.go 'package claude

import "steuerpilot-go/internal/models"

// GetOptimierungen sends the Steuerfall to Claude and returns optimisation suggestions.
// TODO: implement — see SPEC.md §8.2

var _ = models.Optimierung{} // prevent unused import error during scaffolding'

stub internal/export/pdf.go 'package export

import "steuerpilot-go/internal/models"

// GeneratePDF renders a PDF summary of the tax case and returns the bytes.
// Uses github.com/go-pdf/fpdf — NOT github.com/jung-kurt/gofpdf (archived).
// TODO: implement — see SPEC.md §9 and metamorphosis.md §5 Phase 6

var _ = models.Steuerfall{} // prevent unused import error during scaffolding'

stub middleware/session.go 'package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory"

	"steuerpilot-go/config"
)

var store *session.Store

// Session initialises the in-memory session store and attaches it to every request.
// Uses server-side storage — the cookie holds only a session ID, not the Steuerfall payload,
// which avoids the 4 KB browser cookie limit.
func Session(cfg config.Config) fiber.Handler {
	store = session.New(session.Config{
		Storage:        memory.New(),
		Expiration:     4 * time.Hour,
		KeyLookup:      "cookie:session_id",
		CookieSecure:   !cfg.IsDev,
		CookieHTTPOnly: true,
		CookieSameSite: "Strict",
	})
	return func(c *fiber.Ctx) error {
		c.Locals("session_store", store)
		return c.Next()
	}
}'

stub handlers/pages.go 'package handlers

import "github.com/gofiber/fiber/v2"

func (h *Handler) Landing(c *fiber.Ctx) error {
	// TODO: render templates/pages/landing.templ
	return c.SendString("Landing — TODO")
}

func (h *Handler) Upload(c *fiber.Ctx) error {
	// TODO: render templates/pages/upload.templ
	return c.SendString("Upload — TODO")
}

func (h *Handler) Ergebnis(c *fiber.Ctx) error {
	// TODO: render templates/pages/ergebnis.templ
	return c.SendString("Ergebnis — TODO")
}'

stub handlers/wizard.go 'package handlers

import "github.com/gofiber/fiber/v2"

var wizardSteps = []string{"personalien", "einkommen", "abzuege", "vermoegen", "zusammenfassung"}

func (h *Handler) WizardStep(c *fiber.Ctx) error {
	// TODO: render templates/wizard/<step>.templ
	step := c.Params("step")
	return c.SendString("Wizard step: " + step + " — TODO")
}

func (h *Handler) WizardSubmit(c *fiber.Ctx) error {
	// TODO: parse form, save to session, redirect to next step
	// HTMX redirect pattern: if HX-Request header present, use HX-Redirect header
	return c.SendString("WizardSubmit — TODO")
}

func (h *Handler) WizardBack(c *fiber.Ctx) error {
	return c.SendString("WizardBack — TODO")
}'

stub handlers/htmx.go 'package handlers

import "github.com/gofiber/fiber/v2"

// HTMX partial handlers — return HTML fragments, not full pages.

func (h *Handler) ExtractionPreview(c *fiber.Ctx) error {
	// TODO: receive file, base64-encode, call claude.ExtractDocument, render extractionpreview.templ
	return c.SendString("ExtractionPreview — TODO")
}

func (h *Handler) KindAdd(c *fiber.Ctx) error    { return c.SendString("KindAdd — TODO") }
func (h *Handler) KindRemove(c *fiber.Ctx) error  { return c.SendString("KindRemove — TODO") }
func (h *Handler) KontoAdd(c *fiber.Ctx) error    { return c.SendString("KontoAdd — TODO") }
func (h *Handler) KontoRemove(c *fiber.Ctx) error { return c.SendString("KontoRemove — TODO") }
func (h *Handler) TaxCalculate(c *fiber.Ctx) error { return c.SendString("TaxCalculate — TODO") }'

stub handlers/api.go 'package handlers

import "github.com/gofiber/fiber/v2"

func (h *Handler) HandleUpload(c *fiber.Ctx) error {
	// TODO: validate file, call claude.ExtractDocument, store in session
	return c.SendString("HandleUpload — TODO")
}

func (h *Handler) AcceptExtraction(c *fiber.Ctx) error {
	// TODO: merge session ExtractionResult into session Steuerfall, clear extraction
	return c.SendString("AcceptExtraction — TODO")
}

func (h *Handler) HandleOptimize(c *fiber.Ctx) error {
	// TODO: rate-limit check, call claude.GetOptimierungen, render optimierungen.templ
	return c.SendString("HandleOptimize — TODO")
}

func (h *Handler) ExportPDF(c *fiber.Ctx) error {
	// TODO: call export.GeneratePDF, stream bytes with Content-Disposition attachment
	return c.SendString("ExportPDF — TODO")
}

func (h *Handler) Reset(c *fiber.Ctx) error {
	// TODO: session.ClearSession(c), HX-Redirect to /
	return c.SendString("Reset — TODO")
}

// Handler is the top-level handler struct, shared across all handler files.
type Handler struct {
	cfg    interface{} // config.Config — avoids circular import in scaffold
	params interface{} // models.SteuerparameterDB
}

func New(cfg, params interface{}) *Handler {
	return &Handler{cfg: cfg, params: params}
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).SendString(err.Error())
}'

# ── templ stubs ───────────────────────────────────────────────────────────────

echo "→ Writing templ package stubs..."

for f in \
  "templates/layout/base.templ:layout" \
  "templates/layout/header.templ:layout" \
  "templates/layout/footer.templ:layout" \
  "templates/pages/landing.templ:pages" \
  "templates/pages/upload.templ:pages" \
  "templates/pages/wizard.templ:pages" \
  "templates/pages/ergebnis.templ:pages" \
  "templates/wizard/personalien.templ:wizard" \
  "templates/wizard/einkommen.templ:wizard" \
  "templates/wizard/abzuege.templ:wizard" \
  "templates/wizard/vermoegen.templ:wizard" \
  "templates/wizard/zusammenfassung.templ:wizard" \
  "templates/components/stepindicator.templ:components" \
  "templates/components/dropzone.templ:components" \
  "templates/components/extractionpreview.templ:components" \
  "templates/components/optimizationcard.templ:components" \
  "templates/components/deductionbreakdown.templ:components" \
  "templates/components/formfield.templ:components" \
  "templates/partials/kindrow.templ:partials" \
  "templates/partials/kontorow.templ:partials" \
  "templates/partials/fieldeditor.templ:partials" \
  "templates/partials/taxresult.templ:partials" \
  "templates/partials/optimierungen.templ:partials"
do
  path="${f%%:*}"
  pkg="${f##*:}"
  stub "$path" "package $pkg
// TODO: implement — see metamorphosis.md and SPEC.md"
done

# ── static placeholder ────────────────────────────────────────────────────────

touch "$BASE/static/.gitkeep"

# ── copy steuerparameter.json if missing ──────────────────────────────────────

SRC="$(dirname "$BASE")/steuerparameter.json"
DST="$BASE/docs/steuerparameter.json"
if [ ! -s "$DST" ] && [ -f "$SRC" ]; then
  cp "$SRC" "$DST"
  echo "→ Copied steuerparameter.json"
fi

# ── ownership ─────────────────────────────────────────────────────────────────

echo "→ Setting ownership to $OWNER ..."
chown -R "$OWNER" "$(dirname "$BASE")"

echo ""
echo "Done. $(find "$BASE" -type f | wc -l) files in steuerpilot-go/"
echo ""
echo "Next steps (as fsncps):"
echo "  cd go_version/steuerpilot-go"
echo "  cp .env.example .env && \$EDITOR .env    # add ANTHROPIC_API_KEY"
echo "  make tools                               # install templ + air"
echo "  make build                               # verify it compiles"
echo "  make test-calc                           # run calculator tests"

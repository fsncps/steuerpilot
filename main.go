package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"steuerpilot/config"
	"steuerpilot/handlers"
	"steuerpilot/internal/claude"
	"steuerpilot/internal/tax"
	appmiddleware "steuerpilot/middleware"
)

func main() {
	cfg := config.Load()

	// Initialise Claude client
	claude.Init(cfg.AnthropicAPIKey)

	// Load Steuerparameter at startup — fail fast if missing or malformed
	params, err := tax.LoadSteuerparameter(cfg.SteuerparameterPath)
	if err != nil {
		log.Fatalf("Steuerparameter konnten nicht geladen werden: %v", err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: handlers.ErrorHandler,
	})

	// --- Middleware ---
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(appmiddleware.Session(cfg))

	// --- Static files ---
	app.Static("/", "./static")

	// --- Routes ---
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
	htmx.Post("/wizard/kind/:i/toggle", h.KindToggleAusbildung)
	htmx.Post("/wizard/konto/add", h.KontoAdd)
	htmx.Delete("/wizard/konto/:i", h.KontoRemove)
	htmx.Get("/tax/calculate", h.TaxCalculate)

	app.Get("/ergebnis", h.Ergebnis)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	log.Printf("SteuerPilot SG läuft auf :%s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
}

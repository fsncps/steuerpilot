package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"

	"steuerpilot/config"
	"steuerpilot/internal/claude"
	"steuerpilot/internal/models"
	"steuerpilot/internal/session"
	"steuerpilot/templates/components"
	"steuerpilot/templates/partials"
)

// Handler is the top-level handler struct, shared across all handler files.
type Handler struct {
	cfg    *config.Config
	params models.SteuerparameterDB
}

func New(cfg *config.Config, params models.SteuerparameterDB) *Handler {
	return &Handler{cfg: cfg, params: params}
}

// render writes a templ component to the response.
func render(c *fiber.Ctx, comp templ.Component) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return comp.Render(c.Context(), c.Response().BodyWriter())
}

// htmxRedirect sends HX-Redirect for HTMX requests, otherwise a 302.
func htmxRedirect(c *fiber.Ctx, url string) error {
	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", url)
		return c.SendStatus(fiber.StatusOK)
	}
	return c.Redirect(url)
}

// HandleUpload processes a document upload via Claude Vision and stores the result in session.
func (h *Handler) HandleUpload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return render(c, components.ExtractionPreview("", "Keine Datei gefunden: "+err.Error(), nil))
	}
	docType := c.FormValue("docType", "lohnausweis")

	f, err := file.Open()
	if err != nil {
		return render(c, components.ExtractionPreview(docType, "Datei konnte nicht geöffnet werden.", nil))
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		return render(c, components.ExtractionPreview(docType, "Datei konnte nicht gelesen werden.", nil))
	}

	mimeType := file.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	b64 := base64.StdEncoding.EncodeToString(raw)

	resultJSON, err := claude.ExtractDocument(h.cfg.AnthropicAPIKey, mimeType, b64, docType)
	if err != nil {
		return render(c, components.ExtractionPreview(docType, "Extraktion fehlgeschlagen: "+err.Error(), nil))
	}

	session.SetExtractionResult(c, &models.SessionExtractionResult{
		Type:    docType,
		Payload: resultJSON,
	})

	// Build display fields from flat JSON map for the preview
	var fields [][2]string
	var m map[string]interface{}
	if json.Unmarshal(resultJSON, &m) == nil {
		for k, v := range m {
			fields = append(fields, [2]string{k, fmt.Sprintf("%v", v)})
		}
	}

	return render(c, components.ExtractionPreview(docType, "", fields))
}

// AcceptExtraction merges the stored extraction into the Steuerfall and clears it.
func (h *Handler) AcceptExtraction(c *fiber.Ctx) error {
	result := session.GetExtractionResult(c)
	if result == nil {
		return c.Redirect("/upload")
	}
	sf := session.GetSteuerfall(c)
	mergeExtraction(result, &sf)
	session.SaveSteuerfall(c, sf)
	session.ClearExtractionResult(c)
	return c.Redirect("/wizard/personalien")
}

// HandleOptimize calls Claude for optimization suggestions and renders the partial.
func (h *Handler) HandleOptimize(c *fiber.Ctx) error {
	sf := session.GetSteuerfall(c)
	opts, err := claude.GetOptimierungen(h.cfg.AnthropicAPIKey, sf)
	if err != nil {
		opts = []models.Optimierung{}
	}
	sf.Optimierungen = opts
	session.SaveSteuerfall(c, sf)
	return render(c, partials.Optimierungen(opts))
}

// ExportPDF streams a PDF of the tax result (not yet implemented).
func (h *Handler) ExportPDF(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).SendString("PDF-Export noch nicht implementiert.")
}

// Reset clears the session and redirects to the landing page.
func (h *Handler) Reset(c *fiber.Ctx) error {
	session.ClearSession(c)
	return htmxRedirect(c, "/")
}

// ErrorHandler is the Fiber application error handler.
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).SendString(err.Error())
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// mergeExtraction applies extracted OCR data into the Steuerfall.
func mergeExtraction(r *models.SessionExtractionResult, sf *models.Steuerfall) {
	switch r.Type {
	case "lohnausweis":
		var raw models.LohnausweisRaw
		if json.Unmarshal(r.Payload, &raw) != nil {
			return
		}
		sf.Einkommen.Haupterwerb.Arbeitgeber = raw.ArbeitgeberName
		sf.Einkommen.Haupterwerb.Bruttolohn = raw.Bruttolohn
		sf.Einkommen.Haupterwerb.Nettolohn = raw.Nettolohn
		sf.Einkommen.Haupterwerb.AhvIvEoAlvNbuv = raw.Sozialabgaben
		sf.Einkommen.Haupterwerb.HatGA = raw.FeldFGAOderGeschaeftsauto
		sf.Einkommen.Haupterwerb.HatKantine = raw.FeldGKantine
		if raw.BvgOrdentlich != nil {
			sf.Einkommen.Haupterwerb.BvgOrdentlich = *raw.BvgOrdentlich
		}
		if raw.Quellensteuer != nil {
			sf.Einkommen.Haupterwerb.Quellensteuer = *raw.Quellensteuer
		}
		if raw.Kinderzulagen != nil {
			sf.Einkommen.Kinderzulagen = *raw.Kinderzulagen
		}
	case "kontoauszug":
		var raw models.KontoauszugRaw
		if json.Unmarshal(r.Payload, &raw) != nil {
			return
		}
		for _, k := range raw.Konten {
			konto := models.Bankkonto{
				Bank:        k.Bank,
				Bezeichnung: k.Bezeichnung,
				Waehrung:    k.Waehrung,
				Saldo:       k.Saldo,
			}
			if k.IBAN != nil {
				konto.IBAN = *k.IBAN
			}
			if k.Zinsertrag != nil {
				konto.Zinsertrag = *k.Zinsertrag
			}
			if k.Verrechnungssteuer != nil {
				konto.Verrechnungssteuer = *k.Verrechnungssteuer
			}
			sf.Vermoegen.Bankguthaben = append(sf.Vermoegen.Bankguthaben, konto)
		}
	case "3a":
		var raw models.Saeule3aRaw
		if json.Unmarshal(r.Payload, &raw) != nil {
			return
		}
		sf.Abzuege.Saeule3a = raw.Einzahlung
	}
}

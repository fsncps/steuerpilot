package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"steuerpilot/internal/models"
	"steuerpilot/internal/session"
	"steuerpilot/internal/tax"
	"steuerpilot/templates/partials"
)

// ExtractionPreview delegates to HandleUpload — used for HTMX-based preview before accepting.
func (h *Handler) ExtractionPreview(c *fiber.Ctx) error {
	return h.HandleUpload(c)
}

func (h *Handler) KindAdd(c *fiber.Ctx) error {
	sf := session.GetSteuerfall(c)
	sf.Personalien.Kinder = append(sf.Personalien.Kinder, models.Kind{})
	session.SaveSteuerfall(c, sf)
	return render(c, partials.KindList(sf.Personalien.Kinder))
}

func (h *Handler) KindRemove(c *fiber.Ctx) error {
	sf := session.GetSteuerfall(c)
	i, _ := strconv.Atoi(c.Params("i"))
	if i >= 0 && i < len(sf.Personalien.Kinder) {
		sf.Personalien.Kinder = append(sf.Personalien.Kinder[:i], sf.Personalien.Kinder[i+1:]...)
	}
	session.SaveSteuerfall(c, sf)
	return render(c, partials.KindList(sf.Personalien.Kinder))
}

func (h *Handler) KindToggleAusbildung(c *fiber.Ctx) error {
	sf := session.GetSteuerfall(c)
	i, _ := strconv.Atoi(c.Params("i"))
	if i >= 0 && i < len(sf.Personalien.Kinder) {
		sf.Personalien.Kinder[i].InAusbildung = !sf.Personalien.Kinder[i].InAusbildung
	}
	session.SaveSteuerfall(c, sf)
	return render(c, partials.KindList(sf.Personalien.Kinder))
}

func (h *Handler) KontoAdd(c *fiber.Ctx) error {
	sf := session.GetSteuerfall(c)
	sf.Vermoegen.Bankguthaben = append(sf.Vermoegen.Bankguthaben, models.Bankkonto{Waehrung: "CHF"})
	session.SaveSteuerfall(c, sf)
	return render(c, partials.KontoList(sf.Vermoegen.Bankguthaben))
}

func (h *Handler) KontoRemove(c *fiber.Ctx) error {
	sf := session.GetSteuerfall(c)
	i, _ := strconv.Atoi(c.Params("i"))
	if i >= 0 && i < len(sf.Vermoegen.Bankguthaben) {
		sf.Vermoegen.Bankguthaben = append(sf.Vermoegen.Bankguthaben[:i], sf.Vermoegen.Bankguthaben[i+1:]...)
	}
	session.SaveSteuerfall(c, sf)
	return render(c, partials.KontoList(sf.Vermoegen.Bankguthaben))
}

func (h *Handler) TaxCalculate(c *fiber.Ctx) error {
	sf := session.GetSteuerfall(c)
	ergebnis := tax.BerechneSteuern(sf, h.params)
	return render(c, partials.TaxResult(ergebnis))
}

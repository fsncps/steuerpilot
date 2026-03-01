package handlers

import (
	"github.com/gofiber/fiber/v2"

	"steuerpilot/internal/claude"
	"steuerpilot/internal/session"
	"steuerpilot/internal/tax"
	"steuerpilot/templates/pages"
)

func (h *Handler) Setup(c *fiber.Ctx) error {
	if claude.IsInitialized() {
		return c.Redirect("/")
	}
	return render(c, pages.Setup(""))
}

func (h *Handler) Landing(c *fiber.Ctx) error {
	return render(c, pages.Landing())
}

func (h *Handler) Upload(c *fiber.Ctx) error {
	return render(c, pages.Upload())
}

func (h *Handler) Ergebnis(c *fiber.Ctx) error {
	sf := session.GetSteuerfall(c)
	ergebnis := tax.BerechneSteuern(sf, h.params)
	return render(c, pages.Ergebnis(sf, ergebnis))
}

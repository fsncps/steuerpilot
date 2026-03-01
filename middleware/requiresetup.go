package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"steuerpilot/internal/claude"
)

// RequireSetup redirects to /setup if the Anthropic client is not yet initialised.
// Passes through /setup itself, /health, and static assets.
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

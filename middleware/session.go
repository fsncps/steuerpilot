package middleware

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
}

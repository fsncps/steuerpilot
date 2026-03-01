package session

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

func GetExtractionResult(c *fiber.Ctx) *models.SessionExtractionResult {
	return load(c).ExtractionResult
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
}

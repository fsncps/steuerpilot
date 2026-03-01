package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// extractModel is used for all document extraction calls.
const extractModel = anthropic.ModelClaude3_5SonnetLatest

// jsonRe matches a JSON object, with or without a markdown code fence.
var jsonRe = regexp.MustCompile(`(?s)\{.*\}`)

// ExtractDocument calls Claude Vision on a base64-encoded file and returns the
// extracted data as raw JSON bytes matching the appropriate *Raw model struct.
// mimeType: "image/jpeg" | "image/png" | "image/webp" | "application/pdf"
// docType:  "lohnausweis" | "kontoauszug" | "3a"
func ExtractDocument(apiKey, mimeType, base64Data, docType string) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("claude: client not initialised — call claude.Init first")
	}

	// ── Build the document/image content block ────────────────────────────────
	var mediaBlock anthropic.ContentBlockParamUnion
	if mimeType == "application/pdf" {
		mediaBlock = anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{
			Data: base64Data,
		})
	} else {
		mediaBlock = anthropic.NewImageBlockBase64(mimeType, base64Data)
	}

	// ── Call the API ──────────────────────────────────────────────────────────
	msg, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     extractModel,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					mediaBlock,
					{OfText: &anthropic.TextBlockParam{Text: prompt(docType)}},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("claude API: %w", err)
	}

	// ── Extract text from response ────────────────────────────────────────────
	var text string
	for _, block := range msg.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}

	raw := extractJSON(text)
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("claude response nicht parseable als JSON: %q", raw)
	}
	return []byte(raw), nil
}

// extractJSON strips markdown fences and returns the first JSON object found.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip ```json ... ``` or ``` ... ```
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	// Find outermost { ... }
	if m := jsonRe.FindString(s); m != "" {
		return m
	}
	return s
}

// ── Prompts ───────────────────────────────────────────────────────────────────

func prompt(docType string) string {
	switch docType {
	case "lohnausweis":
		return `Analysiere diesen Schweizer Lohnausweis und extrahiere die Felder als JSON.
Beträge als Dezimalzahlen (z.B. 95'000.00 → 95000.00). Felder die nicht erkennbar sind → null.
Antworte NUR mit dem JSON-Objekt, ohne weitere Erklärungen:

{
  "arbeitgeber_name": "",
  "arbeitgeber_ort": "",
  "arbeitnehmer_name": "",
  "ahv_nummer": null,
  "ziff8_bruttolohn": 0.00,
  "ziff9_sozialabgaben": 0.00,
  "ziff10_1_bvg_ordentlich": null,
  "ziff10_2_bvg_einkauf": null,
  "ziff11_nettolohn": 0.00,
  "ziff12_quellensteuer": null,
  "ziff13_1_spesen_effektiv": null,
  "ziff13_2_spesen_pauschal": null,
  "ziff15_aussendienst_prozent": null,
  "ziff5_kinderzulagen": null,
  "feld_f_ga_oder_geschaeftsauto": false,
  "feld_g_kantine": false,
  "konfidenz": { "gesamt": "hoch", "unsichere_felder": [] }
}`

	case "kontoauszug":
		return `Analysiere diesen Schweizer Kontoauszug / Steuerausweis und extrahiere alle Konten.
Beträge als Dezimalzahlen. Felder die nicht erkennbar sind → null.
Antworte NUR mit dem JSON-Objekt:

{
  "stichtag": "31.12.2024",
  "konten": [
    {
      "bank": "",
      "kontonummer": null,
      "iban": null,
      "bezeichnung": "",
      "waehrung": "CHF",
      "saldo": 0.00,
      "zinsertrag": null,
      "verrechnungssteuer": null
    }
  ],
  "konfidenz": { "gesamt": "hoch", "unsichere_felder": [] }
}`

	case "3a":
		return `Analysiere diesen Schweizer Säule-3a-Beleg (Bank oder Versicherung) und extrahiere die Daten.
Beträge als Dezimalzahlen.
Antworte NUR mit dem JSON-Objekt:

{
  "institut": "",
  "steuerjahr": 2024,
  "einzahlung": 0.00,
  "art": "bank",
  "saldo_jahresende": null,
  "konfidenz": { "gesamt": "hoch", "unsichere_felder": [] }
}`

	default:
		return "Extrahiere alle relevanten Finanzdaten aus diesem Dokument als JSON."
	}
}

package export

import "steuerpilot-go/internal/models"

// GeneratePDF renders a PDF summary of the tax case and returns the bytes.
// Uses github.com/go-pdf/fpdf — NOT github.com/jung-kurt/gofpdf (archived).
// TODO: implement — see SPEC.md §9 and metamorphosis.md §5 Phase 6

var _ = models.Steuerfall{} // prevent unused import error during scaffolding

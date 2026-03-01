package claude

import (
	"fmt"

	"steuerpilot-go/internal/models"
)

// GetOptimierungen sends the Steuerfall to Claude and returns optimisation suggestions.
// TODO: implement — see SPEC.md §8.2
func GetOptimierungen(apiKey string, sf models.Steuerfall) ([]models.Optimierung, error) {
	return nil, fmt.Errorf("claude.GetOptimierungen: not yet implemented")
}

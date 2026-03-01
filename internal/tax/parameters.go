package tax

import (
	"encoding/json"
	"os"
	"sort"

	"steuerpilot/internal/models"
)

// LoadSteuerparameter reads and parses docs/steuerparameter.json.
// Called once at startup; panics if the file is missing or malformed.
func LoadSteuerparameter(path string) (models.SteuerparameterDB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return models.SteuerparameterDB{}, err
	}
	var params models.SteuerparameterDB
	if err := json.Unmarshal(data, &params); err != nil {
		return models.SteuerparameterDB{}, err
	}
	return params, nil
}

// LoadSteuerparameterFromBytes parses a JSON byte slice — used with go:embed.
func LoadSteuerparameterFromBytes(data []byte) (models.SteuerparameterDB, error) {
	var params models.SteuerparameterDB
	if err := json.Unmarshal(data, &params); err != nil {
		return models.SteuerparameterDB{}, err
	}
	return params, nil
}

// GetAlleGemeinden returns all municipality names from the Steuerfuesse map,
// sorted alphabetically — used to populate the Gemeinde <select>.
func GetAlleGemeinden(params models.SteuerparameterDB) []string {
	names := make([]string, 0, len(params.Steuerfuesse.Gemeinden))
	for name := range params.Steuerfuesse.Gemeinden {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

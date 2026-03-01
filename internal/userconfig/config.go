package userconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type UserConfig struct {
	AnthropicAPIKey string `json:"anthropic_api_key"`
}

func configPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "SteuerPilot", "config.json")
	default:
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "steuerpilot", "config.json")
	}
}

// Load reads the persisted user config. Returns empty config (not error) if file absent.
func Load() (UserConfig, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return UserConfig{}, nil
	}
	if err != nil {
		return UserConfig{}, err
	}
	var cfg UserConfig
	return cfg, json.Unmarshal(data, &cfg)
}

// Save writes the user config, creating the directory if needed.
func Save(cfg UserConfig) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

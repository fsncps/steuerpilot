package userconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoad(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	cfg := UserConfig{AnthropicAPIKey: "sk-ant-test123"}
	require.NoError(t, Save(cfg))

	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "sk-ant-test123", loaded.AnthropicAPIKey)
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Empty(t, cfg.AnthropicAPIKey)
}

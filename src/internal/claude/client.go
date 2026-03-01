package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

var client *anthropic.Client

// Init initialises the shared Anthropic client. Call once at startup.
func Init(apiKey string) {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	client = &c
}

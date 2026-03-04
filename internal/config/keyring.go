package config

import (
	"github.com/zalando/go-keyring"
)

const serviceName = "alaya-tui"

// KnownProviders lists the API key providers we know about.
var KnownProviders = []string{"anthropic", "openai", "openrouter", "groq"}

// providerEnvVars maps provider names to the env var name for their API key.
var providerEnvVars = map[string]string{
	"anthropic":  "ANTHROPIC_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
	"groq":       "GROQ_API_KEY",
}

// SetAPIKey stores an API key in the OS keychain.
func SetAPIKey(provider, key string) error {
	return keyring.Set(serviceName, provider, key)
}

// GetAPIKey retrieves an API key from the OS keychain.
func GetAPIKey(provider string) (string, error) {
	return keyring.Get(serviceName, provider)
}

// DeleteAPIKey removes an API key from the OS keychain.
func DeleteAPIKey(provider string) error {
	return keyring.Delete(serviceName, provider)
}

// APIKeyEnvVars returns a map of environment variables for all stored API keys.
// This is used to inject keys into agent subprocess environments.
func APIKeyEnvVars() map[string]string {
	env := make(map[string]string)
	for provider, envVar := range providerEnvVars {
		key, err := GetAPIKey(provider)
		if err == nil && key != "" {
			env[envVar] = key
		}
	}
	return env
}

// MaskKey returns a masked version of an API key for display (e.g., "****...3kF9").
func MaskKey(key string) string {
	if len(key) < 4 {
		return "****"
	}
	return "****..." + key[len(key)-4:]
}

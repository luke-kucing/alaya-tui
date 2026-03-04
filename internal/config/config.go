package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type AgentConfig struct {
	Name        string            `toml:"name"`
	Command     string            `toml:"command"`
	Description string            `toml:"description"`
	Env         map[string]string `toml:"env"`
}

type Config struct {
	VaultDir     string        `toml:"vault_dir"`
	DefaultAgent string        `toml:"default_agent"`
	Agents       []AgentConfig `toml:"agents"`

	path string // file path, not serialized
}

// DefaultConfigPath returns ~/.config/alaya-tui/config.toml.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "alaya-tui", "config.toml")
}

// Load reads config from the given path, or creates a default config if the file doesn't exist.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	cfg := DefaultConfig()
	cfg.path = path

	data, err := os.ReadFile(path) // #nosec G304 -- path is from user config flag or default
	if err != nil {
		if os.IsNotExist(err) {
			// Write default config
			if err := cfg.Save(); err != nil {
				return cfg, err
			}
			return cfg, nil
		}
		return nil, err
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.path = path
	return cfg, nil
}

// Save writes the config to its file path.
func (c *Config) Save() error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	f, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	return enc.Encode(c)
}

// DefaultConfig returns sensible defaults for a new installation.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		VaultDir:     filepath.Join(home, "notes"),
		DefaultAgent: "claude",
		Agents: []AgentConfig{
			{
				Name:        "claude",
				Command:     "claude",
				Description: "Claude Code CLI",
			},
		},
		path: DefaultConfigPath(),
	}
}

// SetPath sets the config file path (used when --config flag is provided).
func (c *Config) SetPath(path string) {
	c.path = path
}

// FindAgent returns the agent config with the given name, or nil if not found.
func (c *Config) FindAgent(name string) *AgentConfig {
	for i := range c.Agents {
		if c.Agents[i].Name == name {
			return &c.Agents[i]
		}
	}
	return nil
}

// RemoveAgent removes the agent with the given name. Returns true if found.
func (c *Config) RemoveAgent(name string) bool {
	for i, a := range c.Agents {
		if a.Name == name {
			c.Agents = append(c.Agents[:i], c.Agents[i+1:]...)
			return true
		}
	}
	return false
}

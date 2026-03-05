package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lukehinds/alaya-tui/internal/config"
	"github.com/lukehinds/alaya-tui/internal/tui"
)

func main() {
	var (
		vaultDir  string
		agentName string
		cfgPath   string
	)

	flag.StringVar(&vaultDir, "vault-dir", "", "Vault root directory (overrides config and ALAYA_VAULT_DIR)")
	flag.StringVar(&agentName, "agent", "", "Agent name to start with (overrides config default)")
	flag.StringVar(&cfgPath, "config", "", "Config file path (default: ~/.config/alaya-tui/config.toml)")
	flag.Parse()

	// Load config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Resolve vault directory: flag > ALAYA_VAULT_DIR > ZK_NOTEBOOK_DIR (compat) > config
	if vaultDir == "" {
		vaultDir = os.Getenv("ALAYA_VAULT_DIR")
	}
	if vaultDir == "" {
		vaultDir = os.Getenv("ZK_NOTEBOOK_DIR")
	}
	if vaultDir == "" {
		vaultDir = cfg.VaultDir
	}
	vaultDir = expandHome(vaultDir)

	// Resolve agent
	if agentName != "" {
		cfg.DefaultAgent = agentName
	}

	app := tui.NewApp(cfg, vaultDir)
	p := tea.NewProgram(&app, tea.WithAltScreen())
	app.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

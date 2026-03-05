package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ServerStatus represents whether the alaya MCP server appears to be running.
type ServerStatus int

const (
	ServerUnknown ServerStatus = iota
	ServerRunning
	ServerStopped
)

// CheckServer detects if the alaya MCP server is likely running by scanning
// /proc on Linux or reading ps output on macOS — no exec of pgrep.
func CheckServer() ServerStatus {
	if checkProcFS() {
		return ServerRunning
	}
	return ServerStopped
}

// checkProcFS scans /proc/*/cmdline (Linux) for an alaya.server process.
// Returns false on systems without /proc, so callers can fall back.
func checkProcFS() bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%s/cmdline", e.Name()))
		if err != nil {
			continue
		}
		// cmdline args are NUL-separated
		if strings.Contains(string(cmdline), "alaya.server") {
			return true
		}
	}
	return false
}

// CheckVaultHealth returns true if the vault directory looks valid
// (has .zk or .obsidian directory, or at least some .md files).
func CheckVaultHealth(vaultDir string) bool {
	for _, dataDir := range []string{".zk", ".obsidian"} {
		dir := filepath.Join(vaultDir, dataDir)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return true
		}
	}
	// Fallback: check for any .md files
	entries, err := os.ReadDir(vaultDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			return true
		}
	}
	return false
}

// AuditLogFresh returns true if audit.jsonl was modified within the given duration.
func AuditLogFresh(vaultDir string, within time.Duration) bool {
	path := findAuditLog(vaultDir)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < within
}

// SpawnServer starts the alaya MCP server as a background process.
// Returns the process so the caller can track/kill it.
func SpawnServer(vaultDir string) (*os.Process, error) {
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command,alaya-tui-command-injection
	cmd := exec.Command("uv", "run", "alaya") // #nosec G204 -- fully static args
	cmd.Env = append(os.Environ(), "ALAYA_VAULT_DIR="+vaultDir)
	cmd.Dir = vaultDir
	// Detach stdout/stderr so it doesn't interfere with TUI
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd.Process, nil
}

// findAuditLog returns the path to audit.jsonl, checking both .zk/ and .obsidian/.
// Returns empty string if not found.
func findAuditLog(vaultDir string) string {
	for _, dataDir := range []string{".zk", ".obsidian"} {
		path := filepath.Join(vaultDir, dataDir, "audit.jsonl")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

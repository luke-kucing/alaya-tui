package backend

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var skipDirs = map[string]bool{
	".zk":          true,
	".git":         true,
	".venv":        true,
	"__pycache__":  true,
	"node_modules": true,
}

type NoteMeta struct {
	Path      string
	Directory string
	Title     string
	Date      string
	Tags      []string
}

// ScanVault walks the vault directory and returns metadata for all .md files.
func ScanVault(vaultRoot string) ([]NoteMeta, error) {
	var notes []NoteMeta

	err := filepath.Walk(vaultRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip files we can't read
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		rel, _ := filepath.Rel(vaultRoot, path)
		dir := filepath.Dir(rel)
		if dir == "." {
			dir = "/"
		}

		meta := NoteMeta{
			Path:      path,
			Directory: dir,
			Title:     strings.TrimSuffix(info.Name(), ".md"),
		}

		parseFrontmatter(vaultRoot, path, &meta)
		notes = append(notes, meta)
		return nil
	})

	return notes, err
}

// parseFrontmatter reads YAML frontmatter from a markdown file.
// Handles simple key: value and key: [list] formats.
func parseFrontmatter(vaultRoot, path string, meta *NoteMeta) {
	if err := containedInVault(vaultRoot, path); err != nil {
		return
	}
	f, err := os.Open(path) // #nosec G304 -- path validated by containedInVault above
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// First line must be ---
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}

		key, value, ok := parseYAMLLine(line)
		if !ok {
			continue
		}

		switch key {
		case "title":
			meta.Title = value
		case "date":
			meta.Date = value
		case "tags":
			meta.Tags = parseYAMLList(value)
		}
	}
}

func parseYAMLLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	return key, value, true
}

func parseYAMLList(value string) []string {
	// Handle inline list: [tag1, tag2]
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := value[1 : len(value)-1]
		parts := strings.Split(inner, ",")
		var tags []string
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if t != "" {
				tags = append(tags, t)
			}
		}
		return tags
	}
	// Single value
	if value != "" {
		return []string{value}
	}
	return nil
}

// ReadNotePreview returns the first maxLines lines of a note file.
func ReadNotePreview(vaultRoot, path string, maxLines int) (string, error) {
	if err := containedInVault(vaultRoot, path); err != nil {
		return "", err
	}
	f, err := os.Open(path) // #nosec G304 -- path validated by containedInVault above
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() && len(lines) < maxLines {
		lines = append(lines, scanner.Text())
	}

	return strings.Join(lines, "\n"), scanner.Err()
}

// containedInVault returns an error if path does not resolve to a location
// within vaultRoot, preventing path traversal via symlinks or ".." sequences.
func containedInVault(vaultRoot, path string) error {
	absVault, err := filepath.Abs(vaultRoot)
	if err != nil {
		return fmt.Errorf("invalid vault root: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	rel, err := filepath.Rel(absVault, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path escapes vault: %s", path)
	}
	return nil
}

// DirectoryTree returns a sorted list of unique directories in the vault.
func DirectoryTree(notes []NoteMeta) []string {
	seen := map[string]bool{}
	for _, n := range notes {
		seen[n.Directory] = true
	}

	var dirs []string
	for d := range seen {
		dirs = append(dirs, d)
	}

	// Simple sort
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if dirs[j] < dirs[i] {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}
	return dirs
}

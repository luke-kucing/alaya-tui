package backend

import (
	"bufio"
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

		parseFrontmatter(path, &meta)
		notes = append(notes, meta)
		return nil
	})

	return notes, err
}

// parseFrontmatter reads YAML frontmatter from a markdown file.
// Handles simple key: value and key: [list] formats.
func parseFrontmatter(path string, meta *NoteMeta) {
	f, err := os.Open(path)
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
func ReadNotePreview(path string, maxLines int) (string, error) {
	f, err := os.Open(path)
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

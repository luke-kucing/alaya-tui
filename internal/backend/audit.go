package backend

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type AuditEntry struct {
	Ts         float64        `json:"ts"`
	Tool       string         `json:"tool"`
	Args       map[string]any `json:"args"`
	Status     string         `json:"status"`
	DurationMs float64        `json:"duration_ms"`
	Summary    string         `json:"summary"`
}

// TailAuditLog watches the audit.jsonl file and sends new entries on the returned channel.
// It polls every 500ms for new content. Close the done channel to stop tailing.
func TailAuditLog(vaultRoot string, done <-chan struct{}) (<-chan AuditEntry, error) {
	path := filepath.Join(vaultRoot, ".zk", "audit.jsonl")

	ch := make(chan AuditEntry, 64)

	go func() {
		defer close(ch)

		var offset int64

		// Read existing entries first
		offset = readFrom(path, 0, ch)

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				info, err := os.Stat(path)
				if err != nil {
					continue
				}
				if info.Size() > offset {
					offset = readFrom(path, offset, ch)
				}
			}
		}
	}()

	return ch, nil
}

// readFrom reads JSONL lines from the given offset and sends parsed entries to ch.
// Returns the new offset after reading.
func readFrom(path string, offset int64, ch chan<- AuditEntry) int64 {
	f, err := os.Open(path)
	if err != nil {
		return offset
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		return offset
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry AuditEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		ch <- entry
	}

	pos, err := f.Seek(0, 1)
	if err != nil {
		return offset
	}
	return pos
}

// LoadAuditLog reads all existing entries from the audit log.
func LoadAuditLog(vaultRoot string) ([]AuditEntry, error) {
	path := filepath.Join(vaultRoot, ".zk", "audit.jsonl")

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []AuditEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry AuditEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

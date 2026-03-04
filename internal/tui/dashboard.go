package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/lukehinds/alaya-tui/internal/backend"
)

type DashboardModel struct {
	notes      []backend.NoteMeta
	entries    []backend.AuditEntry
	agentName  string
	vaultDir   string
	width      int
	height     int
}

func NewDashboardModel(vaultDir, agentName string) DashboardModel {
	return DashboardModel{
		vaultDir:  vaultDir,
		agentName: agentName,
	}
}

func (m *DashboardModel) SetNotes(notes []backend.NoteMeta) {
	m.notes = notes
}

func (m *DashboardModel) SetEntries(entries []backend.AuditEntry) {
	m.entries = entries
}

func (m *DashboardModel) AddEntry(e backend.AuditEntry) {
	m.entries = append(m.entries, e)
}

func (m *DashboardModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m DashboardModel) View() string {
	var sections []string

	// Header
	header := titleStyle.Render("Dashboard")
	sections = append(sections, header)

	// Vault stats
	dirCounts := map[string]int{}
	tagCounts := map[string]int{}
	for _, n := range m.notes {
		dirCounts[n.Directory]++
		for _, t := range n.Tags {
			tagCounts[t]++
		}
	}

	vaultInfo := subtitleStyle.Render("Vault") + "\n"
	vaultInfo += fmt.Sprintf("  %s %s\n", labelStyle.Render("Path:"), valueStyle.Render(m.vaultDir))
	vaultInfo += fmt.Sprintf("  %s %s\n", labelStyle.Render("Notes:"), valueStyle.Render(fmt.Sprintf("%d", len(m.notes))))
	vaultInfo += fmt.Sprintf("  %s %s\n", labelStyle.Render("Directories:"), valueStyle.Render(fmt.Sprintf("%d", len(dirCounts))))
	vaultInfo += fmt.Sprintf("  %s %s", labelStyle.Render("Active Agent:"), valueStyle.Render(m.agentName))
	sections = append(sections, vaultInfo)

	// Top tags
	if len(tagCounts) > 0 {
		type tagCount struct {
			tag   string
			count int
		}
		var sorted []tagCount
		for t, c := range tagCounts {
			sorted = append(sorted, tagCount{t, c})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].count > sorted[j].count
		})

		tagInfo := subtitleStyle.Render("Top Tags") + "\n"
		limit := 10
		if len(sorted) < limit {
			limit = len(sorted)
		}
		for _, tc := range sorted[:limit] {
			tagInfo += fmt.Sprintf("  %s %s\n", labelStyle.Render(fmt.Sprintf("#%s", tc.tag)), mutedStyle.Render(fmt.Sprintf("(%d)", tc.count)))
		}
		sections = append(sections, strings.TrimRight(tagInfo, "\n"))
	}

	// Recent activity
	activityInfo := subtitleStyle.Render("Recent Activity") + "\n"
	if len(m.entries) == 0 {
		activityInfo += mutedStyle.Render("  No audit entries yet")
	} else {
		start := len(m.entries) - 5
		if start < 0 {
			start = 0
		}
		for _, e := range m.entries[start:] {
			ts := time.Unix(int64(e.Ts), 0).Format("15:04:05")
			status := successStyle.Render("ok")
			if e.Status == "error" {
				status = errorStyle.Render("err")
			}
			activityInfo += fmt.Sprintf("  %s  %-20s %s  %s\n",
				mutedStyle.Render(ts),
				valueStyle.Render(e.Tool),
				status,
				mutedStyle.Render(truncate(e.Summary, 40)),
			)
		}
	}

	// Error count
	var errCount int
	for _, e := range m.entries {
		if e.Status == "error" {
			errCount++
		}
	}
	errStr := successStyle.Render(fmt.Sprintf("%d errors", errCount))
	if errCount > 0 {
		errStr = errorStyle.Render(fmt.Sprintf("%d errors", errCount))
	}
	activityInfo += fmt.Sprintf("\n  %s %s", labelStyle.Render("Total:"), valueStyle.Render(fmt.Sprintf("%d calls", len(m.entries))))
	activityInfo += fmt.Sprintf("  %s", errStr)
	sections = append(sections, activityInfo)

	content := strings.Join(sections, "\n\n")

	// Constrain width
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	return lipgloss.NewStyle().Width(w).Render(content)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

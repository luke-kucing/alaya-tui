package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lukehinds/alaya-tui/internal/backend"
)

type ActivityModel struct {
	entries    []backend.AuditEntry
	filtered  []backend.AuditEntry
	filter     string
	filtering  bool
	cursor     int
	offset     int
	width      int
	height     int
}

func NewActivityModel() ActivityModel {
	return ActivityModel{}
}

func (m *ActivityModel) SetEntries(entries []backend.AuditEntry) {
	m.entries = entries
	m.applyFilter()
}

func (m *ActivityModel) AddEntry(e backend.AuditEntry) {
	m.entries = append(m.entries, e)
	m.applyFilter()
	// Auto-scroll to bottom
	if !m.filtering {
		m.cursor = len(m.filtered) - 1
		m.ensureVisible()
	}
}

func (m *ActivityModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *ActivityModel) applyFilter() {
	if m.filter == "" {
		m.filtered = m.entries
		return
	}
	m.filtered = nil
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.Tool), strings.ToLower(m.filter)) {
			m.filtered = append(m.filtered, e)
		}
	}
}

func (m *ActivityModel) ensureVisible() {
	visible := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m ActivityModel) visibleRows() int {
	// Header (2 lines) + filter line + bottom padding
	rows := m.height - 6
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m ActivityModel) Update(msg tea.Msg) (ActivityModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter", "esc":
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.applyFilter()
				}
			default:
				if len(msg.String()) == 1 {
					m.filter += msg.String()
					m.applyFilter()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "/":
			m.filtering = true
			m.filter = ""
		case "j", "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case "g":
			m.cursor = 0
			m.ensureVisible()
		case "G":
			m.cursor = len(m.filtered) - 1
			m.ensureVisible()
		}
	}
	return m, nil
}

func (m ActivityModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Activity Log"))
	b.WriteString("\n")

	// Filter bar
	if m.filtering {
		b.WriteString(fmt.Sprintf("  Filter: %s_\n", m.filter))
	} else if m.filter != "" {
		b.WriteString(fmt.Sprintf("  %s %s  %s\n", labelStyle.Render("Filter:"), valueStyle.Render(m.filter), mutedStyle.Render("(/ to change, esc to clear)")))
	} else {
		b.WriteString(mutedStyle.Render("  / to filter") + "\n")
	}

	// Header row
	header := fmt.Sprintf("  %-10s %-22s %-8s %-10s %s",
		labelStyle.Render("Time"),
		labelStyle.Render("Tool"),
		labelStyle.Render("Status"),
		labelStyle.Render("Duration"),
		labelStyle.Render("Summary"),
	)
	b.WriteString(header + "\n")
	b.WriteString(mutedStyle.Render("  "+strings.Repeat("-", m.width-6)) + "\n")

	if len(m.filtered) == 0 {
		b.WriteString(mutedStyle.Render("  No entries") + "\n")
	} else {
		visible := m.visibleRows()
		end := m.offset + visible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := m.offset; i < end; i++ {
			e := m.filtered[i]
			ts := time.Unix(int64(e.Ts), 0).Format("15:04:05")

			status := successStyle.Render("ok")
			if e.Status == "error" {
				status = errorStyle.Render("error")
			}

			dur := fmt.Sprintf("%.0fms", e.DurationMs)

			summaryWidth := m.width - 60
			if summaryWidth < 10 {
				summaryWidth = 10
			}
			summary := truncate(e.Summary, summaryWidth)

			line := fmt.Sprintf("  %-10s %-22s %-8s %-10s %s",
				mutedStyle.Render(ts),
				valueStyle.Render(truncate(e.Tool, 20)),
				status,
				mutedStyle.Render(dur),
				mutedStyle.Render(summary),
			)

			if i == m.cursor {
				line = selectedStyle.Render("> ") + line[2:]
			}

			b.WriteString(line + "\n")
		}
	}

	// Footer
	b.WriteString(fmt.Sprintf("\n  %s", mutedStyle.Render(fmt.Sprintf("%d entries", len(m.filtered)))))

	w := m.width - 4
	if w < 40 {
		w = 40
	}
	return lipgloss.NewStyle().Width(w).Render(b.String())
}

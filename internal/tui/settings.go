package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lukehinds/alaya-tui/internal/config"
)

type settingsSection int

const (
	sectionAgents settingsSection = iota
	sectionAPIKeys
	sectionVault
	sectionDefault
)

type SettingsModel struct {
	cfg          *config.Config
	section      settingsSection
	cursor       int
	editing      bool
	input        textinput.Model
	editField    string // what we're editing
	editTarget   string // agent name or provider name
	statusMsg    string
	width        int
	height       int
}

func NewSettingsModel(cfg *config.Config) SettingsModel {
	ti := textinput.New()
	ti.CharLimit = 256

	return SettingsModel{
		cfg:   cfg,
		input: ti,
	}
}

func (m *SettingsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editing {
			switch msg.String() {
			case "enter":
				m.applyEdit()
				m.editing = false
				m.input.Blur()
			case "esc":
				m.editing = false
				m.input.Blur()
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			m.cursor++
			m.clampCursor()
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "tab":
			m.section = (m.section + 1) % 4
			m.cursor = 0
		case "enter", "e":
			m.startEdit()
		case "d":
			m.deleteItem()
		case "a":
			m.addItem()
		}
	}
	return m, nil
}

func (m *SettingsModel) clampCursor() {
	max := m.maxCursor()
	if m.cursor >= max {
		m.cursor = max - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m SettingsModel) maxCursor() int {
	switch m.section {
	case sectionAgents:
		return len(m.cfg.Agents)
	case sectionAPIKeys:
		return len(config.KnownProviders)
	case sectionVault:
		return 1
	case sectionDefault:
		return len(m.cfg.Agents)
	}
	return 1
}

func (m *SettingsModel) startEdit() {
	switch m.section {
	case sectionAgents:
		if m.cursor < len(m.cfg.Agents) {
			agent := m.cfg.Agents[m.cursor]
			m.editField = "agent_command"
			m.editTarget = agent.Name
			m.input.SetValue(agent.Command)
			m.input.Focus()
			m.editing = true
		}
	case sectionAPIKeys:
		if m.cursor < len(config.KnownProviders) {
			provider := config.KnownProviders[m.cursor]
			m.editField = "api_key"
			m.editTarget = provider
			m.input.SetValue("")
			m.input.Placeholder = fmt.Sprintf("Enter %s API key...", provider)
			m.input.Focus()
			m.editing = true
		}
	case sectionVault:
		m.editField = "vault_dir"
		m.input.SetValue(m.cfg.VaultDir)
		m.input.Focus()
		m.editing = true
	case sectionDefault:
		if m.cursor < len(m.cfg.Agents) {
			m.cfg.DefaultAgent = m.cfg.Agents[m.cursor].Name
			m.cfg.Save()
			m.statusMsg = fmt.Sprintf("Default agent set to '%s'", m.cfg.DefaultAgent)
		}
	}
}

func (m *SettingsModel) applyEdit() {
	value := m.input.Value()

	switch m.editField {
	case "agent_command":
		for i := range m.cfg.Agents {
			if m.cfg.Agents[i].Name == m.editTarget {
				m.cfg.Agents[i].Command = value
				break
			}
		}
		m.cfg.Save()
		m.statusMsg = fmt.Sprintf("Updated command for '%s'", m.editTarget)

	case "api_key":
		if value != "" {
			if err := config.SetAPIKey(m.editTarget, value); err != nil {
				m.statusMsg = errorStyle.Render("Failed to save key: " + err.Error())
			} else {
				m.statusMsg = fmt.Sprintf("API key for '%s' saved to keychain", m.editTarget)
			}
		}

	case "vault_dir":
		m.cfg.VaultDir = value
		m.cfg.Save()
		m.statusMsg = "Vault directory updated"

	case "new_agent_name":
		// Second step: get command
		m.editField = "new_agent_command"
		m.editTarget = value
		m.input.SetValue("")
		m.input.Placeholder = "Command (e.g., ollama run qwen2.5)..."
		m.input.Focus()
		m.editing = true

	case "new_agent_command":
		m.cfg.Agents = append(m.cfg.Agents, config.AgentConfig{
			Name:    m.editTarget,
			Command: value,
		})
		m.cfg.Save()
		m.statusMsg = fmt.Sprintf("Added agent '%s'", m.editTarget)
	}
}

func (m *SettingsModel) deleteItem() {
	switch m.section {
	case sectionAgents:
		if m.cursor < len(m.cfg.Agents) {
			name := m.cfg.Agents[m.cursor].Name
			m.cfg.RemoveAgent(name)
			m.cfg.Save()
			m.statusMsg = fmt.Sprintf("Removed agent '%s'", name)
			m.clampCursor()
		}
	case sectionAPIKeys:
		if m.cursor < len(config.KnownProviders) {
			provider := config.KnownProviders[m.cursor]
			config.DeleteAPIKey(provider)
			m.statusMsg = fmt.Sprintf("Deleted API key for '%s'", provider)
		}
	}
}

func (m *SettingsModel) addItem() {
	if m.section == sectionAgents {
		m.editField = "new_agent_name"
		m.input.SetValue("")
		m.input.Placeholder = "Agent name..."
		m.input.Focus()
		m.editing = true
	}
}

func (m SettingsModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Section tabs
	sections := []string{"Agents", "API Keys", "Vault", "Default Agent"}
	for i, name := range sections {
		if settingsSection(i) == m.section {
			b.WriteString(activeTabStyle.Render(name))
		} else {
			b.WriteString(inactiveTabStyle.Render(name))
		}
		if i < len(sections)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n\n")

	switch m.section {
	case sectionAgents:
		b.WriteString(m.viewAgents())
	case sectionAPIKeys:
		b.WriteString(m.viewAPIKeys())
	case sectionVault:
		b.WriteString(m.viewVault())
	case sectionDefault:
		b.WriteString(m.viewDefaultAgent())
	}

	// Editing input
	if m.editing {
		b.WriteString("\n\n" + m.input.View())
	}

	// Status message
	if m.statusMsg != "" {
		b.WriteString("\n\n" + successStyle.Render(m.statusMsg))
	}

	// Help
	b.WriteString("\n\n" + helpStyle.Render("Tab: switch section | e: edit | a: add | d: delete | Enter: select"))

	w := m.width - 4
	if w < 40 {
		w = 40
	}
	return lipgloss.NewStyle().Width(w).Render(b.String())
}

func (m SettingsModel) viewAgents() string {
	var b strings.Builder
	if len(m.cfg.Agents) == 0 {
		b.WriteString(mutedStyle.Render("No agents configured. Press 'a' to add one."))
		return b.String()
	}

	for i, agent := range m.cfg.Agents {
		prefix := "  "
		if i == m.cursor {
			prefix = selectedStyle.Render("> ")
		}

		name := valueStyle.Render(agent.Name)
		if agent.Name == m.cfg.DefaultAgent {
			name += successStyle.Render(" (default)")
		}

		b.WriteString(fmt.Sprintf("%s%s\n", prefix, name))
		b.WriteString(fmt.Sprintf("    %s %s\n", labelStyle.Render("Command:"), mutedStyle.Render(agent.Command)))
		if agent.Description != "" {
			b.WriteString(fmt.Sprintf("    %s %s\n", labelStyle.Render("Desc:"), mutedStyle.Render(agent.Description)))
		}
	}
	return b.String()
}

func (m SettingsModel) viewAPIKeys() string {
	var b strings.Builder

	for i, provider := range config.KnownProviders {
		prefix := "  "
		if i == m.cursor {
			prefix = selectedStyle.Render("> ")
		}

		key, err := config.GetAPIKey(provider)
		status := errorStyle.Render("[not set]")
		if err == nil && key != "" {
			status = successStyle.Render(config.MaskKey(key) + " [set]")
		}

		b.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, valueStyle.Render(provider), status))
	}

	return b.String()
}

func (m SettingsModel) viewVault() string {
	prefix := selectedStyle.Render("> ")
	return fmt.Sprintf("%s%s %s\n", prefix, labelStyle.Render("Vault Dir:"), valueStyle.Render(m.cfg.VaultDir))
}

func (m SettingsModel) viewDefaultAgent() string {
	var b strings.Builder
	for i, agent := range m.cfg.Agents {
		prefix := "  "
		if i == m.cursor {
			prefix = selectedStyle.Render("> ")
		}
		name := valueStyle.Render(agent.Name)
		if agent.Name == m.cfg.DefaultAgent {
			name += successStyle.Render("  (current)")
		}
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, name))
	}
	b.WriteString("\n" + mutedStyle.Render("Press Enter to set as default"))
	return b.String()
}

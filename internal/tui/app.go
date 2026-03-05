package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lukehinds/alaya-tui/internal/backend"
	"github.com/lukehinds/alaya-tui/internal/config"
)

type tab int

const (
	tabDashboard tab = iota
	tabActivity
	tabNotes
	tabChat
	tabSettings
)

var tabNames = []string{"Dashboard", "Activity", "Notes", "Chat", "Settings"}

// auditMsg delivers a new audit entry from the tail goroutine.
type auditMsg backend.AuditEntry

// serverStatusMsg delivers a server health check result.
type serverStatusMsg backend.ServerStatus

// serverSpawnedMsg signals that we tried to spawn the server.
type serverSpawnedMsg struct {
	proc *os.Process
	err  error
}

type AppModel struct {
	cfg       *config.Config
	vaultDir  string
	activeTab tab
	width     int
	height    int

	dashboard DashboardModel
	activity  ActivityModel
	notes     NotesModel
	chat      ChatModel
	settings  SettingsModel

	auditCh      <-chan backend.AuditEntry
	auditDone    chan struct{}
	program      *tea.Program
	serverProc   *os.Process // non-nil if we spawned the MCP server
}

func NewApp(cfg *config.Config, vaultDir string) AppModel {
	agentName := cfg.DefaultAgent
	if agentName == "" && len(cfg.Agents) > 0 {
		agentName = cfg.Agents[0].Name
	}

	return AppModel{
		cfg:       cfg,
		vaultDir:  vaultDir,
		dashboard: NewDashboardModel(vaultDir, agentName),
		activity:  NewActivityModel(),
		notes:     NewNotesModel(vaultDir),
		chat:      NewChatModel(cfg),
		settings:  NewSettingsModel(cfg),
		auditDone: make(chan struct{}),
	}
}

// SetProgram sets the tea.Program reference for sending async messages.
func (m *AppModel) SetProgram(p *tea.Program) {
	m.program = p
	// Wire up the chat subprocess message sender
	programSend = func(msg tea.Msg) {
		if m.program != nil {
			m.program.Send(msg)
		}
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadVault,
		m.startAuditTail,
		m.checkServer,
		m.serverTick(),
	)
}

func (m AppModel) checkServer() tea.Msg {
	status := backend.CheckServer()
	return serverStatusMsg(status)
}

// serverTick re-checks server status every 10 seconds.
func (m AppModel) serverTick() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return serverStatusMsg(backend.CheckServer())
	})
}

func (m AppModel) loadVault() tea.Msg {
	notes, _ := backend.ScanVault(m.vaultDir)
	return vaultLoadedMsg(notes)
}

type vaultLoadedMsg []backend.NoteMeta

func (m AppModel) startAuditTail() tea.Msg {
	ch, err := backend.TailAuditLog(m.vaultDir, m.auditDone)
	if err != nil {
		return nil
	}
	return auditStartedMsg{ch: ch}
}

type auditStartedMsg struct {
	ch <-chan backend.AuditEntry
}

// waitForAudit returns a command that waits for the next audit entry.
func (m AppModel) waitForAudit() tea.Cmd {
	if m.auditCh == nil {
		return nil
	}
	ch := m.auditCh
	return func() tea.Msg {
		entry, ok := <-ch
		if !ok {
			return nil
		}
		return auditMsg(entry)
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := m.height - 4 // tab bar + status bar
		m.dashboard.SetSize(m.width, contentHeight)
		m.activity.SetSize(m.width, contentHeight)
		m.notes.SetSize(m.width, contentHeight)
		m.chat.SetSize(m.width, contentHeight)
		m.settings.SetSize(m.width, contentHeight)

	case serverStatusMsg:
		m.dashboard.SetServerStatus(backend.ServerStatus(msg))
		m.dashboard.SetVaultHealthy(backend.CheckVaultHealth(m.vaultDir))
		cmds = append(cmds, m.serverTick())

	case serverSpawnedMsg:
		if msg.err != nil {
			m.dashboard.SetServerStatus(backend.ServerStopped)
		} else {
			m.serverProc = msg.proc
			m.dashboard.SetServerStatus(backend.ServerRunning)
		}

	case vaultLoadedMsg:
		notes := []backend.NoteMeta(msg)
		m.dashboard.SetNotes(notes)
		m.notes.SetNotes(notes)

	case auditStartedMsg:
		m.auditCh = msg.ch
		// Also load existing entries
		entries, _ := backend.LoadAuditLog(m.vaultDir)
		m.dashboard.SetEntries(entries)
		m.activity.SetEntries(entries)
		cmds = append(cmds, m.waitForAudit())

	case auditMsg:
		entry := backend.AuditEntry(msg)
		m.dashboard.AddEntry(entry)
		m.activity.AddEntry(entry)
		cmds = append(cmds, m.waitForAudit())

	case tea.KeyMsg:
		// Global key bindings (only when not in editing mode)
		if !m.isEditing() {
			switch msg.String() {
			case "ctrl+c", "q":
				m.cleanup()
				return m, tea.Quit
			case "1":
				m.switchTab(tabDashboard)
				return m, nil
			case "2":
				m.switchTab(tabActivity)
				return m, nil
			case "3":
				m.switchTab(tabNotes)
				return m, nil
			case "4":
				m.switchTab(tabChat)
				return m, nil
			case "5":
				m.switchTab(tabSettings)
				return m, nil
			case "s":
				if m.activeTab == tabDashboard && m.dashboard.serverStatus == backend.ServerStopped {
					vd := m.vaultDir
					return m, func() tea.Msg {
						proc, err := backend.SpawnServer(vd)
						if err != nil {
							return serverSpawnedMsg{err: err}
						}
						return serverSpawnedMsg{err: nil, proc: proc}
					}
				}
			case "tab":
				next := (m.activeTab + 1) % 5
				m.switchTab(next)
				return m, nil
			}
		} else if msg.String() == "ctrl+c" {
			m.cleanup()
			return m, tea.Quit
		}

	// Chat subprocess messages
	case chatOutputMsg, chatErrorMsg, chatExitMsg:
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	// Route to active tab
	switch m.activeTab {
	case tabActivity:
		var cmd tea.Cmd
		m.activity, cmd = m.activity.Update(msg)
		cmds = append(cmds, cmd)
	case tabNotes:
		var cmd tea.Cmd
		m.notes, cmd = m.notes.Update(msg)
		cmds = append(cmds, cmd)
	case tabChat:
		var cmd tea.Cmd
		m.chat, cmd = m.chat.Update(msg)
		cmds = append(cmds, cmd)
	case tabSettings:
		var cmd tea.Cmd
		m.settings, cmd = m.settings.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) switchTab(t tab) {
	if m.activeTab == tabChat {
		m.chat.Blur()
	}
	m.activeTab = t
	if t == tabChat {
		m.chat.Focus()
	}
}

func (m AppModel) isEditing() bool {
	if m.activeTab == tabChat {
		return true
	}
	if m.activeTab == tabActivity {
		return m.activity.filtering
	}
	if m.activeTab == tabSettings {
		return m.settings.editing
	}
	return false
}

func (m *AppModel) cleanup() {
	close(m.auditDone)
	m.chat.killAgent()
	if m.serverProc != nil {
		_ = m.serverProc.Kill()
	}
}

func (m AppModel) View() string {
	// Tab bar
	var tabs []string
	for i, name := range tabNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if tab(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Content
	var content string
	switch m.activeTab {
	case tabDashboard:
		content = m.dashboard.View()
	case tabActivity:
		content = m.activity.View()
	case tabNotes:
		content = m.notes.View()
	case tabChat:
		content = m.chat.View()
	case tabSettings:
		content = m.settings.View()
	}

	// Status bar
	status := statusBarStyle.Width(m.width).Render(
		fmt.Sprintf(" alaya-tui | %s | q: quit | Tab: switch",
			strings.TrimSpace(m.vaultDir)),
	)

	return tabBar + "\n\n" + content + "\n" + status
}

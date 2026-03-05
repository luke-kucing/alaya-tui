package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lukehinds/alaya-tui/internal/config"
)

// Messages for subprocess output
type chatOutputMsg string
type chatErrorMsg string
type chatExitMsg struct{}

type ChatModel struct {
	cfg       *config.Config
	agentName string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	viewport  viewport.Model
	input     textinput.Model
	output    []string
	started   bool
	width     int
	height    int
}

func NewChatModel(cfg *config.Config) ChatModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 1000

	vp := viewport.New(80, 20)

	return ChatModel{
		cfg:       cfg,
		agentName: cfg.DefaultAgent,
		viewport:  vp,
		input:     ti,
	}
}

func (m *ChatModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w - 4
	m.viewport.Height = h - 8 // room for header + input + status
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}
}

func (m *ChatModel) Focus() {
	m.input.Focus()
	if !m.started {
		m.spawnAgent()
	}
}

func (m *ChatModel) Blur() {
	m.input.Blur()
}

func (m *ChatModel) spawnAgent() {
	agent := m.cfg.FindAgent(m.agentName)
	if agent == nil {
		m.output = append(m.output, errorStyle.Render(fmt.Sprintf("Agent '%s' not configured. Go to Settings tab to add agents.", m.agentName)))
		m.updateViewport()
		return
	}

	parts := strings.Fields(agent.Command)
	if len(parts) == 0 {
		m.output = append(m.output, errorStyle.Render("Empty command for agent"))
		m.updateViewport()
		return
	}

	// Validate the executable is a bare name or absolute path — no shell metacharacters.
	// This prevents config entries like "bash -c 'rm -rf ~'" from being split and executed.
	exe := parts[0]
	if err := validateAgentExecutable(exe); err != nil {
		m.output = append(m.output, errorStyle.Render("Invalid agent command: "+err.Error()))
		m.updateViewport()
		return
	}

	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command,alaya-tui-command-injection
	m.cmd = exec.Command(exe, parts[1:]...) // #nosec G204 -- exe validated by validateAgentExecutable above

	// Build environment: inherit current env + config env + API keys
	env := os.Environ()
	for k, v := range agent.Env {
		env = append(env, k+"="+v)
	}
	for k, v := range config.APIKeyEnvVars() {
		env = append(env, k+"="+v)
	}
	m.cmd.Env = env

	var err error
	m.stdin, err = m.cmd.StdinPipe()
	if err != nil {
		m.output = append(m.output, errorStyle.Render("Failed to create stdin pipe: "+err.Error()))
		m.updateViewport()
		return
	}

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		m.output = append(m.output, errorStyle.Render("Failed to create stdout pipe: "+err.Error()))
		m.updateViewport()
		return
	}

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		m.output = append(m.output, errorStyle.Render("Failed to create stderr pipe: "+err.Error()))
		m.updateViewport()
		return
	}

	if err := m.cmd.Start(); err != nil {
		m.output = append(m.output, errorStyle.Render("Failed to start agent: "+err.Error()))
		m.updateViewport()
		return
	}

	m.started = true
	m.output = append(m.output, mutedStyle.Render(fmt.Sprintf("--- Started %s ---", m.agentName)))
	m.updateViewport()

	// Read stdout in background
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			programSend(chatOutputMsg(scanner.Text()))
		}
	}()

	// Read stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			programSend(chatErrorMsg(scanner.Text()))
		}
	}()

	// Wait for exit in background
	go func() {
		_ = m.cmd.Wait()
		programSend(chatExitMsg{})
	}()
}

// validateAgentExecutable rejects executables containing shell metacharacters or
// relative path components, reducing the risk of command injection via config.
func validateAgentExecutable(exe string) error {
	const banned = "|&;<>()$`\\\"'*?[]#~=%!{}"
	for _, ch := range banned {
		if strings.ContainsRune(exe, ch) {
			return fmt.Errorf("disallowed character %q in command", ch)
		}
	}
	// Reject relative paths like "../foo" — allow bare names or absolute paths only.
	if !filepath.IsAbs(exe) && strings.Contains(exe, string(filepath.Separator)) {
		return fmt.Errorf("relative path not allowed; use a command name or absolute path")
	}
	return nil
}

// programSend is set by the app model to send messages to the Bubble Tea program.
// This is a simple approach; a production app would use tea.Program.Send().
var programSend func(tea.Msg)

func (m *ChatModel) killAgent() {
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		_ = m.cmd.Wait()
	}
	m.cmd = nil
	m.stdin = nil
	m.started = false
}

func (m *ChatModel) updateViewport() {
	content := strings.Join(m.output, "\n")
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case chatOutputMsg:
		m.output = append(m.output, string(msg))
		m.updateViewport()

	case chatErrorMsg:
		m.output = append(m.output, errorStyle.Render(string(msg)))
		m.updateViewport()

	case chatExitMsg:
		m.output = append(m.output, mutedStyle.Render(fmt.Sprintf("--- %s exited ---", m.agentName)))
		m.started = false
		m.updateViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			text := m.input.Value()
			if text == "" {
				return m, nil
			}
			m.input.SetValue("")

			// Handle commands
			if strings.HasPrefix(text, ":agent ") {
				name := strings.TrimPrefix(text, ":agent ")
				m.killAgent()
				m.agentName = strings.TrimSpace(name)
				m.output = append(m.output, mutedStyle.Render(fmt.Sprintf("--- Switching to %s ---", m.agentName)))
				m.updateViewport()
				m.spawnAgent()
				return m, nil
			}
			if text == ":restart" {
				m.killAgent()
				m.output = append(m.output, mutedStyle.Render("--- Restarting ---"))
				m.updateViewport()
				m.spawnAgent()
				return m, nil
			}

			// Send to subprocess
			m.output = append(m.output, successStyle.Render("> ")+text)
			m.updateViewport()

			if m.stdin != nil {
				fmt.Fprintln(m.stdin, text)
			}
			return m, nil
		}
	}

	// Update sub-components
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m ChatModel) View() string {
	header := titleStyle.Render("Chat")
	agentStatus := mutedStyle.Render(fmt.Sprintf("  Agent: %s", m.agentName))
	if m.started {
		agentStatus += successStyle.Render("  [running]")
	} else {
		agentStatus += errorStyle.Render("  [stopped]")
	}

	vp := m.viewport.View()
	input := m.input.View()

	help := helpStyle.Render("  :agent <name> to switch | :restart to restart")

	w := m.width - 4
	if w < 40 {
		w = 40
	}
	return lipgloss.NewStyle().Width(w).Render(
		header + agentStatus + "\n\n" + vp + "\n\n" + input + "\n" + help,
	)
}

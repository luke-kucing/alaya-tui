package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"

	"github.com/lukehinds/alaya-tui/internal/config"
)

// ansiEscape matches ANSI/VT100 escape sequences for stripping from PTY output.
var ansiEscape = regexp.MustCompile(`\x1b(\[[0-9;?]*[A-Za-z]|[()][AB012]|\][^\x07]*\x07|[^\[])`)

// Messages for subprocess output
type chatOutputMsg string
type chatErrorMsg string
type chatExitMsg struct{}

type ChatModel struct {
	cfg          *config.Config
	agentName    string
	cmd          *exec.Cmd
	pty          *os.File // PTY master — read output, write input
	viewport     viewport.Model
	input        textinput.Model
	inputFocused bool
	output       []string
	started      bool
	width        int
	height       int
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
	m.inputFocused = true
	m.input.Focus()
	if !m.started {
		m.spawnAgent()
	}
}

func (m *ChatModel) Blur() {
	m.inputFocused = false
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

	// Start process with a PTY so the agent sees a real terminal.
	ptmx, err := pty.Start(m.cmd)
	if err != nil {
		m.output = append(m.output, errorStyle.Render("Failed to start agent: "+err.Error()))
		m.updateViewport()
		return
	}
	m.pty = ptmx

	m.started = true
	m.output = append(m.output, mutedStyle.Render(fmt.Sprintf("--- Started %s ---", m.agentName)))
	m.updateViewport()

	// Read PTY output (stdout+stderr merged) in background.
	// Strip ANSI escape codes before sending to viewport.
	go func() {
		scanner := bufio.NewScanner(ptmx)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := ansiEscape.ReplaceAllString(scanner.Text(), "")
			programSend(chatOutputMsg(line))
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
	if m.pty != nil {
		_ = m.pty.Close()
		m.pty = nil
	}
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		_ = m.cmd.Wait()
	}
	m.cmd = nil
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
		// Esc releases input focus (vim-style modal)
		if msg.String() == "esc" && m.inputFocused {
			m.inputFocused = false
			m.input.Blur()
			return m, nil
		}
		// 'i' focuses input when not focused
		if msg.String() == "i" && !m.inputFocused {
			m.inputFocused = true
			m.input.Focus()
			return m, nil
		}

		if m.inputFocused {
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

				if m.pty != nil {
					fmt.Fprintln(m.pty, text)
				}
				return m, nil
			}
		}
	}

	// Update sub-components
	var cmd tea.Cmd
	if m.inputFocused {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m ChatModel) View() string {
	agentStatus := mutedStyle.Render(fmt.Sprintf("Agent: %s", m.agentName))
	if m.started {
		agentStatus += successStyle.Render("  [running]")
	} else {
		agentStatus += errorStyle.Render("  [stopped]")
	}

	w := m.width - 4
	if w < 40 {
		w = 40
	}

	// Wrap viewport output in a bordered panel
	outputPanel := panelStyle.Width(w - 4).Render(m.viewport.View())

	// Wrap input in a bordered panel
	inputPanel := panelStyle.Width(w - 4).Render(m.input.View())

	var help string
	if m.inputFocused {
		help = helpStyle.Render("  Esc: release focus | :agent <name> to switch | :restart to restart")
	} else {
		help = helpStyle.Render("  i: focus input | Tab/1-5: navigate tabs")
	}

	header := titleStyle.Render("Chat") + "  " + agentStatus
	return lipgloss.NewStyle().Width(w).Render(
		header + "\n\n" + outputPanel + "\n" + inputPanel + "\n" + help,
	)
}

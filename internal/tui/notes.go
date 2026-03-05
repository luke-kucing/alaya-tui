package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lukehinds/alaya-tui/internal/backend"
)

type treeNode struct {
	name     string
	path     string // full path for files
	isDir    bool
	expanded bool
	depth    int
	children []*treeNode
}

type NotesModel struct {
	vaultRoot string
	notes     []backend.NoteMeta
	tree      []*treeNode
	flat      []treeNode // flattened visible nodes
	cursor    int
	offset    int
	preview   string
	width     int
	height    int
}

func NewNotesModel(vaultRoot string) NotesModel {
	return NotesModel{vaultRoot: vaultRoot}
}

func (m *NotesModel) SetNotes(notes []backend.NoteMeta) {
	m.notes = notes
	m.buildTree()
	m.flatten()
}

func (m *NotesModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *NotesModel) buildTree() {
	root := map[string]*treeNode{}

	for _, n := range m.notes {
		parts := strings.Split(n.Directory, "/")
		if n.Directory == "/" {
			parts = nil
		}

		// Ensure directory nodes exist
		var current *treeNode
		for i, p := range parts {
			if p == "" {
				continue
			}
			key := strings.Join(parts[:i+1], "/")
			if _, ok := root[key]; !ok {
				node := &treeNode{name: p, isDir: true, depth: i, expanded: true}
				root[key] = node
				if current != nil {
					current.children = append(current.children, node)
				}
			}
			current = root[key]
		}

		// Add file node
		fileNode := &treeNode{
			name:  n.Title,
			path:  n.Path,
			isDir: false,
			depth: len(parts),
		}
		if current != nil {
			current.children = append(current.children, fileNode)
		} else {
			// Root-level file
			root["__file__"+n.Path] = fileNode
		}
	}

	// Collect top-level nodes
	m.tree = nil
	seen := map[*treeNode]bool{}
	for _, node := range root {
		if node.depth == 0 && !seen[node] {
			m.tree = append(m.tree, node)
			seen[node] = true
		}
	}
}

func (m *NotesModel) flatten() {
	m.flat = nil
	for _, node := range m.tree {
		m.flattenNode(node)
	}
}

func (m *NotesModel) flattenNode(node *treeNode) {
	m.flat = append(m.flat, *node)
	if node.isDir && node.expanded {
		for _, child := range node.children {
			m.flattenNode(child)
		}
	}
}

func (m *NotesModel) findTreeNode(idx int) *treeNode {
	if idx < 0 || idx >= len(m.flat) {
		return nil
	}
	target := m.flat[idx]

	var find func([]*treeNode) *treeNode
	find = func(nodes []*treeNode) *treeNode {
		for _, n := range nodes {
			if n.name == target.name && n.path == target.path && n.depth == target.depth {
				return n
			}
			if result := find(n.children); result != nil {
				return result
			}
		}
		return nil
	}
	return find(m.tree)
}

func (m *NotesModel) ensureVisible() {
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

func (m NotesModel) visibleRows() int {
	rows := m.height - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m NotesModel) Update(msg tea.Msg) (NotesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.flat)-1 {
				m.cursor++
				m.ensureVisible()
				m.loadPreview()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
				m.loadPreview()
			}
		case "enter":
			node := m.findTreeNode(m.cursor)
			if node != nil && node.isDir {
				node.expanded = !node.expanded
				m.flatten()
			}
		}
	}
	return m, nil
}

func (m *NotesModel) loadPreview() {
	if m.cursor < 0 || m.cursor >= len(m.flat) {
		m.preview = ""
		return
	}
	node := m.flat[m.cursor]
	if node.isDir || node.path == "" {
		m.preview = ""
		return
	}
	content, err := backend.ReadNotePreview(m.vaultRoot, node.path, 30)
	if err != nil {
		m.preview = errorStyle.Render("Error: " + err.Error())
		return
	}
	m.preview = content
}

func (m NotesModel) View() string {
	leftWidth := m.width/3 - 2
	rightWidth := m.width*2/3 - 4
	if leftWidth < 20 {
		leftWidth = 20
	}
	if rightWidth < 20 {
		rightWidth = 20
	}

	// Left pane: tree
	var left strings.Builder
	left.WriteString(subtitleStyle.Render("Files") + "\n\n")

	visible := m.visibleRows()
	end := m.offset + visible
	if end > len(m.flat) {
		end = len(m.flat)
	}

	for i := m.offset; i < end; i++ {
		node := m.flat[i]
		indent := strings.Repeat("  ", node.depth)

		var icon string
		if node.isDir {
			if node.expanded {
				icon = "v "
			} else {
				icon = "> "
			}
		} else {
			icon = "  "
		}

		line := indent + icon + node.name
		if i == m.cursor {
			line = selectedStyle.Render(line)
		} else if node.isDir {
			line = subtitleStyle.Render(line)
		} else {
			line = valueStyle.Render(line)
		}

		left.WriteString(line + "\n")
	}

	leftPane := lipgloss.NewStyle().Width(leftWidth).Render(left.String())

	// Right pane: preview
	var right strings.Builder
	right.WriteString(subtitleStyle.Render("Preview") + "\n\n")

	if m.preview == "" {
		right.WriteString(mutedStyle.Render("Select a note to preview"))
	} else {
		lines := strings.Split(m.preview, "\n")
		for _, line := range lines {
			right.WriteString(valueStyle.Render(line) + "\n")
		}
	}

	rightPane := panelStyle.Width(rightWidth).Render(right.String())

	// Combine
	combined := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, "  ", rightPane)

	header := titleStyle.Render("Notes Browser")
	return header + "\n" + combined
}

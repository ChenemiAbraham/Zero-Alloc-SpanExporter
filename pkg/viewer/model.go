package viewer

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourusername/ltt/pkg/exporter"
	"github.com/yourusername/ltt/pkg/protocol"
)

// Model represents the Bubbletea application model
type Model struct {
	tree      *TraceTree
	renderer  *WaterfallRenderer
	selected  int
	width     int
	height    int
	viewport  int
	filter    TraceFilter
	filtering bool

	socketPath string
	reader     *exporter.SocketReader
	ctx        context.Context
	cancel     context.CancelFunc
	spanChan   chan *protocol.SpanMessage
	program    *tea.Program
}

// NewModel creates a new TUI model
func NewModel(socketPath string) *Model {
	ctx, cancel := context.WithCancel(context.Background())

	return &Model{
		tree:       NewTraceTree(),
		renderer:   NewWaterfallRenderer(120),
		selected:   0,
		socketPath: socketPath,
		ctx:        ctx,
		cancel:     cancel,
		spanChan:   make(chan *protocol.SpanMessage, 100),
	}
}

// SetProgram sets the tea.Program instance for sending messages
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.connectSocket(),
		tickCmd(),
	)
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.renderer.Width = msg.Width
		return m, nil

	case spanReceivedMsg:
		m.tree.AddSpan(msg.span)
		return m, nil

	case tickMsg:
		return m, tickCmd()

	case errorMsg:
		// Handle error (could show in status bar)
		return m, tickCmd()
	}

	return m, nil
}

// View renders the UI
func (m *Model) View() string {
	// Header
	view := m.renderer.RenderHeader() + "\n\n"

	// Waterfall
	lines := m.renderer.RenderTree(m.tree)
	visibleStart := m.viewport
	visibleEnd := m.viewport + m.height - 6

	if visibleEnd > len(lines) {
		visibleEnd = len(lines)
	}

	for i := visibleStart; i < visibleEnd; i++ {
		view += lines[i] + "\n"
	}

	// Stats
	stats := m.tree.GetStats()
	view += "\n" + m.renderer.RenderStats(stats) + "\n"

	// Help
	view += m.renderer.RenderHelp() + "\n"

	return view
}

// handleKeyPress handles keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.cancel()
		return m, tea.Quit

	case "up", "k":
		if m.selected > 0 {
			m.selected--
			m.updateSelection()
			m.adjustViewport()
		}
		return m, nil

	case "down", "j":
		visible := m.tree.FlattenVisible()
		if m.selected < len(visible)-1 {
			m.selected++
			m.updateSelection()
			m.adjustViewport()
		}
		return m, nil

	case "enter", " ":
		m.toggleExpanded()
		return m, nil

	case "/":
		m.filtering = !m.filtering
		return m, nil

	case "e":
		// TODO: Export to file
		return m, nil

	case "r":
		// Refresh/clear
		m.tree = NewTraceTree()
		m.selected = 0
		return m, nil
	}

	return m, nil
}

// updateSelection updates the selected state on nodes
func (m *Model) updateSelection() {
	visible := m.tree.FlattenVisible()
	for i, node := range visible {
		node.Selected = (i == m.selected)
	}
}

// adjustViewport adjusts viewport to keep selected item visible
func (m *Model) adjustViewport() {
	if m.selected < m.viewport {
		m.viewport = m.selected
	} else if m.selected >= m.viewport+m.height-6 {
		m.viewport = m.selected - m.height + 7
	}

	if m.viewport < 0 {
		m.viewport = 0
	}
}

// toggleExpanded toggles the expanded state of selected node
func (m *Model) toggleExpanded() {
	visible := m.tree.FlattenVisible()
	if m.selected < len(visible) {
		visible[m.selected].Expanded = !visible[m.selected].Expanded
	}
}

// connectSocket connects to the socket and starts reading
func (m *Model) connectSocket() tea.Cmd {
	return func() tea.Msg {
		conn, err := exporter.Dial(m.socketPath)
		if err != nil {
			return errorMsg{err}
		}

		m.reader = exporter.NewSocketReader(conn)

		go m.readSpans()

		return nil
	}
}

// readSpans reads spans from the socket in a background goroutine
func (m *Model) readSpans() {
	defer close(m.spanChan)

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
			// Read raw message bytes (without length prefix)
			msgBytes, err := m.reader.ReadMessage(m.ctx)
			if err != nil {
				// Connection error or EOF
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Decode span message from bytes (payload only, no length prefix)
			span, err := protocol.DecodePayload(msgBytes)
			if err != nil {
				// Decode error, skip this message
				continue
			}

			// Send span to UI via channel
			select {
			case m.spanChan <- span:
				// Span queued successfully
			case <-m.ctx.Done():
				return
			default:
				// Channel full, drop span
			}

			// Notify UI thread about new span
			if m.program != nil {
				m.program.Send(spanReceivedMsg{span: span})
			}
		}
	}
}

// Messages
type spanReceivedMsg struct {
	span *protocol.SpanMessage
}

type errorMsg struct {
	err error
}

type tickMsg time.Time

// tickCmd returns a tick command for periodic UI updates
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

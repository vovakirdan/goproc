package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"goproc/internal/app"
)

// Controller defines the subset of app.App behaviour the TUI needs.
type Controller interface {
	Status() (app.DaemonStatus, error)
	StartDaemon() (*app.DaemonHandle, error)
	List(context.Context, app.ListParams) ([]app.Process, error)
}

// Model represents the Bubble Tea state.
type Model struct {
	controller Controller

	list      list.Model
	processes []app.Process
	selected  map[uint64]bool

	daemonStatus app.DaemonStatus
	statusMsg    string

	err     error
	loading bool

	width  int
	height int

	filters app.ListFilters

	lastUpdated time.Time
}

// New constructs a TUI model with default styles.
func New(ctrl Controller) *Model {
	delegate := newProcessDelegate()
	lst := list.New([]list.Item{}, delegate, 0, 0)
	lst.Title = "Processes"
	lst.SetShowHelp(false)
	lst.SetShowStatusBar(false)
	lst.SetFilteringEnabled(false)
	lst.SetShowPagination(false)
	lst.DisableQuitKeybindings()

	return &Model{
		controller: ctrl,
		list:       lst,
		filters:    app.ListFilters{},
		statusMsg:  "Checking daemon status…",
		loading:    true,
		selected:   make(map[uint64]bool),
	}
}

// Run spins up the Bubble Tea program with sensible defaults.
func Run(ctrl Controller) error {
	m := New(ctrl)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	_, err := prog.Run()
	return err
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(checkDaemonStatusCmd(m.controller), loadProcessesCmd(m.controller, m.filters))
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.height > 4 {
			m.list.SetSize(msg.Width, msg.Height-4)
		}

	case daemonStatusMsg:
		m.daemonStatus = msg.status
		if msg.status.Running {
			if msg.status.PID > 0 {
				m.statusMsg = fmt.Sprintf("Daemon running (pid %d). Press r to refresh, q to quit.", msg.status.PID)
			} else {
				m.statusMsg = "Daemon running. Press r to refresh, q to quit."
			}
		} else {
			m.statusMsg = "Daemon is not running. Press s to start it."
			m.processes = nil
			m.list.SetItems(nil)
		}

	case processesLoadedMsg:
		m.loading = false
		m.err = nil
		m.processes = msg.processes
		newSelected := make(map[uint64]bool)
		items := make([]list.Item, 0, len(msg.processes))
		for _, proc := range msg.processes {
			selected := m.selected[proc.ID]
			if selected {
				newSelected[proc.ID] = true
			}
			items = append(items, processItem{Process: proc, Selected: selected})
		}
		m.selected = newSelected
		m.list.SetItems(items)
		m.lastUpdated = time.Now()

	case daemonStartedMsg:
		m.statusMsg = "Daemon started."
		return m, tea.Batch(checkDaemonStatusCmd(m.controller), loadProcessesCmd(m.controller, m.filters))

	case errMsg:
		m.loading = false
		m.err = msg.err

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, loadProcessesCmd(m.controller, m.filters)
		case "s":
			if !m.daemonStatus.Running {
				m.statusMsg = "Starting daemon…"
				return m, startDaemonCmd(m.controller)
			}
		case " ":
			m.toggleCurrentSelection()
		case "c":
			if len(m.selected) > 0 {
				m.clearSelection()
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m *Model) View() string {
	var b strings.Builder

	statusStyle := lipgloss.NewStyle().Bold(true)
	if !m.daemonStatus.Running {
		statusStyle = statusStyle.Foreground(lipgloss.Color("203"))
	} else {
		statusStyle = statusStyle.Foreground(lipgloss.Color("42"))
	}
	b.WriteString(statusStyle.Render(m.statusMsg))
	b.WriteByte('\n')

	if m.loading {
		b.WriteString("Loading processes…\n")
	} else if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteByte('\n')
	}

	if len(m.list.Items()) == 0 && !m.loading && m.err == nil && m.daemonStatus.Running {
		b.WriteString("No processes found.\n")
	} else {
		b.WriteString(m.list.View())
		b.WriteByte('\n')
	}

	if current := m.currentProcess(); current != nil {
		detail := fmt.Sprintf(
			"id=%d pid=%d alive=%t\nname=%s\ncmd=%s\ntags=[%s]\ngroups=[%s]",
			current.ID,
			current.PID,
			current.Alive,
			valueOrDash(current.Name),
			current.Cmd,
			strings.Join(current.Tags, ","),
			strings.Join(current.Groups, ","),
		)
		detailStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).MarginBottom(1)
		b.WriteString(detailStyle.Render(detail))
		b.WriteByte('\n')
	}

	help := "Commands: q quit • r reload • s start daemon • space select • c clear selection"
	if count := len(m.selected); count > 0 {
		help += fmt.Sprintf(" • selected=%d", count)
	}
	if !m.lastUpdated.IsZero() {
		help += fmt.Sprintf(" • last update %s", m.lastUpdated.Format(time.Kitchen))
	}
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

// processItem adapts app.Process to the bubbles list item interface.
type processItem struct {
	Process  app.Process
	Selected bool
}

func (p processItem) Title() string {
	return valueOrDash(p.Process.Name)
}

func (p processItem) Description() string {
	return p.Process.Cmd
}

func (p processItem) FilterValue() string {
	return fmt.Sprintf("%d %d %s %s %s", p.Process.ID, p.Process.PID, p.Process.Name, strings.Join(p.Process.Tags, " "), strings.Join(p.Process.Groups, " "))
}

func (m *Model) toggleCurrentSelection() {
	if len(m.processes) == 0 {
		return
	}
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.processes) {
		return
	}
	item, ok := m.list.Items()[idx].(processItem)
	if !ok {
		return
	}
	if item.Selected {
		delete(m.selected, item.Process.ID)
	} else {
		m.selected[item.Process.ID] = true
	}
	item.Selected = !item.Selected
	m.list.SetItem(idx, item)
}

func (m *Model) clearSelection() {
	m.selected = make(map[uint64]bool)
	items := m.list.Items()
	for i, it := range items {
		if pi, ok := it.(processItem); ok && pi.Selected {
			pi.Selected = false
			m.list.SetItem(i, pi)
		}
	}
}

func (m *Model) currentProcess() *app.Process {
	if len(m.processes) == 0 {
		return nil
	}
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.processes) {
		return nil
	}
	return &m.processes[idx]
}

func valueOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

type processDelegate struct {
	styles processItemStyles
}

type processItemStyles struct {
	title         lipgloss.Style
	selectedTitle lipgloss.Style
	desc          lipgloss.Style
	selectedDesc  lipgloss.Style
	meta          lipgloss.Style
	selectedMeta  lipgloss.Style
	alive         lipgloss.Style
	dead          lipgloss.Style
	indicator     lipgloss.Style
	bullet        lipgloss.Style
}

func newProcessDelegate() list.ItemDelegate {
	styles := processItemStyles{
		title:         lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
		selectedTitle: lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
		desc:          lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		selectedDesc:  lipgloss.NewStyle().Foreground(lipgloss.Color("251")),
		meta:          lipgloss.NewStyle().Foreground(lipgloss.Color("239")),
		selectedMeta:  lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		alive:         lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		dead:          lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		indicator:     lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
		bullet:        lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
	}
	return processDelegate{styles: styles}
}

func (d processDelegate) Height() int { return 3 }

func (d processDelegate) Spacing() int { return 1 }

func (d processDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d processDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(processItem)
	if !ok {
		return
	}

	selected := index == m.Index()

	indicator := "∙"
	if item.Selected {
		indicator = "●"
	}
	indicatorRendered := d.styles.bullet.Render(indicator)
	if selected {
		indicatorRendered = d.styles.indicator.Render(indicator)
	}

	statusText := "dead"
	statusStyle := d.styles.dead
	if item.Process.Alive {
		statusText = "alive"
		statusStyle = d.styles.alive
	}

	titleStyle := d.styles.title
	descStyle := d.styles.desc
	metaStyle := d.styles.meta
	if selected {
		titleStyle = d.styles.selectedTitle
		descStyle = d.styles.selectedDesc
		metaStyle = d.styles.selectedMeta
	}

	title := fmt.Sprintf("%s  pid=%d  %s", valueOrDash(item.Process.Name), item.Process.PID, statusStyle.Render(statusText))
	desc := fmt.Sprintf("cmd: %s", item.Process.Cmd)
	meta := fmt.Sprintf("tags: [%s]  groups: [%s]", strings.Join(item.Process.Tags, ","), strings.Join(item.Process.Groups, ","))

	fmt.Fprintf(w, "%s %s\n", indicatorRendered, titleStyle.Render(title))
	fmt.Fprintf(w, "  %s\n", descStyle.Render(desc))
	fmt.Fprintf(w, "  %s\n", metaStyle.Render(meta))
}

type daemonStatusMsg struct {
	status app.DaemonStatus
}

type processesLoadedMsg struct {
	processes []app.Process
}

type daemonStartedMsg struct{}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

func checkDaemonStatusCmd(ctrl Controller) tea.Cmd {
	return func() tea.Msg {
		status, err := ctrl.Status()
		if err != nil {
			return errMsg{err}
		}
		return daemonStatusMsg{status: status}
	}
}

func loadProcessesCmd(ctrl Controller, filters app.ListFilters) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		procs, err := ctrl.List(ctx, app.ListParams{
			Filters: filters,
			Timeout: 4 * time.Second,
		})
		if err != nil {
			return errMsg{err}
		}
		return processesLoadedMsg{processes: procs}
	}
}

func startDaemonCmd(ctrl Controller) tea.Cmd {
	return func() tea.Msg {
		if _, err := ctrl.StartDaemon(); err != nil {
			return errMsg{err}
		}
		// Give the daemon a moment to bind the socket.
		time.Sleep(300 * time.Millisecond)
		return daemonStartedMsg{}
	}
}

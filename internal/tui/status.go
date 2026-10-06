package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/amarin/kforward/internal/discovery"
	"github.com/amarin/kforward/internal/forward"
	"github.com/amarin/kforward/internal/selector"
	"github.com/amarin/kforward/internal/state"
	"github.com/amarin/kforward/internal/term"
)

const refreshInterval = 2 * time.Second

type (
	tickMsg     struct{}
	execDoneMsg struct{ err error } // an add or context-switch prompt finished
	actionMsg   struct {
		status string
		err    error
	}
)

// funcCmd lets tea.Exec hand the terminal over to a plain Go function.
type funcCmd func() error

func (f funcCmd) Run() error        { return f() }
func (funcCmd) SetStdin(io.Reader)  {}
func (funcCmd) SetStdout(io.Writer) {}
func (funcCmd) SetStderr(io.Writer) {}

type statusModel struct {
	ctx      context.Context
	add      func(context.Context) error
	forwards []state.Forward
	current  string // kubectl's current context
	cursor   int
	confirm  bool // waiting for y/n before deleting the selected forward
	busy     bool
	status   string
	err      error
	quitting bool
}

// RunStatus shows the port forwards in an interactive table. Enter toggles the
// selected forward between up and down, d deletes it after a confirmation, and
// a runs add to create a new forward, and c switches kubectl's current context.
func RunStatus(ctx context.Context, add func(context.Context) error) error {
	forwards, err := state.ListForwards()
	if err != nil {
		return err
	}
	if len(forwards) == 0 {
		fmt.Println("No port forwards configured.")
		return nil
	}

	_, err = tea.NewProgram(statusModel{ctx: ctx, add: add, forwards: forwards, current: currentContext()}).Run()
	return err
}

func (m statusModel) Init() tea.Cmd { return tick() }

// execDone reports a finished prompt; aborting it with Esc is not an error.
func execDone(err error) tea.Msg {
	if errors.Is(err, huh.ErrUserAborted) {
		err = nil
	}
	return execDoneMsg{err: err}
}

func currentContext() string {
	current, _ := discovery.CurrentContext()
	return current
}

func switchContext() error {
	names, current, err := discovery.Contexts()
	if err != nil {
		return err
	}
	chosen, err := selector.ChooseContext(names, current)
	if err != nil {
		return err
	}
	return discovery.UseContext(chosen)
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.reload()
		return m, tick()

	case actionMsg:
		m.busy = false
		m.status, m.err = msg.status, msg.err
		m.reload()
		if len(m.forwards) == 0 {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case execDoneMsg:
		m.err = msg.err
		m.current = currentContext()
		m.reload()
		return m, nil

	case tea.KeyMsg:
		if m.busy {
			return m, nil
		}
		if m.confirm {
			return m.updateConfirm(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m statusModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.forwards)-1 {
			m.cursor++
		}
	case "enter":
		f := m.forwards[m.cursor]
		m.busy, m.status, m.err = true, "", nil
		return m, toggle(m.ctx, f)
	case "d":
		m.confirm, m.status, m.err = true, "", nil
	case "a":
		m.status, m.err = "", nil
		return m, tea.Exec(funcCmd(func() error { return m.add(m.ctx) }), execDone)
	case "c":
		m.status, m.err = "", nil
		return m, tea.Exec(funcCmd(switchContext), execDone)
	}
	return m, nil
}

func (m statusModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		f := m.forwards[m.cursor]
		m.confirm, m.busy = false, true
		return m, remove(f)
	case "n", "N", "esc", "ctrl+c":
		m.confirm = false
	}
	return m, nil
}

// reload re-reads the forwards from disk, keeping the cursor on the same one.
func (m *statusModel) reload() {
	var selected string
	if m.cursor < len(m.forwards) {
		selected = m.forwards[m.cursor].FilePath
	}

	forwards, err := state.ListForwards()
	if err != nil {
		m.err = err
		return
	}
	m.forwards = forwards

	for i, f := range forwards {
		if f.FilePath == selected {
			m.cursor = i
			return
		}
	}
	m.cursor = min(m.cursor, max(len(forwards)-1, 0))
}

func (m statusModel) View() string {
	var b strings.Builder

	fmt.Fprintf(&b, "Context: %s\n\n", m.current)

	nsW, nameW, ctxW := len("NAMESPACE"), len("NAME"), len("CONTEXT")
	for _, f := range m.forwards {
		nsW = max(nsW, len(f.Namespace))
		nameW = max(nameW, len(f.Name))
		ctxW = max(ctxW, len(contextLabel(f)))
	}

	// The badge is "[ down ]" at its widest (8 visible columns); escape codes
	// make fmt padding unreliable, so pad it manually.
	const badgeW = 8
	fmt.Fprintf(&b, "  %-*s  %-*s  %-*s  %-*s  %-5s  %-6s  %s\n", badgeW, "STATUS", ctxW, "CONTEXT", nsW, "NAMESPACE", nameW, "NAME", "LOCAL", "REMOTE", "PID")

	for i, f := range m.forwards {
		running := forward.IsRunning(f.PID)
		cursor := "  "
		if i == m.cursor && !m.quitting {
			cursor = "> "
		}
		badge := term.StatusBadge(running)
		pad := strings.Repeat(" ", badgeW-badgeVisibleLen(running))
		fmt.Fprintf(&b, "%s%s%s  %-*s  %-*s  %-*s  %-5d  %-6d  %d\n", cursor, badge, pad, ctxW, contextLabel(f), nsW, f.Namespace, nameW, f.Name, f.LocalPort, f.RemotePort, f.PID)
	}

	if m.quitting {
		return b.String()
	}

	b.WriteString("\n")
	switch {
	case m.busy:
		b.WriteString("Working...\n")
	case m.confirm:
		f := m.forwards[m.cursor]
		fmt.Fprintf(&b, "Delete %s/%s (localhost:%d -> :%d)? [y/N]\n", f.Namespace, f.Name, f.LocalPort, f.RemotePort)
	default:
		if m.err != nil {
			fmt.Fprintf(&b, "Error: %v\n", m.err)
		} else if m.status != "" {
			b.WriteString(m.status + "\n")
		}
		b.WriteString("↑/↓ select · enter toggle up/down · a add · c context · d delete · q quit\n")
	}
	return b.String()
}

// contextLabel is the context a forward was created with; records made before
// contexts were tracked have none and use the current one when started.
func contextLabel(f state.Forward) string {
	if f.Context == "" {
		return "-"
	}
	return f.Context
}

func badgeVisibleLen(up bool) int {
	if up {
		return len("[ up ]")
	}
	return len("[ down ]")
}

// toggle stops a running forward (keeping its record so it shows as down) or
// starts a down one again.
func toggle(ctx context.Context, f state.Forward) tea.Cmd {
	return func() tea.Msg {
		if forward.IsRunning(f.PID) {
			if err := forward.Pause(f); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{status: fmt.Sprintf("Stopped %s/%s", f.Namespace, f.Name)}
		}

		if u, err := forward.FindPortUser(f.LocalPort); err != nil {
			return actionMsg{err: err}
		} else if u != nil {
			return actionMsg{err: fmt.Errorf("port %d is in use by %s (PID %d)", f.LocalPort, u.Name, u.PID)}
		}

		client, err := discovery.NewClient(f.Context)
		if err != nil {
			return actionMsg{err: err}
		}
		target, err := client.ResolveTarget(ctx, f.Namespace, f.Name)
		if err != nil {
			return actionMsg{err: err}
		}
		started, err := forward.Recreate(f, target, client.Context())
		if err != nil {
			return actionMsg{err: err}
		}

		// kubectl exits right away when it cannot forward (bad port, no pod).
		time.Sleep(500 * time.Millisecond)
		if !forward.IsRunning(started.PID) {
			return actionMsg{err: fmt.Errorf("%s/%s exited right after starting; check the cluster and ports", f.Namespace, f.Name)}
		}
		return actionMsg{status: fmt.Sprintf("Started %s/%s", f.Namespace, f.Name)}
	}
}

func remove(f state.Forward) tea.Cmd {
	return func() tea.Msg {
		if err := forward.Stop(f); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{status: fmt.Sprintf("Deleted %s/%s", f.Namespace, f.Name)}
	}
}

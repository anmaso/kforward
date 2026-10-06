package selector

import (
	"fmt"
	"strings"

	"github.com/amarin/kforward/internal/discovery"
	"github.com/amarin/kforward/internal/forward"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
)

// newForm builds a form that can be aborted with Esc as well as Ctrl+C.
func newForm(groups ...*huh.Group) *huh.Form {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "quit"))
	return huh.NewForm(groups...).WithKeyMap(km)
}

func ChooseTarget(candidates []discovery.Target) (discovery.Target, error) {
	if len(candidates) == 0 {
		return discovery.Target{}, fmt.Errorf("no candidates to choose from")
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	nameW, typeW := len("NAME"), len("TYPE")
	for _, t := range candidates {
		nameW = max(nameW, len(t.Namespace)+1+len(t.Name))
		typeW = max(typeW, len(t.KindLabel()))
	}

	options := make([]huh.Option[string], len(candidates))
	keys := make([]string, len(candidates))
	for i, t := range candidates {
		key := fmt.Sprintf("%d", i)
		keys[i] = key
		row := fmt.Sprintf("%-*s  %-*s  %s", nameW, t.Namespace+"/"+t.Name, typeW, t.KindLabel(), t.PortsLabel())
		options[i] = huh.NewOption(row, key)
	}
	header := fmt.Sprintf("  %-*s  %-*s  %s", nameW, "NAME", typeW, "TYPE", "PORTS")

	var selected string
	form := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select resource").
				Description(header + "\nType to filter. Use arrow keys and Enter.").
				Options(options...).
				Filtering(true).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return discovery.Target{}, fmt.Errorf("selection cancelled: %w", err)
	}

	for i, key := range keys {
		if key == selected {
			return candidates[i], nil
		}
	}
	return discovery.Target{}, fmt.Errorf("invalid selection")
}

func ChoosePort(ports []int32) (int32, error) {
	if len(ports) == 0 {
		return 0, fmt.Errorf("no ports to choose from")
	}
	if len(ports) == 1 {
		return ports[0], nil
	}

	options := make([]huh.Option[string], len(ports))
	keys := make([]string, len(ports))
	for i, p := range ports {
		key := fmt.Sprintf("%d", i)
		keys[i] = key
		options[i] = huh.NewOption(fmt.Sprintf("%d", p), key)
	}

	var selected string
	form := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select remote port").
				Description("Multiple ports available. Use arrow keys and Enter.").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return 0, fmt.Errorf("selection cancelled: %w", err)
	}

	for i, key := range keys {
		if key == selected {
			return ports[i], nil
		}
	}
	return 0, fmt.Errorf("invalid selection")
}

func ChooseMapping(ports []int32) (forward.Mapping, error) {
	const manualKey = "__manual__"

	options := make([]huh.Option[string], 0, len(ports)+1)
	for _, p := range ports {
		label := fmt.Sprintf("%d:%d", p, p)
		options = append(options, huh.NewOption(label, label))
	}
	options = append(options, huh.NewOption("Enter manually (local:remote)", manualKey))

	var selected string
	if err := newForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Select port mapping").
			Description("Choose a port from the resource, or enter manually.").
			Options(options...).
			Value(&selected),
	)).Run(); err != nil {
		return forward.Mapping{}, fmt.Errorf("selection cancelled: %w", err)
	}

	if selected != manualKey {
		return forward.ParseMapping(selected)
	}

	var manual string
	if err := newForm(huh.NewGroup(
		huh.NewInput().
			Title("Port mapping").
			Description("Format: local:remote  e.g. 8080:3000").
			Value(&manual),
	)).Run(); err != nil {
		return forward.Mapping{}, fmt.Errorf("input cancelled: %w", err)
	}
	return forward.ParseMapping(manual)
}

func ConfirmKillProcess(u *forward.PortUser, port int) (bool, error) {
	name := u.Name
	if name == "" {
		name = fmt.Sprintf("PID %d", u.PID)
	}

	var desc strings.Builder
	fmt.Fprintf(&desc, "Process : %s (PID %d)\n", name, u.PID)
	if u.User != "" {
		fmt.Fprintf(&desc, "User    : %s\n", u.User)
	}
	if u.Elapsed != "" {
		fmt.Fprintf(&desc, "Running : %s\n", u.Elapsed)
	}
	if u.Args != "" {
		fmt.Fprintf(&desc, "Command : %s\n", u.Args)
	}
	desc.WriteString("\nKill it and proceed?")

	var confirmed bool
	form := newForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Port %d is already in use", port)).
			Description(desc.String()).
			Value(&confirmed),
	))
	if err := form.Run(); err != nil {
		return false, fmt.Errorf("prompt cancelled: %w", err)
	}
	return confirmed, nil
}

func ChooseForward(forwards []string) (string, error) {
	if len(forwards) == 0 {
		return "", fmt.Errorf("no port forwards to choose from")
	}
	if len(forwards) == 1 {
		return forwards[0], nil
	}

	options := make([]huh.Option[string], len(forwards))
	for i, label := range forwards {
		options[i] = huh.NewOption(label, label)
	}

	var selected string
	form := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select port forward to remove").
				Description("Use arrow keys and Enter.").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("selection cancelled: %w", err)
	}
	return selected, nil
}

// Menu actions returned by ChooseAction.
const (
	ActionStatus = "status"
	ActionAdd    = "add"
	ActionRemove = "remove"
)

func ChooseAction() (string, error) {
	var selected string
	form := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Description("Use arrow keys and Enter. Esc to quit.").
				Options(
					huh.NewOption("status  - list port forwards and their status", ActionStatus),
					huh.NewOption("add     - create a new port forward (interactive)", ActionAdd),
					huh.NewOption("remove  - stop an existing port forward", ActionRemove),
				).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("selection cancelled: %w", err)
	}
	return selected, nil
}

func ChooseContext(names []string, current string) (string, error) {
	if len(names) == 0 {
		return "", fmt.Errorf("no kubectl contexts found")
	}

	options := make([]huh.Option[string], len(names))
	for i, n := range names {
		label := n
		if n == current {
			label += "  (current)"
		}
		options[i] = huh.NewOption(label, n)
	}

	selected := current
	form := newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select kubectl context").
				Description("Type to filter. Use arrow keys and Enter.").
				Options(options...).
				Filtering(true).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("selection cancelled: %w", err)
	}
	return selected, nil
}

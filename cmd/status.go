package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/amarin/kforward/internal/forward"
	"github.com/amarin/kforward/internal/state"
	"github.com/amarin/kforward/internal/term"
	"github.com/amarin/kforward/internal/tui"
	"github.com/spf13/cobra"
)

var statusInteractive bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List port forwards and their status",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStatus(cmd.Context(), statusInteractive); err != nil {
			exitWithError(err)
		}
	},
}

func init() {
	statusCmd.Flags().BoolVarP(&statusInteractive, "interactive", "i", false, "select a forward to toggle up/down (enter) delete (d) or add (a)")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(ctx context.Context, interactive bool) error {
	if interactive {
		return tui.RunStatus(ctx, func(ctx context.Context) error { return runAdd(ctx, nil, true) })
	}

	forwards, err := state.ListForwards()
	if err != nil {
		return err
	}
	if len(forwards) == 0 {
		fmt.Println("No port forwards configured.")
		return nil
	}

	nsW, nameW, ctxW := len("NAMESPACE"), len("NAME"), len("CONTEXT")
	for _, f := range forwards {
		nsW = max(nsW, len(f.Namespace))
		nameW = max(nameW, len(f.Name))
		ctxW = max(ctxW, len(contextLabel(f)))
	}

	// The badge is "[ down ]" at its widest (8 visible columns); escape codes
	// make fmt padding unreliable, so pad it manually.
	const badgeW = 8
	fmt.Printf("%-*s  %-*s  %-*s  %-*s  %-5s  %-6s  %s\n", badgeW, "STATUS", ctxW, "CONTEXT", nsW, "NAMESPACE", nameW, "NAME", "LOCAL", "REMOTE", "PID")

	for _, f := range forwards {
		running := forward.IsRunning(f.PID)
		pad := badgeW - len("[ up ]")
		if !running {
			pad = badgeW - len("[ down ]")
		}
		fmt.Printf("%s%s  %-*s  %-*s  %-*s  %-5d  %-6d  %d\n", term.StatusBadge(running), strings.Repeat(" ", pad), ctxW, contextLabel(f), nsW, f.Namespace, nameW, f.Name, f.LocalPort, f.RemotePort, f.PID)
	}
	return nil
}

func contextLabel(f state.Forward) string {
	if f.Context == "" {
		return "-"
	}
	return f.Context
}

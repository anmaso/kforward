package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/amarin/kforward/internal/discovery"
	"github.com/amarin/kforward/internal/forward"
	"github.com/amarin/kforward/internal/selector"
	"github.com/amarin/kforward/internal/state"
	"github.com/amarin/kforward/internal/term"
)

var statusNoRecreate bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "List port forwards and their status",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runStatus(cmd.Context(), statusNoRecreate); err != nil {
			exitWithError(err)
		}
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusNoRecreate, "no-recreate", false, "list status only; do not offer to recreate down forwards")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(ctx context.Context, noRecreate bool) error {
	forwards, err := state.ListForwards()
	if err != nil {
		return err
	}
	if len(forwards) == 0 {
		fmt.Println("No port forwards configured.")
		return nil
	}

	nsW, nameW := len("NAMESPACE"), len("NAME")
	for _, f := range forwards {
		nsW = max(nsW, len(f.Namespace))
		nameW = max(nameW, len(f.Name))
	}

	// The badge is "[ down ]" at its widest (8 visible columns); escape codes
	// make fmt padding unreliable, so pad it manually.
	const badgeW = 8
	fmt.Printf("%-*s  %-*s  %-*s  %-5s  %-6s  %s\n", badgeW, "STATUS", nsW, "NAMESPACE", nameW, "NAME", "LOCAL", "REMOTE", "PID")

	var down []state.Forward
	for _, f := range forwards {
		running := forward.IsRunning(f.PID)
		if !running {
			down = append(down, f)
		}
		pad := badgeW - len("[ up ]")
		if !running {
			pad = badgeW - len("[ down ]")
		}
		fmt.Printf("%s%s  %-*s  %-*s  %-5d  %-6d  %d\n", term.StatusBadge(running), strings.Repeat(" ", pad), nsW, f.Namespace, nameW, f.Name, f.LocalPort, f.RemotePort, f.PID)
	}

	if len(down) == 0 || noRecreate {
		return nil
	}

	// Leave breathing room so the status table stands apart from the prompt.
	fmt.Print("\n\n")

	selected, err := selector.ChooseRecreateForwards(down)
	if err != nil {
		return err
	}
	if selected == selector.RecreateSkip {
		return nil
	}

	toRecreate := down
	if selected != selector.RecreateAll {
		toRecreate = nil
		for _, f := range down {
			if f.FilePath == selected {
				toRecreate = []state.Forward{f}
				break
			}
		}
		if len(toRecreate) == 0 {
			return fmt.Errorf("selected port forward not found")
		}
	}

	client, err := discovery.NewClient()
	if err != nil {
		return err
	}

	for _, f := range toRecreate {
		target, err := client.ResolveTarget(ctx, f.Namespace, f.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", f.Label(), err)
			continue
		}

		recreated, err := forward.Recreate(f, target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to recreate %s: %v\n", f.Label(), err)
			continue
		}

		fmt.Printf("%s recreated  %s\n", term.StatusBadge(true), recreated.Label())
	}

	return nil
}

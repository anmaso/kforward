package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/amarin/kforward/internal/discovery"
	"github.com/amarin/kforward/internal/forward"
	"github.com/amarin/kforward/internal/selector"
	"github.com/amarin/kforward/internal/state"
)

var addCmd = &cobra.Command{
	Use:   "add [name] [local:remote]",
	Short: "Create a new port forward",
	Args: func(cmd *cobra.Command, args []string) error {
		interactive, _ := cmd.Flags().GetBool("interactive")
		if interactive {
			if len(args) != 0 {
				return fmt.Errorf("--interactive does not accept positional arguments")
			}
			return nil
		}
		if len(args) < 1 || len(args) > 2 {
			return fmt.Errorf("accepts 1 or 2 args, received %d", len(args))
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		interactive, _ := cmd.Flags().GetBool("interactive")
		if err := runAdd(cmd.Context(), args, interactive); err != nil {
			exitWithError(err)
		}
	},
}

func init() {
	addCmd.Flags().BoolP("interactive", "i", false, "List all cluster resources and select interactively")
	rootCmd.AddCommand(addCmd)
}

func runAdd(ctx context.Context, args []string, interactive bool) error {
	client, err := discovery.NewClient("")
	if err != nil {
		return err
	}

	var target discovery.Target

	if interactive {
		targets, err := client.ListAllTargets(ctx)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return fmt.Errorf("no services or deployments found in the cluster")
		}
		target, err = selector.ChooseTarget(targets)
		if err != nil {
			return err
		}
	} else {
		query := args[0]
		targets, err := client.FindTargets(ctx, query)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return fmt.Errorf("no service or deployment matching %q", query)
		}
		target, err = selector.ChooseTarget(targets)
		if err != nil {
			return err
		}
	}

	var mapping forward.Mapping
	if !interactive && len(args) == 2 {
		mapping, err = forward.ParseMapping(args[1])
		if err != nil {
			return err
		}
	} else if interactive || len(target.Ports) > 1 {
		mapping, err = selector.ChooseMapping(target.Ports)
		if err != nil {
			return err
		}
	} else {
		remote, err := selector.ChoosePort(target.Ports)
		if err != nil {
			return err
		}
		mapping = forward.Mapping{
			Local:  int(remote),
			Remote: remote,
		}
	}

	if err := ensurePortFree(mapping.Local); err != nil {
		return err
	}

	created, err := forward.Start(target, mapping, client.Context())
	if err != nil {
		return err
	}

	fmt.Printf("Port forward started: %s\n", created.Label())
	fmt.Printf("PID file: %s\n", created.FilePath)
	return nil
}

func ensurePortFree(port int) error {
	// Check kforward-managed forwards first so we give a clear message instead
	// of showing the underlying kubectl process as an unknown intruder.
	if managed, err := findManagedForward(port); err == nil && managed != nil {
		return fmt.Errorf(
			"port %d is already managed by kforward (%s/%s localhost:%d -> :%d, PID %d)\n"+
				"Run `kforward remove` to stop it first",
			port, managed.Namespace, managed.Name, managed.LocalPort, managed.RemotePort, managed.PID,
		)
	}

	u, err := forward.FindPortUser(port)
	if err != nil {
		return err
	}
	if u == nil {
		return nil
	}

	ok, err := selector.ConfirmKillProcess(u, port)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("port %d is in use by %s (PID %d); aborted", port, u.Name, u.PID)
	}

	return forward.KillProcess(u.PID)
}

func findManagedForward(localPort int) (*state.Forward, error) {
	forwards, err := state.ListForwards()
	if err != nil {
		return nil, err
	}
	for i := range forwards {
		if forwards[i].LocalPort == localPort {
			return &forwards[i], nil
		}
	}
	return nil, nil
}

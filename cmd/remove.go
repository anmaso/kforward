package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/amarin/kforward/internal/forward"
	"github.com/amarin/kforward/internal/selector"
	"github.com/amarin/kforward/internal/state"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Stop an existing port forward",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runRemove(); err != nil {
			exitWithError(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove() error {
	forwards, err := state.ListForwards()
	if err != nil {
		return err
	}
	if len(forwards) == 0 {
		return fmt.Errorf("no active port forwards found")
	}

	labels := make([]string, len(forwards))
	byLabel := make(map[string]state.Forward, len(forwards))
	for i, f := range forwards {
		labels[i] = f.Label()
		byLabel[f.Label()] = f
	}

	selected, err := selector.ChooseForward(labels)
	if err != nil {
		return err
	}

	chosen, ok := byLabel[selected]
	if !ok {
		return fmt.Errorf("selected port forward not found")
	}

	if err := forward.Stop(chosen); err != nil {
		return err
	}

	fmt.Printf("Removed port forward: %s\n", chosen.Label())
	return nil
}

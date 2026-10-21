package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootInteractive bool

var rootCmd = &cobra.Command{
	Use:   "kforward",
	Short: "Manage Kubernetes port forwards",
	Long:  "kforward creates and manages background kubectl port-forward processes.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// With -i, skip the summary and open the interactive menu.
		if rootInteractive {
			if err := runMenu(cmd.Context()); err != nil {
				exitWithError(err)
			}
			return
		}

		// With no arguments, print the status, then the help, and exit.
		if err := runStatus(cmd.Context(), false); err != nil {
			exitWithError(err)
		}
		fmt.Println()
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&rootInteractive, "interactive", "i", false, "open an interactive menu to choose a command")
}

func Execute() error {
	return rootCmd.Execute()
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

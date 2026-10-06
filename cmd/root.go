package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "kforward",
	Short: "Manage Kubernetes port forwards",
	Long:  "kforward creates and manages background kubectl port-forward processes.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
		fmt.Println()
		if err := runMenu(cmd.Context()); err != nil {
			exitWithError(err)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "eve-rails",
	Short:         "Rails-inspired convention layer for Eve agent fleets",
	Long:          "eve-rails is a Rails-inspired convention layer for Eve agent fleets.",
	Version:       "0.1.1",
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

// Execute runs the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(wizardCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(renderCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(outdatedCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(hotloadCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(previewCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(graphCmd)
}

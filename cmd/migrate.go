package cmd

import (
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/migrate"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Plan or apply behavior and memory migrations",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		if !command.Flags().Changed("env") {
			env = "staging"
		}
		plan, err := migrate.Run(migrate.Options{
			Agent:    agentOpt,
			Env:      env,
			Manifest: manifest,
			Fleet:    fleet,
			Apply:    apply,
			DryRun:   dryRun,
			JSON:     jsonOutput,
		})
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(plan)
		}
		manifestPath := fleet
		if manifestPath == "" {
			manifestPath = manifest
		}
		plan.Print(manifestPath)
		return nil
	},
}

func init() {
	migrateCmd.Flags().StringVar(&agentOpt, "agent", "", "Agent to migrate")
	migrateCmd.Flags().StringVar(&fleet, "fleet", "", "Fleet manifest to migrate")
	migrateCmd.Flags().StringVar(&env, "env", "staging", "Environment name")
	migrateCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	migrateCmd.Flags().BoolVar(&apply, "apply", false, "Apply migration status changes. Omit for dry-run plan")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show migration plan without writing files")
	migrateCmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
}

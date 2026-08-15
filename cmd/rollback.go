package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/process"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/rollback"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Plan rollback by agent or component version",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		if !command.Flags().Changed("env") {
			env = "staging"
		}
		manifestData, err := config.LoadManifest(manifest)
		if err != nil {
			return err
		}
		catalogData, err := config.LoadCatalog(catalog)
		if err != nil {
			return err
		}
		report, err := rollback.Plan(rollback.Options{
			Agent:       agentOpt,
			To:          to,
			Deployment:  deployment,
			Component:   component,
			Env:         env,
			Manifest:    manifest,
			Catalog:     catalog,
			TemplateDir: templateDir,
			DryRun:      dryRun,
			JSON:        jsonOutput,
		}, manifestData, catalogData)
		if err != nil {
			return err
		}

		if jsonOutput {
			if err := printJSON(report); err != nil {
				return err
			}
			if report.Blocked {
				return fmt.Errorf("rollback requires migration")
			}
			return nil
		}

		report.Print()
		if report.Blocked {
			return fmt.Errorf("rollback requires migration")
		}
		if len(report.DelegatedCommand) == 0 || dryRun {
			return nil
		}
		agentDir := filepath.Join("agents", agentOpt)
		return process.Run(agentDir, report.DelegatedCommand)
	},
}

func init() {
	rollbackCmd.Flags().StringVar(&agentOpt, "agent", "", "Agent to roll back")
	rollbackCmd.Flags().StringVar(&to, "to", "", "Target agent version")
	rollbackCmd.Flags().StringVar(&deployment, "deployment", "", "Deployed Eve/Vercel deployment id to roll back to")
	rollbackCmd.Flags().StringVar(&deployment, "deployment-id", "", "Deployed Eve/Vercel deployment id to roll back to")
	rollbackCmd.Flags().StringVar(&component, "component", "", "Component rollback reference such as skill:handle_refund@1.0.0")
	rollbackCmd.Flags().StringVar(&env, "env", "staging", "Deployment environment")
	rollbackCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	rollbackCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	rollbackCmd.Flags().StringVar(&templateDir, "template-dir", "templates/agent", "Path to the template directory")
	rollbackCmd.Flags().StringVar(&templateDir, "templates", "templates/agent", "Path to the template directory")
	rollbackCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show rollback plan and delegated command without invoking Eve")
	rollbackCmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
	_ = rollbackCmd.MarkFlagRequired("agent")
}

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/deploy"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/process"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Run deploy gates and delegate non-dry runs to Eve/Vercel",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		if !command.Flags().Changed("env") {
			env = "staging"
		}
		manifestPath := fleet
		if manifestPath == "" {
			manifestPath = manifest
		}
		manifestData, err := config.LoadManifest(manifestPath)
		if err != nil {
			return err
		}
		catalogData, err := config.LoadCatalog(catalog)
		if err != nil {
			return err
		}
		report, err := deploy.Preflight(deploy.Options{
			Agent:            agentOpt,
			Env:              env,
			RequireEvals:     requireEvals,
			RequireDoctor:    requireDoctor,
			RequireApprovals: requireApprovals,
			Canary:           uint64(canary),
			Promote:          promote,
			RollbackTo:       rollbackTo,
			DryRun:           dryRun,
			JSON:             jsonOutput,
			Manifest:         manifestPath,
			Catalog:          catalog,
			TemplateDir:      templateDir,
		}, manifestData, catalogData)
		if err != nil {
			return err
		}
		passed := report.Passed()

		if jsonOutput {
			if err := printJSON(report); err != nil {
				return err
			}
			if !passed {
				return fmt.Errorf("deploy preflight failed")
			}
			return nil
		}

		fmt.Printf("Deploy preflight for %s\n", report.Env)
		if report.Agent != "" {
			fmt.Printf("Agent: %s\n", report.Agent)
		}
		if canary != 0 {
			fmt.Printf("Canary: %d%%\n", canary)
		}
		for _, gate := range report.Gates {
			label := "pass"
			if !gate.Passed {
				label = "fail"
			}
			fmt.Printf("[%s] %s - %s\n", label, gate.Name, gate.Message)
		}
		fmt.Printf("Delegated command: %s\n", joinCommand(report.DelegatedCommand))
		if dryRun {
			fmt.Println("Dry run only; Eve deploy was not invoked.")
		}

		if !passed {
			return fmt.Errorf("deploy preflight failed")
		}
		if dryRun {
			return nil
		}
		if agentOpt == "" {
			return fmt.Errorf("non-dry deploy requires --agent <name>")
		}
		agentDir := filepath.Join("agents", agentOpt)
		return process.Run(agentDir, report.DelegatedCommand)
	},
}

func joinCommand(commandLine []string) string {
	var result string
	for i, part := range commandLine {
		if i > 0 {
			result += " "
		}
		result += part
	}
	return result
}

func init() {
	deployCmd.Flags().StringVar(&agentOpt, "agent", "", "Agent to deploy")
	deployCmd.Flags().StringVar(&fleet, "fleet", "", "Fleet manifest to deploy")
	deployCmd.Flags().StringVar(&env, "env", "staging", "Deployment environment")
	deployCmd.Flags().BoolVar(&requireEvals, "require-evals", false, "Require eval pass before deployment")
	deployCmd.Flags().BoolVar(&requireDoctor, "require-doctor", false, "Require doctor pass before deployment")
	deployCmd.Flags().BoolVar(&requireApprovals, "require-approvals", false, "Require approval coverage before deployment")
	deployCmd.Flags().Uint8Var(&canary, "canary", 0, "Percentage of traffic for canary deployment")
	deployCmd.Flags().BoolVar(&promote, "promote", false, "Promote a previous or canary deployment")
	deployCmd.Flags().StringVar(&rollbackTo, "rollback-to", "", "Roll back to deployment id")
	deployCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show deploy gates and delegated command without invoking Eve")
	deployCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	deployCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	deployCmd.Flags().StringVar(&templateDir, "templates", "templates/agent", "Path to the template directory")
	deployCmd.Flags().StringVar(&templateDir, "template-dir", "templates/agent", "Path to the template directory")
	deployCmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
}

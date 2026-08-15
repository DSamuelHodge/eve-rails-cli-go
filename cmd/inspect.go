package cmd

import (
	"fmt"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/doctor"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/inspect"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect one agent's composition",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		manifestData, err := config.LoadManifest(manifest)
		if err != nil {
			return err
		}
		catalogData, err := config.LoadCatalog(catalog)
		if err != nil {
			return err
		}
		report := doctor.ValidateManifest(manifestData, catalogData)
		if err := doctor.EnsureValid(report); err != nil {
			return err
		}
		var agentData *config.AgentManifest
		for i := range manifestData.Agents {
			if manifestData.Agents[i].Name == agentOpt {
				agentData = &manifestData.Agents[i]
				break
			}
		}
		if agentData == nil {
			return fmt.Errorf("agent '%s' not found", agentOpt)
		}
		summary, err := inspect.Summarize(agentData, manifestData, catalogData, templateDir)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(summary)
		}

		fmt.Println(summary.Name)
		fmt.Printf("  version: %s\n", summary.Version)
		fmt.Printf("  owner: %s\n", orMissing(summary.Owner))
		fmt.Printf("  model: %s\n", orMissing(summary.Model))
		fmt.Printf("  responsibility: %s\n", summary.Responsibility)
		fmt.Printf("  tools: %s\n", inspect.FormatComponents(summary.Tools))
		fmt.Printf("  skills: %s\n", inspect.FormatComponents(summary.Skills))
		fmt.Printf("  runtime sandbox: %s\n", orUnspecified(summary.RuntimePolicy))
		fmt.Printf("  runtime allowed tools: %s\n", inspect.FormatList(allowedTools(summary.RuntimePolicy)))
		fmt.Println("  subagents:")
		for _, subagent := range summary.Subagents {
			title := subagent.Title
			if title == "" {
				title = "untitled role"
			}
			fmt.Printf("    - %s (%s)\n", subagent.Name, title)
			fmt.Printf("      sandbox: %s\n", orUnspecified(subagent.RuntimePolicy))
			if subagent.Responsibility != "" {
				fmt.Printf("      responsibility: %s\n", subagent.Responsibility)
			}
		}
		fmt.Printf("  channels: %s\n", inspect.FormatList(summary.Channels))
		fmt.Printf("  schedules: %s\n", inspect.FormatList(summary.Schedules))
		fmt.Printf("  approvals: %d\n", len(summary.Approvals))
		fmt.Printf("  evals: %s\n", inspect.FormatList(summary.Evals))
		fmt.Printf("  memory: %s\n", inspect.FormatComponents(summary.Memory))
		fmt.Printf("  generated fresh: %v\n", summary.Generated.Fresh)
		fmt.Printf("  deployable: %v\n", summary.Deployment.Deployable)
		return nil
	},
}

func orMissing(value string) string {
	if value == "" {
		return "<missing>"
	}
	return value
}

func orUnspecified(policy *config.RuntimePolicy) string {
	if policy == nil || policy.Sandbox == "" {
		return "<unspecified>"
	}
	return policy.Sandbox
}

func allowedTools(policy *config.RuntimePolicy) []string {
	if policy == nil {
		return nil
	}
	return policy.AllowedTools
}

func init() {
	inspectCmd.Flags().StringVar(&agentOpt, "agent", "", "Agent to inspect")
	inspectCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	inspectCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	inspectCmd.Flags().StringVar(&templateDir, "template-dir", "templates/agent", "Path to the template directory")
	inspectCmd.Flags().StringVar(&templateDir, "templates", "templates/agent", "Path to the template directory")
	inspectCmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
	_ = inspectCmd.MarkFlagRequired("agent")
}

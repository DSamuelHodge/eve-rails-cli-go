package cmd

import (
	"fmt"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/doctor"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/render"
	"github.com/spf13/cobra"
)

func planBatchPrint(manifestData *config.FleetManifest, catalogData *config.CatalogManifest) error {
	report := doctor.ValidateManifest(manifestData, catalogData)
	batch, err := render.PlanBatch(manifestData, catalogData, templates)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(map[string]any{
			"manifest":    manifest,
			"catalog":     catalog,
			"templates":   templates,
			"agent_count": len(manifestData.Agents),
			"summary":     batch.Summary(),
			"operations":  batch.Report(),
			"errors":      report.Errors,
			"warnings":    report.Warnings,
		})
	}

	fmt.Printf("Plan for %s\n", manifest)
	fmt.Printf("Catalog: %s\n", catalog)
	fmt.Printf("Templates: %s\n", templates)
	fmt.Printf("Agents: %d\n", len(manifestData.Agents))
	render.PrintBatchSummary(batch)
	fmt.Printf("Shared: %d tools, %d skills, %d memory schemas\n",
		len(manifestData.Shared.Tools), len(manifestData.Shared.Skills), len(manifestData.Shared.Memory))

	for i := range manifestData.Agents {
		agent := &manifestData.Agents[i]
		fmt.Println()
		fmt.Printf("- %s\n", agent.Name)
		fmt.Printf("  responsibility: %s\n", agent.Responsibility)
		version := agent.Version
		if version == "" {
			version = "1.0.0"
		}
		fmt.Printf("  version: %s\n", version)
		agentModel := agent.Model
		if agentModel == "" {
			agentModel = manifestData.Defaults.Model
		}
		if agentModel == "" {
			agentModel = "<missing>"
		}
		fmt.Printf("  model: %s\n", agentModel)
		agentOwner := agent.Owner
		if agentOwner == "" {
			agentOwner = manifestData.Defaults.Owner
		}
		if agentOwner == "" {
			agentOwner = "<missing>"
		}
		fmt.Printf("  owner: %s\n", agentOwner)
		fmt.Printf("  tools: %d\n", len(agent.Tools))
		fmt.Printf("  skills: %d\n", len(agent.Skills))
		fmt.Printf("  subagents: %d\n", len(agent.Subagents))
		fmt.Printf("  channels: %d\n", len(agent.Channels)+len(manifestData.Defaults.Channels))
		fmt.Printf("  schedules: %d\n", len(config.EffectiveSchedules(agent, manifestData)))
		approvals := "<none>"
		if len(agent.Approvals) == 0 {
			approvals = manifestData.Defaults.Approvals
			if approvals == "" {
				approvals = "<none>"
			}
		} else {
			approvals = "custom"
		}
		fmt.Printf("  approvals: %s\n", approvals)
		fmt.Printf("  evals: %d\n", len(agent.Evals)+len(manifestData.Defaults.Evals))
		fmt.Printf("  memory: %d\n", len(agent.Memory))
	}

	doctor.PrintValidationReport(report)
	return nil
}

var planCmd = &cobra.Command{
	Use:   "plan [manifest]",
	Short: "Show the files and components that would be generated from a manifest",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) > 0 {
			manifest = args[0]
		}
		manifestData, err := config.LoadManifest(manifest)
		if err != nil {
			return err
		}
		catalogData, err := config.LoadCatalog(catalog)
		if err != nil {
			return err
		}
		return planBatchPrint(manifestData, catalogData)
	},
}

// applyBatch validates a manifest and renders generated files, sharing the
// implementation between the apply command and the interactive wizard.
func applyBatch(manifestData *config.FleetManifest, catalogData *config.CatalogManifest) error {
	report := doctor.ValidateManifest(manifestData, catalogData)
	if err := doctor.EnsureValid(report); err != nil {
		return err
	}
	batch, err := render.PlanBatch(manifestData, catalogData, templates)
	if err != nil {
		return err
	}

	for _, operation := range batch.Operations {
		switch operation.Action {
		case render.BatchSkip:
			continue
		default:
			if err := writeGeneratedFile(operation.Path, operation.Content); err != nil {
				return err
			}
		}
	}

	if jsonOutput {
		return printJSON(map[string]any{
			"manifest":    manifest,
			"catalog":     catalog,
			"templates":   templates,
			"agent_count": len(manifestData.Agents),
			"summary":     batch.Summary(),
			"operations":  batch.Report(),
		})
	}

	fmt.Printf("Applied %s\n", manifest)
	render.PrintBatchSummary(batch)
	for _, operation := range batch.Operations {
		if operation.Action != render.BatchSkip {
			fmt.Printf("%s %s\n", operation.Action, operation.Path)
		}
	}
	return nil
}

var applyCmd = &cobra.Command{
	Use:   "apply [manifest]",
	Short: "Apply a manifest by rendering generated files",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) > 0 {
			manifest = args[0]
		}
		manifestData, err := config.LoadManifest(manifest)
		if err != nil {
			return err
		}
		catalogData, err := config.LoadCatalog(catalog)
		if err != nil {
			return err
		}
		return applyBatch(manifestData, catalogData)
	},
}

var outdatedCmd = &cobra.Command{
	Use:   "outdated [manifest]",
	Short: "Show available component updates",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) > 0 {
			manifest = args[0]
		}
		manifestData, err := config.LoadManifest(manifest)
		if err != nil {
			return err
		}
		catalogData, err := config.LoadCatalog(catalog)
		if err != nil {
			return err
		}
		updatesList := collectVersionReports(manifestData, catalogData, "")

		if jsonOutput {
			return printJSON(map[string]any{
				"manifest": manifest,
				"catalog":  catalog,
				"updates":  updatesList,
			})
		}

		fmt.Printf("Outdated report for %s\n", manifest)
		printVersionReports(updatesList)
		return nil
	},
}

func addManifestFlags(command *cobra.Command) {
	command.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	command.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	command.Flags().StringVar(&templates, "templates", "templates/agent", "Path to the template directory")
	command.Flags().StringVar(&templates, "template-dir", "templates/agent", "Path to the template directory")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
}

func writeGeneratedFile(path, content string) error {
	if parent := parentDir(path); parent != "" {
		if err := mkdirAll(parent); err != nil {
			return err
		}
	}
	return writeFile(path, content)
}

func init() {
	addManifestFlags(planCmd)
	addManifestFlags(applyCmd)
	addManifestFlags(outdatedCmd)
}

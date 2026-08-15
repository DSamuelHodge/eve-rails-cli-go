package cmd

import (
	"fmt"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/doctor"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/render"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate project, manifest, templates, versions, and deployment readiness",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		env = ""
		manifestData, err := config.LoadManifest(manifest)
		if err != nil {
			return err
		}
		catalogData, err := config.LoadCatalog(catalog)
		if err != nil {
			return err
		}
		report, err := doctor.RunDoctor(manifestData, catalogData, doctor.Options{
			All:          all,
			Updates:      updates,
			Templates:    templatesFlag,
			Fix:          fix,
			DryRun:       dryRun,
			Env:          env,
			Connections:  connections,
			Budgets:      budgets,
			JSON:         jsonOutput,
			Manifest:     manifest,
			Catalog:      catalog,
			TemplateDir:  templateDir,
			Environments: environments,
		})
		if err != nil {
			return err
		}
		passed := !report.HasFailures()

		if jsonOutput {
			output := map[string]any{
				"manifest":       manifest,
				"catalog":        catalog,
				"templates_path": templateDir,
				"all":            all,
				"updates":        updates,
				"templates":      templatesFlag,
				"dry_run":        dryRun,
				"passed":         passed,
				"checks":         report.Checks,
			}
			if err := printJSON(output); err != nil {
				return err
			}
			if !passed {
				return fmt.Errorf("doctor failed")
			}
			return nil
		}

		fmt.Printf("Doctor for %s\n", manifest)
		fmt.Printf("Catalog: %s\n", catalog)
		fmt.Printf("Templates: %s\n", templateDir)
		fmt.Printf("Agents checked: %d\n", len(manifestData.Agents))
		doctor.PrintReport(report)

		if fix {
			changed, err := applyDoctorFixes(manifestData, catalogData)
			if err != nil {
				return err
			}
			if len(changed) == 0 {
				fmt.Println("No generated-file repairs needed.")
			} else {
				for _, operation := range changed {
					if dryRun {
						fmt.Printf("would %s %s\n", operation.Action, operation.Path)
					} else {
						fmt.Printf("%s %s\n", operation.Action, operation.Path)
					}
				}
			}
		}

		if passed {
			return nil
		}
		return fmt.Errorf("doctor failed")
	},
}

func applyDoctorFixes(manifestData *config.FleetManifest, catalogData *config.CatalogManifest) ([]render.BatchOperationReport, error) {
	plan, err := render.PlanBatch(manifestData, catalogData, templateDir)
	if err != nil {
		return nil, err
	}
	var changed []render.BatchOperationReport
	for _, operation := range plan.Operations {
		if operation.Action == render.BatchSkip {
			continue
		}
		changed = append(changed, render.BatchOperationReport{Path: operation.Path, Action: operation.Action})
		if dryRun {
			continue
		}
		if err := writeGeneratedFile(operation.Path, operation.Content); err != nil {
			return nil, err
		}
	}
	return changed, nil
}

func init() {
	doctorCmd.Flags().BoolVar(&all, "all", false, "Validate all agents")
	doctorCmd.Flags().BoolVar(&updates, "updates", false, "Validate update and lockfile compatibility")
	doctorCmd.Flags().BoolVar(&templatesFlag, "templates", false, "Validate template rendering behavior")
	doctorCmd.Flags().BoolVar(&fix, "fix", false, "Plan safe mechanical repairs")
	doctorCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show planned repairs and checks without writing files")
	doctorCmd.Flags().StringVar(&env, "env", "", "Environment name from environments.yml")
	doctorCmd.Flags().BoolVar(&connections, "connections", false, "Validate required connections and secrets for the selected environment")
	doctorCmd.Flags().BoolVar(&budgets, "budgets", false, "Validate cost, token, and timeout budgets")
	doctorCmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
	doctorCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	doctorCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	doctorCmd.Flags().StringVar(&templateDir, "template-dir", "templates/agent", "Path to the template directory")
	doctorCmd.Flags().StringVar(&environments, "environments", "manifests/environments.yml", "Path to environment policy config")
}

package cmd

import (
	"fmt"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/doctor"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/render"
	"github.com/spf13/cobra"
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render generated files from YAML and templates",
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

		agents, err := config.SelectedAgents(manifestData, agentOpt)
		if err != nil {
			return err
		}
		if !all && agentOpt == "" {
			return fmt.Errorf("pass --agent <name> or --all")
		}

		renderer, err := render.Load(templates)
		if err != nil {
			return err
		}
		var stale []string
		for _, agent := range agents {
			files, err := renderer.RenderAgent(agent, manifestData, catalogData)
			if err != nil {
				return err
			}
			for _, rendered := range files {
				if check {
					matches, err := fileMatches(rendered.Path, rendered.Content)
					if err != nil {
						return err
					}
					if !matches {
						stale = append(stale, rendered.Path)
					}
				} else {
					if err := writeGeneratedFile(rendered.Path, rendered.Content); err != nil {
						return err
					}
					fmt.Printf("wrote %s\n", rendered.Path)
				}
			}
		}

		if check {
			if len(stale) == 0 {
				fmt.Println("Generated files are fresh.")
			} else {
				for _, path := range stale {
					fmt.Printf("stale %s\n", path)
				}
				return fmt.Errorf("%d generated file(s) are stale", len(stale))
			}
		}
		return nil
	},
}

func fileMatches(path, expected string) (bool, error) {
	return readFileMatches(path, expected)
}

func init() {
	renderCmd.Flags().StringVar(&agentOpt, "agent", "", "Render a single agent by name")
	renderCmd.Flags().BoolVar(&all, "all", false, "Render every agent in the manifest")
	renderCmd.Flags().BoolVar(&check, "check", false, "Check whether generated files are fresh without writing")
	renderCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	renderCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	renderCmd.Flags().StringVar(&templates, "templates", "templates/agent", "Path to the template directory")
	renderCmd.Flags().StringVar(&templates, "template-dir", "templates/agent", "Path to the template directory")
}

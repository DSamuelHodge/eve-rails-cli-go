package cmd

import (
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/doctor"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/graph"
	"github.com/spf13/cobra"
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Print a fleet graph",
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
		return graph.Run(graph.Options{
			All:         all,
			Agent:       agentOpt,
			Format:      graph.Format(format),
			Mode:        graph.Mode(mode),
			Manifest:    manifest,
			Catalog:     catalog,
			TemplateDir: templateDir,
			JSON:        jsonOutput,
		}, manifestData, catalogData)
	},
}

func init() {
	graphCmd.Flags().BoolVar(&all, "all", false, "Include every agent in the manifest")
	graphCmd.Flags().StringVar(&agentOpt, "agent", "", "Graph one agent")
	graphCmd.Flags().StringVar(&format, "format", "text", "Output format")
	graphCmd.Flags().StringVar(&mode, "mode", "all", "Graph concern to show")
	graphCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	graphCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	graphCmd.Flags().StringVar(&templateDir, "template-dir", "templates/agent", "Path to the template directory")
	graphCmd.Flags().StringVar(&templateDir, "templates", "templates/agent", "Path to the template directory")
}

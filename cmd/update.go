package cmd

import (
	"fmt"
	"os"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/versioning"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Plan or apply component updates",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
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
		if agentOpt != "" {
			found := false
			for i := range manifestData.Agents {
				if manifestData.Agents[i].Name == agentOpt {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("agent '%s' not found", agentOpt)
			}
		}
		updatesList := collectVersionReports(manifestData, catalogData, agentOpt)

		if jsonOutput {
			return printJSON(map[string]any{
				"manifest": manifestPath,
				"catalog":  catalog,
				"mode":     "update-plan",
				"updates":  updatesList,
			})
		}

		fmt.Printf("Update plan for %s\n", manifestPath)
		printVersionReports(updatesList)
		if apply {
			changed, err := applyUpdates(manifestPath, updatesList)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("Updated %s\n", manifestPath)
			} else {
				fmt.Println("No applicable updates.")
			}
		}
		return nil
	},
}

func applyUpdates(manifestPath string, reports []versioning.VersionReport) (bool, error) {
	var applicable []struct {
		report   versioning.VersionReport
		resolved string
	}
	for _, report := range reports {
		if !updateAllowed(report.Update) || report.Resolved == "" || report.Requested == report.Resolved {
			continue
		}
		applicable = append(applicable, struct {
			report   versioning.VersionReport
			resolved string
		}{report: report, resolved: report.Resolved})
	}

	if len(applicable) == 0 {
		return false, nil
	}

	source, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, fmt.Errorf("failed to read manifest '%s': %w", manifestPath, err)
	}
	var manifestValue yaml.Node
	if err := yaml.Unmarshal(source, &manifestValue); err != nil {
		return false, fmt.Errorf("failed to parse manifest '%s': %w", manifestPath, err)
	}

	for _, entry := range applicable {
		if err := updateManifestComponentVersion(&manifestValue, entry.report, entry.resolved); err != nil {
			return false, err
		}
	}

	rendered, err := yaml.Marshal(&manifestValue)
	if err != nil {
		return false, fmt.Errorf("failed to render updated manifest '%s': %w", manifestPath, err)
	}
	if err := os.WriteFile(manifestPath, rendered, 0o644); err != nil {
		return false, fmt.Errorf("failed to write manifest '%s': %w", manifestPath, err)
	}
	fmt.Printf("Resolved versions from %s\n", catalog)
	return true, nil
}

func updateAllowed(update versioning.UpdateKind) bool {
	return (update == versioning.UpdatePatch && patch) ||
		(update == versioning.UpdateMinor && minor) ||
		(update == versioning.UpdateMajor && major)
}

func updateManifestComponentVersion(manifest *yaml.Node, report versioning.VersionReport, resolved string) error {
	root := manifest.Content[0]
	var agentsNode *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "agents" {
			agentsNode = root.Content[i+1]
			break
		}
	}
	if agentsNode == nil || agentsNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("manifest must contain an agents list")
	}
	var agentNode *yaml.Node
	for _, node := range agentsNode.Content {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "name" && node.Content[i+1].Value == report.Agent {
				agentNode = node
				break
			}
		}
		if agentNode != nil {
			break
		}
	}
	if agentNode == nil {
		return fmt.Errorf("agent '%s' not found", report.Agent)
	}

	section := ""
	switch report.ComponentKind {
	case "tool":
		section = "tools"
	case "skill":
		section = "skills"
	case "memory":
		section = "memory"
	default:
		return fmt.Errorf("cannot safely apply %s updates in manifest maps", report.ComponentKind)
	}

	var sectionNode *yaml.Node
	for i := 0; i+1 < len(agentNode.Content); i += 2 {
		if agentNode.Content[i].Value == section {
			sectionNode = agentNode.Content[i+1]
			break
		}
	}
	if sectionNode == nil || sectionNode.Kind != yaml.MappingNode {
		return fmt.Errorf("agent '%s' has no %s map entry to update", report.Agent, section)
	}
	for i := 0; i+1 < len(sectionNode.Content); i += 2 {
		if sectionNode.Content[i].Value == report.Component {
			sectionNode.Content[i+1].Value = resolved
			return nil
		}
	}
	return fmt.Errorf("agent '%s' has no %s entry '%s'", report.Agent, section, report.Component)
}

func init() {
	updateCmd.Flags().StringVar(&agentOpt, "agent", "", "Agent to update")
	updateCmd.Flags().StringVar(&fleet, "fleet", "", "Fleet manifest to update")
	updateCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	updateCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	updateCmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
	updateCmd.Flags().BoolVar(&patch, "patch", false, "Allow patch updates")
	updateCmd.Flags().BoolVar(&minor, "minor", false, "Allow minor updates")
	updateCmd.Flags().BoolVar(&major, "major", false, "Allow major updates")
	updateCmd.Flags().BoolVar(&plan, "plan", false, "Show plan without applying")
	updateCmd.Flags().BoolVar(&apply, "apply", false, "Apply the update")
}

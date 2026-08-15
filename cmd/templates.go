package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	agenttemplates "github.com/DSamuelHodge/eve-rails-cli-go/templates"
	"github.com/spf13/cobra"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Inspect and upgrade project render templates",
	Args:  cobra.NoArgs,
}

var templatesCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Report whether project templates match this CLI's embedded templates",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		report, err := agenttemplates.Compare(templateDir)
		if err != nil {
			return err
		}
		stale := len(report.Missing) > 0 || len(report.Differing) > 0
		if jsonOutput {
			return printJSON(map[string]any{
				"template_dir": templateDir,
				"fresh":        !stale,
				"missing":      report.Missing,
				"differing":    report.Differing,
			})
		}
		fmt.Printf("Templates: %s\n", templateDir)
		if stale {
			fmt.Printf("Project templates are older than this CLI.\n")
			if len(report.Missing) > 0 {
				fmt.Printf("Missing: %v\n", report.Missing)
			}
			if len(report.Differing) > 0 {
				fmt.Printf("Differing: %v\n", report.Differing)
			}
			fmt.Println("Run `eve-rails templates upgrade` to refresh them.")
		} else {
			fmt.Println("Project templates match this CLI.")
		}
		return nil
	},
}

var templatesUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Copy this CLI's embedded templates into the project template directory",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, args []string) error {
		changes, err := agenttemplates.Upgrade(templateDir)
		if err != nil {
			return err
		}
		for _, change := range changes {
			if dryRun {
				if !jsonOutput {
					fmt.Printf("would %s %s\n", change.Action, change.Path)
				}
				continue
			}
			if err := writeTemplateChange(change); err != nil {
				return err
			}
			if !jsonOutput {
				fmt.Printf("%s %s\n", change.Action, change.Path)
			}
		}
		if jsonOutput {
			changeList := make([]map[string]string, 0, len(changes))
			for _, change := range changes {
				changeList = append(changeList, map[string]string{"path": change.Path, "action": change.Action})
			}
			return printJSON(map[string]any{
				"template_dir": templateDir,
				"dry_run":      dryRun,
				"changes":      changeList,
			})
		}
		fmt.Printf("Templates: %s\n", templateDir)
		if len(changes) == 0 {
			fmt.Println("No template changes.")
		}
		return nil
	},
}

func writeTemplateChange(change agenttemplates.Change) error {
	if parent := filepath.Dir(change.Path); parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("failed to create '%s': %w", parent, err)
		}
	}
	return os.WriteFile(change.Path, []byte(change.Content), 0o644)
}

func init() {
	templatesCmd.AddCommand(templatesCheckCmd)
	templatesCmd.AddCommand(templatesUpgradeCmd)

	templatesCmd.PersistentFlags().StringVar(&templateDir, "template-dir", "templates/agent", "Path to the template directory")
	templatesCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show planned changes without writing files")
	templatesCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
}

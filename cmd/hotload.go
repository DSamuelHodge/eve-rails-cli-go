package cmd

import (
	"fmt"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/hotload"
	"github.com/spf13/cobra"
)

var hotloadCmd = &cobra.Command{
	Use:   "hotload <component>",
	Short: "Classify a component update as hot-loadable or redeploy-only",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		ref, err := hotload.Parse(args[0])
		if err != nil {
			return err
		}
		classification, err := hotload.Classify(ref, current)
		if err != nil {
			return err
		}

		if jsonOutput {
			if err := printJSON(map[string]any{
				"agent":          agentOpt,
				"component":      ref,
				"current":        current,
				"classification": classification,
			}); err != nil {
				return err
			}
			if classification.Action != hotload.ActionHotload {
				return fmt.Errorf("component is not hot-loadable")
			}
			return nil
		}

		fmt.Printf("Hot-load check for agent %s\n", agentOpt)
		fmt.Printf("%s:%s %s -> %s\n", ref.Kind, ref.Name, current, ref.Version)
		fmt.Printf("%s - %s\n", classification.Action, classification.Reason)

		if classification.Action != hotload.ActionHotload {
			return fmt.Errorf("component is not hot-loadable")
		}
		return nil
	},
}

func init() {
	hotloadCmd.Flags().StringVar(&agentOpt, "agent", "", "Agent receiving the compatible update")
	hotloadCmd.Flags().StringVar(&current, "current", "1.0.0", "Current running component version to compare against")
	hotloadCmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
	_ = hotloadCmd.MarkFlagRequired("agent")
}

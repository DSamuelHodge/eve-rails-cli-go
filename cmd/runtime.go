package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/process"
	"github.com/spf13/cobra"
)

type runtimeKind string

const (
	runtimeEval    runtimeKind = "eval"
	runtimeTest    runtimeKind = "test"
	runtimePreview runtimeKind = "preview"
)

func newRuntimeCmd(kind runtimeKind) *cobra.Command {
	return &cobra.Command{
		Use:   string(kind),
		Short: runtimeShort(kind),
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			if !command.Flags().Changed("env") {
				env = "development"
			}
			manifestData, err := config.LoadManifest(manifest)
			if err != nil {
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
			agentDir := filepath.Join("agents", agentData.Name)
			commands := runtimeCommands(kind)

			if jsonOutput {
				return printJSON(map[string]any{
					"agent":     agentOpt,
					"env":       env,
					"kind":      kind,
					"agent_dir": agentDir,
					"dry_run":   dryRun,
					"commands":  commands,
				})
			}

			fmt.Printf("%s for agent %s\n", kind, agentData.Name)
			for _, commandLine := range commands {
				fmt.Printf("$ %s\n", joinCommand(commandLine))
			}
			if dryRun || kind == runtimePreview {
				if kind == runtimePreview {
					fmt.Println("Preview is long-running; run the command above to start Eve dev.")
				}
				return nil
			}
			for _, commandLine := range commands {
				if err := process.Run(agentDir, commandLine); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func runtimeShort(kind runtimeKind) string {
	switch kind {
	case runtimeEval:
		return "Delegate eval execution to Eve from a generated agent directory"
	case runtimeTest:
		return "Run CLI checks plus generated agent TypeScript and Eve runtime checks"
	case runtimePreview:
		return "Preview or print the Eve dev command for a generated agent"
	}
	return string(kind)
}

func runtimeCommands(kind runtimeKind) [][]string {
	switch kind {
	case runtimeEval:
		return [][]string{{"npm", "exec", "--", "eve", "eval"}}
	case runtimeTest:
		return [][]string{
			{"npm", "run", "typecheck"},
			{"npm", "exec", "--", "eve", "info", "--json"},
		}
	case runtimePreview:
		return [][]string{{"npm", "exec", "--", "eve", "dev", "--no-ui"}}
	}
	return nil
}

func addRuntimeFlags(command *cobra.Command) {
	command.Flags().StringVar(&agentOpt, "agent", "", "Agent to run")
	command.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	command.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	command.Flags().StringVar(&env, "env", "development", "Environment name")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Print command and checks without invoking long-running commands")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
}

var evalCmd = newRuntimeCmd(runtimeEval)
var testCmd = newRuntimeCmd(runtimeTest)
var previewCmd = newRuntimeCmd(runtimePreview)

func init() {
	addRuntimeFlags(evalCmd)
	addRuntimeFlags(testCmd)
	addRuntimeFlags(previewCmd)
	_ = evalCmd.MarkFlagRequired("agent")
	_ = testCmd.MarkFlagRequired("agent")
	_ = previewCmd.MarkFlagRequired("agent")
}

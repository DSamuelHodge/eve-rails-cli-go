package cmd

import (
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/generate"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate an agent component",
	Args:  cobra.NoArgs,
}

var (
	generateName          string
	generateVersion       string
	generateOwner         string
	generateModel         string
	generateDescription   string
	generateSideEffects   string
	generateRetention     string
	generateWithTools     []string
	generateWithSkills    []string
	generateWithSubagents []string
	generateWithChannels  []string
	generateWithEvals     []string
	generateWithMemory    []string
	generateWithSchedules []string
	generateApproval      string
	generateSchedule      string
	generateKind          string
	generateAllowFrom     string
	generateMessagingFrom string
	generateConnectUID    string
	generateBotUsername   string
	generateBotName       string
	generateRisk          string
	generateAuth          string
	generateVisibility    string
	generateCostBudget    float64
	generateTokenBudget   uint64
	generateTimeout       string
)

func generateOptions() generate.Options {
	var costBudget *float64
	if generateCostBudget != 0 {
		value := generateCostBudget
		costBudget = &value
	}
	var tokenBudget *uint64
	if generateTokenBudget != 0 {
		value := generateTokenBudget
		tokenBudget = &value
	}
	return generate.Options{
		Name:          generateName,
		Version:       generateVersion,
		Owner:         generateOwner,
		Description:   generateDescription,
		Model:         generateModel,
		SideEffects:   generateSideEffects,
		Retention:     generateRetention,
		WithTools:     generateWithTools,
		WithSkills:    generateWithSkills,
		WithSubagents: generateWithSubagents,
		WithChannels:  generateWithChannels,
		WithSchedules: generateWithSchedules,
		WithEvals:     generateWithEvals,
		WithMemory:    generateWithMemory,
		Approval:      generateApproval,
		Schedule:      generateSchedule,
		Kind:          generateKind,
		AllowFrom:     generateAllowFrom,
		MessagingFrom: generateMessagingFrom,
		ConnectUID:    generateConnectUID,
		BotUsername:   generateBotUsername,
		BotName:       generateBotName,
		Risk:          generateRisk,
		Auth:          generateAuth,
		Visibility:    generateVisibility,
		CostBudget:    costBudget,
		TokenBudget:   tokenBudget,
		Timeout:       generateTimeout,
		DryRun:        dryRun,
		Force:         force,
		JSON:          jsonOutput,
		Manifest:      manifest,
		Catalog:       catalog,
	}
}

func newGenerateCmd(kind generate.Kind, use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			generateName = args[0]
			options := generateOptions()
			changes, err := generate.Generate(kind, options)
			if err != nil {
				return err
			}
			if options.JSON {
				changeList := make([]map[string]string, 0, len(changes))
				for _, change := range changes {
					changeList = append(changeList, map[string]string{"path": change.Path, "action": string(change.Action)})
				}
				return printJSON(map[string]any{
					"kind":    kind,
					"name":    options.Name,
					"dry_run": options.DryRun,
					"force":   options.Force,
					"changes": changeList,
				})
			}
			return nil
		},
	}
}

var generateAgentCmd = newGenerateCmd(generate.KindAgent, "agent <name>", "Generate an agent manifest entry")
var generateToolCmd = newGenerateCmd(generate.KindTool, "tool <name>", "Generate a tool catalog entry and stub")
var generateSkillCmd = newGenerateCmd(generate.KindSkill, "skill <name>", "Generate a skill catalog entry and stub")
var generateSubagentCmd = newGenerateCmd(generate.KindSubagent, "subagent <name>", "Generate a subagent instructions file")
var generateChannelCmd = newGenerateCmd(generate.KindChannel, "channel <name>", "Generate a channel catalog entry and stub")
var generateScheduleCmd = newGenerateCmd(generate.KindSchedule, "schedule <name>", "Generate a schedule catalog entry and stub")
var generateApprovalCmd = newGenerateCmd(generate.KindApproval, "approval <name>", "Generate an approval catalog entry and stub")
var generateEvalCmd = newGenerateCmd(generate.KindEval, "eval <name>", "Generate an eval catalog entry and stub")
var generateMemoryCmd = newGenerateCmd(generate.KindMemory, "memory <name>", "Generate a memory catalog entry and stub")
var generateMigrationCmd = newGenerateCmd(generate.KindMigration, "migration <name>", "Generate a timestamped migration file")

func generateBatchRun(command *cobra.Command, args []string) error {
	manifestData, err := config.LoadManifest(manifest)
	if err != nil {
		return err
	}
	catalogData, err := config.LoadCatalog(catalog)
	if err != nil {
		return err
	}
	return planBatchPrint(manifestData, catalogData)
}

var generateBatchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Generate every agent from the manifest",
	Args:  cobra.NoArgs,
	RunE:  generateBatchRun,
}

func addGenerateFlags(command *cobra.Command) {
	command.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	command.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	command.Flags().StringVar(&generateVersion, "version", "1.0.0", "Component version")
	command.Flags().StringVar(&generateOwner, "owner", "", "Agent owner")
	command.Flags().StringVar(&generateModel, "model", "", "Agent model")
	command.Flags().StringVar(&generateDescription, "description", "", "Agent or component responsibility/description")
	command.Flags().StringVar(&generateSideEffects, "side-effects", "read", "Tool side-effect class")
	command.Flags().StringVar(&generateRetention, "retention", "", "Memory retention period, such as 180d")
	command.Flags().StringSliceVar(&generateWithTools, "with-tools", nil, "Tools to attach when generating an agent")
	command.Flags().StringSliceVar(&generateWithSkills, "with-skills", nil, "Skills to attach when generating an agent")
	command.Flags().StringSliceVar(&generateWithSubagents, "with-subagents", nil, "Subagents to attach when generating an agent")
	command.Flags().StringSliceVar(&generateWithChannels, "with-channels", nil, "Channels to attach when generating an agent")
	command.Flags().StringSliceVar(&generateWithEvals, "with-evals", nil, "Evals to attach when generating an agent")
	command.Flags().StringSliceVar(&generateWithMemory, "with-memory", nil, "Memory schemas to attach when generating an agent")
	command.Flags().StringSliceVar(&generateWithSchedules, "with-schedules", nil, "Schedules to attach when generating an agent")
	command.Flags().StringVar(&generateApproval, "approval", "", "Approval policy to apply to generated agent tools")
	command.Flags().StringVar(&generateSchedule, "schedule", "", "Cron expression for a generated schedule component")
	command.Flags().StringVar(&generateKind, "kind", "", "Channel kind for generated channel components, such as eve, slack, discord, telegram, or twilio")
	command.Flags().StringVar(&generateAllowFrom, "allow-from", "", "Allowed inbound Twilio sender, list, or env reference for generated Twilio channels")
	command.Flags().StringVar(&generateMessagingFrom, "messaging-from", "", "Outbound Twilio sender number or env reference for generated Twilio channels")
	command.Flags().StringVar(&generateConnectUID, "connect-uid", "", "Vercel Connect UID for generated Slack, Linear, or GitHub channels")
	command.Flags().StringVar(&generateBotUsername, "bot-username", "", "Telegram bot username for generated Telegram channels")
	command.Flags().StringVar(&generateBotName, "bot-name", "", "Bot name for generated GitHub channels")
	command.Flags().StringVar(&generateRisk, "risk", "", "Risk classification")
	command.Flags().StringVar(&generateAuth, "auth", "", "Auth profile for generated Eve routes, such as platform-oauth or http-basic-env")
	command.Flags().StringVar(&generateVisibility, "visibility", "", "Visibility policy")
	command.Flags().Float64Var(&generateCostBudget, "cost-budget", 0, "Cost budget for generated agent metadata")
	command.Flags().Uint64Var(&generateTokenBudget, "token-budget", 0, "Token budget for generated agent metadata")
	command.Flags().StringVar(&generateTimeout, "timeout", "", "Timeout for generated agent metadata")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Show planned changes without writing files")
	command.Flags().BoolVar(&force, "force", false, "Overwrite existing generated files where safe")
	command.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
}

func init() {
	generateCmd.AddCommand(generateAgentCmd)
	generateCmd.AddCommand(generateToolCmd)
	generateCmd.AddCommand(generateSkillCmd)
	generateCmd.AddCommand(generateSubagentCmd)
	generateCmd.AddCommand(generateChannelCmd)
	generateCmd.AddCommand(generateScheduleCmd)
	generateCmd.AddCommand(generateApprovalCmd)
	generateCmd.AddCommand(generateEvalCmd)
	generateCmd.AddCommand(generateMemoryCmd)
	generateCmd.AddCommand(generateMigrationCmd)
	generateCmd.AddCommand(generateBatchCmd)

	addGenerateFlags(generateAgentCmd)
	addGenerateFlags(generateToolCmd)
	addGenerateFlags(generateSkillCmd)
	addGenerateFlags(generateSubagentCmd)
	addGenerateFlags(generateChannelCmd)
	addGenerateFlags(generateScheduleCmd)
	addGenerateFlags(generateApprovalCmd)
	addGenerateFlags(generateEvalCmd)
	addGenerateFlags(generateMemoryCmd)
	addGenerateFlags(generateMigrationCmd)

	generateBatchCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	generateBatchCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	generateBatchCmd.Flags().StringVar(&templates, "templates", "templates/agent", "Path to the template directory")
	generateBatchCmd.Flags().StringVar(&templates, "template-dir", "templates/agent", "Path to the template directory")
	generateBatchCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show planned changes without writing files")
	generateBatchCmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON")
}

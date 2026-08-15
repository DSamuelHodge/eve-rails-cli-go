package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/generate"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/wizard"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// wizardErr is returned when the wizard cannot run interactively.
type wizardErr struct{ message string }

func (err wizardErr) Error() string { return err.message }

// requireTTY ensures the wizard only runs against an interactive terminal.
// Agents and CI never enter the TUI; they get a pointer to the flag-based
// commands instead.
func requireTTY() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return wizardErr{"wizard requires an interactive terminal; use init, generate, and apply with flags (or --json for machine output) instead"}
	}
	if jsonOutput {
		return wizardErr{"wizard does not support --json; use init, generate, and apply with --json for machine output"}
	}
	return nil
}

func wizardState() *wizard.State {
	projectDir, err := os.Getwd()
	if err != nil {
		projectDir = "eve-rails-project"
	}
	state := &wizard.State{
		Template: "basic",
		Model:    "openai/gpt-5.5",
		Owner:    "agent-platform",
	}
	if _, statErr := os.Stat(manifest); statErr == nil {
		if data, loadErr := config.LoadManifest(manifest); loadErr == nil {
			state.Model = data.Defaults.Model
			state.Owner = data.Defaults.Owner
		}
	} else {
		state.ProjectName = filepath.Base(projectDir)
	}
	return state
}

// wizardInitStep collects the project init fields for greenfield projects.
func wizardInitStep(state *wizard.State) error {
	var template string
	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Placeholder("eve-rails-project").
				Value(&state.ProjectName),
			huh.NewSelect[string]().
				Title("Starter template").
				Options(
					huh.NewOption("Basic", "basic"),
					huh.NewOption("Customer support", "customer-support"),
				).
				Value(&template),
			huh.NewInput().
				Title("Default model").
				Value(&state.Model),
			huh.NewInput().
				Title("Default owner").
				Value(&state.Owner),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Create project '%s' with the %s template?", state.ProjectName, template)).
				Affirmative("Create").
				Negative("Review").
				Value(&confirm),
		),
	)
	if err := form.Run(); err != nil {
		return err
	}
	if template != "" {
		state.Template = template
	}
	if confirm {
		return wizardApplyInit(state)
	}
	return nil
}

// wizardApplyInit writes the project skeleton using the shared init changes.
func wizardApplyInit(state *wizard.State) error {
	params := state.ToInitParams()
	if params.Template != "basic" && params.Template != "customer-support" {
		return fmt.Errorf("unknown template '%s'; expected basic or customer-support", params.Template)
	}
	previous := model
	model = params.Model
	defer func() { model = previous }()
	changes := initChanges(params.Name)
	for _, change := range changes {
		if err := config.EnsureWritable(change.Path, force || dryRun); err != nil {
			return err
		}
	}
	for _, change := range changes {
		if parent := filepath.Dir(change.Path); parent != "." && parent != "" {
			if err := os.MkdirAll(parent, 0o755); err != nil {
				return fmt.Errorf("failed to create '%s': %w", parent, err)
			}
		}
		if err := os.WriteFile(change.Path, []byte(change.Content), 0o644); err != nil {
			return fmt.Errorf("failed to write '%s': %w", change.Path, err)
		}
		fmt.Printf("%s %s\n", change.Action, change.Path)
	}
	fmt.Printf("Created project skeleton in %s\n", params.Name)
	if err := os.Chdir(params.Name); err != nil {
		return fmt.Errorf("failed to enter project '%s': %w", params.Name, err)
	}
	fmt.Printf("Working in %s\n", params.Name)
	return nil
}

// wizardComponentLoop repeats the add-a-component flow until the user is done.
func wizardComponentLoop(state *wizard.State) error {
	for {
		var action string
		options := []huh.Option[string]{huh.NewOption("Done adding components", "done")}
		for _, kind := range wizard.ComponentKindNames() {
			options = append(options, huh.NewOption(kind, kind))
		}
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title("Add a component").
					Options(options...).
					Value(&action),
			),
		).Run()
		if err != nil {
			return err
		}
		if action == "done" {
			return nil
		}
		spec := wizard.ComponentSpec{Kind: generate.Kind(action), Version: "1.0.0"}
		if err := wizardComponentFields(&spec); err != nil {
			return err
		}
		optionsFor := wizard.ComponentOptions(spec, manifest, catalog, dryRun, force)
		changes, err := generate.Generate(generate.Kind(action), optionsFor)
		if err != nil {
			return err
		}
		for _, change := range changes {
			fmt.Printf("%s %s\n", change.Action, change.Path)
		}
		state.Components = append(state.Components, spec)
	}
}

// wizardComponentFields collects kind-specific fields for one component.
func wizardComponentFields(spec *wizard.ComponentSpec) error {
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().Title(fmt.Sprintf("%s name", spec.Kind)).Value(&spec.Name),
		),
	}
	switch spec.Kind {
	case generate.KindTool:
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Side effects").
				Options(huh.NewOptions("read", "write", "money", "none")...).
				Value(&spec.SideEffects),
			huh.NewInput().Title("Description").Value(&spec.Description),
		))
	case generate.KindChannel:
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Channel kind").
				Options(huh.NewOptions("eve", "slack", "discord", "telegram", "github", "twilio")...).
				Value(&spec.KindField),
			huh.NewInput().Title("Vercel Connect UID (slack/linear/github)").Value(&spec.ConnectUID),
			huh.NewInput().Title("Allowed inbound Twilio sender").Value(&spec.AllowFrom),
			huh.NewInput().Title("Outbound Twilio sender").Value(&spec.MessagingFrom),
			huh.NewInput().Title("Telegram bot username").Value(&spec.BotUsername),
			huh.NewInput().Title("GitHub bot name").Value(&spec.BotName),
		))
	case generate.KindSchedule:
		groups = append(groups, huh.NewGroup(
			huh.NewInput().Title("Cron expression").Placeholder("0 9 * * 1-5").Value(&spec.Schedule),
		))
	case generate.KindMemory:
		groups = append(groups, huh.NewGroup(
			huh.NewInput().Title("Retention period").Placeholder("180d").Value(&spec.Retention),
		))
	case generate.KindSubagent:
		groups = append(groups, huh.NewGroup(
			huh.NewInput().Title("Description").Value(&spec.Description),
		))
	}
	return huh.NewForm(groups...).Run()
}

// wizardAgentStep collects the agent definition and its attachments.
func wizardAgentStep(state *wizard.State) error {
	catalogData, err := config.LoadCatalog(catalog)
	if err != nil {
		return err
	}
	attachments := wizard.AvailableAttachments(catalogData, nil)
	agent := &wizard.AgentSpec{}
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().Title("Agent name").Value(&agent.Name),
			huh.NewInput().Title("Description").Value(&agent.Description),
			huh.NewInput().Title("Model").Placeholder("openai/gpt-5.5").Value(&agent.Model),
		),
	}
	groups = append(groups, attachGroup("Tools", agent.WithTools, attachments.Tools)...)
	groups = append(groups, attachGroup("Skills", agent.WithSkills, attachments.Skills)...)
	groups = append(groups, attachGroup("Channels", agent.WithChannels, attachments.Channels)...)
	groups = append(groups, attachGroup("Schedules", agent.WithSchedules, attachments.Schedules)...)
	groups = append(groups, attachGroup("Evals", agent.WithEvals, attachments.Evals)...)
	groups = append(groups, attachGroup("Memory", agent.WithMemory, attachments.Memory)...)
	groups = append(groups, huh.NewGroup(
		huh.NewSelect[string]().
			Title("Approval policy").
			Options(huh.NewOptions("", "required", "admin")...).
			Value(&agent.Approval),
		huh.NewSelect[string]().
			Title("Auth profile").
			Options(huh.NewOptions("", "platform-oauth", "http-basic-env")...).
			Value(&agent.Auth),
		huh.NewSelect[string]().
			Title("Visibility").
			Options(huh.NewOptions("", "internal", "public")...).
			Value(&agent.Visibility),
	))

	form := huh.NewForm(groups...)
	if err := form.Run(); err != nil {
		return err
	}
	state.Agent = agent
	return nil
}

// attachGroup builds a single multi-select group, or none when there are no
// options to choose from.
func attachGroup(title string, target []string, options []string) []*huh.Group {
	if len(options) == 0 {
		return nil
	}
	bound := &target
	optionList := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		optionList = append(optionList, huh.NewOption(option, option))
	}
	return []*huh.Group{
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(title).
				Options(optionList...).
				Value(bound),
		),
	}
}

// wizardAgentConfirm shows the planned batch and asks before applying.
func wizardAgentConfirm(state *wizard.State) error {
	manifestData, err := config.LoadManifest(manifest)
	if err != nil {
		return err
	}
	catalogData, err := config.LoadCatalog(catalog)
	if err != nil {
		return err
	}
	if dryRun {
		return planBatchPrint(manifestData, catalogData)
	}
	var apply bool
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Apply the generated agent and files?").
				Affirmative("Apply").
				Negative("Stop").
				Value(&apply),
		),
	).Run()
	if err != nil {
		return err
	}
	if !apply {
		fmt.Println("Stopped before applying; run 'eve-rails-cli-go apply' when ready.")
		return nil
	}
	return applyBatch(manifestData, catalogData)
}

func runWizard(command *cobra.Command, args []string) error {
	if err := requireTTY(); err != nil {
		return err
	}
	state := wizardState()
	if _, err := os.Stat(manifest); err != nil {
		if err := wizardInitStep(state); err != nil {
			return err
		}
	}
	if err := wizardComponentLoop(state); err != nil {
		return err
	}
	if err := wizardAgentStep(state); err != nil {
		return err
	}
	agentChanges, err := generate.Generate(generate.KindAgent, wizard.AgentOptions(state.Agent, manifest, catalog, dryRun, force))
	if err != nil {
		return err
	}
	for _, change := range agentChanges {
		fmt.Printf("%s %s\n", change.Action, change.Path)
	}
	return wizardAgentConfirm(state)
}

var wizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Interactive terminal wizard for fleet setup",
	Long:  "Guide a new or existing fleet through project init, component generation, and agent creation in an interactive terminal. Requires a TTY; agents and CI should use init, generate, and apply with flags instead.",
	Args:  cobra.NoArgs,
	RunE:  runWizard,
}

func init() {
	wizardCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show planned changes without writing files")
	wizardCmd.Flags().BoolVar(&jsonOutput, "json", false, "Unsupported; guards against accidental TUI use in automation")
	wizardCmd.Flags().StringVar(&manifest, "manifest", "manifests/agents.yml", "Path to the fleet manifest")
	wizardCmd.Flags().StringVar(&catalog, "catalog", "manifests/catalog.yml", "Path to the reusable component catalog")
	wizardCmd.Flags().StringVar(&templates, "templates", "templates/agent", "Path to the template directory")
	wizardCmd.Flags().StringVar(&templates, "template-dir", "templates/agent", "Path to the template directory")
}

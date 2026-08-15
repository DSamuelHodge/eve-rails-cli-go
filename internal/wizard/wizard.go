package wizard

import "github.com/DSamuelHodge/eve-rails-cli-go/internal/generate"

// ComponentSpec captures one component generation requested in the wizard.
type ComponentSpec struct {
	Kind          generate.Kind
	Name          string
	Version       string
	Description   string
	SideEffects   string
	Retention     string
	Schedule      string
	KindField     string
	AllowFrom     string
	MessagingFrom string
	ConnectUID    string
	BotUsername   string
	BotName       string
	Approval      string
	Risk          string
	Auth          string
	Visibility    string
}

// AgentSpec captures the agent generation requested in the wizard.
type AgentSpec struct {
	Name          string
	Description   string
	Model         string
	WithTools     []string
	WithSkills    []string
	WithSubagents []string
	WithChannels  []string
	WithSchedules []string
	WithEvals     []string
	WithMemory    []string
	Approval      string
	Auth          string
	Visibility    string
}

// State captures the full wizard session.
type State struct {
	ProjectName string
	Template    string
	Model       string
	Owner       string
	Components  []ComponentSpec
	Agent       *AgentSpec
}

// ComponentOptions converts one ComponentSpec into generate options.
func ComponentOptions(spec ComponentSpec, manifest, catalog string, dryRun, force bool) generate.Options {
	return generate.Options{
		Name:          spec.Name,
		Version:       spec.Version,
		Description:   spec.Description,
		SideEffects:   spec.SideEffects,
		Retention:     spec.Retention,
		Schedule:      spec.Schedule,
		Kind:          spec.KindField,
		AllowFrom:     spec.AllowFrom,
		MessagingFrom: spec.MessagingFrom,
		ConnectUID:    spec.ConnectUID,
		BotUsername:   spec.BotUsername,
		BotName:       spec.BotName,
		Approval:      spec.Approval,
		Risk:          spec.Risk,
		Auth:          spec.Auth,
		Visibility:    spec.Visibility,
		DryRun:        dryRun,
		Force:         force,
		Manifest:      manifest,
		Catalog:       catalog,
	}
}

// AgentOptions converts an AgentSpec into generate options.
func AgentOptions(spec *AgentSpec, manifest, catalog string, dryRun, force bool) generate.Options {
	return generate.Options{
		Name:          spec.Name,
		Description:   spec.Description,
		Model:         spec.Model,
		WithTools:     spec.WithTools,
		WithSkills:    spec.WithSkills,
		WithSubagents: spec.WithSubagents,
		WithChannels:  spec.WithChannels,
		WithSchedules: spec.WithSchedules,
		WithEvals:     spec.WithEvals,
		WithMemory:    spec.WithMemory,
		Approval:      spec.Approval,
		Auth:          spec.Auth,
		Visibility:    spec.Visibility,
		DryRun:        dryRun,
		Force:         force,
		Manifest:      manifest,
		Catalog:       catalog,
	}
}

package wizard

import (
	"reflect"
	"testing"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/generate"
)

func TestComponentOptionsMapping(t *testing.T) {
	spec := ComponentSpec{
		Kind:          generate.KindTool,
		Name:          "search",
		Version:       "1.0.0",
		Description:   "Search customers",
		SideEffects:   "read",
		KindField:     "tool",
		AllowFrom:     "+15551234567",
		MessagingFrom: "env:TWILIO_FROM_NUMBER",
		ConnectUID:    "slack/uid",
		BotUsername:   "bot",
		BotName:       "ghbot",
		Approval:      "required",
		Risk:          "high",
		Auth:          "platform-oauth",
		Visibility:    "internal",
	}
	options := ComponentOptions(spec, "manifests/agents.yml", "manifests/catalog.yml", true, false)
	if options.Name != "search" || options.Version != "1.0.0" || options.SideEffects != "read" {
		t.Fatalf("unexpected tool options: %+v", options)
	}
	if options.Kind != "tool" || !options.DryRun || options.Force {
		t.Fatalf("unexpected flags: %+v", options)
	}
	if options.Manifest != "manifests/agents.yml" || options.Catalog != "manifests/catalog.yml" {
		t.Fatalf("unexpected paths: %+v", options)
	}
}

func TestAgentOptionsMapping(t *testing.T) {
	spec := &AgentSpec{
		Name:         "support",
		Description:  "Handles support",
		Model:        "openai/gpt-5.5",
		WithTools:    []string{"search"},
		WithSkills:   []string{"summarize"},
		WithChannels: []string{"slack"},
		Approval:     "required",
		Auth:         "http-basic-env",
		Visibility:   "internal",
	}
	options := AgentOptions(spec, "manifests/agents.yml", "manifests/catalog.yml", false, true)
	if options.Name != "support" || options.Description != "Handles support" || options.Model != "openai/gpt-5.5" {
		t.Fatalf("unexpected agent options: %+v", options)
	}
	if !reflect.DeepEqual(options.WithTools, []string{"search"}) {
		t.Fatalf("unexpected with-tools: %v", options.WithTools)
	}
	if !options.Force || options.DryRun {
		t.Fatalf("unexpected flags: %+v", options)
	}
}

func TestAvailableAttachmentsSortsCatalogNames(t *testing.T) {
	catalog := &config.CatalogManifest{
		Tools: map[string]config.CatalogComponent{
			"zebra": {}, "alpha": {},
		},
		Skills: map[string]config.CatalogComponent{
			"triage": {},
		},
		Channels: map[string]config.CatalogComponent{
			"slack": {},
		},
		Schedules: map[string]config.CatalogComponent{
			"daily": {},
		},
		Evals: map[string]config.CatalogComponent{
			"standard": {},
		},
		Memory: map[string]config.CatalogComponent{
			"crm": {},
		},
	}
	attachments := AvailableAttachments(catalog, []string{"researcher", "researcher", ""})
	if !reflect.DeepEqual(attachments.Tools, []string{"alpha", "zebra"}) {
		t.Fatalf("unexpected tools: %v", attachments.Tools)
	}
	if !reflect.DeepEqual(attachments.Subagents, []string{"researcher"}) {
		t.Fatalf("unexpected subagents: %v", attachments.Subagents)
	}
	if !reflect.DeepEqual(attachments.Skills, []string{"triage"}) {
		t.Fatalf("unexpected skills: %v", attachments.Skills)
	}
}

func TestToInitParamsUsesState(t *testing.T) {
	state := &State{ProjectName: "demo", Template: "basic", Model: "openai/gpt-5.5", Owner: "team"}
	params := state.ToInitParams()
	if params.Name != "demo" || params.Template != "basic" || params.Model != "openai/gpt-5.5" || params.Owner != "team" {
		t.Fatalf("unexpected init params: %+v", params)
	}
}

func TestComponentKindNames(t *testing.T) {
	names := ComponentKindNames()
	expected := []string{
		"tool", "skill", "subagent", "channel", "schedule",
		"approval", "eval", "memory", "migration",
	}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("unexpected kind names: %v", names)
	}
}

func TestAvailableAttachmentsEmptyCatalog(t *testing.T) {
	attachments := AvailableAttachments(&config.CatalogManifest{}, nil)
	for _, list := range [][]string{attachments.Tools, attachments.Skills, attachments.Channels, attachments.Schedules, attachments.Evals, attachments.Memory, attachments.Subagents} {
		if len(list) != 0 {
			t.Fatalf("expected empty list, got %v", list)
		}
	}
}

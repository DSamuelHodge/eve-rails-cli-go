package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func parseManifest(t *testing.T, source string) *FleetManifest {
	t.Helper()
	var manifest FleetManifest
	if err := yaml.Unmarshal([]byte(source), &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	return &manifest
}

func TestManifestParsesSubagentObjectsAndStrings(t *testing.T) {
	source := `
defaults:
  model: openai/gpt-5.5
  owner: agent-platform
  channels: []
  schedules: []
  evals: []
agents:
  - name: support
    version: 1.0.0
    subagents:
      - analyst
      - name: reviewer
        title: Code Reviewer
`
	manifest := parseManifest(t, source)
	if len(manifest.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(manifest.Agents))
	}
	agent := manifest.Agents[0]
	if len(agent.Subagents) != 2 {
		t.Fatalf("expected 2 subagents, got %d", len(agent.Subagents))
	}
	if agent.Subagents[0].Name != "analyst" {
		t.Fatalf("expected first subagent name 'analyst', got %q", agent.Subagents[0].Name)
	}
	if agent.Subagents[1].Name != "reviewer" || agent.Subagents[1].Title != "Code Reviewer" {
		t.Fatalf("unexpected second subagent: %+v", agent.Subagents[1])
	}
}

func TestManifestParsesDefaultsToolsSkillsMemory(t *testing.T) {
	source := `
defaults:
  model: openai/gpt-5.5
  owner: agent-platform
  channels: [slack, telegram]
  tools:
    search_customers: 1.0.0
  skills:
    summarize: 2.1.0
  memory:
    customer_profile: 1.0.0
agents:
  - name: support
    version: 1.0.0
    responsibility: handles support
`
	manifest := parseManifest(t, source)
	defaults := manifest.Defaults
	if len(defaults.Channels) != 2 || defaults.Channels[0] != "slack" || defaults.Channels[1] != "telegram" {
		t.Fatalf("unexpected default channels: %v", defaults.Channels)
	}
	if defaults.Tools["search_customers"] != "1.0.0" {
		t.Fatalf("unexpected default tools: %v", defaults.Tools)
	}
	if defaults.Skills["summarize"] != "2.1.0" {
		t.Fatalf("unexpected default skills: %v", defaults.Skills)
	}
	if defaults.Memory["customer_profile"] != "1.0.0" {
		t.Fatalf("unexpected default memory: %v", defaults.Memory)
	}
}

func TestPresetChannelKnownSlugs(t *testing.T) {
	for _, slug := range []string{"slack", "telegram", "discord", "teams", "twilio", "linear", "github", "eve"} {
		if PresetChannel(slug) == nil {
			t.Errorf("expected preset channel for %s", slug)
		}
	}
	if PresetChannel("custom_channel") != nil {
		t.Error("expected no preset for unknown channel")
	}
	if !IsPresetChannel("slack") {
		t.Error("expected slack to be recognized as a preset")
	}
	if IsPresetChannel("custom_channel") {
		t.Error("expected custom_channel not to be recognized as a preset")
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func TestManifestParsesApprovalBlocking(t *testing.T) {
	source := `
approvals:
  required:
    version: 1.0.0
    blocking: true
  logged:
    version: 1.0.0
    blocking: false
  legacy:
    version: 1.0.0
`
	var catalog CatalogManifest
	if err := yaml.Unmarshal([]byte(source), &catalog); err != nil {
		t.Fatalf("failed to parse catalog: %v", err)
	}
	if catalog.Approvals["required"].Blocking == nil || !*catalog.Approvals["required"].Blocking {
		t.Error("expected required policy to parse as blocking=true")
	}
	if catalog.Approvals["logged"].Blocking == nil || *catalog.Approvals["logged"].Blocking {
		t.Error("expected logged policy to parse as blocking=false")
	}
	if catalog.Approvals["legacy"].Blocking != nil {
		t.Error("expected legacy policy to leave blocking unset")
	}
}

func TestApprovalGateFor(t *testing.T) {
	write := SideEffectsWrite
	read := SideEffectsRead
	catalog := &CatalogManifest{
		Tools: map[string]CatalogComponent{
			"send_email":  {SideEffects: &write},
			"list_emails": {SideEffects: &read},
		},
		Approvals: map[string]CatalogComponent{
			"required":  {Blocking: boolPtr(true)},
			"audit_log": {Blocking: boolPtr(false)},
			"legacy":    {},
		},
	}

	if got := ApprovalGateFor("send_email", map[string]string{"send_email": "required"}, catalog); got != "always()" {
		t.Errorf("blocking policy: expected always(), got %s", got)
	}
	if got := ApprovalGateFor("send_email", map[string]string{"send_email": "audit_log"}, catalog); got != "never()" {
		t.Errorf("non-blocking policy: expected never(), got %s", got)
	}
	if got := ApprovalGateFor("send_email", map[string]string{"send_email": "legacy"}, catalog); got != "always()" {
		t.Errorf("unset blocking: expected always() default, got %s", got)
	}
	if got := ApprovalGateFor("send_email", nil, catalog); got != "always()" {
		t.Errorf("risky with no policy: expected always(), got %s", got)
	}
	if got := ApprovalGateFor("list_emails", nil, catalog); got != "never()" {
		t.Errorf("read with no policy: expected never(), got %s", got)
	}
}

func TestApprovalGateForRequiredApprovals(t *testing.T) {
	write := SideEffectsWrite
	read := SideEffectsRead
	catalog := &CatalogManifest{
		Tools: map[string]CatalogComponent{
			"write_fs": {
				SideEffects:       &write,
				RequiredApprovals: []string{"audit_log"},
			},
			"query_metrics": {
				SideEffects:       &read,
				RequiredApprovals: []string{"required"},
			},
		},
		Approvals: map[string]CatalogComponent{
			"required":  {Blocking: boolPtr(true)},
			"audit_log": {Blocking: boolPtr(false)},
		},
	}

	if got := ApprovalGateFor("write_fs", nil, catalog); got != "never()" {
		t.Errorf("risky tool with non-blocking required approval: expected never(), got %s", got)
	}
	if got := ApprovalGateFor("query_metrics", nil, catalog); got != "always()" {
		t.Errorf("read tool with blocking required approval: expected always(), got %s", got)
	}
}

func TestObservabilityAcceptsBool(t *testing.T) {
	var manifest EnvironmentsManifest
	if err := yaml.Unmarshal([]byte(`
environments:
  development:
    observability: false
  staging:
    observability: true
`), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Environments["development"].Observability.Bool() {
		t.Fatal("expected development observability false")
	}
	if !manifest.Environments["staging"].Observability.Bool() {
		t.Fatal("expected staging observability true")
	}
}

func TestIsSlug(t *testing.T) {
	valid := []string{"support", "data-analyst", "web_crawler", "a1-b2"}
	for _, value := range valid {
		if !IsSlug(value) {
			t.Errorf("expected %q to be a slug", value)
		}
	}
	invalid := []string{"Support", "data analyst", "with/slash"}
	for _, value := range invalid {
		if IsSlug(value) {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}

func TestRiskyToolDetection(t *testing.T) {
	catalog := &CatalogManifest{
		Tools: map[string]CatalogComponent{
			"read-tool":  {SideEffects: ptrSideEffects(SideEffectsRead)},
			"write-tool": {SideEffects: ptrSideEffects(SideEffectsWrite)},
			"prod-tool":  {SideEffects: ptrSideEffects(SideEffectsProduction)},
		},
	}
	if IsRiskyTool("read-tool", catalog) {
		t.Error("expected read tool to not be risky")
	}
	if !IsRiskyTool("write-tool", catalog) {
		t.Error("expected write tool to be risky")
	}
	if !IsRiskyTool("prod-tool", catalog) {
		t.Error("expected production tool to be risky")
	}
}

func TestExplicitNoneSideEffectsIsNotRisky(t *testing.T) {
	catalog := &CatalogManifest{
		Tools: map[string]CatalogComponent{
			"noop": {SideEffects: ptrSideEffects(SideEffectsNone)},
		},
	}
	if IsRiskyTool("noop", catalog) {
		t.Error("expected none side effects tool to not be risky")
	}
}

func ptrSideEffects(effects SideEffects) *SideEffects {
	return &effects
}

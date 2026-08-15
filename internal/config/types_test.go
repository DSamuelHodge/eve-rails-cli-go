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

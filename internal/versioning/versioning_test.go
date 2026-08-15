package versioning

import (
	"testing"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
)

func validManifest() *config.FleetManifest {
	return &config.FleetManifest{
		Defaults: config.ManifestDefaults{
			Model: "openai/gpt-5.5",
			Owner: "agent-platform",
		},
		Agents: []config.AgentManifest{
			{
				Name:           "support",
				Version:        "1.0.0",
				Responsibility: "handles support",
				Tools:          config.ComponentMap{"search": "1.0.0"},
				Skills:         config.ComponentMap{"summarize": "^1.0.0"},
				Memory:         config.ComponentMap{"crm": "1.0.0"},
			},
		},
	}
}

func validCatalog() *config.CatalogManifest {
	read := config.SideEffectsRead
	write := config.SideEffectsWrite
	_ = write
	return &config.CatalogManifest{
		Tools: map[string]config.CatalogComponent{
			"search": {Version: "1.1.0", SideEffects: &read},
		},
		Skills: map[string]config.CatalogComponent{
			"summarize": {Version: "1.2.0"},
		},
		Memory: map[string]config.CatalogComponent{
			"crm": {Version: "1.0.0"},
		},
		Evals: map[string]config.CatalogComponent{
			"quality": {Version: "1.0.0"},
		},
	}
}

func TestCollectVersionReportsDetectsUpdates(t *testing.T) {
	reports := CollectVersionReports(validManifest(), validCatalog(), "")
	if len(reports) != 3 {
		t.Fatalf("expected 3 reports, got %d", len(reports))
	}
	byComponent := map[string]UpdateKind{}
	for _, report := range reports {
		byComponent[report.Component] = report.Update
	}
	if byComponent["search"] != UpdateMinor {
		t.Errorf("expected search minor update, got %s", byComponent["search"])
	}
	if byComponent["summarize"] != UpdateCurrent {
		t.Errorf("expected summarize current (^ matches), got %s", byComponent["summarize"])
	}
	if byComponent["crm"] != UpdateCurrent {
		t.Errorf("expected crm current, got %s", byComponent["crm"])
	}
}

func TestVersionRangesResolve(t *testing.T) {
	catalog := validCatalog()
	summarize := catalog.Skills["summarize"]
	if got := ResolveVersion("^1.0.0", &summarize); got != "1.2.0" {
		t.Errorf("expected ^1.0.0 to resolve to 1.2.0, got %s", got)
	}
	compatible := config.CatalogComponent{Version: "1.0.5"}
	if got := ResolveVersion("~1.0.0", &compatible); got != "1.0.5" {
		t.Errorf("expected ~1.0.0 to resolve to 1.0.5, got %s", got)
	}
	if got := ResolveVersion("2.0.0", &summarize); got != "" {
		t.Errorf("expected incompatible 2.0.0 to not resolve, got %s", got)
	}
}

func TestVersionRequestMatches(t *testing.T) {
	cases := []struct {
		requested, available string
		want                 bool
	}{
		{"1.0.0", "1.0.0", true},
		{"^1.0.0", "1.2.0", true},
		{"~1.0.0", "1.0.5", true},
		{"2.0.0", "1.2.0", false},
	}
	for _, tc := range cases {
		if got := VersionRequestMatches(tc.requested, tc.available); got != tc.want {
			t.Errorf("VersionRequestMatches(%s, %s) = %v, want %v", tc.requested, tc.available, got, tc.want)
		}
	}
}

func TestEffectiveEvalsMergesDefaults(t *testing.T) {
	manifest := validManifest()
	manifest.Defaults.Evals = []string{"default-eval"}
	manifest.Agents[0].Evals = []string{"agent-eval"}
	evals := EffectiveEvals(&manifest.Agents[0], manifest)
	if len(evals) != 2 {
		t.Fatalf("expected 2 evals, got %d", len(evals))
	}
}

func TestApplyVersionPolicy(t *testing.T) {
	manifest := validManifest()
	pin := config.PolicyPin
	manifest.VersionPolicy = config.VersionPolicy{
		Default: &pin,
	}
	if got := ApplyVersionPolicy(UpdateMinor, "tool", "search", manifest); got != UpdateMajor {
		t.Errorf("expected minor blocked to major by pin policy, got %s", got)
	}
	if got := ApplyVersionPolicy(UpdateCurrent, "tool", "search", manifest); got != UpdateCurrent {
		t.Errorf("expected current to remain current under pin policy, got %s", got)
	}
}

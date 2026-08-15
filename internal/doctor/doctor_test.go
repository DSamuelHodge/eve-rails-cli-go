package doctor

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/render"
)

func readSideEffects(effects config.SideEffects) *config.SideEffects {
	return &effects
}

func testTemplatesDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "templates", "agent"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func testCatalog() *config.CatalogManifest {
	return &config.CatalogManifest{
		Tools: map[string]config.CatalogComponent{
			"search": {
				Version:              "1.0.0",
				SideEffects:          readSideEffects(config.SideEffectsRead),
				SandboxCompatibility: []string{"read-only"},
			},
			"write-fs": {
				Version:              "1.0.0",
				SideEffects:          readSideEffects(config.SideEffectsWrite),
				SandboxCompatibility: []string{"workspace-write"},
			},
		},
		Skills: map[string]config.CatalogComponent{
			"summarize": {Version: "1.0.0"},
		},
		Memory: map[string]config.CatalogComponent{
			"crm": {Version: "1.0.0", Retention: "180d"},
		},
		Approvals: map[string]config.CatalogComponent{
			"required": {Version: "1.0.0"},
		},
		Evals: map[string]config.CatalogComponent{
			"quality": {Version: "1.0.0"},
		},
	}
}

func testManifest() *config.FleetManifest {
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
				Skills:         config.ComponentMap{"summarize": "1.0.0"},
				Evals:          []string{"quality"},
				Memory:         config.ComponentMap{"crm": "1.0.0"},
			},
		},
	}
}

func TestValidManifestResolvesAgainstCatalog(t *testing.T) {
	report := ValidateManifest(testManifest(), testCatalog())
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors, got: %v", report.Errors)
	}
}

func TestMissingCatalogComponentFails(t *testing.T) {
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"missing-tool": "1.0.0"}
	report := ValidateManifest(manifest, testCatalog())
	found := false
	for _, err := range report.Errors {
		if strings.Contains(err, "missing-tool") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing component error, got: %v", report.Errors)
	}
}

func TestVersionMismatchFails(t *testing.T) {
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"search": "2.0.0"}
	report := ValidateManifest(manifest, testCatalog())
	found := false
	for _, err := range report.Errors {
		if strings.Contains(err, "does not resolve") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected version mismatch error, got: %v", report.Errors)
	}
}

func TestRiskyToolWithoutApprovalFails(t *testing.T) {
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"write-fs": "1.0.0"}
	report := ValidateManifest(manifest, testCatalog())
	found := false
	for _, err := range report.Errors {
		if strings.Contains(err, "risky tool") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected risky tool error, got: %v", report.Errors)
	}
}

func TestMemoryWithoutRetentionFails(t *testing.T) {
	catalog := testCatalog()
	bad := catalog.Memory["crm"]
	bad.Retention = ""
	catalog.Memory["crm"] = bad
	report := ValidateManifest(testManifest(), catalog)
	found := false
	for _, err := range report.Errors {
		if strings.Contains(err, "retention") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected retention error, got: %v", report.Errors)
	}
}

func TestRuntimePolicyRejectsWriteToolInReadSandbox(t *testing.T) {
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"write-fs": "1.0.0"}
	manifest.Agents[0].RuntimePolicy = &config.RuntimePolicy{
		Sandbox:      "read-only",
		AllowedTools: []string{"write-fs"},
	}
	report := ValidateManifest(manifest, testCatalog())
	found := false
	for _, err := range report.Errors {
		if strings.Contains(err, "runtime policy") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected runtime policy error, got: %v", report.Errors)
	}
}

func TestExplicitNoneSideEffectsWarnsButIsNotRisky(t *testing.T) {
	catalog := testCatalog()
	catalog.Tools["noop"] = config.CatalogComponent{Version: "1.0.0", SideEffects: readSideEffects(config.SideEffectsNone)}
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"noop": "1.0.0"}
	report := ValidateManifest(manifest, catalog)
	for _, err := range report.Errors {
		if strings.Contains(err, "risky tool") {
			t.Fatalf("expected no risky tool error, got: %v", err)
		}
	}
	warned := false
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "explicitly declares side_effects: none") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected explicit-none warning, got: %v", report.Warnings)
	}
}

func TestDoctorReportsMissingChannelConfig(t *testing.T) {
	catalog := testCatalog()
	catalog.Channels = map[string]config.CatalogComponent{
		"slack": {Version: "1.0.0", Kind: "slack"},
	}
	manifest := testManifest()
	manifest.Agents[0].Channels = []string{"slack"}
	report, err := RunDoctor(manifest, catalog, Options{})
	if err != nil {
		t.Fatalf("unexpected doctor error: %v", err)
	}
	found := false
	for _, check := range report.Checks {
		if check.Name == "channel-config" && check.Status == StatusFail {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected channel-config failure, checks: %+v", report.Checks)
	}
}

func TestValidateChannelAcceptsPresets(t *testing.T) {
	catalog := testCatalog()
	catalog.Channels = map[string]config.CatalogComponent{}
	manifest := testManifest()
	manifest.Agents[0].Channels = []string{"slack", "telegram"}
	report := ValidateManifest(manifest, catalog)
	for _, err := range report.Errors {
		if strings.Contains(err, "missing channel") {
			t.Fatalf("expected preset channels to pass, got error: %s", err)
		}
	}
}

func TestValidateChannelRejectsUnknown(t *testing.T) {
	catalog := testCatalog()
	catalog.Channels = map[string]config.CatalogComponent{}
	manifest := testManifest()
	manifest.Agents[0].Channels = []string{"my_custom_surface"}
	report := ValidateManifest(manifest, catalog)
	found := false
	for _, err := range report.Errors {
		if strings.Contains(err, "missing channel 'my_custom_surface'") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unknown channel to be rejected, errors: %v", report.Errors)
	}
}

func TestValidateSubagentChannelsWarn(t *testing.T) {
	manifest := testManifest()
	manifest.Agents[0].Subagents = config.SubagentList{
		{Name: "triage", Channels: []string{"slack"}},
	}
	report := ValidateManifest(manifest, testCatalog())
	found := false
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "channels are root-only") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected subagent channel warning, warnings: %v", report.Warnings)
	}
}

func TestValidateDefaultsToolsSkillsMemory(t *testing.T) {
	manifest := testManifest()
	manifest.Defaults.Tools = config.ComponentMap{"search": "1.0.0"}
	manifest.Defaults.Skills = config.ComponentMap{"summarize": "1.0.0"}
	manifest.Defaults.Memory = config.ComponentMap{"crm": "1.0.0"}
	report := ValidateManifest(manifest, testCatalog())
	if len(report.Errors) != 0 {
		t.Fatalf("expected valid defaults to pass, errors: %v", report.Errors)
	}
}

func TestValidateDefaultsMissingToolFails(t *testing.T) {
	manifest := testManifest()
	manifest.Defaults.Tools = config.ComponentMap{"ghost_tool": "1.0.0"}
	report := ValidateManifest(manifest, testCatalog())
	found := false
	for _, err := range report.Errors {
		if strings.Contains(err, "missing tool 'ghost_tool'") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing defaults tool to fail, errors: %v", report.Errors)
	}
}

func TestValidateApprovalPolicyMissingBlockingWarns(t *testing.T) {
	catalog := testCatalog()
	catalog.Approvals = map[string]config.CatalogComponent{
		"required": {Version: "1.0.0"},
	}
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"search": "1.0.0"}
	manifest.Agents[0].Approvals = map[string]string{"search": "required"}
	report := ValidateManifest(manifest, catalog)
	found := false
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "does not declare blocking") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing-blocking warning, warnings: %v", report.Warnings)
	}
}

func TestValidateApprovalPolicyBlockingDeclaredNoWarn(t *testing.T) {
	catalog := testCatalog()
	blocking := true
	catalog.Approvals = map[string]config.CatalogComponent{
		"required": {Version: "1.0.0", Blocking: &blocking},
	}
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"search": "1.0.0"}
	manifest.Agents[0].Approvals = map[string]string{"search": "required"}
	report := ValidateManifest(manifest, catalog)
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "does not declare blocking") {
			t.Fatalf("unexpected missing-blocking warning: %v", warning)
		}
	}
}

func TestRenderedApprovalGateChecksMatch(t *testing.T) {
	catalog := testCatalog()
	blocking := true
	catalog.Approvals = map[string]config.CatalogComponent{
		"required": {Version: "1.0.0", Blocking: &blocking},
	}
	manifest := testManifest()
	manifest.Agents[0].Approvals = map[string]string{"search": "required"}
	plan, err := render.PlanBatch(manifest, catalog, testTemplatesDir(t))
	if err != nil {
		t.Fatal(err)
	}
	var checks []Check
	addRenderedApprovalGateChecks(&checks, manifest, catalog, plan)
	for _, check := range checks {
		if check.Name != "rendered-approval-gates" {
			continue
		}
		if check.Status != StatusPass {
			t.Fatalf("expected rendered-approval-gates to pass, got: %s", check.Message)
		}
	}
}

func TestRenderedApprovalGateChecksCatchMismatch(t *testing.T) {
	catalog := testCatalog()
	blocking := true
	catalog.Approvals = map[string]config.CatalogComponent{
		"required": {Version: "1.0.0", Blocking: &blocking},
	}
	manifest := testManifest()
	manifest.Agents[0].Approvals = map[string]string{"search": "required"}
	plan, err := render.PlanBatch(manifest, catalog, testTemplatesDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for i := range plan.Operations {
		if strings.HasSuffix(plan.Operations[i].Path, "tools/search.ts") {
			plan.Operations[i].Content = strings.ReplaceAll(
				plan.Operations[i].Content, "approval: always()", "approval: never()")
		}
	}
	var checks []Check
	addRenderedApprovalGateChecks(&checks, manifest, catalog, plan)
	for _, check := range checks {
		if check.Name != "rendered-approval-gates" {
			continue
		}
		if check.Status != StatusFail {
			t.Fatalf("expected rendered-approval-gates to fail on mismatch, got: %s", check.Status)
		}
	}
}

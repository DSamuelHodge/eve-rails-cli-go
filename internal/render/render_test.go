package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
)

func testTemplates(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("..", "..", "templates", "agent"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("templates not found at %s: %v", src, err)
	}
	return src
}

func testCatalog() *config.CatalogManifest {
	read := config.SideEffectsRead
	return &config.CatalogManifest{
		Tools: map[string]config.CatalogComponent{
			"search": {Version: "1.0.0", SideEffects: &read},
		},
		Skills: map[string]config.CatalogComponent{
			"summarize": {Version: "1.0.0"},
		},
		Channels: map[string]config.CatalogComponent{
			"slack": {Version: "1.0.0", Kind: "slack", ConnectUID: "slack/my-agent"},
		},
		Schedules: map[string]config.CatalogComponent{
			"daily": {Version: "1.0.0"},
		},
		Evals: map[string]config.CatalogComponent{
			"quality": {Version: "1.0.0"},
		},
	}
}

func boolPtr(value bool) *bool {
	return &value
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
				Channels:       []string{"slack"},
				Schedules:      []string{"daily"},
				Evals:          []string{"quality"},
				Memory:         config.ComponentMap{},
			},
		},
	}
}

func TestRendererOutputsCoreAgentFiles(t *testing.T) {
	renderer, err := Load(testTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&testManifest().Agents[0], testManifest(), testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, file := range files {
		paths[file.Path] = true
	}
	expected := []string{
		"agents/support/package.json",
		"agents/support/tsconfig.json",
		"agents/support/agent/instructions.md",
		"agents/support/agent/agent.ts",
		"agents/support/agent/agent.manifest.yml",
		"agents/support/agent/versions.lock",
		"agents/support/agent/tools/search.ts",
		"agents/support/agent/skills/summarize.md",
		"agents/support/agent/channels/slack.ts",
		"agents/support/agent/schedules/daily.ts",
	}
	for _, path := range expected {
		if !paths[path] {
			t.Errorf("expected generated file %s, got paths: %v", path, paths)
		}
	}
}

func TestPlanBatchClassifiesActions(t *testing.T) {
	temp := t.TempDir()
	templateDir := filepath.Join(temp, "templates", "agent")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Copy real templates into a temp dir so the plan can render.
	src := testTemplates(t)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(templateDir, entry.Name()), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := PlanBatch(testManifest(), testCatalog(), templateDir)
	if err != nil {
		t.Fatal(err)
	}
	created := 0
	skipped := 0
	for _, operation := range plan.Operations {
		switch operation.Action {
		case BatchCreate:
			created++
		case BatchSkip:
			skipped++
		}
	}
	if created == 0 {
		t.Fatalf("expected created files, got %d", created)
	}
	if skipped != 0 {
		t.Fatalf("expected no skips on first plan, got %d", skipped)
	}
}

func TestBatchActionFor(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "file.txt")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	action, err := BatchActionFor(path, "content")
	if err != nil {
		t.Fatal(err)
	}
	if action != BatchSkip {
		t.Errorf("expected skip, got %s", action)
	}
	action, err = BatchActionFor(path, "changed")
	if err != nil {
		t.Fatal(err)
	}
	if action != BatchUpdate {
		t.Errorf("expected update, got %s", action)
	}
	action, err = BatchActionFor(filepath.Join(temp, "missing.txt"), "content")
	if err != nil {
		t.Fatal(err)
	}
	if action != BatchCreate {
		t.Errorf("expected create, got %s", action)
	}
}

func TestComponentDigestIsContentAddressed(t *testing.T) {
	content := "export const tool = { name: \"search\" };\n"
	first := ComponentDigest(content)
	second := ComponentDigest(content)
	if first != second {
		t.Errorf("expected stable digest, got %s and %s", first, second)
	}
	if !stringsHasPrefix(first, "sha256:") {
		t.Errorf("expected sha256 prefix, got %s", first)
	}
	changed := ComponentDigest("export const tool = { name: \"search-v2\" };\n")
	if first == changed {
		t.Errorf("expected content change to change the digest, got %s", first)
	}
	multi := ComponentDigest(content, "// header")
	if multi == first {
		t.Errorf("expected multi-part digest to differ from single-part digest")
	}
}

func TestVersionsLockDigestsMatchRenderedContent(t *testing.T) {
	renderer, err := Load(testTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&testManifest().Agents[0], testManifest(), testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	var lockContent string
	for _, file := range files {
		byPath[file.Path] = file.Content
		if file.Path == "agents/support/agent/versions.lock" {
			lockContent = file.Content
		}
	}
	if lockContent == "" {
		t.Fatal("expected a versions.lock")
	}
	if !stringsContains(lockContent, ComponentDigest(byPath["agents/support/agent/tools/search.ts"])) {
		t.Error("expected tool digest to match rendered tool.ts content")
	}
	if !stringsContains(lockContent, ComponentDigest(byPath["agents/support/agent/skills/summarize.md"])) {
		t.Error("expected skill digest to match rendered skill.md content")
	}
	if !stringsContains(lockContent, ComponentDigest(byPath["agents/support/agent/channels/slack.ts"])) {
		t.Error("expected channel digest to match rendered channel .ts content")
	}
	if !stringsContains(lockContent, ComponentDigest(
		byPath["agents/support/agent/schedules/daily.ts"],
	)) {
		t.Error("expected schedule digest to match rendered schedule.ts content")
	}
}

func stringsHasPrefix(value, prefix string) bool {
	return len(value) >= len(prefix) && value[:len(prefix)] == prefix
}

func TestRenderAgentManifestIncludesTopology(t *testing.T) {
	manifest := testManifest()
	manifest.Agents[0].Topology = &config.AgentTopology{
		Department:  "support",
		ParentAgent: true,
	}
	manifest.Agents[0].Subagents = config.SubagentList{
		{Name: "reviewer", Title: "Code Reviewer"},
	}
	renderer, err := Load(testTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&manifest.Agents[0], manifest, testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var manifestContent string
	for _, file := range files {
		if file.Path == "agents/support/agent/agent.manifest.yml" {
			manifestContent = file.Content
		}
	}
	if manifestContent == "" {
		t.Fatal("expected generated manifest content")
	}
	if !stringsContains(manifestContent, "topology") {
		t.Error("expected topology in generated manifest")
	}
	if !stringsContains(manifestContent, "reviewer") {
		t.Error("expected subagent reviewer in generated manifest")
	}
}

func stringsContains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestRendererSupportsDefaultsPresetsAndSubagentFiles(t *testing.T) {
	manifest := testManifest()
	manifest.Defaults.Channels = []string{"telegram"}
	manifest.Defaults.Tools = config.ComponentMap{"search": "1.0.0"}
	manifest.Defaults.Skills = config.ComponentMap{"summarize": "1.0.0"}
	manifest.Agents[0].Tools = config.ComponentMap{}
	manifest.Agents[0].Skills = config.ComponentMap{}
	manifest.Agents[0].Channels = nil
	manifest.Agents[0].Subagents = config.SubagentList{
		{
			Name:           "triage",
			Title:          "Triage Rep",
			RoleID:         "triage",
			Responsibility: "Classify and route tickets.",
			Tools:          []string{"search"},
			Skills:         []string{"summarize"},
		},
	}

	renderer, err := Load(testTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&manifest.Agents[0], manifest, testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	contents := map[string]string{}
	for _, file := range files {
		paths[file.Path] = true
		contents[file.Path] = file.Content
	}

	// Defaults tools/skills flow through to the agent.
	if !paths["agents/support/agent/tools/search.ts"] {
		t.Error("expected defaults tool to render for agent")
	}
	if !paths["agents/support/agent/skills/summarize.md"] {
		t.Error("expected defaults skill to render for agent")
	}

	// Telegram is an implicit preset: no catalog entry, still rendered.
	if !paths["agents/support/agent/channels/telegram.ts"] {
		t.Error("expected preset telegram channel to render without catalog entry")
	}
	if !stringsContains(contents["agents/support/agent/channels/telegram.ts"], "telegramChannel") {
		t.Error("expected telegram channel to use telegramChannel factory")
	}

	// Subagent tools/skills render as real Eve slots.
	for _, path := range []string{
		"agents/support/agent/subagents/triage/agent.ts",
		"agents/support/agent/subagents/triage/instructions.md",
		"agents/support/agent/subagents/triage/tools/search.ts",
		"agents/support/agent/subagents/triage/skills/summarize.md",
	} {
		if !paths[path] {
			t.Errorf("expected subagent file %s", path)
		}
	}
	if stringsContains(contents["agents/support/agent/subagents/triage/instructions.md"], "Declared channels") {
		t.Error("expected subagent instructions to omit channels (root-only in Eve)")
	}
}

func TestRendererRendersApprovalGate(t *testing.T) {
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"search": "1.0.0"}
	renderer, err := Load(testTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&manifest.Agents[0], manifest, testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	for _, file := range files {
		contents[file.Path] = file.Content
	}
	toolTS := contents["agents/support/agent/tools/search.ts"]
	if !stringsContains(toolTS, `approval: never()`) {
		t.Errorf("expected read tool to render approval: never(), got:\n%s", toolTS)
	}
	if !stringsContains(toolTS, `import { never } from "eve/tools/approval";`) {
		t.Errorf("expected approval helper import, got:\n%s", toolTS)
	}
}

func TestRendererRendersApprovalGateAlwaysForBlockingPolicy(t *testing.T) {
	write := config.SideEffectsWrite
	catalog := testCatalog()
	catalog.Tools["send_email"] = config.CatalogComponent{Version: "1.0.0", SideEffects: &write}
	catalog.Approvals = map[string]config.CatalogComponent{
		"required": {Version: "1.0.0", Blocking: boolPtr(true)},
	}
	manifest := testManifest()
	manifest.Agents[0].Tools = config.ComponentMap{"send_email": "1.0.0"}
	manifest.Agents[0].Approvals = map[string]string{"send_email": "required"}
	renderer, err := Load(testTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&manifest.Agents[0], manifest, catalog)
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	for _, file := range files {
		contents[file.Path] = file.Content
	}
	toolTS := contents["agents/support/agent/tools/send_email.ts"]
	if !stringsContains(toolTS, `approval: always()`) {
		t.Errorf("expected blocking policy to render approval: always(), got:\n%s", toolTS)
	}
	if !stringsContains(toolTS, `import { always } from "eve/tools/approval";`) {
		t.Errorf("expected always import, got:\n%s", toolTS)
	}
}

func TestRendererRendersSandbox(t *testing.T) {
	manifest := testManifest()
	readOnly := "read-only"
	manifest.Agents[0].RuntimePolicy = &config.RuntimePolicy{Sandbox: readOnly}
	renderer, err := Load(testTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&manifest.Agents[0], manifest, testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	for _, file := range files {
		contents[file.Path] = file.Content
	}
	sandboxTS, ok := contents["agents/support/agent/sandbox.ts"]
	if !ok {
		t.Fatal("expected sandbox.ts to be rendered")
	}
	if !stringsContains(sandboxTS, `networkPolicy: "deny-all"`) {
		t.Errorf("expected deny-all network policy for read-only sandbox, got:\n%s", sandboxTS)
	}
}

func TestSandboxNetworkPolicy(t *testing.T) {
	cases := map[string]string{
		"read-only":              "deny-all",
		"workspace-write":        "deny-all",
		"network-read":           "allow-all",
		"external-write-gated":   "allow-all",
		"production-write-gated": "allow-all",
		"approval-gated":         "allow-all",
	}
	for sandbox, expected := range cases {
		if got := SandboxNetworkPolicy(sandbox); got != expected {
			t.Errorf("SandboxNetworkPolicy(%s): expected %s, got %s", sandbox, expected, got)
		}
	}
}

func TestRenderSkillUsesCatalogMarkdown(t *testing.T) {
	temp := t.TempDir()
	skillDir := filepath.Join(temp, "catalog", "skills")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	unique := "Catalog markdown is the source of truth for summarize."
	if err := os.WriteFile(filepath.Join(skillDir, "summarize.md"), []byte("# summarize\n\n"+unique+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	templateDir := testTemplates(t)
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(temp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	renderer, err := Load(templateDir)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&testManifest().Agents[0], testManifest(), testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var skillMD string
	for _, file := range files {
		if file.Path == "agents/support/agent/skills/summarize.md" {
			skillMD = file.Content
		}
	}
	if skillMD == "" {
		t.Fatal("expected generated skill markdown")
	}
	if !stringsContains(skillMD, unique) {
		t.Errorf("expected catalog skill sentence in generated skill, got:\n%s", skillMD)
	}
}

func TestSubagentSkillVersionIsCatalogSemver(t *testing.T) {
	catalog := testCatalog()
	catalog.Skills["summarize"] = config.CatalogComponent{Version: "1.2.3"}
	manifest := testManifest()
	manifest.Agents[0].Subagents = config.SubagentList{
		{Name: "triage", Skills: []string{"summarize"}},
	}
	renderer, err := Load(testTemplates(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderer.RenderAgent(&manifest.Agents[0], manifest, catalog)
	if err != nil {
		t.Fatal(err)
	}
	var skillMD string
	for _, file := range files {
		if file.Path == "agents/support/agent/subagents/triage/skills/summarize.md" {
			skillMD = file.Content
		}
	}
	if skillMD == "" {
		t.Fatal("expected generated subagent skill markdown")
	}
	if stringsContains(skillMD, "Version: catalog") {
		t.Errorf("subagent skill must not contain Version: catalog, got:\n%s", skillMD)
	}
	if !stringsContains(skillMD, "Version: 1.2.3") {
		t.Errorf("expected catalog semver Version: 1.2.3, got:\n%s", skillMD)
	}
}

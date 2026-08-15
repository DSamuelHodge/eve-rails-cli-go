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

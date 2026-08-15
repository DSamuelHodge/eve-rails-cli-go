package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateToolPlansCatalogAndStub(t *testing.T) {
	temp := t.TempDir()
	catalogPath := filepath.Join(temp, "catalog.yml")
	options := Options{
		Name:        "search",
		Version:     "1.0.0",
		SideEffects: "read",
		Catalog:     catalogPath,
	}
	changes, err := Plan(KindTool, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}
	catalogChange := changes[0]
	if !strings.Contains(catalogChange.Content, "  search:\n    version: 1.0.0\n    side_effects: read") {
		t.Errorf("unexpected catalog entry:\n%s", catalogChange.Content)
	}
	if !strings.Contains(changes[1].Path, "catalog/tools/search.ts") {
		t.Errorf("expected tool stub path, got %s", changes[1].Path)
	}
}

func TestGenerateAgentPlansManifestUpdate(t *testing.T) {
	temp := t.TempDir()
	manifestPath := filepath.Join(temp, "agents.yml")
	options := Options{
		Name:        "support",
		Version:     "1.0.0",
		Description: "handles support",
		WithTools:   []string{"search"},
		Manifest:    manifestPath,
	}
	changes, err := Plan(KindAgent, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	content := changes[0].Content
	for _, expected := range []string{
		"- name: support",
		"version: 1.0.0",
		"tools:\n      search: 1.0.0",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("expected %q in generated manifest:\n%s", expected, content)
		}
	}
}

func TestGenerateDuplicateRequiresForce(t *testing.T) {
	temp := t.TempDir()
	catalogPath := filepath.Join(temp, "catalog.yml")
	if err := os.WriteFile(catalogPath, []byte("tools:\n  search:\n    version: 1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	options := Options{Name: "search", Catalog: catalogPath}
	_, err := Plan(KindTool, options)
	if err == nil {
		t.Fatal("expected duplicate to fail without --force")
	}
	options.Force = true
	if _, err := Plan(KindTool, options); err != nil {
		t.Fatalf("expected duplicate to succeed with --force: %v", err)
	}
}

func TestCatalogUpsertKeepsEntryInRequestedSection(t *testing.T) {
	source := "tools:\n  existing:\n    version: 1.0.0\nskills:\n"
	entry := "  search:\n    version: 1.0.0\n"
	result := upsertCatalogEntry(source, "tools:", "search", entry)
	if !strings.Contains(result, "tools:\n  existing:\n    version: 1.0.0\n  search:\n") {
		t.Errorf("expected search entry within tools section:\n%s", result)
	}
	if !strings.Contains(result, "skills:\n") {
		t.Errorf("expected skills section preserved:\n%s", result)
	}
}

func TestGenerateMigrationPlansTimestampedFile(t *testing.T) {
	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(temp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	options := Options{Name: "add_memory"}
	changes, err := Plan(KindMigration, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	base := filepath.Base(changes[0].Path)
	if !strings.Contains(base, "_add_memory.ts") {
		t.Errorf("expected timestamped migration file, got %s", base)
	}
	if !strings.Contains(changes[0].Content, "export async function up()") {
		t.Errorf("expected migration template, got:\n%s", changes[0].Content)
	}
}

func TestGenerateSubagentPlansInstructions(t *testing.T) {
	options := Options{
		Name:        "reviewer",
		Description: "reviews code",
	}
	changes, err := Plan(KindSubagent, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if !strings.Contains(changes[0].Path, "catalog/subagents/reviewer/instructions.md") {
		t.Errorf("unexpected path: %s", changes[0].Path)
	}
	if !strings.Contains(changes[0].Content, "# reviewer") {
		t.Errorf("unexpected content: %s", changes[0].Content)
	}
}

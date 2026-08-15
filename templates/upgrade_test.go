package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareFreshTemplates(t *testing.T) {
	temp := t.TempDir()
	for _, name := range AgentNames() {
		content, err := AgentContent(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(temp, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report, err := Compare(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Missing) != 0 || len(report.Differing) != 0 {
		t.Fatalf("expected fresh templates, missing=%v differing=%v", report.Missing, report.Differing)
	}
	if err := Verify(temp); err != nil {
		t.Fatalf("expected Verify to pass: %v", err)
	}
}

func TestCompareReportsMissingTemplates(t *testing.T) {
	temp := t.TempDir()
	report, err := Compare(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Missing) == 0 {
		t.Fatal("expected missing templates on an empty directory")
	}
	if err := Verify(temp); err == nil {
		t.Fatal("expected Verify to fail for a stale directory")
	}
}

func TestCompareReportsDifferingTemplates(t *testing.T) {
	temp := t.TempDir()
	names := AgentNames()
	content, err := AgentContent(names[0])
	if err != nil {
		t.Fatal(err)
	}
	content = append(content, []byte("\n# customization\n")...)
	if err := os.WriteFile(filepath.Join(temp, names[0]), content, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range names[1:] {
		embedded, err := AgentContent(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(temp, name), embedded, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	report, err := Compare(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Differing) != 1 || report.Differing[0] != names[0] {
		t.Fatalf("expected %q differing, got %v", names[0], report.Differing)
	}
}

func TestUpgradePlansAllTemplates(t *testing.T) {
	temp := t.TempDir()
	changes, err := Upgrade(temp)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != len(AgentNames()) {
		t.Fatalf("expected %d changes, got %d", len(AgentNames()), len(changes))
	}
	for _, change := range changes {
		if change.Action != "create" {
			t.Errorf("expected create for %s, got %s", change.Path, change.Action)
		}
	}
	if err := os.MkdirAll(temp, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if err := os.WriteFile(change.Path, []byte(change.Content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", change.Path, err)
		}
	}
	if err := Verify(temp); err != nil {
		t.Fatalf("expected Verify to pass after applying Upgrade changes: %v", err)
	}
}

func TestUpgradePlansUpdateForCustomizedTemplate(t *testing.T) {
	temp := t.TempDir()
	name := AgentNames()[0]
	embedded, err := AgentContent(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(temp, 0o755); err != nil {
		t.Fatal(err)
	}
	customized := append(embedded, []byte("\n# customization\n")...)
	if err := os.WriteFile(filepath.Join(temp, name), customized, 0o644); err != nil {
		t.Fatal(err)
	}
	changes, err := Upgrade(temp)
	if err != nil {
		t.Fatal(err)
	}
	var updated bool
	for _, change := range changes {
		if change.Path == filepath.Join(temp, name) {
			updated = change.Action == "update"
		}
	}
	if !updated {
		t.Fatalf("expected an update for the customized template %s", name)
	}
}

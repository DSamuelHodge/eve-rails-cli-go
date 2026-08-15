package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationLedgerRoundTripsAppliedStatus(t *testing.T) {
	temp := t.TempDir()
	ledger := filepath.Join(temp, ".eve-rails", "migrations", "staging.applied")
	applied := map[string]bool{
		"agents/migrations/1000_add_memory.ts": true,
		"agents/migrations/2000_add_skill.ts":  true,
	}
	if err := writeAppliedMigrations(ledger, applied); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadAppliedMigrations(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 || !loaded["agents/migrations/1000_add_memory.ts"] {
		t.Fatalf("unexpected loaded ledger: %v", loaded)
	}
}

func TestMigrationScannerIncludesGlobalAndAgentMigrations(t *testing.T) {
	temp := t.TempDir()
	global := filepath.Join(temp, "agents", "migrations")
	agent := filepath.Join(temp, "agents", "support", "agent", "migrations")
	if err := os.MkdirAll(global, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agent, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(global, "1000_global.ts"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(agent, "2000_agent.ts"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(agent, "notes.txt"), []byte(""), 0o644)

	migrations, err := collectMigrations(filepath.Join(temp, "agents"))
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 {
		t.Fatalf("expected 2 ts migrations, got %d: %v", len(migrations), migrations)
	}
	for _, path := range migrations {
		if !strings.HasSuffix(path, ".ts") {
			t.Errorf("unexpected non-ts migration: %s", path)
		}
	}
}

func TestCollectMigrationsMissingPathReturnsEmpty(t *testing.T) {
	migrations, err := collectMigrations(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 0 {
		t.Fatalf("expected no migrations, got %v", migrations)
	}
}

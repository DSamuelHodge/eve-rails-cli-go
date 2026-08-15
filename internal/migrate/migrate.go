package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
)

// Plan is the migration plan for an environment.
type Plan struct {
	Env     string   `json:"env"`
	Agent   string   `json:"agent,omitempty"`
	Pending []string `json:"pending"`
	Ledger  string   `json:"ledger"`
	Apply   bool     `json:"apply"`
}

// Options carries the migrate command flags.
type Options struct {
	Agent    string
	Env      string
	Manifest string
	Fleet    string
	Apply    bool
	DryRun   bool
	JSON     bool
}

// Run executes the migrate command.
func Run(options Options) (*Plan, error) {
	manifestPath := options.Fleet
	if manifestPath == "" {
		manifestPath = options.Manifest
	}
	manifest, err := config.LoadManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	agents, err := config.SelectedAgents(manifest, options.Agent)
	if err != nil {
		return nil, err
	}

	ledger := ledgerPath(options.Env)
	applied, err := loadAppliedMigrations(ledger)
	if err != nil {
		return nil, err
	}
	pending, err := collectMigrations("agents/migrations")
	if err != nil {
		return nil, err
	}
	for _, agent := range agents {
		agentPath := filepath.Join("agents", agent.Name, "agent", "migrations")
		agentMigrations, err := collectMigrations(agentPath)
		if err != nil {
			return nil, err
		}
		pending = append(pending, agentMigrations...)
	}
	sort.Strings(pending)
	pending = dedupe(pending)
	var pendingFiles []string
	for _, path := range pending {
		if _, ok := applied[migrationKey(path)]; !ok {
			pendingFiles = append(pendingFiles, path)
		}
	}

	shouldApply := options.Apply && !options.DryRun
	if shouldApply {
		for _, path := range pendingFiles {
			applied[migrationKey(path)] = true
		}
		if err := writeAppliedMigrations(ledger, applied); err != nil {
			return nil, err
		}
	}

	plan := &Plan{
		Env:     options.Env,
		Agent:   options.Agent,
		Pending: pendingFiles,
		Ledger:  ledger,
		Apply:   shouldApply,
	}
	return plan, nil
}

// Print prints the migration plan.
func (plan *Plan) Print(manifestPath string) {
	fmt.Printf("Migration plan for %s\n", manifestPath)
	fmt.Printf("Environment: %s\n", plan.Env)
	fmt.Printf("Apply: %v\n", plan.Apply)
	fmt.Printf("Ledger: %s\n", plan.Ledger)
	if len(plan.Pending) == 0 {
		fmt.Println("No pending migration files found.")
		return
	}
	for _, path := range plan.Pending {
		fmt.Printf("pending %s\n", path)
	}
}

func ledgerPath(env string) string {
	return filepath.Join(".eve-rails", "migrations", fmt.Sprintf("%s.applied", env))
}

func collectMigrations(path string) ([]string, error) {
	var migrations []string
	err := filepath.WalkDir(path, func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(current, ".ts") {
			migrations = append(migrations, current)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read '%s': %w", path, err)
	}
	return migrations, nil
}

func loadAppliedMigrations(path string) (map[string]bool, error) {
	applied := make(map[string]bool)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return applied, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read '%s': %w", path, err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			applied[line] = true
		}
	}
	return applied, nil
}

func writeAppliedMigrations(path string, applied map[string]bool) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("failed to create '%s': %w", parent, err)
	}
	keys := make([]string, 0, len(applied))
	for key := range applied {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	content := strings.Join(keys, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write '%s': %w", path, err)
	}
	return nil
}

func migrationKey(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func dedupe(values []string) []string {
	seen := make(map[string]bool, len(values))
	var unique []string
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadManifest reads and parses a fleet manifest.
func LoadManifest(path string) (*FleetManifest, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest '%s': %w", path, err)
	}
	var manifest FleetManifest
	if err := yaml.Unmarshal(source, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest '%s': %w", path, err)
	}
	return &manifest, nil
}

// LoadCatalog reads and parses a component catalog.
func LoadCatalog(path string) (*CatalogManifest, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read catalog '%s': %w", path, err)
	}
	var catalog CatalogManifest
	if err := yaml.Unmarshal(source, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse catalog '%s': %w", path, err)
	}
	return &catalog, nil
}

// LoadEnvironments reads and parses environment policy. A missing file yields
// an empty configuration.
func LoadEnvironments(path string) (*EnvironmentsManifest, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &EnvironmentsManifest{}, nil
		}
		return nil, fmt.Errorf("failed to read '%s': %w", path, err)
	}
	var environments EnvironmentsManifest
	if err := yaml.Unmarshal(source, &environments); err != nil {
		return nil, fmt.Errorf("failed to parse environments '%s': %w", path, err)
	}
	return &environments, nil
}

// ReadOrDefault reads a file, returning the default content when it is missing.
func ReadOrDefault(path, fallback string) (string, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return "", fmt.Errorf("failed to read '%s': %w", path, err)
	}
	return string(source), nil
}

// DefaultManifestYAML is the starter fleet manifest written by init.
func DefaultManifestYAML() string {
	return "defaults:\n  model: openai/gpt-5.5\n  owner: agent-platform\n  channels: []\n  schedules: []\n  evals: []\n\nagents:\n"
}

// DefaultCatalogYAML is the starter catalog written by init.
func DefaultCatalogYAML() string {
	return "tools:\nskills:\nevals:\napprovals:\n  required:\n    version: 1.0.0\n  on-risk:\n    version: 1.0.0\nmemory:\nchannels:\nschedules:\n"
}

// SelectedAgents filters agents by name, erroring when the filter matches none.
func SelectedAgents(manifest *FleetManifest, agentFilter string) ([]*AgentManifest, error) {
	if agentFilter == "" {
		var all []*AgentManifest
		for i := range manifest.Agents {
			all = append(all, &manifest.Agents[i])
		}
		return all, nil
	}
	for i := range manifest.Agents {
		if manifest.Agents[i].Name == agentFilter {
			return []*AgentManifest{&manifest.Agents[i]}, nil
		}
	}
	return nil, fmt.Errorf("agent '%s' not found", agentFilter)
}

// IsSlug reports whether value is lowercase kebab-case or snake_case.
func IsSlug(value string) bool {
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// EnsureSlug validates a slug or returns an error.
func EnsureSlug(value string) error {
	if IsSlug(value) {
		return nil
	}
	return fmt.Errorf("'%s' must use lowercase kebab-case or snake_case", value)
}

// EnsureWritable errors when the path exists and force is false.
func EnsureWritable(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("'%s' already exists; pass --force to overwrite", path)
	}
	return nil
}

// EffectiveSchedules merges default, shared, and agent schedules, deduplicated
// in order.
func EffectiveSchedules(agent *AgentManifest, manifest *FleetManifest) []string {
	var schedules []string
	for _, schedule := range append(append(append([]string{}, manifest.Defaults.Schedules...), manifest.Shared.Schedules...), agent.Schedules...) {
		if !contains(schedules, schedule) {
			schedules = append(schedules, schedule)
		}
	}
	return schedules
}

// EffectiveStringList merges default and agent string lists, deduplicated.
func EffectiveStringList(defaults, agentValues []string) []string {
	var values []string
	for _, value := range append(append([]string{}, defaults...), agentValues...) {
		if !contains(values, value) {
			values = append(values, value)
		}
	}
	return values
}

// IsRiskySideEffect reports whether a side-effect class is non-read.
func IsRiskySideEffect(effects *SideEffects) bool {
	if effects == nil {
		return false
	}
	switch *effects {
	case SideEffectsWrite, SideEffectsExternal, SideEffectsMoney, SideEffectsProduction:
		return true
	}
	return false
}

// IsRiskyTool reports whether a catalog tool is non-read.
func IsRiskyTool(tool string, catalog *CatalogManifest) bool {
	component, ok := catalog.Tools[tool]
	if !ok {
		return false
	}
	return IsRiskySideEffect(component.SideEffects)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

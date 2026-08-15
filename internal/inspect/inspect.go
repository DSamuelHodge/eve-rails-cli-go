package inspect

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/doctor"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/render"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/versioning"
)

// Summary is the JSON-safe inspection of one agent.
type Summary struct {
	Name           string                `json:"name"`
	Version        string                `json:"version"`
	Owner          string                `json:"owner,omitempty"`
	Model          string                `json:"model,omitempty"`
	Responsibility string                `json:"responsibility"`
	Tools          map[string]string     `json:"tools"`
	Skills         map[string]string     `json:"skills"`
	Subagents      []SubagentSummary     `json:"subagents"`
	Channels       []string              `json:"channels"`
	Schedules      []string              `json:"schedules"`
	Approvals      map[string]string     `json:"approvals"`
	Evals          []string              `json:"evals"`
	Memory         map[string]string     `json:"memory"`
	RuntimePolicy  *config.RuntimePolicy `json:"runtime_policy,omitempty"`
	Generated      GeneratedSummary      `json:"generated"`
	Deployment     DeploymentSummary     `json:"deployment"`
}

// SubagentSummary is the JSON-safe inspection of one subagent.
type SubagentSummary struct {
	Name           string                `json:"name"`
	Title          string                `json:"title,omitempty"`
	RoleID         string                `json:"role_id,omitempty"`
	Responsibility string                `json:"responsibility,omitempty"`
	Model          string                `json:"model,omitempty"`
	Tools          []string              `json:"tools,omitempty"`
	Skills         []string              `json:"skills,omitempty"`
	Memory         []string              `json:"memory,omitempty"`
	Channels       []string              `json:"channels,omitempty"`
	Approvals      map[string]string     `json:"approvals,omitempty"`
	RuntimePolicy  *config.RuntimePolicy `json:"runtime_policy,omitempty"`
}

// GeneratedSummary describes generated artifact freshness.
type GeneratedSummary struct {
	ManifestPath string `json:"manifest_path"`
	LockfilePath string `json:"lockfile_path"`
	Fresh        bool   `json:"fresh"`
}

// DeploymentSummary describes deployability.
type DeploymentSummary struct {
	DoctorPassed bool `json:"doctor_passed"`
	Deployable   bool `json:"deployable"`
}

// Summarize builds the inspection summary for one agent.
func Summarize(agent *config.AgentManifest, manifest *config.FleetManifest, catalog *config.CatalogManifest, templateDir string) (*Summary, error) {
	report, err := doctor.RunDoctor(manifest, catalog, doctor.Options{
		Updates:      true,
		Templates:    true,
		Budgets:      true,
		Manifest:     "manifests/agents.yml",
		Catalog:      "manifests/catalog.yml",
		TemplateDir:  templateDir,
		Environments: "manifests/environments.yml",
	})
	if err != nil {
		return nil, err
	}
	generated, err := GeneratedSummaryFor(agent, manifest, catalog, templateDir)
	if err != nil {
		return nil, err
	}
	doctorPassed := !report.HasFailures()

	version := agent.Version
	if version == "" {
		version = "1.0.0"
	}
	owner := agent.Owner
	if owner == "" {
		owner = manifest.Defaults.Owner
	}
	model := agent.Model
	if model == "" {
		model = manifest.Defaults.Model
	}

	summary := &Summary{
		Name:           agent.Name,
		Version:        version,
		Owner:          owner,
		Model:          model,
		Responsibility: agent.Responsibility,
		Tools:          resolvedComponentSummary(agent.Tools, catalog.Tools),
		Skills:         resolvedComponentSummary(agent.Skills, catalog.Skills),
		Channels:       config.EffectiveStringList(manifest.Defaults.Channels, agent.Channels),
		Schedules:      config.EffectiveSchedules(agent, manifest),
		Approvals:      agent.Approvals,
		Evals:          versioning.EffectiveEvals(agent, manifest),
		Memory:         resolvedComponentSummary(agent.Memory, catalog.Memory),
		RuntimePolicy:  agent.RuntimePolicy,
		Generated:      *generated,
		Deployment: DeploymentSummary{
			DoctorPassed: doctorPassed,
			Deployable:   doctorPassed && len(versioning.EffectiveEvals(agent, manifest)) > 0,
		},
	}
	for i := range agent.Subagents {
		subagent := &agent.Subagents[i]
		role := subagentRole(agent, subagent, manifest)
		subsummary := SubagentSummary{
			Name:           subagent.Name,
			Title:          firstNonEmpty(subagent.Title, role.Title),
			RoleID:         firstNonEmpty(subagent.RoleID, role.RoleID),
			Responsibility: firstNonEmpty(subagent.Responsibility, role.Responsibility),
			Model:          subagent.Model,
			Tools:          subagent.Tools,
			Skills:         subagent.Skills,
			Memory:         subagent.Memory,
			Channels:       subagent.Channels,
			Approvals:      subagent.Approvals,
			RuntimePolicy:  subagent.RuntimePolicy,
		}
		if subsummary.RuntimePolicy == nil {
			subsummary.RuntimePolicy = role.RuntimePolicy
		}
		if subsummary.RuntimePolicy == nil {
			subsummary.RuntimePolicy = &config.RuntimePolicy{}
		}
		summary.Subagents = append(summary.Subagents, subsummary)
	}
	return summary, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// GeneratedSummaryFor reports whether rendered artifacts are fresh.
func GeneratedSummaryFor(agent *config.AgentManifest, manifest *config.FleetManifest, catalog *config.CatalogManifest, templateDir string) (*GeneratedSummary, error) {
	renderer, err := render.Load(templateDir)
	if err != nil {
		return nil, err
	}
	files, err := renderer.RenderAgent(agent, manifest, catalog)
	if err != nil {
		return nil, err
	}
	fresh := true
	for _, file := range files {
		matches, err := FileMatches(file.Path, file.Content)
		if err != nil {
			return nil, err
		}
		if !matches {
			fresh = false
		}
	}
	outputRoot := "agents/" + agent.Name + "/agent"
	return &GeneratedSummary{
		ManifestPath: outputRoot + "/agent.manifest.yml",
		LockfilePath: outputRoot + "/versions.lock",
		Fresh:        fresh,
	}, nil
}

// FileMatches reports whether on-disk content equals the expected content.
func FileMatches(path, expected string) (bool, error) {
	actual, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read '%s': %w", path, err)
	}
	return string(actual) == expected, nil
}

func resolvedComponentSummary(components config.ComponentMap, catalog map[string]config.CatalogComponent) map[string]string {
	summary := make(map[string]string, len(components))
	for name, requested := range components {
		component, ok := catalog[name]
		if !ok {
			summary[name] = requested
			continue
		}
		resolved := versioning.ResolveVersion(requested, &component)
		if resolved == "" {
			resolved = requested
		}
		summary[name] = resolved
	}
	return summary
}

// subagentRole resolves role metadata from the parent agent topology.
func subagentRole(agent *config.AgentManifest, subagent *config.SubagentManifest, manifest *config.FleetManifest) config.RoleTopology {
	if agent.Topology == nil {
		return config.RoleTopology{}
	}
	for _, principal := range agent.Topology.Principals {
		if principal.Name == subagent.Name {
			return principal
		}
	}
	for _, delegate := range agent.Topology.Delegates {
		if delegate.Name == subagent.Name {
			return delegate
		}
	}
	return config.RoleTopology{}
}

// FormatComponents renders a component map as a dashboard-friendly string.
func FormatComponents(components map[string]string) string {
	if len(components) == 0 {
		return "<none>"
	}
	names := make([]string, 0, len(components))
	for name := range components {
		names = append(names, name)
	}
	sort.Strings(names)
	var parts []string
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s@%s", name, components[name]))
	}
	return strings.Join(parts, ", ")
}

// FormatList renders a string slice as a dashboard-friendly string.
func FormatList(values []string) string {
	if len(values) == 0 {
		return "<none>"
	}
	return strings.Join(values, ", ")
}

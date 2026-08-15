package versioning

import (
	"fmt"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/semver"
)

// UpdateKind is the classification of a version change.
type UpdateKind string

const (
	UpdateCurrent UpdateKind = "current"
	UpdatePatch   UpdateKind = "patch"
	UpdateMinor   UpdateKind = "minor"
	UpdateMajor   UpdateKind = "major"
	UpdateMissing UpdateKind = "missing"
	UpdateInvalid UpdateKind = "invalid"
)

// VersionReport describes one component's update state.
type VersionReport struct {
	Agent         string     `json:"agent"`
	ComponentKind string     `json:"component_kind"`
	Component     string     `json:"component"`
	Requested     string     `json:"requested"`
	Resolved      string     `json:"resolved"`
	Update        UpdateKind `json:"update"`
	AffectedEvals []string   `json:"affected_evals"`
}

// CollectVersionReports reports update state for every agent component.
func CollectVersionReports(manifest *config.FleetManifest, catalog *config.CatalogManifest, agentFilter string) []VersionReport {
	var reports []VersionReport
	for i := range manifest.Agents {
		agent := &manifest.Agents[i]
		if agentFilter != "" && agentFilter != agent.Name {
			continue
		}
		collectComponentReports(&reports, agent, "tool", agent.Tools, catalog.Tools, manifest)
		collectComponentReports(&reports, agent, "skill", agent.Skills, catalog.Skills, manifest)
		collectComponentReports(&reports, agent, "memory", agent.Memory, catalog.Memory, manifest)
		for _, channel := range append(append([]string{}, manifest.Defaults.Channels...), agent.Channels...) {
			collectNamedReport(&reports, agent, "channel", channel, "catalog", catalog.Channels, manifest)
		}
		for _, schedule := range config.EffectiveSchedules(agent, manifest) {
			collectNamedReport(&reports, agent, "schedule", schedule, "catalog", catalog.Schedules, manifest)
		}
		for _, eval := range append(append([]string{}, manifest.Defaults.Evals...), agent.Evals...) {
			collectNamedReport(&reports, agent, "eval", eval, "catalog", catalog.Evals, manifest)
		}
	}
	return reports
}

func collectComponentReports(reports *[]VersionReport, agent *config.AgentManifest, kind string, components config.ComponentMap, catalog map[string]config.CatalogComponent, manifest *config.FleetManifest) {
	for name, requested := range components {
		collectNamedReport(reports, agent, kind, name, requested, catalog, manifest)
	}
}

func collectNamedReport(reports *[]VersionReport, agent *config.AgentManifest, kind, name, requested string, catalog map[string]config.CatalogComponent, manifest *config.FleetManifest) {
	component, found := catalog[name]
	var resolved string
	if found {
		resolved = component.Version
	}
	update := ApplyVersionPolicy(ClassifyUpdate(requested, resolved, catalog, name), kind, name, manifest)
	*reports = append(*reports, VersionReport{
		Agent:         agent.Name,
		ComponentKind: kind,
		Component:     name,
		Requested:     requested,
		Resolved:      resolved,
		Update:        update,
		AffectedEvals: EffectiveEvals(agent, manifest),
	})
}

// ApplyVersionPolicy narrows an update class under the manifest policy.
func ApplyVersionPolicy(update UpdateKind, kind, name string, manifest *config.FleetManifest) UpdateKind {
	var policy *config.PolicyUpdateKind
	switch kind {
	case "approval":
		policy = manifest.VersionPolicy.Approvals
	case "memory":
		policy = manifest.VersionPolicy.Memory
	case "tool":
		if p, ok := manifest.VersionPolicy.Tools[name]; ok {
			policy = &p
		} else {
			policy = manifest.VersionPolicy.Default
		}
	default:
		policy = manifest.VersionPolicy.Default
	}
	if policy == nil {
		return update
	}
	switch *policy {
	case config.PolicyPin:
		if update != UpdateCurrent {
			return UpdateMajor
		}
	case config.PolicyPatch, config.PolicyPatchAuto:
		if update == UpdateMinor || update == UpdateMajor {
			return UpdateMajor
		}
	case config.PolicyMinor:
		if update == UpdateMajor {
			return UpdateMajor
		}
	}
	return update
}

// ResolveVersion resolves a requested version to the catalog version, if it
// matches under the version request rules.
func ResolveVersion(requested string, component *config.CatalogComponent) string {
	if component == nil {
		return ""
	}
	if VersionRequestMatches(requested, component.Version) {
		return component.Version
	}
	return ""
}

// ClassifyUpdate compares a requested version against a catalog version.
func ClassifyUpdate(requested, resolved string, catalog map[string]config.CatalogComponent, name string) UpdateKind {
	component, ok := catalog[name]
	if !ok {
		return UpdateMissing
	}
	if requested == "catalog" {
		return UpdateCurrent
	}
	if (strings.HasPrefix(requested, "^") || strings.HasPrefix(requested, "~")) &&
		VersionRequestMatches(requested, component.Version) {
		return UpdateCurrent
	}
	requestedVersion, err := semver.Parse(strings.TrimLeft(requested, "^~"))
	if err != nil {
		return UpdateInvalid
	}
	catalogVersion, err := semver.Parse(component.Version)
	if err != nil {
		return UpdateInvalid
	}
	if requestedVersion.Equal(catalogVersion) && resolved == component.Version {
		return UpdateCurrent
	}
	switch {
	case requestedVersion.Major != catalogVersion.Major:
		return UpdateMajor
	case requestedVersion.Minor != catalogVersion.Minor:
		return UpdateMinor
	case requestedVersion.Patch != catalogVersion.Patch:
		return UpdatePatch
	}
	return UpdateCurrent
}

// VersionRequestMatches reports whether a requested version accepts the
// available version.
func VersionRequestMatches(requested, available string) bool {
	if requested == "catalog" {
		return true
	}
	if requested == available {
		return true
	}
	availableVersion, err := semver.Parse(available)
	if err != nil {
		return false
	}
	if strings.HasPrefix(requested, "^") {
		base, err := semver.Parse(strings.TrimPrefix(requested, "^"))
		if err != nil {
			return false
		}
		return availableVersion.Major == base.Major && availableVersion.Compare(base) >= 0
	}
	if strings.HasPrefix(requested, "~") {
		base, err := semver.Parse(strings.TrimPrefix(requested, "~"))
		if err != nil {
			return false
		}
		return availableVersion.Major == base.Major && availableVersion.Minor == base.Minor && availableVersion.Compare(base) >= 0
	}
	return false
}

// EffectiveEvals merges default and agent evals.
func EffectiveEvals(agent *config.AgentManifest, manifest *config.FleetManifest) []string {
	return append(append([]string{}, manifest.Defaults.Evals...), agent.Evals...)
}

// EffectiveComponents merges shared component names with agent overrides,
// preserving shared order and applying agent versions.
func EffectiveComponents(shared []string, agentComponents config.ComponentMap) []struct{ Name, Version string } {
	var components []struct{ Name, Version string }
	for _, name := range shared {
		components = append(components, struct{ Name, Version string }{Name: name, Version: "catalog"})
	}
	for name, version := range agentComponents {
		replaced := false
		for i := range components {
			if components[i].Name == name {
				components[i].Version = version
				replaced = true
				break
			}
		}
		if !replaced {
			components = append(components, struct{ Name, Version string }{Name: name, Version: version})
		}
	}
	return components
}

// PrintVersionReports writes a human-readable update report.
func PrintVersionReports(reports []VersionReport) {
	if len(reports) == 0 {
		fmt.Println("No components found.")
		return
	}
	allCurrent := true
	for _, report := range reports {
		if report.Update == UpdateCurrent {
			continue
		}
		allCurrent = false
		resolved := "<unresolved>"
		if report.Resolved != "" {
			resolved = report.Resolved
		}
		fmt.Printf("%s %s:%s requested %s resolved %s [%s] evals: %s\n",
			report.Agent, report.ComponentKind, report.Component, report.Requested,
			resolved, report.Update, strings.Join(report.AffectedEvals, ","))
	}
	if allCurrent {
		fmt.Println("All components are current.")
	}
}

package doctor

import (
	"fmt"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/versioning"
)

// ValidationReport collects manifest validation errors and warnings.
type ValidationReport struct {
	Errors   []string
	Warnings []string
}

var sandboxes = []string{
	"read-only",
	"network-read",
	"workspace-write",
	"external-write-gated",
	"production-write-gated",
	"approval-gated",
}

// ValidateManifest validates the fleet manifest against the catalog.
func ValidateManifest(manifest *config.FleetManifest, catalog *config.CatalogManifest) *ValidationReport {
	report := &ValidationReport{}

	if len(manifest.Agents) == 0 {
		report.Errors = append(report.Errors, "manifest must define at least one agent")
	}

	validateNamedList(report, "shared", "tool", manifest.Shared.Tools, catalog.Tools)
	validateNamedList(report, "shared", "skill", manifest.Shared.Skills, catalog.Skills)
	validateNamedList(report, "shared", "memory", manifest.Shared.Memory, catalog.Memory)
	validateNamedList(report, "shared", "schedule", manifest.Shared.Schedules, catalog.Schedules)
	for tool, requested := range manifest.Defaults.Tools {
		validateComponentVersion(report, "defaults", "tool", tool, requested, catalog.Tools)
	}
	for skill, requested := range manifest.Defaults.Skills {
		validateComponentVersion(report, "defaults", "skill", skill, requested, catalog.Skills)
	}
	for memory, requested := range manifest.Defaults.Memory {
		validateComponentVersion(report, "defaults", "memory", memory, requested, catalog.Memory)
	}
	if manifest.Defaults.Approvals != "" {
		validatePolicy(report, "defaults", manifest.Defaults.Approvals, catalog)
	}
	for tool, component := range catalog.Tools {
		if component.SideEffects != nil && *component.SideEffects == config.SideEffectsNone {
			report.Warnings = append(report.Warnings, fmt.Sprintf("tool '%s' explicitly declares side_effects: none; verify this is intentional", tool))
		}
		for _, approval := range component.RequiredApprovals {
			validatePolicy(report, fmt.Sprintf("tool '%s'", tool), approval, catalog)
		}
		for _, sandbox := range component.SandboxCompatibility {
			validateSandbox(report, fmt.Sprintf("tool '%s'", tool), sandbox)
		}
	}

	for i := range manifest.Agents {
		agent := &manifest.Agents[i]
		if strings.TrimSpace(agent.Name) == "" {
			report.Errors = append(report.Errors, "agent name cannot be empty")
		}
		if !config.IsSlug(agent.Name) {
			report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' must use lowercase kebab-case or snake_case", agent.Name))
		}
		if agent.Version == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' must define version", agent.Name))
		}
		if strings.TrimSpace(agent.Responsibility) == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' must define responsibility", agent.Name))
		}
		if agent.Model == "" && manifest.Defaults.Model == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' must define model or inherit defaults.model", agent.Name))
		}
		if agent.Owner == "" && manifest.Defaults.Owner == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' must define owner or inherit defaults.owner", agent.Name))
		}
		if len(agent.Evals) == 0 && len(manifest.Defaults.Evals) == 0 {
			report.Warnings = append(report.Warnings, fmt.Sprintf("agent '%s' has no evals", agent.Name))
		}
		if agent.RuntimePolicy != nil {
			validateRuntimePolicy(report, fmt.Sprintf("agent '%s'", agent.Name), agent.RuntimePolicy, agent.Tools, agent.Approvals, catalog)
		}
		validateChannelList(report, fmt.Sprintf("agent '%s'", agent.Name), manifest.Defaults.Channels, catalog.Channels)
		validateChannelList(report, fmt.Sprintf("agent '%s'", agent.Name), agent.Channels, catalog.Channels)
		validateNamedList(report, fmt.Sprintf("agent '%s'", agent.Name), "schedule", manifest.Defaults.Schedules, catalog.Schedules)
		validateNamedList(report, fmt.Sprintf("agent '%s'", agent.Name), "schedule", agent.Schedules, catalog.Schedules)
		validateNamedList(report, fmt.Sprintf("agent '%s'", agent.Name), "eval", manifest.Defaults.Evals, catalog.Evals)
		validateNamedList(report, fmt.Sprintf("agent '%s'", agent.Name), "eval", agent.Evals, catalog.Evals)

		for tool, requested := range agent.Tools {
			if strings.TrimSpace(requested) == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' tool '%s' has empty version", agent.Name, tool))
			}
			validateComponentVersion(report, fmt.Sprintf("agent '%s'", agent.Name), "tool", tool, requested, catalog.Tools)
			if config.IsRiskyTool(tool, catalog) {
				if _, ok := agent.Approvals[tool]; !ok {
					report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' risky tool '%s' must have approval policy", agent.Name, tool))
				}
			}
			if component, ok := catalog.Tools[tool]; ok {
				for _, approval := range component.RequiredApprovals {
					covered := false
					for _, policy := range agent.Approvals {
						if policy == approval {
							covered = true
							break
						}
					}
					if !covered {
						report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' tool '%s' requires approval policy '%s'", agent.Name, tool, approval))
					}
					warnMissingBlocking(report, fmt.Sprintf("agent '%s'", agent.Name), tool, approval, catalog)
				}
			}
		}

		for skill, requested := range agent.Skills {
			validateComponentVersion(report, fmt.Sprintf("agent '%s'", agent.Name), "skill", skill, requested, catalog.Skills)
		}

		for memory, requested := range agent.Memory {
			if strings.TrimSpace(requested) == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' memory '%s' has empty version", agent.Name, memory))
			}
			validateComponentVersion(report, fmt.Sprintf("agent '%s'", agent.Name), "memory", memory, requested, catalog.Memory)
			if component, ok := catalog.Memory[memory]; ok && strings.TrimSpace(component.Retention) == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' memory '%s' must declare retention in catalog", agent.Name, memory))
			}
		}

		for tool, policy := range agent.Approvals {
			if _, ok := agent.Tools[tool]; !ok {
				report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' approval policy for '%s' does not match an agent tool", agent.Name, tool))
			}
			if strings.TrimSpace(policy) == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' approval policy for '%s' cannot be empty", agent.Name, tool))
			}
			validatePolicy(report, fmt.Sprintf("agent '%s'", agent.Name), policy, catalog)
			warnMissingBlocking(report, fmt.Sprintf("agent '%s'", agent.Name), tool, policy, catalog)
		}

		for j := range agent.Subagents {
			subagent := &agent.Subagents[j]
			if strings.TrimSpace(subagent.Name) == "" {
				report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' subagent name cannot be empty", agent.Name))
			}
			if !config.IsSlug(subagent.Name) {
				report.Errors = append(report.Errors, fmt.Sprintf("agent '%s' subagent '%s' must use lowercase kebab-case or snake_case", agent.Name, subagent.Name))
			}
			scope := fmt.Sprintf("agent '%s' subagent '%s'", agent.Name, subagent.Name)
			validateNamedList(report, scope, "tool", subagent.Tools, catalog.Tools)
			validateNamedList(report, scope, "skill", subagent.Skills, catalog.Skills)
			validateNamedList(report, scope, "memory", subagent.Memory, catalog.Memory)
			if len(subagent.Channels) > 0 {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s declares channels, but Eve channels are root-only and are not generated for subagents", scope))
			}
			for _, policy := range subagent.Approvals {
				validatePolicy(report, scope, policy, catalog)
			}
			for tool, policy := range subagent.Approvals {
				warnMissingBlocking(report, scope, tool, policy, catalog)
			}
			if subagent.RuntimePolicy != nil {
				tools := make(config.ComponentMap)
				for _, tool := range subagent.Tools {
					tools[tool] = "catalog"
				}
				validateRuntimePolicy(report, scope, subagent.RuntimePolicy, tools, subagent.Approvals, catalog)
			}
		}
	}

	return report
}

func validateRuntimePolicy(report *ValidationReport, scope string, policy *config.RuntimePolicy, tools config.ComponentMap, approvals map[string]string, catalog *config.CatalogManifest) {
	sandbox := policy.Sandbox
	if sandbox == "" {
		sandbox = "read-only"
	}
	validateSandbox(report, scope, sandbox)

	for _, tool := range policy.AllowedTools {
		if _, ok := tools[tool]; !ok {
			report.Errors = append(report.Errors, fmt.Sprintf("%s runtime policy allows tool '%s' that is not declared", scope, tool))
		}
	}

	for _, approval := range policy.Approvals {
		validatePolicy(report, scope, approval, catalog)
	}

	for tool := range tools {
		component, ok := catalog.Tools[tool]
		if !ok {
			continue
		}
		if !toolAllowedBySandbox(component.SideEffects, sandbox) {
			report.Errors = append(report.Errors, fmt.Sprintf("%s runtime policy sandbox '%s' is incompatible with tool '%s'", scope, sandbox, tool))
		}
		if len(component.SandboxCompatibility) > 0 && !containsString(component.SandboxCompatibility, sandbox) {
			report.Errors = append(report.Errors, fmt.Sprintf("%s runtime policy sandbox '%s' is not listed in tool '%s' sandbox_compatibility", scope, sandbox, tool))
		}
		if config.IsRiskySideEffect(component.SideEffects) {
			_, approved := approvals[tool]
			if !approved && len(policy.Approvals) == 0 {
				report.Errors = append(report.Errors, fmt.Sprintf("%s runtime policy grants non-read tool '%s' without approval coverage", scope, tool))
			}
		}
	}
}

func validateSandbox(report *ValidationReport, scope, sandbox string) {
	if !containsString(sandboxes, sandbox) {
		report.Errors = append(report.Errors, fmt.Sprintf("%s references unsupported runtime policy sandbox '%s'", scope, sandbox))
	}
}

func toolAllowedBySandbox(sideEffects *config.SideEffects, sandbox string) bool {
	effects := config.SideEffectsRead
	if sideEffects != nil {
		effects = *sideEffects
	}
	switch effects {
	case config.SideEffectsNone, config.SideEffectsRead:
		return true
	case config.SideEffectsWrite:
		switch sandbox {
		case "workspace-write", "external-write-gated", "production-write-gated", "approval-gated":
			return true
		}
	case config.SideEffectsExternal:
		switch sandbox {
		case "external-write-gated", "production-write-gated", "approval-gated":
			return true
		}
	case config.SideEffectsMoney, config.SideEffectsProduction:
		switch sandbox {
		case "production-write-gated", "approval-gated":
			return true
		}
	}
	return false
}

func validatePolicy(report *ValidationReport, scope, policy string, catalog *config.CatalogManifest) {
	if _, ok := catalog.Approvals[policy]; !ok {
		report.Errors = append(report.Errors, fmt.Sprintf("%s references missing approval policy '%s'", scope, policy))
	}
}

// warnMissingBlocking warns when an approval policy assigned to a tool does not
// declare `blocking`, so the rendered approval gate defaults to always() and the
// policy's semantics depend on prose rather than a typed field.
func warnMissingBlocking(report *ValidationReport, scope, tool, policy string, catalog *config.CatalogManifest) {
	component, ok := catalog.Approvals[policy]
	if !ok {
		return
	}
	if component.Blocking != nil {
		return
	}
	report.Warnings = append(report.Warnings, fmt.Sprintf("%s approval policy '%s' for tool '%s' does not declare blocking; the rendered gate defaults to always() (blocking)", scope, policy, tool))
}

func validateNamedList(report *ValidationReport, scope, kind string, names []string, catalog map[string]config.CatalogComponent) {
	for _, name := range names {
		if _, ok := catalog[name]; !ok {
			report.Errors = append(report.Errors, fmt.Sprintf("%s references missing %s '%s'", scope, kind, name))
		}
	}
}

// validateChannelList validates channel names, treating known platform presets
// (slack, telegram, ...) as implicitly available even without a catalog entry.
func validateChannelList(report *ValidationReport, scope string, names []string, catalog map[string]config.CatalogComponent) {
	for _, name := range names {
		if _, ok := catalog[name]; ok {
			continue
		}
		if config.IsPresetChannel(name) {
			continue
		}
		report.Errors = append(report.Errors, fmt.Sprintf("%s references missing channel '%s'; add it to catalog.yml or use a platform preset such as slack or telegram", scope, name))
	}
}

func validateComponentVersion(report *ValidationReport, scope, kind, name, requested string, catalog map[string]config.CatalogComponent) {
	component, ok := catalog[name]
	if !ok {
		report.Errors = append(report.Errors, fmt.Sprintf("%s references missing %s '%s'", scope, kind, name))
		return
	}
	if !versioning.VersionRequestMatches(requested, component.Version) {
		report.Errors = append(report.Errors, fmt.Sprintf("%s %s '%s' requests version %s, which does not resolve to catalog version %s", scope, kind, name, requested, component.Version))
	}
}

// PrintValidationReport prints the validation report.
func PrintValidationReport(report *ValidationReport) {
	if len(report.Errors) == 0 && len(report.Warnings) == 0 {
		fmt.Println()
		fmt.Println("Validation: passed")
		return
	}
	if len(report.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, warning := range report.Warnings {
			fmt.Printf("- %s\n", warning)
		}
	}
	if len(report.Errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, err := range report.Errors {
			fmt.Printf("- %s\n", err)
		}
	}
}

// EnsureValid returns an error when validation errors exist.
func EnsureValid(report *ValidationReport) error {
	if len(report.Errors) == 0 {
		return nil
	}
	return fmt.Errorf("validation failed with %d error(s): %s", len(report.Errors), strings.Join(report.Errors, "; "))
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

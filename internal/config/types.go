package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ComponentMap maps a component name to a requested version.
type ComponentMap map[string]string

// PresetChannel returns a synthesized channel component for a known platform
// slug, so channels like slack and telegram work with no catalog.yml entry.
func PresetChannel(name string) *CatalogComponent {
	switch name {
	case "slack":
		return &CatalogComponent{Kind: "slack", Version: "1.0.0", ConnectUID: "slack/my-agent", Description: "Slack mentions and DMs via Vercel Connect."}
	case "telegram":
		return &CatalogComponent{Kind: "telegram", Version: "1.0.0", BotUsername: "my_bot", Description: "Telegram bot webhooks."}
	case "discord":
		return &CatalogComponent{Kind: "discord", Version: "1.0.0", Description: "Discord slash commands and components."}
	case "teams":
		return &CatalogComponent{Kind: "teams", Version: "1.0.0", Description: "Microsoft Teams messages and adaptive cards."}
	case "twilio":
		return &CatalogComponent{Kind: "twilio", Version: "1.0.0", AllowFrom: "env:TWILIO_ALLOWED_FROM", MessagingFrom: "env:TWILIO_FROM_NUMBER", Description: "SMS via Twilio."}
	case "linear":
		return &CatalogComponent{Kind: "linear", Version: "1.0.0", Description: "Linear issue delegation."}
	case "github":
		return &CatalogComponent{Kind: "github", Version: "1.0.0", Description: "GitHub mentions and PR review."}
	case "eve":
		return &CatalogComponent{Kind: "eve", Version: "1.0.0", Description: "Default HTTP session channel."}
	}
	return nil
}

// IsPresetChannel reports whether name is a known platform channel preset.
func IsPresetChannel(name string) bool {
	return PresetChannel(name) != nil
}

// FleetManifest is the root fleet intent document.
type FleetManifest struct {
	Defaults      ManifestDefaults `yaml:"defaults"`
	VersionPolicy VersionPolicy    `yaml:"version_policy"`
	Shared        SharedComponents `yaml:"shared"`
	Agents        []AgentManifest  `yaml:"agents"`
}

// ManifestDefaults holds fleet-wide defaults inherited by agents.
type ManifestDefaults struct {
	Model       string       `yaml:"model"`
	Owner       string       `yaml:"owner"`
	Auth        string       `yaml:"auth"`
	Visibility  string       `yaml:"visibility"`
	CostBudget  *float64     `yaml:"cost_budget"`
	TokenBudget *uint64      `yaml:"token_budget"`
	Timeout     string       `yaml:"timeout"`
	Channels    []string     `yaml:"channels"`
	Schedules   []string     `yaml:"schedules"`
	Evals       []string     `yaml:"evals"`
	Approvals   string       `yaml:"approvals"`
	Tools       ComponentMap `yaml:"tools"`
	Skills      ComponentMap `yaml:"skills"`
	Memory      ComponentMap `yaml:"memory"`
}

// VersionPolicy controls update classification for component kinds.
type VersionPolicy struct {
	Default   *PolicyUpdateKind           `yaml:"default"`
	Approvals *PolicyUpdateKind           `yaml:"approvals"`
	Memory    *PolicyUpdateKind           `yaml:"memory"`
	Tools     map[string]PolicyUpdateKind `yaml:"tools"`
}

// PolicyUpdateKind is a kebab-case update policy.
type PolicyUpdateKind string

const (
	PolicyPin       PolicyUpdateKind = "pin"
	PolicyPatch     PolicyUpdateKind = "patch"
	PolicyPatchAuto PolicyUpdateKind = "patch-auto"
	PolicyMinor     PolicyUpdateKind = "minor"
	PolicyMajor     PolicyUpdateKind = "major"
)

// SharedComponents are component names shared fleet-wide.
type SharedComponents struct {
	Tools     []string `yaml:"tools"`
	Skills    []string `yaml:"skills"`
	Memory    []string `yaml:"memory"`
	Schedules []string `yaml:"schedules"`
}

// AgentManifest describes one agent composition.
type AgentManifest struct {
	Name           string            `yaml:"name"`
	Version        string            `yaml:"version"`
	Owner          string            `yaml:"owner"`
	Responsibility string            `yaml:"responsibility"`
	Model          string            `yaml:"model"`
	Tools          ComponentMap      `yaml:"tools"`
	Skills         ComponentMap      `yaml:"skills"`
	Subagents      SubagentList      `yaml:"subagents"`
	Channels       []string          `yaml:"channels"`
	Schedules      []string          `yaml:"schedules"`
	Approvals      map[string]string `yaml:"approvals"`
	Evals          []string          `yaml:"evals"`
	Memory         ComponentMap      `yaml:"memory"`
	Risk           string            `yaml:"risk"`
	Auth           string            `yaml:"auth"`
	Visibility     string            `yaml:"visibility"`
	CostBudget     *float64          `yaml:"cost_budget"`
	TokenBudget    *uint64           `yaml:"token_budget"`
	Timeout        string            `yaml:"timeout"`
	Topology       *AgentTopology    `yaml:"x_topology"`
	RuntimePolicy  *RuntimePolicy    `yaml:"x_runtime_policy"`
}

// AgentTopology describes organizational placement and roles.
type AgentTopology struct {
	Department  string         `yaml:"department" json:"department,omitempty"`
	ParentAgent bool           `yaml:"parent_agent" json:"parent_agent"`
	Principals  []RoleTopology `yaml:"principals" json:"principals,omitempty"`
	Delegates   []RoleTopology `yaml:"delegates" json:"delegates,omitempty"`
}

// RoleTopology describes a principal or delegate role.
type RoleTopology struct {
	Name           string         `yaml:"name" json:"name"`
	Title          string         `yaml:"title" json:"title"`
	RoleID         string         `yaml:"role_id" json:"role_id"`
	Responsibility string         `yaml:"responsibility" json:"responsibility"`
	RuntimePolicy  *RuntimePolicy `yaml:"runtime_policy" json:"runtime_policy,omitempty"`
}

// RuntimePolicy is a sandbox and approval boundary.
type RuntimePolicy struct {
	Sandbox          string   `yaml:"sandbox" json:"sandbox,omitempty"`
	AllowedTools     []string `yaml:"allowed_tools" json:"allowed_tools,omitempty"`
	Approvals        []string `yaml:"approvals" json:"approvals,omitempty"`
	ForbiddenActions []string `yaml:"forbidden_actions" json:"forbidden_actions,omitempty"`
}

// SubagentManifest describes a subagent of a parent agent.
type SubagentManifest struct {
	Name           string            `yaml:"name" json:"name"`
	Title          string            `yaml:"title" json:"title,omitempty"`
	RoleID         string            `yaml:"role_id" json:"role_id,omitempty"`
	Responsibility string            `yaml:"responsibility" json:"responsibility,omitempty"`
	Model          string            `yaml:"model" json:"model,omitempty"`
	Tools          []string          `yaml:"tools" json:"tools,omitempty"`
	Skills         []string          `yaml:"skills" json:"skills,omitempty"`
	Memory         []string          `yaml:"memory" json:"memory,omitempty"`
	Channels       []string          `yaml:"channels" json:"channels,omitempty"`
	Approvals      map[string]string `yaml:"approvals" json:"approvals,omitempty"`
	RuntimePolicy  *RuntimePolicy    `yaml:"runtime_policy" json:"runtime_policy,omitempty"`
}

// SubagentList supports YAML entries that are either a plain name string or a
// full subagent object (serde untagged enum parity).
type SubagentList []SubagentManifest

func (list *SubagentList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("subagents must be a list")
	}
	var result []SubagentManifest
	for _, item := range node.Content {
		var sub SubagentManifest
		if item.Kind == yaml.ScalarNode {
			sub = SubagentManifest{Name: item.Value}
		} else {
			if err := item.Decode(&sub); err != nil {
				return err
			}
		}
		result = append(result, sub)
	}
	*list = result
	return nil
}

// CatalogManifest holds reusable components.
type CatalogManifest struct {
	Tools     map[string]CatalogComponent `yaml:"tools"`
	Skills    map[string]CatalogComponent `yaml:"skills"`
	Evals     map[string]CatalogComponent `yaml:"evals"`
	Approvals map[string]CatalogComponent `yaml:"approvals"`
	Memory    map[string]CatalogComponent `yaml:"memory"`
	Channels  map[string]CatalogComponent `yaml:"channels"`
	Schedules map[string]CatalogComponent `yaml:"schedules"`
}

// CatalogComponent is one reusable component definition.
type CatalogComponent struct {
	Version              string       `yaml:"version"`
	Description          string       `yaml:"description"`
	Kind                 string       `yaml:"kind"`
	SideEffects          *SideEffects `yaml:"side_effects"`
	Blocking             *bool        `yaml:"blocking"`
	RequiredApprovals    []string     `yaml:"required_approvals"`
	RequiredEnv          []string     `yaml:"required_env"`
	RequiredConnectors   []string     `yaml:"required_connectors"`
	SandboxCompatibility []string     `yaml:"sandbox_compatibility"`
	FailureModes         []string     `yaml:"failure_modes"`
	Retention            string       `yaml:"retention"`
	Schedule             string       `yaml:"schedule"`
	AllowFrom            string       `yaml:"allow_from"`
	MessagingFrom        string       `yaml:"messaging_from"`
	ConnectUID           string       `yaml:"connect_uid"`
	BotUsername          string       `yaml:"bot_username"`
	BotName              string       `yaml:"bot_name"`
	Owner                string       `yaml:"owner"`
	Auth                 string       `yaml:"auth"`
	Visibility           string       `yaml:"visibility"`
}

// EnvironmentsManifest maps environment names to policies.
type EnvironmentsManifest struct {
	Environments map[string]EnvironmentPolicy `yaml:"environments"`
}

// EnvironmentPolicy is a deployment environment policy.
type EnvironmentPolicy struct {
	RequiredEnv         []string      `yaml:"required_env"`
	RequiredSecrets     []string      `yaml:"required_secrets"`
	RequiredConnections []string      `yaml:"required_connections"`
	Observability       Observability `yaml:"observability"`
}

// Observability mirrors serde's bool-or-string coercion for the
// observability field.
type Observability struct {
	value *bool
}

func (o *Observability) UnmarshalYAML(node *yaml.Node) error {
	o.value = nil
	if node.Kind != yaml.ScalarNode {
		return nil
	}
	switch node.Tag {
	case "!!bool":
		var value bool
		if err := node.Decode(&value); err != nil {
			return err
		}
		o.value = &value
	case "!!str":
		value := false
		switch node.Value {
		case "required", "enabled", "verbose", "true", "yes":
			value = true
		}
		o.value = &value
	}
	return nil
}

// Bool returns the effective observability value (false when unset).
func (o Observability) Bool() bool {
	return o.value != nil && *o.value
}

// SideEffects is a kebab-case tool side-effect class.
type SideEffects string

const (
	SideEffectsNone       SideEffects = "none"
	SideEffectsRead       SideEffects = "read"
	SideEffectsWrite      SideEffects = "write"
	SideEffectsExternal   SideEffects = "external"
	SideEffectsMoney      SideEffects = "money"
	SideEffectsProduction SideEffects = "production"
)

// ParseSideEffects validates a CLI-provided side-effects value.
func ParseSideEffects(value string) (SideEffects, error) {
	switch SideEffects(value) {
	case SideEffectsNone, SideEffectsRead, SideEffectsWrite, SideEffectsExternal,
		SideEffectsMoney, SideEffectsProduction:
		return SideEffects(value), nil
	default:
		return "", fmt.Errorf("unknown side-effects '%s'; expected none, read, write, external, money, or production", value)
	}
}

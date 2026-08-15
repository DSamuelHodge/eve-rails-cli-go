package hotload

import (
	"fmt"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/semver"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/versioning"
)

// ComponentKind is the kind of component in a component reference.
type ComponentKind string

const (
	KindTool     ComponentKind = "tool"
	KindSkill    ComponentKind = "skill"
	KindSubagent ComponentKind = "subagent"
	KindChannel  ComponentKind = "channel"
	KindSchedule ComponentKind = "schedule"
	KindApproval ComponentKind = "approval"
	KindEval     ComponentKind = "eval"
	KindMemory   ComponentKind = "memory"
	KindRuntime  ComponentKind = "runtime"
)

// String returns the kebab-case kind name.
func (kind ComponentKind) String() string {
	return string(kind)
}

// Action is the classified hot-load outcome.
type Action string

const (
	ActionHotload   Action = "hotload"
	ActionRestart   Action = "restart"
	ActionRedeploy  Action = "redeploy"
	ActionMigration Action = "migration"
	ActionDeny      Action = "deny"
)

// String returns the kebab-case action name.
func (action Action) String() string {
	return string(action)
}

// Ref is a parsed kind:name@version component reference.
type Ref struct {
	Kind    ComponentKind `json:"kind"`
	Name    string        `json:"name"`
	Version string        `json:"version"`
}

// Parse parses a kind:name@version component reference.
func Parse(value string) (*Ref, error) {
	kindPart, rest, ok := strings.Cut(value, ":")
	if !ok {
		return nil, fmt.Errorf("component ref '%s' must look like kind:name@version", value)
	}
	name, version, ok := strings.Cut(rest, "@")
	if !ok {
		return nil, fmt.Errorf("component ref '%s' must include @version", value)
	}
	kind, err := parseKind(kindPart)
	if err != nil {
		return nil, err
	}
	if err := config.EnsureSlug(name); err != nil {
		return nil, err
	}
	return &Ref{Kind: kind, Name: name, Version: version}, nil
}

func parseKind(value string) (ComponentKind, error) {
	switch value {
	case "tool":
		return KindTool, nil
	case "skill":
		return KindSkill, nil
	case "subagent":
		return KindSubagent, nil
	case "channel":
		return KindChannel, nil
	case "schedule":
		return KindSchedule, nil
	case "approval":
		return KindApproval, nil
	case "eval":
		return KindEval, nil
	case "memory":
		return KindMemory, nil
	case "runtime":
		return KindRuntime, nil
	}
	return "", fmt.Errorf("unknown component kind '%s'", value)
}

// Classification is the result of classifying a component change.
type Classification struct {
	Action Action                `json:"action"`
	Reason string                `json:"reason"`
	Update versioning.UpdateKind `json:"update"`
}

// ClassifyVersionChange classifies the semver change from current to next.
func ClassifyVersionChange(current, next string) versioning.UpdateKind {
	if current == next {
		return versioning.UpdateCurrent
	}
	currentSemver, errCurrent := semver.Parse(current)
	nextSemver, errNext := semver.Parse(next)
	if errCurrent != nil || errNext != nil {
		return versioning.UpdateInvalid
	}
	if currentSemver.Major != nextSemver.Major {
		return versioning.UpdateMajor
	}
	if currentSemver.Minor != nextSemver.Minor {
		return versioning.UpdateMinor
	}
	return versioning.UpdatePatch
}

// Classify classifies whether a component change can hot-load.
func Classify(component *Ref, current string) (*Classification, error) {
	update := ClassifyVersionChange(current, component.Version)
	var action Action
	var reason string
	switch component.Kind {
	case KindSkill:
		if update == versioning.UpdatePatch {
			action, reason = ActionHotload, "skill patch preserves trigger/tool/output contract by policy"
		}
	case KindEval:
		if update == versioning.UpdatePatch || update == versioning.UpdateMinor {
			action, reason = ActionHotload, "eval additions are safe to hot-load"
		}
	case KindApproval:
		if update == versioning.UpdatePatch {
			action, reason = ActionHotload, "approval patch is treated as stricter policy metadata"
		}
	case KindChannel:
		if update == versioning.UpdatePatch {
			action, reason = ActionRestart, "channel adapter patch requires process restart"
		}
	case KindSchedule:
		action, reason = ActionRedeploy, "schedule changes can increase autonomous activity and require redeploy"
	case KindTool:
		if update == versioning.UpdatePatch {
			action, reason = ActionRedeploy, "tool patches may alter side effects or schema and require redeploy"
		}
	case KindMemory:
		action, reason = ActionMigration, "memory schema changes require migration"
	case KindRuntime:
		action, reason = ActionRedeploy, "runtime changes require redeploy"
	case KindSubagent:
		action, reason = ActionRedeploy, "subagent contract changes require redeploy"
	}
	if action == "" {
		switch update {
		case versioning.UpdateMajor:
			action, reason = ActionRedeploy, "major updates require explicit redeploy"
		case versioning.UpdateInvalid:
			action, reason = ActionDeny, "invalid version change cannot be classified"
		default:
			action, reason = ActionRedeploy, "change is not eligible for conservative hot-load"
		}
	}
	return &Classification{Action: action, Reason: reason, Update: update}, nil
}

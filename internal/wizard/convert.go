package wizard

import (
	"sort"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
)

// AttachLists groups reusable component names by attach category.
type AttachLists struct {
	Tools     []string
	Skills    []string
	Channels  []string
	Schedules []string
	Evals     []string
	Memory    []string
	Subagents []string
}

// AvailableAttachments converts a catalog into sorted attach lists for
// multi-select fields. Subagents come from the on-disk catalog subagent
// directory when present.
func AvailableAttachments(catalog *config.CatalogManifest, subagentNames []string) AttachLists {
	return AttachLists{
		Tools:     sortedKeys(catalog.Tools),
		Skills:    sortedKeys(catalog.Skills),
		Channels:  sortedKeys(catalog.Channels),
		Schedules: sortedKeys(catalog.Schedules),
		Evals:     sortedKeys(catalog.Evals),
		Memory:    sortedKeys(catalog.Memory),
		Subagents: sortedUnique(subagentNames),
	}
}

// InitParams carries the values needed to write a project skeleton.
type InitParams struct {
	Name     string
	Template string
	Model    string
	Owner    string
}

// ToInitParams derives init parameters from the wizard state.
func (state *State) ToInitParams() InitParams {
	return InitParams{
		Name:     state.ProjectName,
		Template: state.Template,
		Model:    state.Model,
		Owner:    state.Owner,
	}
}

// ComponentKindNames returns the selectable component kinds for the wizard
// loop, in display order. The agent is handled by its own dedicated step.
func ComponentKindNames() []string {
	return []string{
		"tool",
		"skill",
		"subagent",
		"channel",
		"schedule",
		"approval",
		"eval",
		"memory",
		"migration",
	}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

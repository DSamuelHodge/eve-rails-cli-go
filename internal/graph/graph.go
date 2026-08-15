package graph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/inspect"
)

// Mode selects which edges are included in the graph.
type Mode string

const (
	ModeAll       Mode = "all"
	ModeTopology  Mode = "topology"
	ModeTools     Mode = "tools"
	ModeApprovals Mode = "approvals"
	ModeRuntime   Mode = "runtime"
	ModeChannels  Mode = "channels"
)

// String returns the kebab-case mode name.
func (mode Mode) String() string {
	return string(mode)
}

// Format is the output format for the graph.
type Format string

const (
	FormatText    Format = "text"
	FormatMermaid Format = "mermaid"
	FormatJSON    Format = "json"
)

// Options carries the graph command flags.
type Options struct {
	All         bool
	Agent       string
	Format      Format
	Mode        Mode
	Manifest    string
	Catalog     string
	TemplateDir string
	JSON        bool
}

// Run builds and prints the graph.
func Run(options Options, manifest *config.FleetManifest, catalog *config.CatalogManifest) error {
	var agents []*config.AgentManifest
	if options.All {
		agents = make([]*config.AgentManifest, 0, len(manifest.Agents))
		for i := range manifest.Agents {
			agents = append(agents, &manifest.Agents[i])
		}
	} else if options.Agent != "" {
		var found *config.AgentManifest
		for i := range manifest.Agents {
			if manifest.Agents[i].Name == options.Agent {
				found = &manifest.Agents[i]
				break
			}
		}
		if found == nil {
			return fmt.Errorf("agent '%s' not found", options.Agent)
		}
		agents = []*config.AgentManifest{found}
	} else {
		return fmt.Errorf("pass --agent <name> or --all")
	}

	var summaries []*inspect.Summary
	for _, agent := range agents {
		summary, err := inspect.Summarize(agent, manifest, catalog, options.TemplateDir)
		if err != nil {
			return err
		}
		summaries = append(summaries, summary)
	}

	format := options.Format
	if format == "" {
		if options.JSON {
			format = FormatJSON
		} else {
			format = FormatText
		}
	}
	mode := options.Mode
	if mode == "" {
		mode = ModeAll
	}

	switch format {
	case FormatJSON:
		fmt.Println(JSON(summaries, mode))
	case FormatMermaid:
		printMermaid(summaries, mode)
	default:
		printText(summaries, mode)
	}
	return nil
}

func printText(summaries []*inspect.Summary, mode Mode) {
	for _, summary := range summaries {
		fmt.Println(summary.Name)
		if modeIncludes(mode, ModeAll, ModeTopology, ModeRuntime) {
			for _, subagent := range summary.Subagents {
				fmt.Printf("  -> subagent/%s\n", subagent.Name)
				if modeIncludes(mode, ModeAll, ModeRuntime) {
					fmt.Printf("     runtime sandbox: %s\n", sandboxLabel(subagent.RuntimePolicy))
				}
			}
		}
		if modeIncludes(mode, ModeAll, ModeTools) {
			for _, name := range sortedKeys(summary.Tools) {
				fmt.Printf("  -> tools/%s@%s\n", name, summary.Tools[name])
			}
		}
		if modeIncludes(mode, ModeAll, ModeTopology) {
			for _, name := range sortedKeys(summary.Skills) {
				fmt.Printf("  -> skills/%s@%s\n", name, summary.Skills[name])
			}
		}
		if modeIncludes(mode, ModeAll, ModeChannels) {
			for _, channel := range summary.Channels {
				fmt.Printf("  -> channels/%s\n", channel)
			}
		}
		if modeIncludes(mode, ModeAll, ModeTopology) {
			for _, schedule := range summary.Schedules {
				fmt.Printf("  -> schedules/%s\n", schedule)
			}
		}
		if modeIncludes(mode, ModeAll, ModeApprovals) {
			for _, name := range sortedKeys(summary.Approvals) {
				fmt.Printf("  -> approvals/%s:%s\n", name, summary.Approvals[name])
			}
		}
		if modeIncludes(mode, ModeAll) {
			for _, eval := range summary.Evals {
				fmt.Printf("  -> evals/%s\n", eval)
			}
			for _, name := range sortedKeys(summary.Memory) {
				fmt.Printf("  -> memory/%s@%s\n", name, summary.Memory[name])
			}
		}
	}
}

func printMermaid(summaries []*inspect.Summary, mode Mode) {
	fmt.Println("graph TD")
	for _, summary := range summaries {
		agentNode := fmt.Sprintf("agent_%s", Node(summary.Name))
		fmt.Printf("  %s[\"agent:%s\"]\n", agentNode, summary.Name)
		if modeIncludes(mode, ModeAll, ModeTopology, ModeRuntime) {
			for _, subagent := range summary.Subagents {
				subagentNode := fmt.Sprintf("subagent_%s", Node(subagent.Name))
				fmt.Printf("  %s --> %s[\"subagent:%s\"]\n", agentNode, subagentNode, subagent.Name)
				if modeIncludes(mode, ModeAll, ModeRuntime) {
					sandbox := sandboxLabel(subagent.RuntimePolicy)
					fmt.Printf("  %s --> sandbox_%s[\"sandbox:%s\"]\n", subagentNode, Node(sandbox), sandbox)
				}
			}
		}
		if modeIncludes(mode, ModeAll, ModeTools) {
			for _, name := range sortedKeys(summary.Tools) {
				fmt.Printf("  %s --> tool_%s[\"tool:%s@%s\"]\n", agentNode, Node(name), name, summary.Tools[name])
			}
		}
		if modeIncludes(mode, ModeAll, ModeTopology) {
			for _, name := range sortedKeys(summary.Skills) {
				fmt.Printf("  %s --> skill_%s[\"skill:%s@%s\"]\n", agentNode, Node(name), name, summary.Skills[name])
			}
		}
		if modeIncludes(mode, ModeAll, ModeChannels) {
			for _, channel := range summary.Channels {
				fmt.Printf("  %s --> channel_%s[\"channel:%s\"]\n", agentNode, Node(channel), channel)
			}
		}
		if modeIncludes(mode, ModeAll, ModeTopology) {
			for _, schedule := range summary.Schedules {
				fmt.Printf("  %s --> schedule_%s[\"schedule:%s\"]\n", agentNode, Node(schedule), schedule)
			}
		}
		if modeIncludes(mode, ModeAll, ModeApprovals) {
			for _, name := range sortedKeys(summary.Approvals) {
				fmt.Printf("  %s --> approval_%s_%s[\"approval:%s:%s\"]\n", agentNode, Node(name), Node(summary.Approvals[name]), name, summary.Approvals[name])
			}
		}
		if modeIncludes(mode, ModeAll) {
			for _, eval := range summary.Evals {
				fmt.Printf("  %s --> eval_%s[\"eval:%s\"]\n", agentNode, Node(eval), eval)
			}
			for _, name := range sortedKeys(summary.Memory) {
				fmt.Printf("  %s --> memory_%s[\"memory:%s@%s\"]\n", agentNode, Node(name), name, summary.Memory[name])
			}
		}
	}
}

// Node sanitizes a value into a mermaid-safe node id.
func Node(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

// JSON builds the JSON graph payload.
func JSON(summaries []*inspect.Summary, mode Mode) string {
	var agents []any
	for _, summary := range summaries {
		switch mode {
		case ModeTopology:
			agents = append(agents, map[string]any{
				"name":      summary.Name,
				"subagents": summary.Subagents,
				"skills":    summary.Skills,
				"schedules": summary.Schedules,
			})
		case ModeTools:
			agents = append(agents, map[string]any{
				"name":  summary.Name,
				"tools": summary.Tools,
			})
		case ModeApprovals:
			agents = append(agents, map[string]any{
				"name":      summary.Name,
				"approvals": summary.Approvals,
			})
		case ModeRuntime:
			var subagents []map[string]any
			for _, subagent := range summary.Subagents {
				subagents = append(subagents, map[string]any{
					"name":           subagent.Name,
					"runtime_policy": subagent.RuntimePolicy,
				})
			}
			agents = append(agents, map[string]any{
				"name":           summary.Name,
				"runtime_policy": summary.RuntimePolicy,
				"subagents":      subagents,
			})
		case ModeChannels:
			agents = append(agents, map[string]any{
				"name":     summary.Name,
				"channels": summary.Channels,
			})
		default:
			agents = append(agents, summary)
		}
	}
	return fmt.Sprintf("{\n  \"mode\": %s,\n  \"agents\": %s\n}", quote(mode.String()), renderJSON(agents))
}

func sandboxLabel(policy *config.RuntimePolicy) string {
	if policy == nil || policy.Sandbox == "" {
		return "<unspecified>"
	}
	return policy.Sandbox
}

func modeIncludes(mode Mode, candidates ...Mode) bool {
	for _, candidate := range candidates {
		if mode == candidate {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func quote(value string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for _, char := range value {
		switch char {
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		default:
			builder.WriteRune(char)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}

func renderJSON(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "null"
	}
	return string(data)
}

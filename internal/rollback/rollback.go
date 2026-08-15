package rollback

import (
	"fmt"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/hotload"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/render"
)

// Operation is one file restore planned during rollback.
type Operation struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// Report is the rollback plan.
type Report struct {
	Agent            string      `json:"agent"`
	Target           string      `json:"target"`
	Env              string      `json:"env"`
	DryRun           bool        `json:"dry_run"`
	Deployment       string      `json:"deployment,omitempty"`
	DelegatedCommand []string    `json:"delegated_command"`
	Operations       []Operation `json:"operations"`
	Blocked          bool        `json:"blocked"`
	Reason           string      `json:"reason,omitempty"`
}

// Options carries the rollback command flags.
type Options struct {
	Agent       string
	To          string
	Deployment  string
	Component   string
	Env         string
	Manifest    string
	Catalog     string
	TemplateDir string
	DryRun      bool
	JSON        bool
}

// Plan computes the rollback plan.
func Plan(options Options, manifest *config.FleetManifest, catalog *config.CatalogManifest) (*Report, error) {
	var agent *config.AgentManifest
	for i := range manifest.Agents {
		if manifest.Agents[i].Name == options.Agent {
			agent = &manifest.Agents[i]
			break
		}
	}
	if agent == nil {
		return nil, fmt.Errorf("agent '%s' not found", options.Agent)
	}

	target := options.Deployment
	if target == "" {
		target = options.To
	}
	if target == "" {
		target = options.Component
	}
	if target == "" {
		return nil, fmt.Errorf("pass --to <version>, --deployment <id>, or --component kind:name@version")
	}

	var component *hotload.Ref
	var err error
	if options.Component != "" {
		component, err = hotload.Parse(options.Component)
		if err != nil {
			return nil, err
		}
	}
	if options.Deployment != "" && component != nil {
		return nil, fmt.Errorf("--deployment cannot be combined with --component")
	}

	blocked := component != nil && component.Kind == hotload.KindMemory
	var reason string
	if blocked {
		reason = "memory rollback requires an explicit migration plan"
	}

	var delegatedCommand []string
	if options.Deployment != "" {
		delegatedCommand = []string{"npm", "exec", "--", "vercel", "rollback", options.Deployment, "--yes"}
	}

	renderer, err := render.Load(options.TemplateDir)
	if err != nil {
		return nil, err
	}
	files, err := renderer.RenderAgent(agent, manifest, catalog)
	if err != nil {
		return nil, err
	}

	var operations []Operation
	for _, file := range files {
		if strings.HasSuffix(file.Path, "agent.manifest.yml") || strings.HasSuffix(file.Path, "versions.lock") {
			operations = append(operations, Operation{Path: file.Path, Action: "restore"})
		}
	}

	return &Report{
		Agent:            options.Agent,
		Target:           target,
		Env:              options.Env,
		DryRun:           options.DryRun,
		Deployment:       options.Deployment,
		DelegatedCommand: delegatedCommand,
		Operations:       operations,
		Blocked:          blocked,
		Reason:           reason,
	}, nil
}

// Print prints the rollback plan.
func (report *Report) Print() {
	fmt.Printf("Rollback plan for %s\n", report.Agent)
	for _, operation := range report.Operations {
		fmt.Printf("%s %s\n", operation.Action, operation.Path)
	}
	if report.Reason != "" {
		fmt.Println(report.Reason)
	}
	if len(report.DelegatedCommand) > 0 {
		fmt.Printf("Delegated command: %s\n", strings.Join(report.DelegatedCommand, " "))
		if report.DryRun {
			fmt.Println("Dry run only; Eve rollback was not invoked.")
		}
	}
}

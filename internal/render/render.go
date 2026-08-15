package render

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/versioning"
)

// RenderedFile is one generated output path and its content.
type RenderedFile struct {
	Path    string
	Content string
}

// TemplateNames are the templates loaded from the template directory.
var TemplateNames = []string{
	"instructions.md.tmpl",
	"agent.ts.tmpl",
	"tool.ts.tmpl",
	"skill.md.tmpl",
	"schedule.ts.tmpl",
	"approval.ts.tmpl",
	"eval.ts.tmpl",
	"memory.ts.tmpl",
	"fixture.json.tmpl",
	"agent.README.md.tmpl",
}

// Renderer renders agent outputs from templates.
type Renderer struct {
	templates map[string]*template.Template
}

// AgentContext is the template context for an agent.
type AgentContext struct {
	Name           string
	Version        string
	Owner          string
	Responsibility string
	Model          string
	Topology       config.AgentTopology
	RuntimePolicy  config.RuntimePolicy
	Subagents      []string
}

// Load loads all templates from the template directory.
func Load(templateDir string) (*Renderer, error) {
	funcs := template.FuncMap{
		"join": func(sep string, items []string) string {
			return strings.Join(items, sep)
		},
		"default": func(fallback, value string) string {
			if value == "" {
				return fallback
			}
			return value
		},
	}
	templates := make(map[string]*template.Template, len(TemplateNames))
	for _, name := range TemplateNames {
		source, err := os.ReadFile(filepath.Join(templateDir, name))
		if err != nil {
			return nil, fmt.Errorf("failed to read template '%s': %w", name, err)
		}
		tmpl, err := template.New(name).Funcs(funcs).Parse(string(source))
		if err != nil {
			return nil, fmt.Errorf("failed to load template '%s': %w", name, err)
		}
		templates[name] = tmpl
	}
	return &Renderer{templates: templates}, nil
}

func (r *Renderer) render(name string, data any) (string, error) {
	tmpl, ok := r.templates[name]
	if !ok {
		return "", fmt.Errorf("unknown template '%s'", name)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template '%s': %w", name, err)
	}
	return buf.String(), nil
}

// RenderAgent renders every generated file for one agent.
func (r *Renderer) RenderAgent(agent *config.AgentManifest, manifest *config.FleetManifest, catalog *config.CatalogManifest) ([]RenderedFile, error) {
	appRoot := filepath.Join("agents", agent.Name)
	outputRoot := filepath.Join(appRoot, "agent")
	railsRoot := filepath.Join(appRoot, ".eve-rails")

	model := agent.Model
	if model == "" {
		model = manifest.Defaults.Model
	}
	owner := agent.Owner
	if owner == "" {
		owner = manifest.Defaults.Owner
	}
	auth := agent.Auth
	if auth == "" {
		auth = manifest.Defaults.Auth
	}
	if auth == "" {
		auth = "platform-oauth"
	}
	version := agent.Version
	if version == "" {
		version = "1.0.0"
	}

	topology := config.AgentTopology{}
	if agent.Topology != nil {
		topology = *agent.Topology
	}
	runtimePolicy := config.RuntimePolicy{}
	if agent.RuntimePolicy != nil {
		runtimePolicy = *agent.RuntimePolicy
	}

	agentContext := AgentContext{
		Name:           agent.Name,
		Version:        version,
		Owner:          owner,
		Responsibility: agent.Responsibility,
		Model:          model,
		Topology:       topology,
		RuntimePolicy:  runtimePolicy,
		Subagents:      SubagentNames(agent),
	}

	instructions, err := r.render("instructions.md.tmpl", map[string]any{"agent": agentContext})
	if err != nil {
		return nil, err
	}
	agentTS, err := r.render("agent.ts.tmpl", map[string]any{"agent": agentContext})
	if err != nil {
		return nil, err
	}
	agentReadme, err := r.render("agent.README.md.tmpl", map[string]any{"agent": agentContext})
	if err != nil {
		return nil, err
	}

	files := []RenderedFile{
		{Path: filepath.Join("agents", agent.Name, "package.json"), Content: renderPackageJSON(agent.Name)},
		{Path: filepath.Join("agents", agent.Name, "tsconfig.json"), Content: renderTSConfigJSON()},
		{Path: filepath.Join("agents", agent.Name, "evals", "evals.config.ts"), Content: renderEvalsConfig()},
		{Path: filepath.Join(outputRoot, "instructions.md"), Content: WithGeneratedHeader(CommentStyleHash, agent.Name, "instructions.md.tmpl", instructions)},
		{Path: filepath.Join(outputRoot, "agent.ts"), Content: WithGeneratedHeader(CommentStyleSlash, agent.Name, "agent.ts.tmpl", agentTS)},
		{Path: filepath.Join(outputRoot, "README.md"), Content: WithGeneratedHeader(CommentStyleHash, agent.Name, "agent.README.md.tmpl", agentReadme)},
		{Path: filepath.Join(outputRoot, "channels", "eve.ts"), Content: renderEveChannel(auth)},
		{Path: filepath.Join(outputRoot, "agent.manifest.yml"), Content: renderAgentManifest(agent, manifest)},
	}

	for _, channel := range config.EffectiveStringList(manifest.Defaults.Channels, agent.Channels) {
		component, ok := catalog.Channels[channel]
		if !ok {
			continue
		}
		if component.Kind == "" || component.Kind == "eve" {
			continue
		}
		files = append(files, RenderedFile{
			Path:    filepath.Join(outputRoot, "channels", channel+".ts"),
			Content: renderPlatformChannel(channel, &component, auth),
		})
	}

	for _, component := range versioning.EffectiveComponents(manifest.Shared.Tools, agent.Tools) {
		catalogComponent, ok := catalog.Tools[component.Name]
		description := fmt.Sprintf("%s generated tool contract.", component.Name)
		sideEffects := "read"
		if ok {
			if catalogComponent.Description != "" {
				description = catalogComponent.Description
			} else if catalogComponent.SideEffects != nil {
				description = fmt.Sprintf("Generated %s %s tool contract.", component.Name, *catalogComponent.SideEffects)
			}
			if catalogComponent.SideEffects != nil {
				sideEffects = string(*catalogComponent.SideEffects)
			}
		}
		var requiredApprovals, requiredEnv, requiredConnectors, sandboxCompatibility, failureModes []string
		if ok {
			requiredApprovals = catalogComponent.RequiredApprovals
			requiredEnv = catalogComponent.RequiredEnv
			requiredConnectors = catalogComponent.RequiredConnectors
			sandboxCompatibility = catalogComponent.SandboxCompatibility
			failureModes = catalogComponent.FailureModes
		}
		toolContext := map[string]any{
			"NameLiteral":                 TSStringLiteral(component.Name),
			"VersionLiteral":              TSStringLiteral(component.Version),
			"DescriptionLiteral":          TSStringLiteral(fmt.Sprintf("%s Side effects: %s.", description, sideEffects)),
			"SideEffectsLiteral":          TSStringLiteral(sideEffects),
			"RequiredApprovalsLiteral":    TSStringArrayLiteral(requiredApprovals),
			"RequiredEnvLiteral":          TSStringArrayLiteral(requiredEnv),
			"RequiredConnectorsLiteral":   TSStringArrayLiteral(requiredConnectors),
			"SandboxCompatibilityLiteral": TSStringArrayLiteral(sandboxCompatibility),
			"FailureModesLiteral":         TSStringArrayLiteral(failureModes),
		}
		toolTS, err := r.render("tool.ts.tmpl", map[string]any{"tool": toolContext})
		if err != nil {
			return nil, err
		}
		files = append(files, RenderedFile{
			Path:    filepath.Join(outputRoot, "tools", component.Name+".ts"),
			Content: WithGeneratedHeader(CommentStyleSlash, agent.Name, "tool.ts.tmpl", toolTS),
		})
	}

	for _, component := range versioning.EffectiveComponents(manifest.Shared.Skills, agent.Skills) {
		skillContext := map[string]any{
			"Name":    component.Name,
			"Version": component.Version,
			"Trigger": fmt.Sprintf("the %s capability is relevant to the user's request", component.Name),
		}
		skillMD, err := r.render("skill.md.tmpl", map[string]any{"skill": skillContext})
		if err != nil {
			return nil, err
		}
		files = append(files, RenderedFile{
			Path:    filepath.Join(outputRoot, "skills", component.Name+".md"),
			Content: WithGeneratedHeader(CommentStyleHash, agent.Name, "skill.md.tmpl", skillMD),
		})
	}

	for i := range agent.Subagents {
		subagent := &agent.Subagents[i]
		role := SubagentRole(agent, subagent)
		subagentModel := subagent.Model
		if subagentModel == "" {
			subagentModel = model
		}
		files = append(files, RenderedFile{
			Path:    filepath.Join(outputRoot, "subagents", subagent.Name, "instructions.md"),
			Content: renderSubagentPlaceholder(agent.Name, subagent, role),
		})
		files = append(files, RenderedFile{
			Path:    filepath.Join(outputRoot, "subagents", subagent.Name, "agent.ts"),
			Content: renderSubagentAgentTS(agent.Name, subagent, subagentModel, role),
		})
	}

	for approval, policy := range agent.Approvals {
		approvalContext := map[string]any{
			"Component": approval,
			"Policy":    policy,
		}
		approvalTS, err := r.render("approval.ts.tmpl", map[string]any{"approval": approvalContext})
		if err != nil {
			return nil, err
		}
		files = append(files, RenderedFile{
			Path:    filepath.Join(railsRoot, "approvals", approval+".ts"),
			Content: WithGeneratedHeader(CommentStyleSlash, agent.Name, "approval.ts.tmpl", approvalTS),
		})
	}

	for _, eval := range versioning.EffectiveEvals(agent, manifest) {
		evalContext := map[string]any{"Name": eval}
		evalTS, err := r.render("eval.ts.tmpl", map[string]any{"eval": evalContext})
		if err != nil {
			return nil, err
		}
		files = append(files, RenderedFile{
			Path:    filepath.Join("agents", agent.Name, "evals", eval+".eval.ts"),
			Content: WithGeneratedHeader(CommentStyleSlash, agent.Name, "eval.ts.tmpl", evalTS),
		})
		files = append(files, RenderedFile{
			Path:    filepath.Join(railsRoot, "evals", eval+".contract.json"),
			Content: renderContractJSON("eval", eval, "Eve eval contract placeholder."),
		})
	}

	for _, component := range versioning.EffectiveComponents(manifest.Shared.Memory, agent.Memory) {
		retention := "session"
		if c, ok := catalog.Memory[component.Name]; ok && c.Retention != "" {
			retention = c.Retention
		}
		memoryContext := map[string]any{
			"Name":      component.Name,
			"Version":   component.Version,
			"Retention": retention,
		}
		memoryTS, err := r.render("memory.ts.tmpl", map[string]any{"memory": memoryContext})
		if err != nil {
			return nil, err
		}
		files = append(files, RenderedFile{
			Path:    filepath.Join(railsRoot, "memory", component.Name+".ts"),
			Content: WithGeneratedHeader(CommentStyleSlash, agent.Name, "memory.ts.tmpl", memoryTS),
		})
	}

	fixtureContext := map[string]any{
		"Name":        fmt.Sprintf("%s_smoke", agent.Name),
		"Kind":        "smoke",
		"Description": fmt.Sprintf("Generated smoke fixture for %s.", agent.Name),
	}
	fixtureJSON, err := r.render("fixture.json.tmpl", map[string]any{"fixture": fixtureContext})
	if err != nil {
		return nil, err
	}
	files = append(files, RenderedFile{
		Path:    filepath.Join(railsRoot, "fixtures", fmt.Sprintf("%s_smoke.json", agent.Name)),
		Content: strings.TrimRight(fixtureJSON, " \t\n") + "\n",
	})

	for _, schedule := range config.EffectiveSchedules(agent, manifest) {
		cron := ""
		if c, ok := catalog.Schedules[schedule]; ok {
			cron = c.Schedule
		}
		scheduleContext := map[string]any{
			"Name":     schedule,
			"Schedule": cron,
		}
		scheduleTS, err := r.render("schedule.ts.tmpl", map[string]any{"schedule": scheduleContext})
		if err != nil {
			return nil, err
		}
		files = append(files, RenderedFile{
			Path:    filepath.Join(outputRoot, "schedules", schedule+".ts"),
			Content: WithGeneratedHeader(CommentStyleSlash, agent.Name, "schedule.ts.tmpl", scheduleTS),
		})
	}

	files = append(files, RenderedFile{
		Path:    filepath.Join(outputRoot, "versions.lock"),
		Content: renderVersionsLock(agent, manifest, catalog, files),
	})

	return files, nil
}

// SubagentNames returns the names of an agent's subagents.
func SubagentNames(agent *config.AgentManifest) []string {
	var names []string
	for i := range agent.Subagents {
		names = append(names, agent.Subagents[i].Name)
	}
	return names
}

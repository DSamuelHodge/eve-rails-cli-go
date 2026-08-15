package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/render"
)

// Status is a doctor check status.
type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check is one named doctor check.
type Check struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message"`
}

// Report is the full set of doctor checks.
type Report struct {
	Checks []Check
}

// Options controls which doctor checks run.
type Options struct {
	All          bool
	Updates      bool
	Templates    bool
	Fix          bool
	DryRun       bool
	Env          string
	Connections  bool
	Budgets      bool
	JSON         bool
	Manifest     string
	Catalog      string
	TemplateDir  string
	Environments string
}

// HasFailures reports whether any check failed.
func (report *Report) HasFailures() bool {
	for _, check := range report.Checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}

// RunDoctor runs the doctor suite against a manifest and catalog.
func RunDoctor(manifest *config.FleetManifest, catalog *config.CatalogManifest, options Options) (*Report, error) {
	var checks []Check
	validation := ValidateManifest(manifest, catalog)

	pushCheck(&checks, "manifest-schema", len(validation.Errors) == 0,
		"manifest schema and required fields are valid",
		fmt.Sprintf("%d manifest validation error(s)", len(validation.Errors)))
	for _, err := range validation.Errors {
		checks = append(checks, Check{Name: "manifest-error", Status: StatusFail, Message: err})
	}
	for _, warning := range validation.Warnings {
		checks = append(checks, Check{Name: "manifest-warning", Status: StatusWarn, Message: warning})
	}

	pushCheck(&checks, "catalog-references", allNotContaining(validation.Errors, "references missing"),
		"all manifest references resolve against catalog",
		"one or more manifest references are missing from catalog")

	pushCheck(&checks, "approval-coverage", allNotContaining(validation.Errors, "risky tool"),
		"risky tools have approval coverage",
		"one or more risky tools lack approval coverage")

	pushCheck(&checks, "memory-retention", allNotContaining(validation.Errors, "must declare retention"),
		"memory schemas declare retention",
		"one or more memory schemas are missing retention")

	pushCheck(&checks, "runtime-policy", allNotContaining(validation.Errors, "runtime policy"),
		"runtime policies are compatible with tools, sandboxes, and approvals",
		"one or more runtime policies are incompatible with tools, sandboxes, or approvals")

	if options.Templates {
		if err := addTemplateChecks(&checks, manifest, catalog, options.TemplateDir); err != nil {
			return nil, err
		}
	}
	if options.Updates {
		if err := addUpdateChecks(&checks, manifest, catalog, options.TemplateDir); err != nil {
			return nil, err
		}
	}
	if options.Budgets {
		addBudgetChecks(&checks, manifest)
	}
	addScheduleSafetyChecks(&checks, manifest, catalog)
	addChannelConfigChecks(&checks, manifest, catalog, options.Env != "")
	if options.Env != "" || options.Connections {
		if err := addEnvironmentChecks(&checks, options); err != nil {
			return nil, err
		}
	}
	if options.Fix {
		if err := addFixPlanChecks(&checks, manifest, catalog, options.TemplateDir); err != nil {
			return nil, err
		}
	}

	return &Report{Checks: checks}, nil
}

func addBudgetChecks(checks *[]Check, manifest *config.FleetManifest) {
	defaultsInvalid := false
	if manifest.Defaults.CostBudget != nil && *manifest.Defaults.CostBudget <= 0 {
		defaultsInvalid = true
	}
	if manifest.Defaults.TokenBudget != nil && *manifest.Defaults.TokenBudget == 0 {
		defaultsInvalid = true
	}
	var invalid []string
	for i := range manifest.Agents {
		agent := &manifest.Agents[i]
		if (agent.CostBudget != nil && *agent.CostBudget <= 0) ||
			(agent.TokenBudget != nil && *agent.TokenBudget == 0) {
			invalid = append(invalid, agent.Name)
		}
	}
	passed := len(invalid) == 0 && !defaultsInvalid
	var message string
	if defaultsInvalid {
		message = fmt.Sprintf("invalid default budget metadata")
		if len(invalid) > 0 {
			message = fmt.Sprintf("invalid default budget metadata and invalid agent budgets: %s", strings.Join(invalid, ","))
		}
	} else {
		message = fmt.Sprintf("invalid budget metadata for agents: %s", strings.Join(invalid, ","))
	}
	pushCheck(checks, "budgets-valid", passed,
		"agent budgets are valid where configured", message)
}

func addScheduleSafetyChecks(checks *[]Check, manifest *config.FleetManifest, catalog *config.CatalogManifest) {
	var missing []string
	for i := range manifest.Agents {
		agent := &manifest.Agents[i]
		for _, schedule := range config.EffectiveSchedules(agent, manifest) {
			metadata, ok := catalog.Schedules[schedule]
			hasOwner := agent.Owner != "" || manifest.Defaults.Owner != "" || (ok && metadata.Owner != "")
			hasAuth := agent.Auth != "" || manifest.Defaults.Auth != "" || (ok && metadata.Auth != "")
			hasVisibility := agent.Visibility != "" || manifest.Defaults.Visibility != "" || (ok && metadata.Visibility != "")
			if !(hasOwner && hasAuth && hasVisibility) {
				missing = append(missing, fmt.Sprintf("%s:%s", agent.Name, schedule))
			}
		}
	}
	pushCheck(checks, "schedule-safety-metadata", len(missing) == 0,
		"schedules have owner/auth/visibility metadata",
		fmt.Sprintf("schedules missing owner/auth/visibility metadata: %s", strings.Join(missing, ",")))
}

func addChannelConfigChecks(checks *[]Check, manifest *config.FleetManifest, catalog *config.CatalogManifest, checkEnv bool) {
	var missingConfig, missingEnv []string
	for i := range manifest.Agents {
		agent := &manifest.Agents[i]
		for _, channel := range config.EffectiveStringList(manifest.Defaults.Channels, agent.Channels) {
			component, ok := catalog.Channels[channel]
			if !ok {
				continue
			}
			kind := component.Kind
			if kind == "" {
				kind = channel
			}
			switch kind {
			case "twilio":
				if strings.TrimSpace(component.AllowFrom) == "" {
					missingConfig = append(missingConfig, fmt.Sprintf("%s:%s:allow_from", agent.Name, channel))
				}
				if checkEnv {
					pushMissingEnv(&missingEnv, component.MessagingFrom, "TWILIO_FROM_NUMBER")
					pushEnvIfMissing(&missingEnv, "TWILIO_ACCOUNT_SID")
					pushEnvIfMissing(&missingEnv, "TWILIO_AUTH_TOKEN")
				}
			case "slack":
				if strings.TrimSpace(component.ConnectUID) == "" {
					missingConfig = append(missingConfig, fmt.Sprintf("%s:%s:connect_uid", agent.Name, channel))
				}
			case "linear":
				if strings.TrimSpace(component.ConnectUID) == "" {
					missingConfig = append(missingConfig, fmt.Sprintf("%s:%s:connect_uid", agent.Name, channel))
				}
				if strings.TrimSpace(component.ConnectUID) == "" && checkEnv {
					pushEnvIfMissing(&missingEnv, "LINEAR_AGENT_ACCESS_TOKEN")
					pushEnvIfMissing(&missingEnv, "LINEAR_WEBHOOK_SECRET")
				}
			case "github":
				if strings.TrimSpace(component.ConnectUID) == "" {
					missingConfig = append(missingConfig, fmt.Sprintf("%s:%s:connect_uid", agent.Name, channel))
				}
				if strings.TrimSpace(component.ConnectUID) == "" && checkEnv {
					pushEnvIfMissing(&missingEnv, "GITHUB_APP_ID")
					pushEnvIfMissing(&missingEnv, "GITHUB_APP_PRIVATE_KEY")
					pushEnvIfMissing(&missingEnv, "GITHUB_WEBHOOK_SECRET")
				}
			case "discord":
				if checkEnv {
					pushEnvIfMissing(&missingEnv, "DISCORD_PUBLIC_KEY")
					pushEnvIfMissing(&missingEnv, "DISCORD_APPLICATION_ID")
					pushEnvIfMissing(&missingEnv, "DISCORD_BOT_TOKEN")
				}
			case "telegram":
				if strings.TrimSpace(component.BotUsername) == "" {
					missingConfig = append(missingConfig, fmt.Sprintf("%s:%s:bot_username", agent.Name, channel))
				}
				if checkEnv {
					pushEnvIfMissing(&missingEnv, "TELEGRAM_BOT_TOKEN")
					pushEnvIfMissing(&missingEnv, "TELEGRAM_WEBHOOK_SECRET_TOKEN")
				}
			case "teams":
				if checkEnv {
					pushEnvIfMissing(&missingEnv, "MICROSOFT_APP_ID")
					pushEnvIfMissing(&missingEnv, "MICROSOFT_APP_PASSWORD")
				}
			}
		}
	}
	sortAndDedupe(&missingEnv)

	pushCheck(checks, "channel-config", len(missingConfig) == 0,
		"channel metadata is complete",
		fmt.Sprintf("channel metadata is missing: %s", strings.Join(missingConfig, ",")))
	if checkEnv {
		pushCheck(checks, "channel-env", len(missingEnv) == 0,
			"channel runtime env vars are present",
			fmt.Sprintf("channel runtime env vars are missing: %s", strings.Join(missingEnv, ",")))
	}
}

func pushMissingEnv(missing *[]string, configured, fallback string) {
	if strings.HasPrefix(configured, "env:") {
		pushEnvIfMissing(missing, strings.TrimSpace(strings.TrimPrefix(configured, "env:")))
	} else {
		pushEnvIfMissing(missing, fallback)
	}
}

func pushEnvIfMissing(missing *[]string, envName string) {
	if envName == "" {
		return
	}
	if _, ok := os.LookupEnv(envName); !ok {
		*missing = append(*missing, envName)
	}
}

func addEnvironmentChecks(checks *[]Check, options Options) error {
	envName := options.Env
	if envName == "" {
		envName = "development"
	}
	environments, err := config.LoadEnvironments(options.Environments)
	if err != nil {
		return err
	}
	policy, ok := environments.Environments[envName]
	if !ok {
		pushCheck(checks, "environment-defined", false,
			"environment exists",
			fmt.Sprintf("environment '%s' is not defined", envName))
		return nil
	}
	pushCheck(checks, "environment-defined", true,
		"environment exists", "environment is missing")
	var missingEnv []string
	for _, key := range append(append([]string{}, policy.RequiredEnv...), policy.RequiredSecrets...) {
		if _, ok := os.LookupEnv(key); !ok {
			missingEnv = append(missingEnv, key)
		}
	}
	pushCheck(checks, "environment-variables", len(missingEnv) == 0,
		"required env vars and secrets are present",
		fmt.Sprintf("missing env vars/secrets: %s", strings.Join(missingEnv, ",")))
	if options.Connections {
		pushCheck(checks, "connections-configured", len(policy.RequiredConnections) == 0,
			"required connections are configured or none are required",
			fmt.Sprintf("manual connection verification required: %s", strings.Join(policy.RequiredConnections, ",")))
	}
	if envName == "production" {
		pushCheck(checks, "observability", policy.Observability.Bool(),
			"production observability is enabled",
			"production environment must enable observability")
	}
	return nil
}

func addFixPlanChecks(checks *[]Check, manifest *config.FleetManifest, catalog *config.CatalogManifest, templateDir string) error {
	plan, err := render.PlanBatch(manifest, catalog, templateDir)
	if err != nil {
		return err
	}
	repairs := 0
	for _, operation := range plan.Operations {
		if operation.Action != render.BatchSkip {
			repairs++
		}
	}
	pushCheck(checks, "fix-plan", true,
		fmt.Sprintf("doctor --fix would apply %d safe generated-file repair(s)", repairs),
		"doctor --fix plan failed")
	return nil
}

func addTemplateChecks(checks *[]Check, manifest *config.FleetManifest, catalog *config.CatalogManifest, templateDir string) error {
	plan, err := render.PlanBatch(manifest, catalog, templateDir)
	if err != nil {
		return err
	}
	stale := 0
	for _, operation := range plan.Operations {
		if operation.Action != render.BatchSkip {
			stale++
		}
	}
	pushCheck(checks, "templates-render", true,
		"templates render without undefined variables",
		"templates failed to render")
	pushCheck(checks, "generated-output-fresh", stale == 0,
		"generated files are fresh",
		fmt.Sprintf("%d generated file(s) are stale or missing", stale))
	pushCheck(checks, "generated-metadata", GeneratedMetadataPresent(plan),
		"generated files include metadata headers",
		"one or more generated files are missing metadata headers")
	addRenderedApprovalGateChecks(checks, manifest, catalog, plan)
	return nil
}

// addRenderedApprovalGateChecks asserts the approval gate rendered into each
// generated tool file matches the manifest's approval-policy assignment, so a
// manifest that passes approval-coverage cannot silently ship an ungated tool.
func addRenderedApprovalGateChecks(checks *[]Check, manifest *config.FleetManifest, catalog *config.CatalogManifest, plan *render.BatchPlan) {
	contentByPath := make(map[string]string, len(plan.Operations))
	for _, operation := range plan.Operations {
		contentByPath[operation.Path] = operation.Content
	}
	var mismatches []string
	for i := range manifest.Agents {
		agent := &manifest.Agents[i]
		for tool := range agent.Tools {
			expected := config.ApprovalGateFor(tool, agent.Approvals, catalog)
			path := filepath.Join("agents", agent.Name, "agent", "tools", tool+".ts")
			if !gateMatches(contentByPath[path], expected) {
				mismatches = append(mismatches, fmt.Sprintf("%s expects %s", path, expected))
			}
		}
		for j := range agent.Subagents {
			subagent := &agent.Subagents[j]
			for _, tool := range subagent.Tools {
				expected := config.ApprovalGateFor(tool, subagent.Approvals, catalog)
				path := filepath.Join("agents", agent.Name, "agent", "subagents", subagent.Name, "tools", tool+".ts")
				if !gateMatches(contentByPath[path], expected) {
					mismatches = append(mismatches, fmt.Sprintf("%s expects %s", path, expected))
				}
			}
		}
	}
	pushCheck(checks, "rendered-approval-gates", len(mismatches) == 0,
		"rendered tool approval gates match manifest approval-policy assignment",
		fmt.Sprintf("approval gate mismatch(es): %s", strings.Join(mismatches, "; ")))
}

// gateMatches reports whether rendered content carries the expected Eve
// approval helper call.
func gateMatches(content, expected string) bool {
	if expected == "" {
		return true
	}
	helper := strings.TrimSuffix(expected, "()")
	return strings.Contains(content, "approval: "+expected) ||
		strings.Contains(content, "approval: "+helper)
}

func addUpdateChecks(checks *[]Check, manifest *config.FleetManifest, catalog *config.CatalogManifest, templateDir string) error {
	plan, err := render.PlanBatch(manifest, catalog, templateDir)
	if err != nil {
		return err
	}
	var lockfiles []render.BatchOperation
	for _, operation := range plan.Operations {
		if strings.HasSuffix(operation.Path, "versions.lock") {
			lockfiles = append(lockfiles, operation)
		}
	}
	staleLockfiles := 0
	for _, operation := range lockfiles {
		if operation.Action != render.BatchSkip {
			staleLockfiles++
		}
	}
	allPresent := len(lockfiles) > 0
	for _, operation := range lockfiles {
		if _, err := os.Stat(operation.Path); os.IsNotExist(err) && operation.Action != render.BatchCreate {
			allPresent = false
		}
	}
	pushCheck(checks, "lockfiles-present", allPresent,
		"agent lockfiles are present",
		"one or more agent lockfiles are missing")
	pushCheck(checks, "lockfiles-current", staleLockfiles == 0,
		"agent lockfiles match rendered component content",
		fmt.Sprintf("%d lockfile(s) are stale", staleLockfiles))
	return nil
}

// GeneratedMetadataPresent reports whether all planned outputs carry metadata.
func GeneratedMetadataPresent(plan *render.BatchPlan) bool {
	for _, operation := range plan.Operations {
		content := operation.Content
		if strings.HasPrefix(content, "# Generated by eve-rails. Do not edit generated regions.") ||
			strings.HasPrefix(content, "// Generated by eve-rails. Do not edit generated regions.") ||
			strings.Contains(content, `"x-eve-rails": "generated"`) ||
			strings.Contains(content, `"kind": "`) {
			continue
		}
		return false
	}
	return true
}

func pushCheck(checks *[]Check, name string, passed bool, passMessage, failMessage string) {
	status := StatusFail
	if passed {
		status = StatusPass
	}
	message := failMessage
	if passed {
		message = passMessage
	}
	*checks = append(*checks, Check{Name: name, Status: status, Message: message})
}

// PrintReport prints the doctor report.
func PrintReport(report *Report) {
	fmt.Println()
	for _, check := range report.Checks {
		fmt.Printf("[%s] %s - %s\n", StatusLabel(check.Status), check.Name, check.Message)
	}
}

// StatusLabel returns the lowercase status label.
func StatusLabel(status Status) string {
	return string(status)
}

func allNotContaining(errors []string, needle string) bool {
	for _, err := range errors {
		if strings.Contains(err, needle) {
			return false
		}
	}
	return true
}

func sortAndDedupe(values *[]string) {
	set := make(map[string]struct{}, len(*values))
	var unique []string
	for _, value := range *values {
		if _, ok := set[value]; ok {
			continue
		}
		set[value] = struct{}{}
		unique = append(unique, value)
	}
	for i := 1; i < len(unique); i++ {
		for j := i; j > 0 && unique[j] < unique[j-1]; j-- {
			unique[j], unique[j-1] = unique[j-1], unique[j]
		}
	}
	*values = unique
}

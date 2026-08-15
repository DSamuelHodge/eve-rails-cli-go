package deploy

import (
	"fmt"
	"strings"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/doctor"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/versioning"
)

// Gate is one deploy preflight gate.
type Gate struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// Report is the deploy preflight result.
type Report struct {
	Env              string   `json:"env"`
	Agent            string   `json:"agent,omitempty"`
	DryRun           bool     `json:"dry_run"`
	Promote          bool     `json:"promote"`
	RollbackTo       string   `json:"rollback_to,omitempty"`
	DelegatedCommand []string `json:"delegated_command"`
	Gates            []Gate   `json:"gates"`
}

// Options carries the deploy command flags.
type Options struct {
	Agent            string
	Env              string
	RequireEvals     bool
	RequireDoctor    bool
	RequireApprovals bool
	Canary           uint64
	Promote          bool
	RollbackTo       string
	DryRun           bool
	JSON             bool
	Manifest         string
	Catalog          string
	TemplateDir      string
}

// Preflight computes the deploy preflight report.
func Preflight(options Options, manifest *config.FleetManifest, catalog *config.CatalogManifest) (*Report, error) {
	if options.Canary != 0 {
		return nil, fmt.Errorf("canary deploy is not supported by the current Eve deploy CLI")
	}
	if options.Promote {
		return nil, fmt.Errorf("promote is not supported by the current Eve deploy CLI")
	}

	var gates []Gate

	doctorOptions := doctor.Options{
		All:          options.Agent == "",
		Updates:      true,
		Templates:    true,
		Budgets:      true,
		Env:          options.Env,
		Manifest:     options.Manifest,
		Catalog:      options.Catalog,
		TemplateDir:  options.TemplateDir,
		Environments: "manifests/environments.yml",
	}
	report, err := doctor.RunDoctor(manifest, catalog, doctorOptions)
	if err != nil {
		return nil, err
	}
	doctorPassed := !report.HasFailures()
	gates = append(gates, Gate{
		Name:    "doctor",
		Passed:  !options.RequireDoctor || doctorPassed,
		Message: booleanMessage(doctorPassed, "doctor checks pass", "doctor checks failed"),
	})

	evalsPassed := evalGatePasses(manifest, options.Agent)
	gates = append(gates, Gate{
		Name:    "evals",
		Passed:  !options.RequireEvals || evalsPassed,
		Message: booleanMessage(evalsPassed, "required eval references are present", "one or more deploy targets have no eval references"),
	})

	validation := doctor.ValidateManifest(manifest, catalog)
	approvalsPassed := true
	for _, err := range validation.Errors {
		if strings.Contains(err, "risky tool") {
			approvalsPassed = false
			break
		}
	}
	gates = append(gates, Gate{
		Name:    "approval-coverage",
		Passed:  !options.RequireApprovals || approvalsPassed,
		Message: "risky tool approval coverage checked",
	})

	gates = append(gates, Gate{
		Name:    "rollback-target",
		Passed:  true,
		Message: "rollback metadata will use generated manifest and versions.lock",
	})

	var delegatedCommand []string
	if options.RollbackTo != "" {
		delegatedCommand = []string{"npm", "exec", "--", "vercel", "rollback", options.RollbackTo, "--yes"}
	} else {
		delegatedCommand = []string{"npm", "exec", "--", "eve", "deploy"}
	}

	return &Report{
		Env:              options.Env,
		Agent:            options.Agent,
		DryRun:           options.DryRun,
		Promote:          options.Promote,
		RollbackTo:       options.RollbackTo,
		DelegatedCommand: delegatedCommand,
		Gates:            gates,
	}, nil
}

func evalGatePasses(manifest *config.FleetManifest, agentFilter string) bool {
	for i := range manifest.Agents {
		agent := &manifest.Agents[i]
		if agentFilter != "" && agentFilter != agent.Name {
			continue
		}
		if len(versioning.EffectiveEvals(agent, manifest)) == 0 {
			return false
		}
	}
	return true
}

// Passed reports whether all gates passed.
func (report *Report) Passed() bool {
	for _, gate := range report.Gates {
		if !gate.Passed {
			return false
		}
	}
	return true
}

func booleanMessage(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

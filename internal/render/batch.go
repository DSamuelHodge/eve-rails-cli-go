package render

import (
	"fmt"
	"os"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
)

// BatchAction is how a generated file compares to the on-disk file.
type BatchAction string

const (
	BatchCreate BatchAction = "create"
	BatchUpdate BatchAction = "update"
	BatchSkip   BatchAction = "skip"
)

// BatchOperation is one generated file and its planned action.
type BatchOperation struct {
	Path    string
	Action  BatchAction
	Content string
}

// BatchPlan is the full set of planned operations.
type BatchPlan struct {
	Operations []BatchOperation
}

// BatchSummary counts planned operations by action.
type BatchSummary struct {
	Create int `json:"create"`
	Update int `json:"update"`
	Skip   int `json:"skip"`
	Total  int `json:"total"`
}

// BatchOperationReport is a JSON-safe operation summary.
type BatchOperationReport struct {
	Path   string      `json:"path"`
	Action BatchAction `json:"action"`
}

// Summary counts planned operations by action.
func (plan *BatchPlan) Summary() BatchSummary {
	summary := BatchSummary{Total: len(plan.Operations)}
	for _, operation := range plan.Operations {
		switch operation.Action {
		case BatchCreate:
			summary.Create++
		case BatchUpdate:
			summary.Update++
		case BatchSkip:
			summary.Skip++
		}
	}
	return summary
}

// Report returns JSON-safe operation summaries.
func (plan *BatchPlan) Report() []BatchOperationReport {
	var reports []BatchOperationReport
	for _, operation := range plan.Operations {
		reports = append(reports, BatchOperationReport{Path: operation.Path, Action: operation.Action})
	}
	return reports
}

// PlanBatch computes the planned operations for every agent.
func PlanBatch(manifest *config.FleetManifest, catalog *config.CatalogManifest, templateDir string) (*BatchPlan, error) {
	renderer, err := Load(templateDir)
	if err != nil {
		return nil, err
	}
	var operations []BatchOperation
	for i := range manifest.Agents {
		rendered, err := renderer.RenderAgent(&manifest.Agents[i], manifest, catalog)
		if err != nil {
			return nil, err
		}
		for _, file := range rendered {
			action, err := BatchActionFor(file.Path, file.Content)
			if err != nil {
				return nil, err
			}
			operations = append(operations, BatchOperation{Path: file.Path, Action: action, Content: file.Content})
		}
	}
	return &BatchPlan{Operations: operations}, nil
}

// BatchActionFor compares on-disk content to expected content.
func BatchActionFor(path, expected string) (BatchAction, error) {
	actual, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return BatchCreate, nil
		}
		return "", fmt.Errorf("failed to read '%s': %w", path, err)
	}
	if string(actual) == expected {
		return BatchSkip, nil
	}
	return BatchUpdate, nil
}

// PrintBatchSummary prints the batch counts.
func PrintBatchSummary(plan *BatchPlan) {
	summary := plan.Summary()
	fmt.Printf("Batch: %d create, %d update, %d skip, %d total files\n",
		summary.Create, summary.Update, summary.Skip, summary.Total)
}

package templates

import (
	"fmt"
	"os"
	"path/filepath"
)

type Change struct {
	Path    string
	Action  string
	Content string
}

type StaleReport struct {
	Missing   []string
	Differing []string
}

func Compare(dir string) (StaleReport, error) {
	var report StaleReport
	for _, name := range AgentNames() {
		embedded, err := AgentContent(name)
		if err != nil {
			return report, err
		}
		onDisk, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			if os.IsNotExist(err) {
				report.Missing = append(report.Missing, name)
				continue
			}
			return report, err
		}
		if string(onDisk) != string(embedded) {
			report.Differing = append(report.Differing, name)
		}
	}
	return report, nil
}

func Upgrade(dir string) ([]Change, error) {
	var changes []Change
	for _, name := range AgentNames() {
		content, err := AgentContent(name)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(dir, name)
		action := "create"
		if onDisk, err := os.ReadFile(path); err == nil && string(onDisk) != string(content) {
			action = "update"
		}
		changes = append(changes, Change{Path: path, Action: action, Content: string(content)})
	}
	return changes, nil
}

func Verify(dir string) error {
	report, err := Compare(dir)
	if err != nil {
		return err
	}
	if len(report.Missing) > 0 || len(report.Differing) > 0 {
		return fmt.Errorf("project templates are older than this CLI; run `eve-rails templates upgrade` (missing: %v, differing: %v)",
			report.Missing, report.Differing)
	}
	return nil
}

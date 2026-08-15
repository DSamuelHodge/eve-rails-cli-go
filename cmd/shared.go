package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/inspect"
	"github.com/DSamuelHodge/eve-rails-cli-go/internal/versioning"
)

var (
	template   string
	model      string
	owner      string
	yes        bool
	dryRun     bool
	force      bool
	jsonOutput bool

	manifest     string
	catalog      string
	templates    string
	templateDir  string
	environments string

	all              bool
	updates          bool
	templatesFlag    bool
	fix              bool
	connections      bool
	budgets          bool
	env              string
	check            bool
	agent            string
	agentOpt         string
	format           string
	mode             string
	current          string
	component        string
	fleet            string
	requireEvals     bool
	requireDoctor    bool
	requireApprovals bool
	canary           uint8
	promote          bool
	rollbackTo       string
	apply            bool
	plan             bool
	patch            bool
	minor            bool
	major            bool
	to               string
	deployment       string
)

func printJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func parentDir(path string) string {
	dir := filepath.Dir(path)
	if dir == "." {
		return ""
	}
	return dir
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func collectVersionReports(manifestData *config.FleetManifest, catalogData *config.CatalogManifest, agentFilter string) []versioning.VersionReport {
	return versioning.CollectVersionReports(manifestData, catalogData, agentFilter)
}

func printVersionReports(reports []versioning.VersionReport) {
	versioning.PrintVersionReports(reports)
}

func readFileMatches(path, expected string) (bool, error) {
	return inspect.FileMatches(path, expected)
}

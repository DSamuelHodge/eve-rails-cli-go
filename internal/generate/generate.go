package generate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/config"
)

// Kind is the type of component being generated.
type Kind string

const (
	KindAgent     Kind = "agent"
	KindTool      Kind = "tool"
	KindSkill     Kind = "skill"
	KindSubagent  Kind = "subagent"
	KindChannel   Kind = "channel"
	KindSchedule  Kind = "schedule"
	KindApproval  Kind = "approval"
	KindEval      Kind = "eval"
	KindMemory    Kind = "memory"
	KindMigration Kind = "migration"
)

// String returns the kebab-case name of the generator kind.
func (kind Kind) String() string {
	return string(kind)
}

// Action is the kind of file change.
type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
)

// Change is one planned file write.
type Change struct {
	Path    string
	Action  Action
	Content string
}

// Options carries the generate command flags.
type Options struct {
	Name          string
	Version       string
	Owner         string
	Description   string
	Model         string
	SideEffects   string
	Retention     string
	WithTools     []string
	WithSkills    []string
	WithSubagents []string
	WithChannels  []string
	WithSchedules []string
	WithEvals     []string
	WithMemory    []string
	Approval      string
	Schedule      string
	Kind          string
	AllowFrom     string
	MessagingFrom string
	ConnectUID    string
	BotUsername   string
	BotName       string
	Risk          string
	Auth          string
	Visibility    string
	CostBudget    *float64
	TokenBudget   *uint64
	Timeout       string
	DryRun        bool
	Force         bool
	JSON          bool
	Manifest      string
	Catalog       string
}

// Plan returns the planned changes for a named generator.
func Plan(kind Kind, options Options) ([]Change, error) {
	if err := config.EnsureSlug(options.Name); err != nil {
		return nil, err
	}
	switch kind {
	case KindAgent:
		return planAgent(options)
	case KindTool, KindSkill, KindChannel, KindSchedule, KindApproval, KindEval, KindMemory:
		return planCatalogComponent(kind, options)
	case KindSubagent:
		return planSubagent(options)
	case KindMigration:
		return planMigration(options)
	}
	return nil, fmt.Errorf("unsupported generator kind '%s'", kind)
}

// Generate writes the planned changes and reports them.
func Generate(kind Kind, options Options) ([]Change, error) {
	changes, err := Plan(kind, options)
	if err != nil {
		return nil, err
	}
	if options.DryRun {
		printPlan(kind, options, changes)
		return changes, nil
	}
	for _, change := range changes {
		if err := writeChange(change); err != nil {
			return nil, err
		}
	}
	return changes, nil
}

func writeChange(change Change) error {
	if parent := filepath.Dir(change.Path); parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("failed to create '%s': %w", parent, err)
		}
	}
	if err := os.WriteFile(change.Path, []byte(change.Content), 0o644); err != nil {
		return fmt.Errorf("failed to write '%s': %w", change.Path, err)
	}
	fmt.Printf("%s %s\n", change.Action, change.Path)
	return nil
}

func planAgent(options Options) ([]Change, error) {
	source, err := config.ReadOrDefault(options.Manifest, config.DefaultManifestYAML())
	if err != nil {
		return nil, err
	}
	if strings.Contains(source, fmt.Sprintf("- name: %s", options.Name)) && !options.Force {
		return nil, fmt.Errorf("agent '%s' already exists in '%s'; pass --force to overwrite generated manifest text", options.Name, options.Manifest)
	}

	var entry strings.Builder
	fmt.Fprintf(&entry, "\n  - name: %s\n    version: %s\n    owner: %s\n    responsibility: %s\n    model: %s\n",
		options.Name, defaultVersion(options.Version), defaultString(options.Owner, "agent-platform"),
		defaultString(options.Description, "Describe this agent's bounded responsibility."),
		defaultString(options.Model, "openai/gpt-5.5"))
	appendGeneratedComponentMap(&entry, "tools", options.WithTools)
	appendGeneratedComponentMap(&entry, "skills", options.WithSkills)
	appendGeneratedStringList(&entry, "subagents", options.WithSubagents)
	appendGeneratedStringList(&entry, "channels", options.WithChannels)
	appendGeneratedStringList(&entry, "schedules", options.WithSchedules)
	appendGeneratedApprovals(&entry, options)
	appendGeneratedStringList(&entry, "evals", options.WithEvals)
	appendGeneratedComponentMap(&entry, "memory", options.WithMemory)
	appendOptionalString(&entry, "risk", options.Risk)
	appendOptionalString(&entry, "auth", options.Auth)
	appendOptionalString(&entry, "visibility", options.Visibility)
	if options.CostBudget != nil {
		fmt.Fprintf(&entry, "    cost_budget: %v\n", *options.CostBudget)
	}
	if options.TokenBudget != nil {
		fmt.Fprintf(&entry, "    token_budget: %d\n", *options.TokenBudget)
	}
	appendOptionalString(&entry, "timeout", options.Timeout)

	content := appendYAMLListEntry(source, "agents:", entry.String())
	return []Change{{
		Path:    options.Manifest,
		Action:  changeAction(options.Manifest),
		Content: content,
	}}, nil
}

func planCatalogComponent(kind Kind, options Options) ([]Change, error) {
	section := catalogSection(kind)
	source, err := config.ReadOrDefault(options.Catalog, config.DefaultCatalogYAML())
	if err != nil {
		return nil, err
	}
	if catalogEntryExists(source, section, options.Name) && !options.Force {
		return nil, fmt.Errorf("%s '%s' already exists in '%s'; pass --force to overwrite generated catalog text", kind, options.Name, options.Catalog)
	}

	entry := catalogEntry(kind, options)
	catalogContent := upsertCatalogEntry(source, section, options.Name, entry)
	changes := []Change{{
		Path:    options.Catalog,
		Action:  changeAction(options.Catalog),
		Content: catalogContent,
	}}

	stub, err := componentStub(kind, options)
	if err != nil {
		return nil, err
	}
	if stub != nil {
		changes = append(changes, *stub)
	}
	return changes, nil
}

func planSubagent(options Options) ([]Change, error) {
	path := filepath.Join("catalog", "subagents", options.Name, "instructions.md")
	if err := config.EnsureWritable(path, options.Force); err != nil {
		return nil, err
	}
	return []Change{{
		Path:    path,
		Action:  ActionCreate,
		Content: fmt.Sprintf("# %s\n\n## Responsibility\n\n%s\n", options.Name, defaultString(options.Description, "Describe this subagent's bounded context.")),
	}}, nil
}

func planMigration(options Options) ([]Change, error) {
	timestamp := migrationTimestamp()
	path := filepath.Join("agents", "migrations", fmt.Sprintf("%d_%s.ts", timestamp, options.Name))
	if err := config.EnsureWritable(path, options.Force); err != nil {
		return nil, err
	}
	return []Change{{
		Path:    path,
		Action:  ActionCreate,
		Content: fmt.Sprintf("export const name = \"%s\";\n\nexport async function up() {{\n  // TODO: implement migration.\n}}\n\nexport async function down() {{\n  // TODO: implement rollback.\n}}\n", options.Name),
	}}, nil
}

func migrationTimestamp() uint64 {
	return uint64(time.Now().UnixNano())
}

func printPlan(kind Kind, options Options, changes []Change) {
	fmt.Printf("Generate %s '%s' (dry run)\n", kind, options.Name)
	for _, change := range changes {
		fmt.Printf("%s %s\n", change.Action, change.Path)
	}
}

func appendGeneratedComponentMap(output *strings.Builder, label string, values []string) {
	fmt.Fprintf(output, "    %s:", label)
	if len(values) == 0 {
		output.WriteString(" {}\n")
		return
	}
	output.WriteString("\n")
	for _, value := range values {
		fmt.Fprintf(output, "      %s: 1.0.0\n", value)
	}
}

func appendGeneratedStringList(output *strings.Builder, label string, values []string) {
	fmt.Fprintf(output, "    %s:", label)
	if len(values) == 0 {
		output.WriteString(" []\n")
		return
	}
	fmt.Fprintf(output, " [%s]\n", strings.Join(values, ", "))
}

func appendGeneratedApprovals(output *strings.Builder, options Options) {
	output.WriteString("    approvals:")
	if options.Approval == "" {
		output.WriteString(" {}\n")
		return
	}
	if len(options.WithTools) == 0 {
		fmt.Fprintf(output, " %s\n", quoted(options.Approval))
		return
	}
	output.WriteString("\n")
	for _, tool := range options.WithTools {
		fmt.Fprintf(output, "      %s: %s\n", tool, options.Approval)
	}
}

func appendOptionalString(output *strings.Builder, label, value string) {
	if strings.TrimSpace(value) != "" {
		fmt.Fprintf(output, "    %s: %s\n", label, quoted(value))
	}
}

func catalogSection(kind Kind) string {
	switch kind {
	case KindTool:
		return "tools:"
	case KindSkill:
		return "skills:"
	case KindChannel:
		return "channels:"
	case KindSchedule:
		return "schedules:"
	case KindApproval:
		return "approvals:"
	case KindEval:
		return "evals:"
	case KindMemory:
		return "memory:"
	}
	return ""
}

func catalogEntry(kind Kind, options Options) string {
	var entry strings.Builder
	fmt.Fprintf(&entry, "  %s:\n    version: %s\n", options.Name, defaultVersion(options.Version))
	switch kind {
	case KindTool:
		fmt.Fprintf(&entry, "    side_effects: %s\n", defaultString(options.SideEffects, "read"))
	case KindMemory:
		fmt.Fprintf(&entry, "    retention: %s\n", defaultString(options.Retention, "180d"))
	case KindSchedule:
		fmt.Fprintf(&entry, "    schedule: %s\n", quoted(defaultString(options.Schedule, "0 9 * * 1-5")))
	case KindChannel:
		fmt.Fprintf(&entry, "    kind: %s\n", defaultString(options.Kind, options.Name))
		appendCatalogOptionalString(&entry, "allow_from", options.AllowFrom)
		appendCatalogOptionalString(&entry, "messaging_from", options.MessagingFrom)
		appendCatalogOptionalString(&entry, "connect_uid", options.ConnectUID)
		appendCatalogOptionalString(&entry, "bot_username", options.BotUsername)
		appendCatalogOptionalString(&entry, "bot_name", options.BotName)
	}
	return entry.String()
}

func appendCatalogOptionalString(output *strings.Builder, label, value string) {
	if strings.TrimSpace(value) != "" {
		fmt.Fprintf(output, "    %s: %s\n", label, quoted(value))
	}
}

func componentStub(kind Kind, options Options) (*Change, error) {
	var dir, extension, content string
	switch kind {
	case KindTool:
		dir, extension = "tools", "ts"
		content = fmt.Sprintf("export async function %s() {\n  throw new Error(\"%s is not implemented yet\");\n}\n", options.Name, options.Name)
	case KindSkill:
		dir, extension = "skills", "md"
		content = fmt.Sprintf("# %s\n\n## Trigger\n\nUse this skill when ...\n\n## Procedure\n\n- Gather context.\n- Use approved tools.\n- Verify the result.\n", options.Name)
	case KindEval:
		dir, extension = "evals", "ts"
		content = fmt.Sprintf("export async function %sEval() {\n  throw new Error(\"%s eval is not implemented yet\");\n}\n", options.Name, options.Name)
	case KindApproval:
		dir, extension = "approvals", "ts"
		content = fmt.Sprintf("export default {\n  name: \"%s\",\n  policy: \"required\",\n};\n", options.Name)
	case KindMemory:
		dir, extension = "memory", "ts"
		content = fmt.Sprintf("export default {\n  name: \"%s\",\n  version: \"%s\",\n  retention: \"%s\",\n};\n", options.Name, defaultVersion(options.Version), defaultString(options.Retention, "180d"))
	case KindChannel:
		dir, extension = "channels", "ts"
		content = fmt.Sprintf("export default {\n  name: \"%s\",\n};\n", options.Name)
	case KindSchedule:
		dir, extension = "schedules", "ts"
		content = fmt.Sprintf("export default {\n  name: \"%s\",\n  schedule: %s,\n};\n", options.Name, quoted(defaultString(options.Schedule, "0 9 * * 1-5")))
	default:
		return nil, nil
	}
	path := filepath.Join("catalog", dir, fmt.Sprintf("%s.%s", options.Name, extension))
	if err := config.EnsureWritable(path, options.Force); err != nil {
		return nil, err
	}
	return &Change{Path: path, Action: ActionCreate, Content: content}, nil
}

func appendYAMLListEntry(source, section, entry string) string {
	if strings.Contains(source, section) {
		return strings.TrimRight(source, "\n") + entry
	}
	return strings.TrimRight(source, "\n") + "\n" + section + entry
}

func upsertCatalogEntry(source, section, name, entry string) string {
	withoutExisting := removeCatalogEntry(source, section, name)
	return insertCatalogEntry(withoutExisting, section, entry)
}

func insertCatalogEntry(source, section, entry string) string {
	var output []string
	inserted := false
	inSection := false
	for _, line := range strings.Split(source, "\n") {
		isTopLevel := !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":")
		if isTopLevel && inSection && !inserted {
			output = append(output, strings.TrimRight(entry, "\n"))
			inserted = true
		}
		output = append(output, line)
		if isTopLevel {
			inSection = line == section
		}
	}
	if inSection && !inserted {
		output = append(output, strings.TrimRight(entry, "\n"))
		inserted = true
	}
	if !inserted {
		output = append(output, section)
		output = append(output, strings.TrimRight(entry, "\n"))
	}
	return strings.Join(output, "\n") + "\n"
}

func removeCatalogEntry(source, section, name string) string {
	var output []string
	inSection := false
	skipping := false
	entryPrefix := fmt.Sprintf("  %s:", name)
	for _, line := range strings.Split(source, "\n") {
		isTopLevel := !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":")
		if isTopLevel {
			inSection = line == section
			skipping = false
		}
		if inSection && line == entryPrefix {
			skipping = true
			continue
		}
		if skipping {
			if strings.HasPrefix(line, "    ") || strings.TrimSpace(line) == "" {
				continue
			}
			skipping = false
		}
		output = append(output, line)
	}
	return strings.Join(output, "\n") + "\n"
}

func catalogEntryExists(source, section, name string) bool {
	inSection := false
	entryPrefix := fmt.Sprintf("  %s:", name)
	for _, line := range strings.Split(source, "\n") {
		isTopLevel := !strings.HasPrefix(line, " ") && strings.HasSuffix(line, ":")
		if isTopLevel {
			inSection = line == section
		}
		if inSection && line == entryPrefix {
			return true
		}
	}
	return false
}

func changeAction(path string) Action {
	if _, err := os.Stat(path); err == nil {
		return ActionUpdate
	}
	return ActionCreate
}

func defaultVersion(version string) string {
	if version == "" {
		return "1.0.0"
	}
	return version
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func quoted(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%q", value)
	}
	return string(data)
}

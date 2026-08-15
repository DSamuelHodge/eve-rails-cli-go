package templates

import "embed"

//go:embed agent/*.tmpl
var agentFS embed.FS

// AgentNames returns the embedded agent template filenames.
func AgentNames() []string {
	entries, err := agentFS.ReadDir("agent")
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// AgentContent returns the content of an embedded agent template.
func AgentContent(name string) ([]byte, error) {
	return agentFS.ReadFile("agent/" + name)
}

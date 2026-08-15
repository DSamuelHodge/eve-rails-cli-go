package process

import (
	"fmt"
	"os"
	"os/exec"
)

// Run executes a command in a working directory and returns an error on failure.
func Run(cwd string, commandLine []string) error {
	if len(commandLine) == 0 {
		return nil
	}
	program := commandLine[0]
	args := commandLine[1:]
	command := exec.Command(program, args...)
	command.Dir = cwd
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	if err := command.Run(); err != nil {
		return fmt.Errorf("command failed: %s", joinCommand(commandLine))
	}
	return nil
}

func joinCommand(commandLine []string) string {
	var result string
	for i, part := range commandLine {
		if i > 0 {
			result += " "
		}
		result += part
	}
	return result
}

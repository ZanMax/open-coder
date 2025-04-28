package command

import (
	"os/exec"
	"strings"
)

type CommandResponse struct {
	Commands    []string `json:"commands,omitempty"`
	Explanation string   `json:"explanation,omitempty"`
	Answer      string   `json:"answer,omitempty"`
}

// ExecuteCommands runs the commands and returns the combined output.
func ExecuteCommands(cwd string, cr CommandResponse) string {
	var sb strings.Builder
	for _, cmdStr := range cr.Commands {
		cmd := exec.Command("bash", "-lc", cmdStr)
		cmd.Dir = cwd
		out, _ := cmd.CombinedOutput()
		sb.Write(out)
		if !strings.HasSuffix(string(out), "\n") {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
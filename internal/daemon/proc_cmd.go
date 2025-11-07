package daemon

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// commandLine attempts to fetch the command line for the provided PID.
// Falls back to a synthetic placeholder if it cannot be determined.
func commandLine(pid int) string {
	if pid <= 0 {
		return fmt.Sprintf("pid:%d", pid)
	}
	if cmd, err := readProcCmdline(pid); err == nil && cmd != "" {
		return cmd
	}
	if cmd, err := readPsCommand(pid); err == nil && cmd != "" {
		return cmd
	}
	return fmt.Sprintf("pid:%d", pid)
}

func readProcCmdline(pid int) (string, error) {
	path := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	parts := bytes.Split(data, []byte{0})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		out = append(out, string(part))
	}
	return strings.Join(out, " "), nil
}

func readPsCommand(pid int) (string, error) {
	cmd := exec.Command("ps", "-o", "command=", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

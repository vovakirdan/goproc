package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"goproc/internal/app"

	"github.com/spf13/cobra"
)

var (
	runTags    []string
	runGroups  []string
	runName    string
	runTimeout int
)

func init() {
	rootCmd.AddCommand(cmdRun)

	cmdRun.Flags().StringSliceVar(&runTags, "tag", nil, "Tag to assign to the tracked process (repeatable)")
	cmdRun.Flags().StringSliceVar(&runGroups, "group", nil, "Group to assign to the tracked process (repeatable)")
	cmdRun.Flags().StringVar(&runName, "name", "", "Optional unique name for the tracked process")
	cmdRun.Flags().IntVar(&runTimeout, "timeout", 3, "Timeout in seconds for contacting the daemon")
}

var cmdRun = &cobra.Command{
	Use:   "run -- <command> [args...]",
	Short: "Launch a new command and register its PID with the daemon",
	Long:  "Starts the provided command, immediately registers the spawned PID with the daemon, and exits while the process keeps running.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		child := exec.Command(args[0], args[1:]...)
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		child.Stdin = os.Stdin
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := child.Start(); err != nil {
			return fmt.Errorf("start command: %w", err)
		}
		defer child.Process.Release()

		name := strings.TrimSpace(runName)
		res, err := controller().Add(cmd.Context(), app.AddParams{
			PID:     child.Process.Pid,
			Tags:    runTags,
			Groups:  runGroups,
			Name:    name,
			Timeout: time.Duration(runTimeout) * time.Second,
		})
		if err != nil {
			_ = child.Process.Kill()
			return err
		}
		if res.AlreadyExists {
			_ = child.Process.Kill()
			fmt.Fprintln(os.Stdout, res.ExistingReason)
			return nil
		}

		fmt.Fprintf(os.Stdout, "Started pid=%d registry id=%d\n", child.Process.Pid, res.ID)
		return nil
	},
}

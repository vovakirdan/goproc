package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"

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
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}
		if runTimeout <= 0 {
			return errors.New("timeout must be greater than 0 seconds")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(runTimeout)*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

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
		req := &goprocv1.AddRequest{
			Pid:    int32(child.Process.Pid),
			Tags:   append([]string(nil), runTags...),
			Groups: append([]string(nil), runGroups...),
			Name:   name,
		}
		resp, err := client.Add(ctx, req)
		if err != nil {
			_ = child.Process.Kill()
			return fmt.Errorf("daemon add RPC failed: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Started pid=%d registry id=%d\n", child.Process.Pid, resp.GetId())
		return nil
	},
}

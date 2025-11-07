package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdAdd)
}

var cmdAdd = &cobra.Command{
	Use:   "add <pid>",
	Short: "Register an existing PID with the daemon",
	Long:  `Adds an existing process (by PID) to the daemon registry. Monitoring logic will be added in later iterations.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}

		pid, err := strconv.Atoi(args[0])
		if err != nil || pid <= 0 {
			return fmt.Errorf("invalid pid %q", args[0])
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		resp, err := client.Add(ctx, &goprocv1.AddRequest{Pid: int32(pid)})
		if err != nil {
			return fmt.Errorf("daemon add RPC failed: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Process %d registered with id %d\n", pid, resp.GetId())
		return nil
	},
}

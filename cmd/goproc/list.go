package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdList)
}

var cmdList = &cobra.Command{
	Use:   "list",
	Short: "List all processes managed by the daemon",
	Long:  `Fetches the process registry from the daemon via gRPC.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		resp, err := client.List(ctx, &goprocv1.ListRequest{})
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}

		if len(resp.GetProcs()) == 0 {
			fmt.Fprintln(os.Stdout, "No processes registered")
			return nil
		}

		for _, proc := range resp.GetProcs() {
			fmt.Fprintf(os.Stdout, "[id=%d] pid=%d alive=%t cmd=%s\n",
				proc.GetId(), proc.GetPid(), proc.GetAlive(), proc.GetCmd())
		}
		return nil
	},
}

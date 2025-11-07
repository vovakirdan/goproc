package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"

	"github.com/spf13/cobra"
)

var (
	resetConfirm string
	resetTimeout int
)

func init() {
	rootCmd.AddCommand(cmdReset)
	cmdReset.Flags().StringVar(&resetConfirm, "confirm", "", `Type "RESET" to acknowledge registry wipe`)
	cmdReset.Flags().IntVar(&resetTimeout, "timeout", 5, "Timeout in seconds for reset RPC")
}

var cmdReset = &cobra.Command{
	Use:   "reset",
	Short: "Erase the registry snapshot and reset IDs",
	Long:  "Removes every tracked process, clears indexes, resets ID counters, and rewrites the snapshot. Requires --confirm RESET.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}
		if strings.TrimSpace(resetConfirm) != "RESET" {
			return errors.New(`destructive command: pass --confirm RESET to proceed`)
		}
		if resetTimeout <= 0 {
			return errors.New("timeout must be greater than 0 seconds")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(resetTimeout)*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		if _, err := client.Reset(ctx, &goprocv1.ResetRequest{}); err != nil {
			return fmt.Errorf("daemon reset RPC failed: %w", err)
		}

		fmt.Fprintln(os.Stdout, "Registry cleared and IDs reset")
		return nil
	},
}

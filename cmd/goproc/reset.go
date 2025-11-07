package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"goproc/internal/app"

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
		err := controller().Reset(cmd.Context(), app.ResetParams{
			Timeout:   time.Duration(resetTimeout) * time.Second,
			Confirmed: strings.TrimSpace(resetConfirm) == "RESET",
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, "Registry cleared and IDs reset")
		return nil
	},
}

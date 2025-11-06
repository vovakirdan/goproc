package main

import (
	"errors"
	"fmt"
	"os"

	"goproc/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdList)
}

var cmdList = &cobra.Command{
	Use: "list",
	Short: "List all processes managed by the daemon",
	Long: `The list command is used to list all processes managed by the daemon.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}
		fmt.Fprintln(os.Stdout, "Listing processes...")
		return nil
	},
}
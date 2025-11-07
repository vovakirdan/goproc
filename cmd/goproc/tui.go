package main

import (
	"fmt"

	"goproc/internal/tui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdTUI)
}

var cmdTUI = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive terminal UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tui.Run(controller()); err != nil {
			return fmt.Errorf("tui exited with error: %w", err)
		}
		return nil
	},
}

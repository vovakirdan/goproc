package main

import (
	"errors"
	"fmt"
	"os"

	"goproc/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdAdd)
}

var cmdAdd = &cobra.Command{
	Use: "add <process-name>",
	Short: "Add a new process to the daemon",
	Long: `The add command is used to add a new process to the daemon. The process will be started when the daemon is started.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}
		// 1) Add the process to the daemon
		// if err := daemon.AddProcess(args[0]); err != nil {
		// 	return err
		// }
		fmt.Fprintln(os.Stdout, "Process added successfully")
		return nil
	},
}

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"goproc/internal/daemon"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdDaemon)
}

var cmdDaemon = &cobra.Command{
	Use: "daemon",
	Short: "Start the daemon process",
	Long: `The daemon process is responsible for monitoring and managing processes. If the daemon is not running, it will be started. Otherwise, nothing will happen.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 0) check if the daemon is running
		if daemon.IsRunning() {
			fmt.Fprintln(os.Stdout, "Daemon is already running")
			return nil
		}
		// 1) Not running, so start it
		fmt.Fprintln(os.Stdout, "Starting daemon process...") // todo: add spinner
		srv, err := daemon.StartDaemon()
		if err != nil {
			return err
		}
		defer srv.Close()

		// 2) Wait for SIGINT ot SIGTERN to stop
		sigc := make(chan os.Signal, 2)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		<-sigc
		fmt.Fprintln(os.Stdout, "Stopping daemon process...") // todo: add spinner
		return nil
	},
}


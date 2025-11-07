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

var daemonForceRestart bool

func init() {
	cmdDaemon.Flags().BoolVarP(&daemonForceRestart, "force", "f", false, "Restart the daemon if it is already running")
}

var cmdDaemon = &cobra.Command{
	Use:   "daemon",
	Short: "Start the daemon process",
	Long:  `The daemon process is responsible for monitoring and managing processes. If the daemon is not running, it will be started. Otherwise, nothing will happen.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 0) check if the daemon is running
		if daemon.IsRunning() {
			if !daemonForceRestart {
				pid, err := daemon.RunningPID()
				var message string
				if pid != 0 {
					message = fmt.Sprintf("Daemon is already running (pid %d). Stop it manually or re-run with --force.", pid)
				} else {
					message = "Daemon is already running. Stop it manually or re-run with --force."
				}
				if err != nil {
					message = fmt.Sprintf("Error checking if daemon is running: %v", err)
				}
				fmt.Fprintln(os.Stdout, message)
				return nil
			}
			fmt.Fprintln(os.Stdout, "Stopping existing daemon process...")
			if err := daemon.StopRunningDaemon(true); err != nil {
				return err
			}
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

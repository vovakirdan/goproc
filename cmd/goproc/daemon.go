package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
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
		app := controller()
		status, statusErr := app.Status()
		if status.Running {
			if !daemonForceRestart {
				message := "Daemon is already running. Stop it manually or re-run with --force."
				if status.PID != 0 {
					message = fmt.Sprintf("Daemon is already running (pid %d). Stop it manually or re-run with --force.", status.PID)
				}
				if statusErr != nil {
					message = fmt.Sprintf("Error checking if daemon is running: %v", statusErr)
				}
				fmt.Fprintln(os.Stdout, message)
				return nil
			}
			fmt.Fprintln(os.Stdout, "Stopping existing daemon process...")
			if err := app.StopDaemon(true); err != nil {
				return err
			}
		}

		// start new daemon
		handle, err := app.StartDaemon()
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "Started demon process")
		runSpin := spinner.New(spinner.CharSets[21], 120*time.Millisecond, spinner.WithWriter(os.Stdout))
		runSpin.Suffix = " Running..."
		runSpin.Start()

		// 2) Wait for SIGINT ot SIGTERN to stop
		sigc := make(chan os.Signal, 2)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		runSpin.Stop()
		return handle.Close()
	},
}

package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"goproc/internal/app"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdAdd)
}

var (
	addTags   []string
	addGroups []string
	addName   string
)

func init() {
	cmdAdd.Flags().StringSliceVar(&addTags, "tag", nil, "Tag to assign to the process (repeatable)")
	cmdAdd.Flags().StringSliceVar(&addGroups, "group", nil, "Group to assign to the process (repeatable)")
	cmdAdd.Flags().StringVar(&addName, "name", "", "Unique name to assign to the process")
}

var cmdAdd = &cobra.Command{
	Use:   "add <pid>",
	Short: "Register an existing PID with the daemon",
	Long:  `Adds an existing process (by PID) to the daemon registry. Monitoring logic will be added in later iterations.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pid, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid pid %q", args[0])
		}

		res, err := controller().Add(cmd.Context(), app.AddParams{
			PID:     pid,
			Tags:    addTags,
			Groups:  addGroups,
			Name:    addName,
			Timeout: 2 * time.Second,
		})
		if err != nil {
			return err
		}
		if res.AlreadyExists {
			fmt.Fprintln(os.Stdout, res.ExistingReason)
			return nil
		}

		fmt.Fprintf(os.Stdout, "Process %d registered with id %d\n", pid, res.ID)
		return nil
	},
}

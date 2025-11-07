package main

import (
	"fmt"
	"os"
	"time"

	"goproc/internal/app"

	"github.com/spf13/cobra"
)

var (
	killTags    []string
	killGroups  []string
	killNames   []string
	killPIDs    []int
	killIDs     []int
	killAll     bool
	killTimeout int
)

func init() {
	rootCmd.AddCommand(cmdKill)
	cmdKill.Flags().StringSliceVar(&killTags, "tag", nil, "Match processes that have any of these tags")
	cmdKill.Flags().StringSliceVar(&killGroups, "group", nil, "Match processes that belong to any of these groups")
	cmdKill.Flags().StringSliceVar(&killNames, "name", nil, "Match processes with these exact names")
	cmdKill.Flags().IntSliceVar(&killPIDs, "pid", nil, "Filter by PID (repeatable)")
	cmdKill.Flags().IntSliceVar(&killIDs, "id", nil, "Filter by registry ID (repeatable)")
	cmdKill.Flags().BoolVar(&killAll, "all", false, "Kill every process that matches the selector")
	cmdKill.Flags().IntVar(&killTimeout, "timeout", 5, "Timeout in seconds for kill/remove operations")
}

var cmdKill = &cobra.Command{
	Use:   "kill",
	Short: "Terminate processes managed by the daemon",
	Long:  "Selects processes via the same filters as `list` and sends a SIGTERM (via the daemon) before removing them from the registry.",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := controller().Kill(cmd.Context(), app.KillParams{
			Filters: app.ListFilters{
				TagsAny:   killTags,
				GroupsAny: killGroups,
				Names:     killNames,
				PIDs:      killPIDs,
				IDs:       killIDs,
			},
			AllowAll:        killAll,
			Timeout:         time.Duration(killTimeout) * time.Second,
			RequireSelector: true,
		})
		if res.Message != "" {
			fmt.Fprintln(os.Stdout, res.Message)
		}
		for _, event := range res.Events {
			name := event.Proc.Name
			if name == "" {
				name = "-"
			}
			switch event.Kind {
			case "success":
				fmt.Fprintf(os.Stdout, "Killed and removed [id=%d] pid=%d name=%s\n", event.Proc.ID, event.Proc.PID, name)
			case "kill_failure":
				fmt.Fprintf(os.Stdout, "Failed to kill [id=%d] pid=%d name=%s: %v\n", event.Proc.ID, event.Proc.PID, name, event.Err)
			case "remove_failure":
				fmt.Fprintf(os.Stdout, "Killed [id=%d] pid=%d name=%s but failed to remove from registry: %v\n", event.Proc.ID, event.Proc.PID, name, event.Err)
			}
		}
		if err != nil {
			return err
		}
		return nil
	},
}

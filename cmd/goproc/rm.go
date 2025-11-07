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
	rmTags        []string
	rmGroups      []string
	rmPIDs        []int
	rmIDs         []int
	rmNames       []string
	rmSearch      string
	rmRemoveAll   bool
	rmTimeoutSecs int
)

func init() {
	rootCmd.AddCommand(cmdRm)

	cmdRm.Flags().StringSliceVar(&rmTags, "tag", nil, "Match processes that have any of these tags")
	cmdRm.Flags().StringSliceVar(&rmGroups, "group", nil, "Match processes that belong to any of these groups")
	cmdRm.Flags().IntSliceVar(&rmPIDs, "pid", nil, "Filter by PID (repeatable)")
	cmdRm.Flags().IntSliceVar(&rmIDs, "id", nil, "Filter by registry ID (repeatable)")
	cmdRm.Flags().StringSliceVar(&rmNames, "name", nil, "Match processes with these exact names")
	cmdRm.Flags().StringVar(&rmSearch, "search", "", "Substring to match against process command")
	cmdRm.Flags().BoolVar(&rmRemoveAll, "all", false, "Remove every process that matches the selector")
	cmdRm.Flags().IntVar(&rmTimeoutSecs, "timeout", 3, "Timeout in seconds for list/remove operations")
}

var cmdRm = &cobra.Command{
	Use:   "rm",
	Short: "Remove processes from the daemon registry",
	Long:  "Looks up processes using the same filters as `list` (tag/group/pid/name) and removes matching entries.",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := controller().Remove(cmd.Context(), app.RemoveParams{
			Filters: app.ListFilters{
				TagsAny:    rmTags,
				GroupsAny:  rmGroups,
				Names:      rmNames,
				TextSearch: rmSearch,
				PIDs:       rmPIDs,
				IDs:        rmIDs,
			},
			AllowAll:        rmRemoveAll,
			Timeout:         time.Duration(rmTimeoutSecs) * time.Second,
			RequireSelector: true,
		})
		if err != nil {
			return err
		}
		if res.Message != "" {
			fmt.Fprintln(os.Stdout, res.Message)
			return nil
		}
		for _, proc := range res.Removed {
			name := proc.Name
			if name == "" {
				name = "-"
			}
			fmt.Fprintf(
				os.Stdout,
				"Removed [id=%d] pid=%d name=%s cmd=%s tags=[%s] groups=[%s]\n",
				proc.ID,
				proc.PID,
				name,
				proc.Cmd,
				strings.Join(proc.Tags, ","),
				strings.Join(proc.Groups, ","),
			)
		}
		return nil
	},
}

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
	listTagsAny    []string
	listTagsAll    []string
	listGroupsAny  []string
	listGroupsAll  []string
	listNames      []string
	listAliveOnly  bool
	listPIDs       []int
	listIDs        []int
	listTextSearch string
)

func init() {
	rootCmd.AddCommand(cmdList)
	cmdList.Flags().StringSliceVar(&listTagsAny, "tag", nil, "Match processes that have any of these tags")
	cmdList.Flags().StringSliceVar(&listTagsAll, "tag-all", nil, "Match processes that have all of these tags")
	cmdList.Flags().StringSliceVar(&listGroupsAny, "group", nil, "Match processes that are in any of these groups")
	cmdList.Flags().StringSliceVar(&listGroupsAll, "group-all", nil, "Match processes that are in all of these groups")
	cmdList.Flags().StringSliceVar(&listNames, "name", nil, "Match processes with these exact names")
	cmdList.Flags().BoolVar(&listAliveOnly, "alive", false, "Only show processes currently considered alive")
	cmdList.Flags().IntSliceVar(&listPIDs, "pid", nil, "Filter by PID (repeatable)")
	cmdList.Flags().IntSliceVar(&listIDs, "id", nil, "Filter by registry ID (repeatable)")
	cmdList.Flags().StringVar(&listTextSearch, "search", "", "Substring to match against command")
}

var cmdList = &cobra.Command{
	Use:   "list",
	Short: "List all processes managed by the daemon",
	Long:  `Fetches the process registry from the daemon via gRPC.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		procs, err := controller().List(cmd.Context(), app.ListParams{
			Timeout: 3 * time.Second,
			Filters: app.ListFilters{
				TagsAny:    listTagsAny,
				TagsAll:    listTagsAll,
				GroupsAny:  listGroupsAny,
				GroupsAll:  listGroupsAll,
				Names:      listNames,
				AliveOnly:  listAliveOnly,
				TextSearch: listTextSearch,
				PIDs:       listPIDs,
				IDs:        listIDs,
			},
		})
		if err != nil {
			return err
		}

		if len(procs) == 0 {
			fmt.Fprintln(os.Stdout, "No processes registered")
			return nil
		}

		for _, proc := range procs {
			name := proc.Name
			if name == "" {
				name = "-"
			}
			fmt.Fprintf(
				os.Stdout,
				"[id=%d] pid=%d name=%s alive=%t cmd=%s tags=[%s] groups=[%s]\n",
				proc.ID,
				proc.PID,
				name,
				proc.Alive,
				proc.Cmd,
				strings.Join(proc.Tags, ","),
				strings.Join(proc.Groups, ","),
			)
		}
		return nil
	},
}

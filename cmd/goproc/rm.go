package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"

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
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}
		if !rmRemoveAll && len(rmTags) == 0 && len(rmGroups) == 0 && len(rmPIDs) == 0 && len(rmIDs) == 0 && len(rmNames) == 0 && rmSearch == "" {
			return errors.New("provide at least one selector (--id/--pid/--tag/--group/--name/--search)")
		}
		if rmTimeoutSecs <= 0 {
			return errors.New("timeout must be greater than 0 seconds")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(rmTimeoutSecs)*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		cleanNames := make([]string, 0, len(rmNames))
		for _, name := range rmNames {
			name = strings.TrimSpace(name)
			if name == "" {
				return errors.New("name filters must not be empty")
			}
			cleanNames = append(cleanNames, name)
		}

		req := &goprocv1.ListRequest{
			TagsAny:    append([]string(nil), rmTags...),
			GroupsAny:  append([]string(nil), rmGroups...),
			Names:      cleanNames,
			TextSearch: rmSearch,
		}
		for _, pid := range rmPIDs {
			if pid <= 0 {
				return fmt.Errorf("invalid pid filter: %d", pid)
			}
			req.Pids = append(req.Pids, int32(pid))
		}
		for _, id := range rmIDs {
			if id <= 0 {
				return fmt.Errorf("invalid id filter: %d", id)
			}
			req.Ids = append(req.Ids, uint64(id))
		}

		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}

		procs := resp.GetProcs()
		if len(procs) == 0 {
			fmt.Fprintln(os.Stdout, "No matching processes registered")
			return nil
		}

		if len(procs) > 1 && !rmRemoveAll {
			var ids []string
			for i := 0; i < len(procs) && i < 5; i++ {
				ids = append(ids, fmt.Sprintf("%d", procs[i].GetId()))
			}
			if len(procs) > 5 {
				ids = append(ids, "...")
			}
			return fmt.Errorf("multiple processes match filters (ids: %s). Use --all to delete all or narrow the selection", strings.Join(ids, ", "))
		}

		for _, proc := range procs {
			if _, err := client.Rm(ctx, &goprocv1.RmRequest{Id: proc.GetId()}); err != nil {
				return fmt.Errorf("remove id %d failed: %w", proc.GetId(), err)
			}
			name := proc.GetName()
			if name == "" {
				name = "-"
			}
			fmt.Fprintf(
				os.Stdout,
				"Removed [id=%d] pid=%d name=%s cmd=%s tags=[%s] groups=[%s]\n",
				proc.GetId(),
				proc.GetPid(),
				name,
				proc.GetCmd(),
				strings.Join(proc.GetTags(), ","),
				strings.Join(proc.GetGroups(), ","),
			)
		}
		return nil
	},
}

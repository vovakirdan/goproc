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
	rmNameMatch   string
	rmRemoveAll   bool
	rmTimeoutSecs int
)

func init() {
	rootCmd.AddCommand(cmdRm)

	cmdRm.Flags().StringSliceVar(&rmTags, "tag", nil, "Match processes that have any of these tags")
	cmdRm.Flags().StringSliceVar(&rmGroups, "group", nil, "Match processes that belong to any of these groups")
	cmdRm.Flags().IntSliceVar(&rmPIDs, "pid", nil, "Filter by PID (repeatable)")
	cmdRm.Flags().IntSliceVar(&rmIDs, "id", nil, "Filter by registry ID (repeatable)")
	cmdRm.Flags().StringVar(&rmNameMatch, "name", "", "Substring to match against process command")
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
		if len(rmTags) == 0 && len(rmGroups) == 0 && len(rmPIDs) == 0 && len(rmIDs) == 0 && rmNameMatch == "" {
			return errors.New("provide at least one selector (--id/--pid/--tag/--group/--name)")
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

		req := &goprocv1.ListRequest{
			TagsAny:    append([]string(nil), rmTags...),
			GroupsAny:  append([]string(nil), rmGroups...),
			TextSearch: rmNameMatch,
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
			req.Ids = append(req.Ids, uint32(id))
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
			fmt.Fprintf(
				os.Stdout,
				"Removed [id=%d] pid=%d cmd=%s tags=[%s] groups=[%s]\n",
				proc.GetId(),
				proc.GetPid(),
				proc.GetCmd(),
				strings.Join(proc.GetTags(), ","),
				strings.Join(proc.GetGroups(), ","),
			)
		}
		return nil
	},
}

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
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}
		if !killAll && len(killTags) == 0 && len(killGroups) == 0 && len(killNames) == 0 && len(killIDs) == 0 && len(killPIDs) == 0 {
			return errors.New("provide at least one selector (--id/--pid/--tag/--group/--name) or pass --all")
		}
		if killTimeout <= 0 {
			return errors.New("timeout must be greater than 0 seconds")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(killTimeout)*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		cleanNames := make([]string, 0, len(killNames))
		for _, name := range killNames {
			name = strings.TrimSpace(name)
			if name == "" {
				return errors.New("name filters must not be empty")
			}
			cleanNames = append(cleanNames, name)
		}

		req := &goprocv1.ListRequest{
			TagsAny:   append([]string(nil), killTags...),
			GroupsAny: append([]string(nil), killGroups...),
			Names:     cleanNames,
		}
		for _, pid := range killPIDs {
			if pid <= 0 {
				return fmt.Errorf("invalid pid filter: %d", pid)
			}
			req.Pids = append(req.Pids, int32(pid))
		}
		for _, id := range killIDs {
			if id <= 0 {
				return fmt.Errorf("invalid id filter: %d", id)
			}
			req.Ids = append(req.Ids, uint64(id))
		}

		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}

		if len(resp.GetProcs()) == 0 {
			fmt.Fprintln(os.Stdout, "No processes match the provided selectors")
			return nil
		}

		alive := make([]*goprocv1.Proc, 0, len(resp.GetProcs()))
		for _, proc := range resp.GetProcs() {
			if proc.GetAlive() {
				alive = append(alive, proc)
			}
		}
		if len(alive) == 0 {
			fmt.Fprintln(os.Stdout, "Matching processes exist but none are currently alive")
			return nil
		}
		if len(alive) > 1 && !killAll {
			var ids []string
			for i := 0; i < len(alive) && i < 5; i++ {
				ids = append(ids, fmt.Sprintf("%d", alive[i].GetId()))
			}
			if len(alive) > 5 {
				ids = append(ids, "...")
			}
			return fmt.Errorf("multiple alive processes match filters (ids: %s). Use --all to terminate all or narrow the selection", strings.Join(ids, ", "))
		}

		var (
			successes int
			failures  int
		)
		for _, proc := range alive {
			prettyName := proc.GetName()
			if prettyName == "" {
				prettyName = "-"
			}
			if _, err := client.Kill(ctx, &goprocv1.KillRequest{Target: &goprocv1.KillRequest_Id{Id: proc.GetId()}}); err != nil {
				failures++
				fmt.Fprintf(os.Stdout, "Failed to kill [id=%d] pid=%d name=%s: %v\n", proc.GetId(), proc.GetPid(), prettyName, err)
				continue
			}
			if _, err := client.Rm(ctx, &goprocv1.RmRequest{Id: proc.GetId()}); err != nil {
				failures++
				fmt.Fprintf(os.Stdout, "Killed [id=%d] pid=%d name=%s but failed to remove from registry: %v\n", proc.GetId(), proc.GetPid(), prettyName, err)
				continue
			}
			successes++
			fmt.Fprintf(os.Stdout, "Killed and removed [id=%d] pid=%d name=%s\n", proc.GetId(), proc.GetPid(), prettyName)
		}

		if successes == len(alive) {
			return nil
		}
		if successes == 0 {
			return errors.New("no processes were killed (see output above)")
		}
		return fmt.Errorf("partially successful: killed %d/%d processes", successes, len(alive))
	},
}

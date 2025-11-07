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
	listTagsAny    []string
	listTagsAll    []string
	listGroupsAny  []string
	listGroupsAll  []string
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
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		req := &goprocv1.ListRequest{
			TagsAny:    append([]string(nil), listTagsAny...),
			TagsAll:    append([]string(nil), listTagsAll...),
			GroupsAny:  append([]string(nil), listGroupsAny...),
			GroupsAll:  append([]string(nil), listGroupsAll...),
			AliveOnly:  listAliveOnly,
			TextSearch: listTextSearch,
		}

		for _, pid := range listPIDs {
			if pid <= 0 {
				return fmt.Errorf("invalid pid filter: %d", pid)
			}
			req.Pids = append(req.Pids, int32(pid))
		}
		for _, id := range listIDs {
			if id <= 0 {
				return fmt.Errorf("invalid id filter: %d", id)
			}
			req.Ids = append(req.Ids, uint32(id))
		}

		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}

		if len(resp.GetProcs()) == 0 {
			fmt.Fprintln(os.Stdout, "No processes registered")
			return nil
		}

		for _, proc := range resp.GetProcs() {
			fmt.Fprintf(
				os.Stdout,
				"[id=%d] pid=%d alive=%t cmd=%s tags=[%s] groups=[%s]\n",
				proc.GetId(),
				proc.GetPid(),
				proc.GetAlive(),
				proc.GetCmd(),
				strings.Join(proc.GetTags(), ","),
				strings.Join(proc.GetGroups(), ","),
			)
		}
		return nil
	},
}

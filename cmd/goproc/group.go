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
	groupRename  string
	groupTimeout int
)

func init() {
	rootCmd.AddCommand(cmdGroup)
	cmdGroup.Flags().StringVar(&groupRename, "rename", "", "Rename the group to a new name before listing")
	cmdGroup.Flags().IntVar(&groupTimeout, "timeout", 3, "Timeout in seconds for daemon request")
}

var cmdGroup = &cobra.Command{
	Use:   "group <name>",
	Short: "List processes that belong to the given group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}
		name := strings.TrimSpace(args[0])
		if name == "" {
			return errors.New("group must not be empty")
		}
		if groupTimeout <= 0 {
			return errors.New("timeout must be greater than 0 seconds")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(groupTimeout)*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		if rename := strings.TrimSpace(groupRename); rename != "" {
			resp, err := client.RenameGroup(ctx, &goprocv1.RenameGroupRequest{From: name, To: rename})
			if err != nil {
				return fmt.Errorf("daemon rename group RPC failed: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Renamed group %q -> %q on %d process(es)\n", name, rename, resp.GetUpdated())
			name = rename
		}

		req := &goprocv1.ListRequest{
			GroupsAll: []string{name},
		}
		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}

		if len(resp.GetProcs()) == 0 {
			fmt.Fprintf(os.Stdout, "No processes found with group %q\n", name)
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

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
	tagRename  string
	tagTimeout int
)

func init() {
	rootCmd.AddCommand(cmdTag)
	cmdTag.Flags().StringVar(&tagRename, "rename", "", "Rename the tag to a new name before listing")
	cmdTag.Flags().IntVar(&tagTimeout, "timeout", 3, "Timeout in seconds for daemon request")
}

var cmdTag = &cobra.Command{
	Use:   "tag <name>",
	Short: "List processes that carry the given tag",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}
		name := strings.TrimSpace(args[0])
		if name == "" {
			return errors.New("tag must not be empty")
		}
		if tagTimeout <= 0 {
			return errors.New("timeout must be greater than 0 seconds")
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(tagTimeout)*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		if rename := strings.TrimSpace(tagRename); rename != "" {
			resp, err := client.RenameTag(ctx, &goprocv1.RenameTagRequest{From: name, To: rename})
			if err != nil {
				return fmt.Errorf("daemon rename tag RPC failed: %w", err)
			}
			fmt.Fprintf(os.Stdout, "Renamed tag %q -> %q on %d process(es)\n", name, rename, resp.GetUpdated())
			name = rename
		}

		req := &goprocv1.ListRequest{
			TagsAll: []string{name},
		}
		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}

		if len(resp.GetProcs()) == 0 {
			fmt.Fprintf(os.Stdout, "No processes found with tag %q\n", name)
			return nil
		}
		for _, proc := range resp.GetProcs() {
			name := proc.GetName()
			if name == "" {
				name = "-"
			}
			fmt.Fprintf(
				os.Stdout,
				"[id=%d] pid=%d name=%s alive=%t cmd=%s tags=[%s] groups=[%s]\n",
				proc.GetId(),
				proc.GetPid(),
				name,
				proc.GetAlive(),
				proc.GetCmd(),
				strings.Join(proc.GetTags(), ","),
				strings.Join(proc.GetGroups(), ","),
			)
		}
		return nil
	},
}

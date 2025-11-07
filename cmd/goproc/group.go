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
		res, err := controller().Group(cmd.Context(), app.GroupParams{
			Name:    args[0],
			Rename:  groupRename,
			Timeout: time.Duration(groupTimeout) * time.Second,
		})
		if err != nil {
			return err
		}

		if res.RenameInfo != nil {
			fmt.Fprintf(os.Stdout, "Renamed group %q -> %q on %d process(es)\n", res.RenameInfo.From, res.RenameInfo.To, res.RenameInfo.Updated)
		}

		if len(res.Processes) == 0 {
			if res.Message != "" {
				fmt.Fprintln(os.Stdout, res.Message)
			}
			return nil
		}

		for _, proc := range res.Processes {
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

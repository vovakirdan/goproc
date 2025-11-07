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
		res, err := controller().Tag(cmd.Context(), app.TagParams{
			Name:    args[0],
			Rename:  tagRename,
			Timeout: time.Duration(tagTimeout) * time.Second,
		})
		if err != nil {
			return err
		}

		if res.RenameInfo != nil {
			fmt.Fprintf(os.Stdout, "Renamed tag %q -> %q on %d process(es)\n", res.RenameInfo.From, res.RenameInfo.To, res.RenameInfo.Updated)
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

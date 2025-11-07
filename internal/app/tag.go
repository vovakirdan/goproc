package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
)

// RenameInfo reports rename operations.
type RenameInfo struct {
	From    string
	To      string
	Updated uint32
}

// TagParams configures the tag command.
type TagParams struct {
	Name    string
	Rename  string
	Timeout time.Duration
}

// TagResult aggregates the rename/list outcome.
type TagResult struct {
	RenameInfo *RenameInfo
	Processes  []Process
	Message    string
}

// Tag lists and optionally renames tags.
func (a *App) Tag(ctx context.Context, params TagParams) (TagResult, error) {
	var result TagResult

	name := strings.TrimSpace(params.Name)
	if name == "" {
		return result, errors.New("tag must not be empty")
	}

	req := &goprocv1.ListRequest{
		TagsAll: []string{name},
	}

	err := a.withClient(ctx, params.Timeout, func(ctx context.Context, client goprocv1.GoProcClient) error {
		if rename := strings.TrimSpace(params.Rename); rename != "" {
			resp, err := client.RenameTag(ctx, &goprocv1.RenameTagRequest{From: name, To: rename})
			if err != nil {
				return fmt.Errorf("daemon rename tag RPC failed: %w", err)
			}
			result.RenameInfo = &RenameInfo{
				From:    name,
				To:      rename,
				Updated: resp.GetUpdated(),
			}
			name = rename
			req.TagsAll = []string{name}
		}

		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}
		for _, p := range resp.GetProcs() {
			result.Processes = append(result.Processes, procFromProto(p))
		}
		if len(result.Processes) == 0 {
			result.Message = fmt.Sprintf("No processes found with tag %q", name)
		}
		return nil
	})

	return result, err
}

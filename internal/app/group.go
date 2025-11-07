package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
)

// GroupParams configures the group command.
type GroupParams struct {
	Name    string
	Rename  string
	Timeout time.Duration
}

// GroupResult aggregates rename/list for groups.
type GroupResult struct {
	RenameInfo *RenameInfo
	Processes  []Process
	Message    string
}

// Group lists and optionally renames groups.
func (a *App) Group(ctx context.Context, params GroupParams) (GroupResult, error) {
	var result GroupResult

	name := strings.TrimSpace(params.Name)
	if name == "" {
		return result, errors.New("group must not be empty")
	}

	req := &goprocv1.ListRequest{
		GroupsAll: []string{name},
	}

	err := a.withClient(ctx, params.Timeout, func(ctx context.Context, client goprocv1.GoProcClient) error {
		if rename := strings.TrimSpace(params.Rename); rename != "" {
			resp, err := client.RenameGroup(ctx, &goprocv1.RenameGroupRequest{From: name, To: rename})
			if err != nil {
				return fmt.Errorf("daemon rename group RPC failed: %w", err)
			}
			result.RenameInfo = &RenameInfo{
				From:    name,
				To:      rename,
				Updated: resp.GetUpdated(),
			}
			name = rename
			req.GroupsAll = []string{name}
		}

		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}
		for _, p := range resp.GetProcs() {
			result.Processes = append(result.Processes, procFromProto(p))
		}
		if len(result.Processes) == 0 {
			result.Message = fmt.Sprintf("No processes found with group %q", name)
		}
		return nil
	})

	return result, err
}

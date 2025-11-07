package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
)

// RemoveParams configures rm command semantics.
type RemoveParams struct {
	Filters         ListFilters
	AllowAll        bool
	Timeout         time.Duration
	RequireSelector bool
}

// RemoveResult reports the registry entries removed.
type RemoveResult struct {
	Removed []Process
	Message string
}

// Remove deletes registry entries matching the filters.
func (a *App) Remove(ctx context.Context, params RemoveParams) (RemoveResult, error) {
	var result RemoveResult
	if params.RequireSelector && !params.AllowAll && emptySelectors(params.Filters) {
		return result, errors.New("provide at least one selector (--id/--pid/--tag/--group/--name/--search)")
	}

	req, err := params.Filters.buildRequest()
	if err != nil {
		return result, err
	}

	err = a.withClient(ctx, params.Timeout, func(ctx context.Context, client goprocv1.GoProcClient) error {
		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}
		if len(resp.GetProcs()) == 0 {
			result.Message = "No matching processes registered"
			return nil
		}
		if len(resp.GetProcs()) > 1 && !params.AllowAll {
			return fmt.Errorf("multiple processes match filters (ids: %s). Use --all to delete all or narrow the selection", joinSampleIDs(resp.GetProcs()))
		}
		for _, protoProc := range resp.GetProcs() {
			if _, err := client.Rm(ctx, &goprocv1.RmRequest{Id: protoProc.GetId()}); err != nil {
				return fmt.Errorf("remove id %d failed: %w", protoProc.GetId(), err)
			}
			result.Removed = append(result.Removed, procFromProto(protoProc))
		}
		return nil
	})

	return result, err
}

func emptySelectors(filters ListFilters) bool {
	return len(filters.TagsAny) == 0 &&
		len(filters.TagsAll) == 0 &&
		len(filters.GroupsAny) == 0 &&
		len(filters.GroupsAll) == 0 &&
		len(filters.Names) == 0 &&
		len(filters.PIDs) == 0 &&
		len(filters.IDs) == 0 &&
		strings.TrimSpace(filters.TextSearch) == ""
}

func joinSampleIDs(procs []*goprocv1.Proc) string {
	limit := 5
	ids := make([]string, 0, limit+1)
	for i := 0; i < len(procs) && i < limit; i++ {
		ids = append(ids, fmt.Sprintf("%d", procs[i].GetId()))
	}
	if len(procs) > limit {
		ids = append(ids, "...")
	}
	return strings.Join(ids, ", ")
}

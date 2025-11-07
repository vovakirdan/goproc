package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
)

// KillParams configures kill command semantics.
type KillParams struct {
	Filters         ListFilters
	AllowAll        bool
	Timeout         time.Duration
	RequireSelector bool
}

// KillEvent describes one action taken during kill/remove.
type KillEvent struct {
	Kind string
	Proc Process
	Err  error
}

// KillResult aggregates the command outcome.
type KillResult struct {
	Events       []KillEvent
	Message      string
	TotalMatches int
	TotalAlive   int
	Successes    int
}

// Kill terminates and removes processes that match the filters.
func (a *App) Kill(ctx context.Context, params KillParams) (KillResult, error) {
	var result KillResult
	if params.RequireSelector && !params.AllowAll && emptySelectors(params.Filters) {
		return result, errors.New("provide at least one selector (--id/--pid/--tag/--group/--name) or pass --all")
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

		result.TotalMatches = len(resp.GetProcs())
		if result.TotalMatches == 0 {
			result.Message = "No processes match the provided selectors"
			return nil
		}

		alive := make([]Process, 0, len(resp.GetProcs()))
		for _, protoProc := range resp.GetProcs() {
			if protoProc.GetAlive() {
				alive = append(alive, procFromProto(protoProc))
			}
		}
		result.TotalAlive = len(alive)
		if len(alive) == 0 {
			result.Message = "Matching processes exist but none are currently alive"
			return nil
		}

		if len(alive) > 1 && !params.AllowAll {
			return fmt.Errorf("multiple alive processes match filters (ids: %s). Use --all to terminate all or narrow the selection", joinProcessesSample(alive))
		}

		for _, proc := range alive {
			if _, err := client.Kill(ctx, &goprocv1.KillRequest{Target: &goprocv1.KillRequest_Id{Id: proc.ID}}); err != nil {
				result.Events = append(result.Events, KillEvent{
					Kind: "kill_failure",
					Proc: proc,
					Err:  fmt.Errorf("kill RPC failed: %w", err),
				})
				continue
			}
			if _, err := client.Rm(ctx, &goprocv1.RmRequest{Id: proc.ID}); err != nil {
				result.Events = append(result.Events, KillEvent{
					Kind: "remove_failure",
					Proc: proc,
					Err:  fmt.Errorf("remove id %d failed: %w", proc.ID, err),
				})
				continue
			}
			result.Events = append(result.Events, KillEvent{
				Kind: "success",
				Proc: proc,
			})
			result.Successes++
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	switch {
	case result.Successes == result.TotalAlive:
		return result, nil
	case result.Successes == 0 && result.TotalAlive > 0:
		return result, errors.New("no processes were killed (see output above)")
	default:
		return result, fmt.Errorf("partially successful: killed %d/%d processes", result.Successes, result.TotalAlive)
	}
}

func joinProcessesSample(procs []Process) string {
	limit := 5
	ids := make([]string, 0, limit+1)
	for i := 0; i < len(procs) && i < limit; i++ {
		ids = append(ids, fmt.Sprintf("%d", procs[i].ID))
	}
	if len(procs) > limit {
		ids = append(ids, "...")
	}
	return strings.Join(ids, ", ")
}

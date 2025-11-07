package app

import (
	"context"
	"fmt"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
)

// ListParams defines filters and timeout.
type ListParams struct {
	Filters ListFilters
	Timeout time.Duration
}

// List fetches registry entries matching the provided filters.
func (a *App) List(ctx context.Context, params ListParams) ([]Process, error) {
	req, err := params.Filters.buildRequest()
	if err != nil {
		return nil, err
	}

	var procs []Process
	err = a.withClient(ctx, params.Timeout, func(ctx context.Context, client goprocv1.GoProcClient) error {
		resp, err := client.List(ctx, req)
		if err != nil {
			return fmt.Errorf("daemon list RPC failed: %w", err)
		}
		procs = make([]Process, 0, len(resp.GetProcs()))
		for _, p := range resp.GetProcs() {
			procs = append(procs, procFromProto(p))
		}
		return nil
	})
	return procs, err
}

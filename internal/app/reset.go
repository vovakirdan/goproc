package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
)

// ResetParams configures the reset command.
type ResetParams struct {
	Timeout   time.Duration
	Confirmed bool
}

// Reset wipes registry state.
func (a *App) Reset(ctx context.Context, params ResetParams) error {
	if !params.Confirmed {
		return errors.New(`destructive command: confirmation required`)
	}

	return a.withClient(ctx, params.Timeout, func(ctx context.Context, client goprocv1.GoProcClient) error {
		if _, err := client.Reset(ctx, &goprocv1.ResetRequest{}); err != nil {
			return fmt.Errorf("daemon reset RPC failed: %w", err)
		}
		return nil
	})
}

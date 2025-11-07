package app

import (
	"context"
	"fmt"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
)

// Ping contacts the daemon and returns its health response.
func (a *App) Ping(ctx context.Context, timeout time.Duration) (string, error) {
	var msg string
	err := a.withClient(ctx, timeout, func(ctx context.Context, client goprocv1.GoProcClient) error {
		resp, err := client.Ping(ctx, &goprocv1.PingRequest{})
		if err != nil {
			return fmt.Errorf("daemon ping RPC failed: %w", err)
		}
		msg = resp.GetOk()
		return nil
	})
	return msg, err
}

package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"
)

// Ping contacts the daemon and returns its health response.
func (a *App) Ping(ctx context.Context, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		return "", errors.New("timeout must be greater than 0")
	}
	if !daemon.IsRunning() {
		return "", errors.New("daemon is not running")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, conn, err := daemon.Dial(ctx)
	if err != nil {
		return "", fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	resp, err := client.Ping(ctx, &goprocv1.PingRequest{})
	if err != nil {
		return "", fmt.Errorf("daemon ping RPC failed: %w", err)
	}
	return resp.GetOk(), nil
}

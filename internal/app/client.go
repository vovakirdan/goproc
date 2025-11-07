package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"
)

func (a *App) withClient(ctx context.Context, timeout time.Duration, fn func(context.Context, goprocv1.GoProcClient) error) error {
	if timeout <= 0 {
		return errors.New("timeout must be greater than 0")
	}
	if !daemon.IsRunning() {
		return errors.New("daemon is not running")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, conn, err := daemon.Dial(ctx)
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	return fn(ctx, client)
}

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"
)

var (
	daemonIsRunning  = daemon.IsRunning
	dialDaemonClient = func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return nil, nil, err
		}
		return client, conn, nil
	}
)

func resetDaemonDeps() {
	daemonIsRunning = daemon.IsRunning
	dialDaemonClient = func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return nil, nil, err
		}
		return client, conn, nil
	}
}

func (a *App) withClient(ctx context.Context, timeout time.Duration, fn func(context.Context, goprocv1.GoProcClient) error) error {
	if timeout <= 0 {
		return errors.New("timeout must be greater than 0")
	}
	if !daemonIsRunning() {
		return errors.New("daemon is not running")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, conn, err := dialDaemonClient(ctx)
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	if conn != nil {
		defer conn.Close()
	}

	return fn(ctx, client)
}

package app

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc"
	goprocv1 "goproc/api/proto/goproc/v1"
)

func TestAppResetRequiresConfirmation(t *testing.T) {
	app := New(Options{})
	err := app.Reset(context.Background(), ResetParams{Timeout: time.Second, Confirmed: false})
	if err == nil || err.Error() != "destructive command: confirmation required" {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestAppResetDaemonNotRunning(t *testing.T) {
	stubDaemon(t, false, nil)
	app := New(Options{})
	err := app.Reset(context.Background(), ResetParams{Timeout: time.Second, Confirmed: true})
	if err == nil || err.Error() != "daemon is not running" {
		t.Fatalf("expected daemon error, got %v", err)
	}
}

func TestAppResetDialError(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("dial failed")
	})
	app := New(Options{})
	err := app.Reset(context.Background(), ResetParams{Timeout: time.Second, Confirmed: true})
	if err == nil || err.Error() != "connect to daemon: dial failed" {
		t.Fatalf("expected dial error, got %v", err)
	}
}

func TestAppResetRPCError(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				return errors.New("rpc failed")
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})
	app := New(Options{})
	err := app.Reset(context.Background(), ResetParams{Timeout: time.Second, Confirmed: true})
	if err == nil || err.Error() != "daemon reset RPC failed: rpc failed" {
		t.Fatalf("expected rpc error, got %v", err)
	}
}

func TestAppResetSuccess(t *testing.T) {
	called := false
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch args.(type) {
				case *goprocv1.ResetRequest:
					called = true
					return nil
				default:
					t.Fatalf("unexpected args %T", args)
				}
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	if err := app.Reset(context.Background(), ResetParams{Timeout: time.Second, Confirmed: true}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("reset RPC was not invoked")
	}
}

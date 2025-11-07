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

func TestAppPingNotRunning(t *testing.T) {
	stubDaemon(t, false, nil)

	app := New(Options{})
	if _, err := app.Ping(context.Background(), time.Second); err == nil || err.Error() != "daemon is not running" {
		t.Fatalf("expected daemon not running error, got %v", err)
	}
}

func TestAppPingSuccess(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				resp, ok := reply.(*goprocv1.PingResponse)
				if !ok {
					t.Fatalf("unexpected reply type %T", reply)
				}
				resp.Ok = "pong"
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	msg, err := app.Ping(context.Background(), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if msg != "pong" {
		t.Fatalf("expected pong, got %q", msg)
	}
}

func TestAppPingDialError(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("dial failed")
	})

	app := New(Options{})
	if _, err := app.Ping(context.Background(), time.Second); err == nil || err.Error() != "connect to daemon: dial failed" {
		t.Fatalf("expected wrapped dial error, got %v", err)
	}
}

func TestAppPingInvalidTimeout(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("should not dial")
	})

	app := New(Options{})
	if _, err := app.Ping(context.Background(), 0); err == nil || err.Error() != "timeout must be greater than 0" {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

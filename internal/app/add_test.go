package app

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	goprocv1 "goproc/api/proto/goproc/v1"
)

func TestAppAddRejectsInvalidPID(t *testing.T) {
	app := New(Options{})
	if _, err := app.Add(context.Background(), AddParams{PID: 0, Timeout: time.Second}); err == nil || err.Error() != "invalid pid 0" {
		t.Fatalf("expected invalid pid error, got %v", err)
	}
}

func TestAppAddDaemonNotRunning(t *testing.T) {
	stubDaemon(t, false, nil)
	app := New(Options{})
	_, err := app.Add(context.Background(), AddParams{PID: 123, Timeout: time.Second})
	if err == nil || err.Error() != "daemon is not running" {
		t.Fatalf("expected daemon not running error, got %v", err)
	}
}

func TestAppAddDialError(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("dial failed")
	})
	app := New(Options{})
	_, err := app.Add(context.Background(), AddParams{PID: 10, Timeout: time.Second})
	if err == nil || err.Error() != "connect to daemon: dial failed" {
		t.Fatalf("expected wrapped dial error, got %v", err)
	}
}

func TestAppAddAlreadyExists(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				return status.Error(codes.AlreadyExists, "pid already tracked")
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	res, err := app.Add(context.Background(), AddParams{PID: 42, Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.AlreadyExists || res.ExistingReason != "pid already tracked" {
		t.Fatalf("expected already exists info, got %+v", res)
	}
}

func TestAppAddSuccess(t *testing.T) {
	var captured *goprocv1.AddRequest
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				req, ok := args.(*goprocv1.AddRequest)
				if !ok {
					t.Fatalf("unexpected args type %T", args)
				}
				captured = req
				resp, _ := reply.(*goprocv1.AddResponse)
				resp.Id = 99
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	params := AddParams{
		PID:     55,
		Tags:    []string{"a", "b"},
		Groups:  []string{"g1"},
		Name:    "  custom  ",
		Timeout: time.Second,
	}

	app := New(Options{})
	res, err := app.Add(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != 99 || res.AlreadyExists {
		t.Fatalf("unexpected result: %+v", res)
	}
	if captured == nil {
		t.Fatal("request was not captured")
	}
	if captured.GetPid() != int32(params.PID) {
		t.Fatalf("expected pid %d, got %d", params.PID, captured.GetPid())
	}
	if captured.GetName() != "custom" {
		t.Fatalf("expected trimmed name, got %q", captured.GetName())
	}
	if !reflect.DeepEqual(captured.GetTags(), params.Tags) {
		t.Fatalf("tags mismatch: %v", captured.GetTags())
	}
	if !reflect.DeepEqual(captured.GetGroups(), params.Groups) {
		t.Fatalf("groups mismatch: %v", captured.GetGroups())
	}
	// Ensure original slices not reused
	if &captured.Tags[0] == &params.Tags[0] {
		t.Fatalf("tags slice reused")
	}
}

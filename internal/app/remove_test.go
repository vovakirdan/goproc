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

func TestAppRemoveRequiresSelector(t *testing.T) {
	app := New(Options{})
	_, err := app.Remove(context.Background(), RemoveParams{
		Filters:         ListFilters{},
		AllowAll:        false,
		RequireSelector: true,
		Timeout:         time.Second,
	})
	if err == nil || err.Error() != "provide at least one selector (--id/--pid/--tag/--group/--name/--search)" {
		t.Fatalf("expected selector error, got %v", err)
	}
}

func TestAppRemoveInvalidFilter(t *testing.T) {
	app := New(Options{})
	_, err := app.Remove(context.Background(), RemoveParams{
		Filters: ListFilters{
			Names: []string{"ok", " "},
		},
		AllowAll:        true,
		RequireSelector: false,
		Timeout:         time.Second,
	})
	if err == nil || err.Error() != "name filters must not be empty" {
		t.Fatalf("expected name validation error, got %v", err)
	}
}

func TestAppRemoveDaemonNotRunning(t *testing.T) {
	stubDaemon(t, false, nil)
	app := New(Options{})
	_, err := app.Remove(context.Background(), RemoveParams{
		Filters:         ListFilters{IDs: []int{1}},
		RequireSelector: true,
		Timeout:         time.Second,
	})
	if err == nil || err.Error() != "daemon is not running" {
		t.Fatalf("expected daemon error, got %v", err)
	}
}

func TestAppRemoveDialError(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("dial failed")
	})
	app := New(Options{})
	_, err := app.Remove(context.Background(), RemoveParams{
		Filters:         ListFilters{IDs: []int{1}},
		RequireSelector: true,
		Timeout:         time.Second,
	})
	if err == nil || err.Error() != "connect to daemon: dial failed" {
		t.Fatalf("expected dial error, got %v", err)
	}
}

func TestAppRemoveNoMatches(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch args.(type) {
				case *goprocv1.ListRequest:
					return nil // empty response
				default:
					t.Fatalf("unexpected method args %T", args)
				}
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	res, err := app.Remove(context.Background(), RemoveParams{
		Filters:         ListFilters{IDs: []int{1}},
		RequireSelector: true,
		Timeout:         time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Message != "No matching processes registered" {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestAppRemoveMultipleWithoutAll(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch r := args.(type) {
				case *goprocv1.ListRequest:
					resp := reply.(*goprocv1.ListResponse)
					resp.Procs = []*goprocv1.Proc{
						{Id: 1}, {Id: 2}, {Id: 3}, {Id: 4}, {Id: 5}, {Id: 6},
					}
					_ = r // silence unused
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
	_, err := app.Remove(context.Background(), RemoveParams{
		Filters:         ListFilters{IDs: []int{1, 2}},
		RequireSelector: true,
		Timeout:         time.Second,
	})
	if err == nil || err.Error() != "multiple processes match filters (ids: 1, 2, 3, 4, 5, ...). Use --all to delete all or narrow the selection" {
		t.Fatalf("expected multi-match error, got %v", err)
	}
}

func TestAppRemoveRmFailure(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch args.(type) {
				case *goprocv1.ListRequest:
					resp := reply.(*goprocv1.ListResponse)
					resp.Procs = []*goprocv1.Proc{{Id: 7, Pid: 77, Cmd: "cmd"}}
					return nil
				case *goprocv1.RmRequest:
					return errors.New("rm failed")
				default:
					t.Fatalf("unexpected args %T", args)
				}
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	_, err := app.Remove(context.Background(), RemoveParams{
		Filters:         ListFilters{IDs: []int{7}},
		AllowAll:        true,
		RequireSelector: true,
		Timeout:         time.Second,
	})
	if err == nil || err.Error() != "remove id 7 failed: rm failed" {
		t.Fatalf("expected rm failure, got %v", err)
	}
}

func TestAppRemoveSuccess(t *testing.T) {
	var removedIDs []uint64
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch req := args.(type) {
				case *goprocv1.ListRequest:
					resp := reply.(*goprocv1.ListResponse)
					resp.Procs = []*goprocv1.Proc{
						{
							Id:           9,
							Pid:          90,
							Cmd:          "cmd",
							Name:         "proc",
							Tags:         []string{"x"},
							Groups:       []string{"y"},
							AddedAtUnix:  10,
							LastSeenUnix: 20,
						},
					}
					if len(req.GetIds()) != 1 || req.GetIds()[0] != 9 {
						t.Fatalf("unexpected list filter: %+v", req)
					}
					return nil
				case *goprocv1.RmRequest:
					removedIDs = append(removedIDs, req.GetId())
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
	res, err := app.Remove(context.Background(), RemoveParams{
		Filters:         ListFilters{IDs: []int{9}},
		RequireSelector: true,
		Timeout:         time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Message != "" {
		t.Fatalf("unexpected message %q", res.Message)
	}
	if len(res.Removed) != 1 || res.Removed[0].ID != 9 || res.Removed[0].PID != 90 || res.Removed[0].Name != "proc" {
		t.Fatalf("unexpected removed slice: %+v", res.Removed)
	}
	if len(removedIDs) != 1 || removedIDs[0] != 9 {
		t.Fatalf("rm not invoked correctly: %v", removedIDs)
	}
}

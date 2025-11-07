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

func TestAppKillRequiresSelector(t *testing.T) {
	app := New(Options{})
	_, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{},
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err == nil || err.Error() != "provide at least one selector (--id/--pid/--tag/--group/--name) or pass --all" {
		t.Fatalf("expected selector error, got %v", err)
	}
}

func TestAppKillDaemonNotRunning(t *testing.T) {
	stubDaemon(t, false, nil)
	app := New(Options{})
	_, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{IDs: []int{1}},
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err == nil || err.Error() != "daemon is not running" {
		t.Fatalf("expected daemon error, got %v", err)
	}
}

func TestAppKillDialError(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("dial failed")
	})
	app := New(Options{})
	_, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{IDs: []int{1}},
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err == nil || err.Error() != "connect to daemon: dial failed" {
		t.Fatalf("expected dial error, got %v", err)
	}
}

func TestAppKillNoMatches(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	res, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{IDs: []int{1}},
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Message != "No processes match the provided selectors" {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestAppKillNoAlive(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				resp := reply.(*goprocv1.ListResponse)
				resp.Procs = []*goprocv1.Proc{{Id: 1, Alive: false}}
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	res, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{IDs: []int{1}},
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Message != "Matching processes exist but none are currently alive" {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestAppKillMultiMatchWithoutAll(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				resp := reply.(*goprocv1.ListResponse)
				resp.Procs = []*goprocv1.Proc{{Id: 1, Alive: true}, {Id: 2, Alive: true}, {Id: 3, Alive: true}}
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	_, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{IDs: []int{1, 2}},
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err == nil || err.Error() != "multiple alive processes match filters (ids: 1, 2, 3). Use --all to terminate all or narrow the selection" {
		t.Fatalf("expected multi-match error, got %v", err)
	}
}

func TestAppKillKillFailure(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch args.(type) {
				case *goprocv1.ListRequest:
					resp := reply.(*goprocv1.ListResponse)
					resp.Procs = []*goprocv1.Proc{{Id: 5, Alive: true, Name: "proc"}}
					return nil
				case *goprocv1.KillRequest:
					return errors.New("kill failed")
				default:
					t.Fatalf("unexpected args %T", args)
				}
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	res, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{IDs: []int{5}},
		AllowAll:        true,
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err == nil || err.Error() != "no processes were killed (see output above)" {
		t.Fatalf("expected failure summary, got res=%+v err=%v", res, err)
	}
	if len(res.Events) != 1 || res.Events[0].Kind != "kill_failure" {
		t.Fatalf("unexpected events: %+v", res.Events)
	}
}

func TestAppKillRemoveFailure(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch args.(type) {
				case *goprocv1.ListRequest:
					resp := reply.(*goprocv1.ListResponse)
					resp.Procs = []*goprocv1.Proc{{Id: 6, Alive: true, Name: "proc"}}
					return nil
				case *goprocv1.KillRequest:
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
	res, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{IDs: []int{6}},
		AllowAll:        true,
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err == nil || err.Error() != "no processes were killed (see output above)" {
		t.Fatalf("expected failure summary, got %v", err)
	}
	if len(res.Events) != 1 || res.Events[0].Kind != "remove_failure" {
		t.Fatalf("unexpected events: %+v", res.Events)
	}
}

func TestAppKillSuccess(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch req := args.(type) {
				case *goprocv1.ListRequest:
					resp := reply.(*goprocv1.ListResponse)
					resp.Procs = []*goprocv1.Proc{{Id: 8, Alive: true, Name: "proc", Pid: 100}}
					if len(req.GetIds()) != 1 || req.GetIds()[0] != 8 {
						t.Fatalf("unexpected filter: %+v", req)
					}
					return nil
				case *goprocv1.KillRequest:
					if req.GetId() != 8 {
						t.Fatalf("expected kill id 8, got %d", req.GetId())
					}
					return nil
				case *goprocv1.RmRequest:
					if req.GetId() != 8 {
						t.Fatalf("expected rm id 8, got %d", req.GetId())
					}
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
	res, err := app.Kill(context.Background(), KillParams{
		Filters:         ListFilters{IDs: []int{8}},
		AllowAll:        true,
		Timeout:         time.Second,
		RequireSelector: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Successes != 1 || len(res.Events) != 1 || res.Events[0].Kind != "success" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

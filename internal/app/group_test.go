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

func TestAppGroupRequiresName(t *testing.T) {
	app := New(Options{})
	_, err := app.Group(context.Background(), GroupParams{Name: "  ", Timeout: time.Second})
	if err == nil || err.Error() != "group must not be empty" {
		t.Fatalf("expected name error, got %v", err)
	}
}

func TestAppGroupDaemonNotRunning(t *testing.T) {
	stubDaemon(t, false, nil)
	app := New(Options{})
	_, err := app.Group(context.Background(), GroupParams{Name: "ops", Timeout: time.Second})
	if err == nil || err.Error() != "daemon is not running" {
		t.Fatalf("expected daemon error, got %v", err)
	}
}

func TestAppGroupDialError(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("dial failed")
	})
	app := New(Options{})
	_, err := app.Group(context.Background(), GroupParams{Name: "ops", Timeout: time.Second})
	if err == nil || err.Error() != "connect to daemon: dial failed" {
		t.Fatalf("expected dial error, got %v", err)
	}
}

func TestAppGroupRenameError(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch args.(type) {
				case *goprocv1.RenameGroupRequest:
					return errors.New("rename failed")
				default:
					t.Fatalf("unexpected args %T", args)
				}
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	app := New(Options{})
	_, err := app.Group(context.Background(), GroupParams{Name: "old", Rename: "new", Timeout: time.Second})
	if err == nil || err.Error() != "daemon rename group RPC failed: rename failed" {
		t.Fatalf("expected rename error, got %v", err)
	}
}

func TestAppGroupNoMatches(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch args.(type) {
				case *goprocv1.ListRequest:
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
	res, err := app.Group(context.Background(), GroupParams{Name: "ops", Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Message != "No processes found with group \"ops\"" {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestAppGroupRenameAndListSuccess(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch req := args.(type) {
				case *goprocv1.RenameGroupRequest:
					if req.GetFrom() != "old" || req.GetTo() != "new" {
						t.Fatalf("unexpected rename req: %+v", req)
					}
					reply.(*goprocv1.RenameGroupResponse).Updated = 2
					return nil
				case *goprocv1.ListRequest:
					resp := reply.(*goprocv1.ListResponse)
					resp.Procs = []*goprocv1.Proc{
						{Id: 1, Pid: 100, Name: "svc", Groups: []string{"new"}},
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
	res, err := app.Group(context.Background(), GroupParams{Name: "old", Rename: "new", Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RenameInfo == nil || res.RenameInfo.From != "old" || res.RenameInfo.To != "new" || res.RenameInfo.Updated != 2 {
		t.Fatalf("unexpected rename info: %+v", res.RenameInfo)
	}
	if len(res.Processes) != 1 || res.Processes[0].Name != "svc" {
		t.Fatalf("unexpected processes: %+v", res.Processes)
	}
}

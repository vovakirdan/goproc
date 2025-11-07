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

func TestAppTagRequiresName(t *testing.T) {
	app := New(Options{})
	_, err := app.Tag(context.Background(), TagParams{Name: "  ", Timeout: time.Second})
	if err == nil || err.Error() != "tag must not be empty" {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestAppTagDaemonNotRunning(t *testing.T) {
	stubDaemon(t, false, nil)
	app := New(Options{})
	_, err := app.Tag(context.Background(), TagParams{Name: "db", Timeout: time.Second})
	if err == nil || err.Error() != "daemon is not running" {
		t.Fatalf("expected daemon error, got %v", err)
	}
}

func TestAppTagDialError(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("dial failed")
	})
	app := New(Options{})
	_, err := app.Tag(context.Background(), TagParams{Name: "cache", Timeout: time.Second})
	if err == nil || err.Error() != "connect to daemon: dial failed" {
		t.Fatalf("expected dial error, got %v", err)
	}
}

func TestAppTagRenameError(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch args.(type) {
				case *goprocv1.RenameTagRequest:
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
	_, err := app.Tag(context.Background(), TagParams{Name: "old", Rename: "new", Timeout: time.Second})
	if err == nil || err.Error() != "daemon rename tag RPC failed: rename failed" {
		t.Fatalf("expected rename error, got %v", err)
	}
}

func TestAppTagNoMatches(t *testing.T) {
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
	res, err := app.Tag(context.Background(), TagParams{Name: "web", Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Message != "No processes found with tag \"web\"" {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestAppTagRenameAndListSuccess(t *testing.T) {
	stubDaemon(t, true, func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				switch req := args.(type) {
				case *goprocv1.RenameTagRequest:
					if req.GetFrom() != "old" || req.GetTo() != "new" {
						t.Fatalf("unexpected rename request: %+v", req)
					}
					reply.(*goprocv1.RenameTagResponse).Updated = 3
					return nil
				case *goprocv1.ListRequest:
					resp := reply.(*goprocv1.ListResponse)
					resp.Procs = []*goprocv1.Proc{
						{Id: 1, Pid: 100, Cmd: "cmd", Name: "svc", Tags: []string{"new"}},
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
	res, err := app.Tag(context.Background(), TagParams{Name: "old", Rename: "new", Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RenameInfo == nil || res.RenameInfo.From != "old" || res.RenameInfo.To != "new" || res.RenameInfo.Updated != 3 {
		t.Fatalf("unexpected rename info: %+v", res.RenameInfo)
	}
	if len(res.Processes) != 1 || res.Processes[0].ID != 1 || res.Processes[0].Name != "svc" {
		t.Fatalf("unexpected processes: %+v", res.Processes)
	}
}

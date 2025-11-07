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

func TestAppListRejectsInvalidNameFilter(t *testing.T) {
	app := New(Options{})
	_, err := app.List(context.Background(), ListParams{
		Timeout: time.Second,
		Filters: ListFilters{Names: []string{"ok", " "}},
	})
	if err == nil || err.Error() != "name filters must not be empty" {
		t.Fatalf("expected name validation error, got %v", err)
	}
}

func TestAppListRejectsInvalidPIDFilter(t *testing.T) {
	app := New(Options{})
	_, err := app.List(context.Background(), ListParams{
		Timeout: time.Second,
		Filters: ListFilters{PIDs: []int{1, -2}},
	})
	if err == nil || err.Error() != "invalid pid filter: -2" {
		t.Fatalf("expected pid validation error, got %v", err)
	}
}

func TestAppListDaemonNotRunning(t *testing.T) {
	stubDaemon(t, false, nil)
	app := New(Options{})
	_, err := app.List(context.Background(), ListParams{
		Timeout: time.Second,
		Filters: ListFilters{},
	})
	if err == nil || err.Error() != "daemon is not running" {
		t.Fatalf("expected daemon not running error, got %v", err)
	}
}

func TestAppListDialError(t *testing.T) {
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		return nil, nil, errors.New("dial failed")
	})
	app := New(Options{})
	_, err := app.List(context.Background(), ListParams{
		Timeout: time.Second,
		Filters: ListFilters{},
	})
	if err == nil || err.Error() != "connect to daemon: dial failed" {
		t.Fatalf("expected dial error, got %v", err)
	}
}

func TestAppListSuccess(t *testing.T) {
	var captured *goprocv1.ListRequest
	stubDaemon(t, true, func(ctx context.Context) (goprocv1.GoProcClient, io.Closer, error) {
		conn := &fakeConn{
			invoke: func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
				req, ok := args.(*goprocv1.ListRequest)
				if !ok {
					t.Fatalf("unexpected args type %T", args)
				}
				captured = req
				resp := reply.(*goprocv1.ListResponse)
				resp.Procs = []*goprocv1.Proc{
					{
						Id:           11,
						Pid:          1234,
						Pgid:         22,
						Cmd:          "cmd",
						Alive:        true,
						Tags:         []string{"t1"},
						Groups:       []string{"g1"},
						Name:         "service",
						AddedAtUnix:  100,
						LastSeenUnix: 200,
					},
				}
				return nil
			},
		}
		return goprocv1.NewGoProcClient(conn), conn, nil
	})

	params := ListParams{
		Timeout: 750 * time.Millisecond,
		Filters: ListFilters{
			TagsAny:    []string{"a"},
			TagsAll:    []string{"b"},
			GroupsAny:  []string{"ga"},
			GroupsAll:  []string{"gb"},
			Names:      []string{"svc"},
			AliveOnly:  true,
			TextSearch: "search",
			PIDs:       []int{99},
			IDs:        []int{1},
		},
	}
	app := New(Options{})
	procs, err := app.List(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(procs) != 1 || procs[0].ID != 11 || procs[0].PID != 1234 || procs[0].Name != "service" {
		t.Fatalf("unexpected procs: %+v", procs)
	}
	if captured == nil {
		t.Fatal("expected captured request")
	}
	if captured.GetTextSearch() != params.Filters.TextSearch || !captured.GetAliveOnly() {
		t.Fatalf("filters not passed correctly: %+v", captured)
	}
}

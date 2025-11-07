package app

import (
	"context"
	"errors"
	"io"
	"testing"

	"google.golang.org/grpc"
	goprocv1 "goproc/api/proto/goproc/v1"
)

type fakeConn struct {
	invoke func(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error
}

func (f *fakeConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	if f.invoke != nil {
		return f.invoke(ctx, method, args, reply, opts...)
	}
	return nil
}

func (f *fakeConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeConn) Close() error { return nil }

func stubDaemon(t *testing.T, running bool, dial func(context.Context) (goprocv1.GoProcClient, io.Closer, error)) {
	t.Helper()
	resetDaemonDeps()
	daemonIsRunning = func() bool { return running }
	if dial == nil {
		dial = func(context.Context) (goprocv1.GoProcClient, io.Closer, error) {
			return nil, nil, errors.New("dial not stubbed")
		}
	}
	dialDaemonClient = dial
	t.Cleanup(resetDaemonDeps)
}

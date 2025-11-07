package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	goprocv1 "goproc/api/proto/goproc/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial opens a gRPC connection to the daemon over the UNIX socket.
func Dial(ctx context.Context) (goprocv1.GoProcClient, *grpc.ClientConn, error) {
	target := socketTarget()
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(unixDialer),
	)
	if err != nil {
		return nil, nil, err
	}
	conn.Connect()
	if err := waitForReady(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return goprocv1.NewGoProcClient(conn), conn, nil
}

func socketTarget() string {
	path := SocketPath()
	if trimmed, ok := strings.CutPrefix(path, "/"); ok {
		return "unix:///" + trimmed
	}
	return "unix://" + path
}

func unixDialer(ctx context.Context, addr string) (net.Conn, error) {
	if trimmed, ok := strings.CutPrefix(addr, "unix://"); ok {
		addr = trimmed
	}
	if addr == "" {
		addr = SocketPath()
	}
	var d net.Dialer
	return d.DialContext(ctx, "unix", addr)
}

func waitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	for {
		switch state := conn.GetState(); state {
		case connectivity.Ready:
			return nil
		case connectivity.Shutdown:
			return errors.New("grpc connection is shut down")
		default:
			if !conn.WaitForStateChange(ctx, state) {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("grpc connection stuck in state %s", state.String())
			}
		}
	}
}

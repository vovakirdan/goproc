package daemon

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"

	"google.golang.org/grpc"
)

// Server wraps the UNIX listener and gRPC server instance.
type Server struct {
	ln         net.Listener
	path       string
	grpcServer *grpc.Server
}

// Close stops the gRPC server and unlinks the socket.
func (s *Server) Close() error {
	var joined error

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.ln != nil {
		if err := s.ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			joined = errors.Join(joined, err)
		}
	}
	if s.path != "" {
		if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			joined = errors.Join(joined, err)
		}
	}
	if err := RemovePID(); err != nil {
		joined = errors.Join(joined, err)
	}
	return joined
}

// StartDaemon binds the UNIX socket and serves the gRPC API.
func StartDaemon() (*Server, error) {
	if err := EnsureRuntimeDir(); err != nil {
		return nil, err
	}
	path := SocketPath()

	if _, err := os.Stat(path); err == nil && !IsRunning() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}

	srv := &Server{
		ln:         ln,
		path:       path,
		grpcServer: grpc.NewServer(),
	}
	svc, err := newService()
	if err != nil {
		srv.Close()
		return nil, err
	}
	goprocv1.RegisterGoProcServer(srv.grpcServer, svc)

	if err := WritePID(os.Getpid()); err != nil {
		srv.Close()
		return nil, err
	}

	go srv.serve()
	return srv, nil
}

func (s *Server) serve() {
	if err := s.grpcServer.Serve(s.ln); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		log.Printf("gRPC server stopped: %v", err)
	}
}

// StopRunningDaemon sends a termination signal to the currently running daemon if any.
func StopRunningDaemon(force bool) error {
	pid, err := RunningPID()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if IsRunning() {
				return fmt.Errorf("daemon is running but PID file %q is missing; stop it manually", PIDPath())
			}
			return nil
		}
		return fmt.Errorf("unable to read daemon PID: %w", err)
	}
	if pid == os.Getpid() {
		return errors.New("refusing to stop current process")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := sendSignal(proc, syscall.SIGTERM); err != nil {
		return err
	}
	if waitForShutdown(3 * time.Second) {
		return nil
	}
	if !force {
		return fmt.Errorf("daemon process %d did not exit after SIGTERM", pid)
	}
	if err := sendSignal(proc, syscall.SIGKILL); err != nil {
		return err
	}
	if waitForShutdown(2 * time.Second) {
		return nil
	}
	return fmt.Errorf("daemon process %d did not exit after SIGKILL", pid)
}

func sendSignal(proc *os.Process, sig syscall.Signal) error {
	if err := proc.Signal(sig); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			_ = RemovePID()
			return nil
		}
		return err
	}
	return nil
}

func waitForShutdown(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !IsRunning() {
			_ = RemovePID()
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

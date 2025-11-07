package daemon

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

// Server wraps tje UNIX listener
type Server struct {
	ln   net.Listener
	path string
}

// Close stops the server and unlinks the socket
func (s *Server) Close() error {
	var err error
	if s.ln != nil {
		err = s.ln.Close()
		if err != nil {
			return err
		}
	}
	if s.path != "" {
		err = os.Remove(s.path)
		if err != nil {
			return err
		}
	}
	return RemovePID()
}

// StartDaemon binds the UNIX socket and serves a tiny PING handler
// Step 1: just heartbeat for now, no real process registry
func StartDaemon() (*Server, error) {
	EnsureRuntimeDir()
	path := SocketPath()

	// If stale socket file exists but daemon is not running, remove it
	if _, err := os.Stat(path); err == nil && !IsRunning() {
		err = os.Remove(path)
		if err != nil {
			return nil, err
		}
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	err = os.Chmod(path, 0o600)
	if err != nil {
		ln.Close()
		return nil, err
	}
	s := &Server{ln: ln, path: path}
	if err := WritePID(os.Getpid()); err != nil {
		s.Close()
		return nil, err
	}
	go s.serve()
	return s, nil
}

func (s *Server) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go handleConnection(c)
	}
}

func handleConnection(c net.Conn) {
	defer c.Close()
	br := bufio.NewReader(c)
	line, err := br.ReadString('\n')
	if err != nil {
		log.Printf("Error reading line: %v", err)
		return
	}
	req := strings.TrimSpace(line)
	switch req {
	case "PING":
		fmt.Fprint(c, "PONG\n")
	default:
		fmt.Fprintf(c, "ERROR: unknown request: %q\n", req)
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

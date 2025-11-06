package daemon

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"log"
)

// Server wraps tje UNIX listener
type Server struct {
	ln net.Listener
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
	return nil
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
		return nil, err
	}
	s := &Server{ln: ln, path: path}
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

// 
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

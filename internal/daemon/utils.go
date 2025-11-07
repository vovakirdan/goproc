package daemon

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SocetBaseName is the UNIX socket filename
const SocketBaseName = "goproc.sock"

const pidFileName = "goproc.pid"

// SocketPath returns the full path to the UNIX socket
// Order of precedence (first wins):
// 1) GOPROC_SOCKET (absolute path to socket)
// 2) if runtime=linux:
//   - GOPROC_RUNTIME_DIR or $XDG_RUNTIME_DIR or /run/user/<UID>
//     else (darwinm *bsd, etc):
//   - GOPROC_RUNTIME_DIR or /tmp
func SocketPath() string {
	if explicit := os.Getenv("GOPROC_SOCKET"); explicit != "" {
		return explicit
	}

	uid := currentUID()

	// Allow override of parent dir
	if rd := os.Getenv("GOPROC_RUNTIME_DIR"); rd != "" {
		return filepath.Join(rd, SocketBaseName)
	}

	if runtime.GOOS == "linux" {
		if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
			return filepath.Join(v, SocketBaseName)
		}
		// Linux per-user runtime dir
		return filepath.Join("/run/user", uid, SocketBaseName)
	}

	// macOS / BSD / other unix: keep it short to avoid sun_path length limit
	return filepath.Join("/tmp", "goproc-"+uid+".sock")
}

// EnsureRuntimeDir attempts to create the XDG_RUNTIME_DIR if it doesn't exist
func EnsureRuntimeDir() error {
	dir := filepath.Dir(SocketPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return nil
}

// PIDPath returns the full path to the PID file
func PIDPath() string {
	return filepath.Join(filepath.Dir(SocketPath()), pidFileName)
}

// WritePID stores the provided pid into the pid file
func WritePID(pid int) error {
	if err := EnsureRuntimeDir(); err != nil {
		return err
	}
	return os.WriteFile(PIDPath(), []byte(fmt.Sprintf("%d\n", pid)), 0o600)
}

// RemovePID removes the pid file if it exists
func RemovePID() error {
	if err := os.Remove(PIDPath()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

// RunningPID returns the pid stored in the pid file if any
func RunningPID() (int, error) {
	data, err := os.ReadFile(PIDPath())
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return pid, nil
}

// IsRunning tries to connect to the UNIX socket and returns true if the connection is successful
func IsRunning() bool { // todo: return bool, error
	var err error
	addr := SocketPath()
	var c net.Conn
	c, err = net.DialTimeout("unix", addr, 200*time.Millisecond)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		log.Printf("Error dialing UNIX socket: %v", err)
		return false
	}
	err = c.Close()
	if err != nil {
		log.Printf("Error closing connection: %v", err)
		return false
	}
	return true
}

func currentUID() string {
	u, err := user.Current()
	if err == nil && u != nil && u.Uid != "" {
		return u.Uid
	}
	return "0"
}

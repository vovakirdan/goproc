package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
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

// IsRunning tries to ping the daemon over gRPC and returns true if it responds.
func IsRunning() bool {
	if _, err := os.Stat(SocketPath()); err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	client, conn, err := Dial(ctx)
	if err != nil {
		return false
	}
	defer conn.Close()

	if _, err := client.Ping(ctx, &goprocv1.PingRequest{}); err != nil {
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

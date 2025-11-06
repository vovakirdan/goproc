package daemon

import (
	"net"
	"os"
	"os/user"
	"path/filepath"
	"time"
	"log"
)

// SocetBaseName is the UNIX socket filename
const SocketBaseName = "goproc.sock"

// SocketPath returns the full path to the UNIX socket
// $XDG_RUNTIME_DIR/goproc.sock, else /run/user/<UID>/goproc.sock
func SocketPath() string {
	if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
		return filepath.Join(v, SocketBaseName)
	}
	u, err := user.Current()
	uid := "0"
	if err != nil {
		return filepath.Join("/run/user", uid, SocketBaseName)
	}
	if u != nil && u.Uid != "" {
		uid = u.Uid
	}
	return filepath.Join("/run/user", uid, SocketBaseName)
}

// EnsureRuntimeDir attempts to create the XDG_RUNTIME_DIR if it doesn't exist
func EnsureRuntimeDir() error {
	if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
		return nil
	}
	dir := filepath.Dir(SocketPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return nil
}

// IsRunning tries to connect to the UNIX socket and returns true if the connection is successful
func IsRunning() bool { // todo: return bool, error
	err := EnsureRuntimeDir()
	if err != nil {
		log.Printf("Error ensuring runtime directory: %v", err)
		return false
	}
	addr := SocketPath()
	var c net.Conn
	c, err = net.DialTimeout("unix", addr, 200*time.Millisecond)
	if err != nil {
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

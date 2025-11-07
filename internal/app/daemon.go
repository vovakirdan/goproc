package app

import "goproc/internal/daemon"

// DaemonStatus represents current information about the daemon process.
type DaemonStatus struct {
	Running bool
	PID     int
}

// Status returns whether the daemon is running and its PID if known.
func (a *App) Status() (DaemonStatus, error) {
	if !daemon.IsRunning() {
		return DaemonStatus{Running: false}, nil
	}
	pid, err := daemon.RunningPID()
	if err != nil {
		return DaemonStatus{Running: true}, err
	}
	return DaemonStatus{Running: true, PID: pid}, nil
}

// StopDaemon attempts to stop the running daemon.
func (a *App) StopDaemon(force bool) error {
	return daemon.StopRunningDaemon(force)
}

// DaemonHandle holds a running daemon instance.
type DaemonHandle struct {
	srv *daemon.Server
}

// Close stops the running daemon instance.
func (h *DaemonHandle) Close() error {
	if h == nil || h.srv == nil {
		return nil
	}
	return h.srv.Close()
}

// StartDaemon starts the daemon and returns a handle for closing it.
func (a *App) StartDaemon() (*DaemonHandle, error) {
	srv, err := daemon.StartDaemon(a.cfgPath)
	if err != nil {
		return nil, err
	}
	return &DaemonHandle{srv: srv}, nil
}

package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"goproc/internal/app"
)

type stubController struct {
	pingFunc func(ctx context.Context, timeout time.Duration) (string, error)
}

func (s *stubController) Ping(ctx context.Context, timeout time.Duration) (string, error) {
	if s.pingFunc != nil {
		return s.pingFunc(ctx, timeout)
	}
	return "", errors.New("ping not implemented")
}

func (s *stubController) Add(ctx context.Context, params app.AddParams) (app.AddResult, error) {
	panic("Add not implemented")
}

func (s *stubController) List(ctx context.Context, params app.ListParams) ([]app.Process, error) {
	panic("List not implemented")
}

func (s *stubController) Remove(ctx context.Context, params app.RemoveParams) (app.RemoveResult, error) {
	panic("Remove not implemented")
}

func (s *stubController) Kill(ctx context.Context, params app.KillParams) (app.KillResult, error) {
	panic("Kill not implemented")
}

func (s *stubController) Tag(ctx context.Context, params app.TagParams) (app.TagResult, error) {
	panic("Tag not implemented")
}

func (s *stubController) Group(ctx context.Context, params app.GroupParams) (app.GroupResult, error) {
	panic("Group not implemented")
}

func (s *stubController) Reset(ctx context.Context, params app.ResetParams) error {
	panic("Reset not implemented")
}

func (s *stubController) Status() (app.DaemonStatus, error) {
	panic("Status not implemented")
}

func (s *stubController) StopDaemon(force bool) error {
	panic("StopDaemon not implemented")
}

func (s *stubController) StartDaemon() (*app.DaemonHandle, error) {
	panic("StartDaemon not implemented")
}

func withController(t *testing.T, stub controllerAPI) {
	t.Helper()
	origFactory := controllerFactory
	controllerFactory = func() controllerAPI {
		return stub
	}
	t.Cleanup(func() {
		controllerFactory = origFactory
	})
}

func withPingOutput(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	origOut := cmdPing.OutOrStdout()
	cmdPing.SetOut(buf)
	return buf, func() {
		cmdPing.SetOut(origOut)
	}
}

func TestPingSuccess(t *testing.T) {
	withController(t, &stubController{
		pingFunc: func(ctx context.Context, timeout time.Duration) (string, error) {
			if timeout != 2*time.Second {
				t.Fatalf("expected timeout 2s, got %v", timeout)
			}
			return "pong", nil
		},
	})
	buf, restore := withPingOutput(t)
	defer restore()

	oldTimeout := pingTimeoutSeconds
	pingTimeoutSeconds = 2
	t.Cleanup(func() { pingTimeoutSeconds = oldTimeout })

	if err := cmdPing.RunE(cmdPing, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if got := buf.String(); got != "pong\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestPingError(t *testing.T) {
	expected := errors.New("daemon down")
	withController(t, &stubController{
		pingFunc: func(ctx context.Context, timeout time.Duration) (string, error) {
			return "", expected
		},
	})
	oldTimeout := pingTimeoutSeconds
	pingTimeoutSeconds = 1
	t.Cleanup(func() { pingTimeoutSeconds = oldTimeout })

	err := cmdPing.RunE(cmdPing, nil)
	if !errors.Is(err, expected) {
		t.Fatalf("expected error %v, got %v", expected, err)
	}
}

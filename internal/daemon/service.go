package daemon

import (
	"context"
	"fmt"
	"syscall"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/registry"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// service implements the GoProc gRPC service backed by the registry.
type service struct {
	goprocv1.UnimplementedGoProcServer

	reg *registry.Registry
}

func newService() (*service, error) {
	reg, err := registry.New(SnapshotPath())
	if err != nil {
		return nil, err
	}
	return &service{reg: reg}, nil
}

func (s *service) Ping(ctx context.Context, _ *goprocv1.PingRequest) (*goprocv1.PingResponse, error) {
	return &goprocv1.PingResponse{Ok: "pong"}, nil
}

func (s *service) Add(ctx context.Context, req *goprocv1.AddRequest) (*goprocv1.AddResponse, error) {
	pid := int(req.GetPid())
	if pid <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pid must be positive")
	}

	if err := syscall.Kill(pid, 0); err != nil {
		return nil, status.Errorf(codes.NotFound, "pid %d not found or no permission: %v", pid, err)
	}

	id, err := s.reg.AddByPID(pid, pgidOf(pid), fmt.Sprintf("pid:%d", pid), nil, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add failed: %v", err)
	}
	return &goprocv1.AddResponse{Id: uint32(id)}, nil
}

func (s *service) List(ctx context.Context, _ *goprocv1.ListRequest) (*goprocv1.ListResponse, error) {
	ps := s.reg.List(registry.ListFilter{})
	resp := &goprocv1.ListResponse{
		Procs: make([]*goprocv1.Proc, 0, len(ps)),
	}
	for i := range ps {
		p := ps[i]
		resp.Procs = append(resp.Procs, &goprocv1.Proc{
			Id:    uint32(p.ID),
			Pid:   int32(p.PID),
			Pgid:  int32(p.PGID),
			Cmd:   p.Cmd,
			Alive: p.Alive,
		})
	}
	return resp, nil
}

func (s *service) Kill(ctx context.Context, req *goprocv1.KillRequest) (*goprocv1.KillResponse, error) {
	if req == nil || req.GetTarget() == nil {
		return nil, status.Error(codes.InvalidArgument, "target is required")
	}

	var pid, pgid int
	switch t := req.GetTarget().(type) {
	case *goprocv1.KillRequest_Id:
		proc, ok := s.reg.Get(registry.ProcID(t.Id))
		if !ok {
			return nil, status.Error(codes.NotFound, "id not found")
		}
		pid = proc.PID
		pgid = proc.PGID
	case *goprocv1.KillRequest_Pid:
		pid = int(t.Pid)
		pgid = pgidOf(pid)
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported target")
	}

	target := pid
	if pgid > 0 {
		target = -pgid
	}
	if err := syscall.Kill(target, syscall.SIGTERM); err != nil {
		return nil, status.Errorf(codes.Internal, "kill failed: %v", err)
	}
	return &goprocv1.KillResponse{}, nil
}

func (s *service) Rm(ctx context.Context, req *goprocv1.RmRequest) (*goprocv1.RmResponse, error) {
	if req.GetId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be provided")
	}
	if ok := s.reg.Remove(registry.ProcID(req.GetId())); !ok {
		return nil, status.Error(codes.NotFound, "id not found")
	}
	return &goprocv1.RmResponse{}, nil
}

func pgidOf(pid int) int {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return 0
	}
	return pgid
}

package daemon

import (
	"context"
	"fmt"
	"sync"

	goprocv1 "goproc/api/proto/goproc/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// service implements the GoProc gRPC service.
type service struct {
	goprocv1.UnimplementedGoProcServer

	mu     sync.Mutex
	nextID uint32
	procs  map[uint32]*goprocv1.Proc
}

// newService creates a new service instance backed by an in-memory registry.
func newService() *service {
	return &service{
		nextID: 1,
		procs:  make(map[uint32]*goprocv1.Proc),
	}
}

func (s *service) Ping(ctx context.Context, _ *goprocv1.PingRequest) (*goprocv1.PingResponse, error) {
	return &goprocv1.PingResponse{Ok: "pong"}, nil
}

func (s *service) Add(ctx context.Context, req *goprocv1.AddRequest) (*goprocv1.AddResponse, error) {
	if req.GetPid() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "pid must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++

	s.procs[id] = &goprocv1.Proc{
		Id:    id,
		Pid:   req.GetPid(),
		Pgid:  0,
		Cmd:   fmt.Sprintf("pid:%d", req.GetPid()),
		Alive: true,
	}

	return &goprocv1.AddResponse{Id: id}, nil
}

func (s *service) List(ctx context.Context, _ *goprocv1.ListRequest) (*goprocv1.ListResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	procs := make([]*goprocv1.Proc, 0, len(s.procs))
	for _, p := range s.procs {
		cloned, ok := proto.Clone(p).(*goprocv1.Proc)
		if !ok {
			return nil, status.Errorf(codes.Internal, "failed to clone process %d", p.GetId())
		}
		procs = append(procs, cloned)
	}

	return &goprocv1.ListResponse{Procs: procs}, nil
}

func (s *service) Kill(ctx context.Context, _ *goprocv1.KillRequest) (*goprocv1.KillResponse, error) {
	return nil, status.Error(codes.Unimplemented, "kill is not implemented yet")
}

func (s *service) Rm(ctx context.Context, req *goprocv1.RmRequest) (*goprocv1.RmResponse, error) {
	if req.GetId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be provided")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.procs[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "process %d not found", req.GetId())
	}

	delete(s.procs, req.GetId())
	return &goprocv1.RmResponse{}, nil
}

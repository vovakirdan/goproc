package daemon

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/config"
	"goproc/internal/registry"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// service implements the GoProc gRPC service backed by the registry.
type service struct {
	goprocv1.UnimplementedGoProcServer

	cfg    config.Config
	reg    *registry.Registry
	cancel context.CancelFunc
}

func newService(cfg config.Config) (*service, error) {
	reg, err := registry.New(SnapshotPath(), cfg.LastSeenUpdateInterval)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &service{
		cfg:    cfg,
		reg:    reg,
		cancel: cancel,
	}
	go s.watchLiveness(ctx)
	return s, nil
}

func (s *service) Close() {
	if s.cancel != nil {
		s.cancel()
	}
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

	id, existed, err := s.reg.AddByPID(pid, pgidOf(pid), fmt.Sprintf("pid:%d", pid), req.GetTags(), req.GetGroups())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add failed: %v", err)
	}
	if existed {
		return nil, status.Errorf(codes.AlreadyExists, "pid %d already registered as id %d", pid, id)
	}
	return &goprocv1.AddResponse{Id: uint64(id)}, nil
}

func (s *service) List(ctx context.Context, req *goprocv1.ListRequest) (*goprocv1.ListResponse, error) {
	filter := registry.ListFilter{
		TagsAny:    req.GetTagsAny(),
		TagsAll:    req.GetTagsAll(),
		GroupsAny:  req.GetGroupsAny(),
		GroupsAll:  req.GetGroupsAll(),
		AliveOnly:  req.GetAliveOnly(),
		TextSearch: req.GetTextSearch(),
	}
	if ids := req.GetIds(); len(ids) > 0 {
		filter.IDs = make([]registry.ProcID, 0, len(ids))
		for _, id := range ids {
			filter.IDs = append(filter.IDs, registry.ProcID(id))
		}
	}
	if pids := req.GetPids(); len(pids) > 0 {
		filter.PIDs = make([]int, 0, len(pids))
		for _, pid := range pids {
			filter.PIDs = append(filter.PIDs, int(pid))
		}
	}

	ps := s.reg.List(filter)
	resp := &goprocv1.ListResponse{
		Procs: make([]*goprocv1.Proc, 0, len(ps)),
	}
	for i := range ps {
		p := ps[i]
		resp.Procs = append(resp.Procs, &goprocv1.Proc{
			Id:           uint64(p.ID),
			Pid:          int32(p.PID),
			Pgid:         int32(p.PGID),
			Cmd:          p.Cmd,
			Alive:        p.Alive,
			Tags:         append([]string(nil), p.Meta.Tags...),
			Groups:       append([]string(nil), p.Meta.Groups...),
			AddedAtUnix:  p.AddedAt.Unix(),
			LastSeenUnix: p.LastSeen.Unix(),
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

func (s *service) RenameTag(ctx context.Context, req *goprocv1.RenameTagRequest) (*goprocv1.RenameTagResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	from := strings.TrimSpace(req.GetFrom())
	to := strings.TrimSpace(req.GetTo())
	if from == "" || to == "" {
		return nil, status.Error(codes.InvalidArgument, "from and to must be provided")
	}
	updated := s.reg.RenameTag(from, to)
	return &goprocv1.RenameTagResponse{Updated: uint32(updated)}, nil
}

func (s *service) RenameGroup(ctx context.Context, req *goprocv1.RenameGroupRequest) (*goprocv1.RenameGroupResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	from := strings.TrimSpace(req.GetFrom())
	to := strings.TrimSpace(req.GetTo())
	if from == "" || to == "" {
		return nil, status.Error(codes.InvalidArgument, "from and to must be provided")
	}
	updated := s.reg.RenameGroup(from, to)
	return &goprocv1.RenameGroupResponse{Updated: uint32(updated)}, nil
}

func (s *service) Reset(ctx context.Context, _ *goprocv1.ResetRequest) (*goprocv1.ResetResponse, error) {
	s.reg.Reset()
	return &goprocv1.ResetResponse{}, nil
}

func pgidOf(pid int) int {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return 0
	}
	return pgid
}

func (s *service) watchLiveness(ctx context.Context) {
	interval := s.cfg.LivenessInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshLiveness()
		}
	}
}

func (s *service) refreshLiveness() {
	procs := s.reg.List(registry.ListFilter{})
	for _, p := range procs {
		err := syscall.Kill(p.PID, 0)
		s.reg.SetAlive(p.ID, err == nil)
	}
}

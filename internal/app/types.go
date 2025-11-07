package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
)

// Process mirrors the daemon registry entry.
type Process struct {
	ID       uint64
	PID      int
	PGID     int
	Cmd      string
	Alive    bool
	Tags     []string
	Groups   []string
	Name     string
	AddedAt  time.Time
	LastSeen time.Time
}

func procFromProto(p *goprocv1.Proc) Process {
	return Process{
		ID:       p.GetId(),
		PID:      int(p.GetPid()),
		PGID:     int(p.GetPgid()),
		Cmd:      p.GetCmd(),
		Alive:    p.GetAlive(),
		Tags:     append([]string(nil), p.GetTags()...),
		Groups:   append([]string(nil), p.GetGroups()...),
		Name:     p.GetName(),
		AddedAt:  time.Unix(p.GetAddedAtUnix(), 0),
		LastSeen: time.Unix(p.GetLastSeenUnix(), 0),
	}
}

// ListFilters aggregates selectors shared across commands.
type ListFilters struct {
	TagsAny    []string
	TagsAll    []string
	GroupsAny  []string
	GroupsAll  []string
	Names      []string
	AliveOnly  bool
	TextSearch string
	PIDs       []int
	IDs        []int
}

func (f ListFilters) buildRequest() (*goprocv1.ListRequest, error) {
	req := &goprocv1.ListRequest{
		TagsAny:    append([]string(nil), f.TagsAny...),
		TagsAll:    append([]string(nil), f.TagsAll...),
		GroupsAny:  append([]string(nil), f.GroupsAny...),
		GroupsAll:  append([]string(nil), f.GroupsAll...),
		AliveOnly:  f.AliveOnly,
		TextSearch: f.TextSearch,
	}

	if names := f.Names; len(names) > 0 {
		req.Names = make([]string, 0, len(names))
		for _, name := range names {
			clean := strings.TrimSpace(name)
			if clean == "" {
				return nil, errors.New("name filters must not be empty")
			}
			req.Names = append(req.Names, clean)
		}
	}
	if pids := f.PIDs; len(pids) > 0 {
		req.Pids = make([]int32, 0, len(pids))
		for _, pid := range pids {
			if pid <= 0 {
				return nil, fmt.Errorf("invalid pid filter: %d", pid)
			}
			req.Pids = append(req.Pids, int32(pid))
		}
	}
	if ids := f.IDs; len(ids) > 0 {
		req.Ids = make([]uint64, 0, len(ids))
		for _, id := range ids {
			if id <= 0 {
				return nil, fmt.Errorf("invalid id filter: %d", id)
			}
			req.Ids = append(req.Ids, uint64(id))
		}
	}

	return req, nil
}

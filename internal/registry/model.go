package registry

import "time"

// ProcID is an internal stable identifier for tracked processes.
type ProcID uint64

// ProcMeta holds user-defined labels.
type ProcMeta struct {
	Tags   []string `json:"tags,omitempty"`   // arbitrary labels
	Groups []string `json:"groups,omitempty"` // used for bulk-ops (kill/list)
}

// Proc holds a tracked process entry. It is immutable outside registry methods.
type Proc struct {
	ID       ProcID    `json:"id"`
	PID      int       `json:"pid"`
	PGID     int       `json:"pgid"`
	Cmd      string    `json:"cmd"`
	Name     string    `json:"name"`
	Alive    bool      `json:"alive"`
	AddedAt  time.Time `json:"added_at"`
	LastSeen time.Time `json:"last_seen"`
	Meta     ProcMeta  `json:"meta"`
}

// ListFilter allows narrowing the registry query.
type ListFilter struct {
	TagsAny    []string // include if has ANY of these tags
	TagsAll    []string // include if has ALL of these tags
	GroupsAny  []string // include if in ANY of these groups
	GroupsAll  []string // include if in ALL of these groups
	AliveOnly  bool
	PIDs       []int
	IDs        []ProcID
	Names      []string
	TextSearch string // naive substring search over Cmd
}

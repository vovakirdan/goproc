package registry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Snapshot schema versioning for forward-compatibility.
const snapshotVersion = 1

type snapshot struct {
	Version int    `json:"version"`
	NextID  uint64 `json:"next_id"`
	Procs   []Proc `json:"procs"`
	Created int64  `json:"created_unix"`
}

func (r *Registry) loadSnapshot(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var s snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s.Version != snapshotVersion {
		// Future migrations can be implemented here; for now we accept the data.
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID = ProcID(s.NextID)
	r.byID = make(map[ProcID]*Proc)
	r.byPID = make(map[int]ProcID)
	r.byTag = make(map[string]map[ProcID]struct{})
	r.byGroup = make(map[string]map[ProcID]struct{})

	for i := range s.Procs {
		p := s.Procs[i]
		// Store pointer to copy to avoid referencing slice backing array.
		proc := p
		r.byID[proc.ID] = &proc
		r.byPID[proc.PID] = proc.ID
		for _, t := range proc.Meta.Tags {
			if _, ok := r.byTag[t]; !ok {
				r.byTag[t] = make(map[ProcID]struct{})
			}
			r.byTag[t][proc.ID] = struct{}{}
		}
		for _, g := range proc.Meta.Groups {
			if _, ok := r.byGroup[g]; !ok {
				r.byGroup[g] = make(map[ProcID]struct{})
			}
			r.byGroup[g][proc.ID] = struct{}{}
		}
	}
	return nil
}

func (r *Registry) saveSnapshot(path string) error {
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	r.mu.RLock()
	s := snapshot{
		Version: snapshotVersion,
		NextID:  uint64(r.nextID),
		Created: now().Unix(),
	}
	s.Procs = make([]Proc, 0, len(r.byID))
	for _, p := range r.byID {
		s.Procs = append(s.Procs, *p)
	}
	r.mu.RUnlock()

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

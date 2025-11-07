package registry

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// Registry is a threadsafe in-memory catalog of tracked processes.
// Secondary indexes enable cheap queries by PID, tag, and group.
type Registry struct {
	mu      sync.RWMutex
	nextID  ProcID
	byID    map[ProcID]*Proc
	byPID   map[int]ProcID
	byTag   map[string]map[ProcID]struct{}
	byGroup map[string]map[ProcID]struct{}
	// Interval between persisted lastSeen bumps while a process remains alive.
	lastSeenInterval time.Duration

	// Where to snapshot. If empty, snapshotting is disabled.
	SnapshotPath string
}

// New loads snapshot if present and returns a ready registry.
func New(snapshotPath string, lastSeenInterval time.Duration) (*Registry, error) {
	if lastSeenInterval <= 0 {
		lastSeenInterval = 30 * time.Second
	}
	r := &Registry{
		nextID:           1,
		byID:             make(map[ProcID]*Proc),
		byPID:            make(map[int]ProcID),
		byTag:            make(map[string]map[ProcID]struct{}),
		byGroup:          make(map[string]map[ProcID]struct{}),
		SnapshotPath:     snapshotPath,
		lastSeenInterval: lastSeenInterval,
	}
	if snapshotPath != "" {
		if err := r.loadSnapshot(snapshotPath); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// AddByPID registers an existing process. Returns the ID plus a flag indicating whether it already existed.
func (r *Registry) AddByPID(pid, pgid int, cmd string, tags, groups []string) (ProcID, bool, error) {
	if pid <= 0 {
		return 0, false, errors.New("pid must be > 0")
	}

	r.mu.Lock()
	if id, ok := r.byPID[pid]; ok {
		r.mu.Unlock()
		return id, true, nil
	}
	id := r.nextID
	r.nextID++

	p := &Proc{
		ID:       id,
		PID:      pid,
		PGID:     pgid,
		Cmd:      cmd,
		Alive:    true, // optimistic; can be updated by watcher later
		AddedAt:  now(),
		LastSeen: now(),
		Meta:     ProcMeta{Tags: norm(tags), Groups: norm(groups)},
	}
	r.byID[id] = p
	r.byPID[pid] = id
	for _, t := range p.Meta.Tags {
		if _, ok := r.byTag[t]; !ok {
			r.byTag[t] = make(map[ProcID]struct{})
		}
		r.byTag[t][id] = struct{}{}
	}
	for _, g := range p.Meta.Groups {
		if _, ok := r.byGroup[g]; !ok {
			r.byGroup[g] = make(map[ProcID]struct{})
		}
		r.byGroup[g][id] = struct{}{}
	}
	r.mu.Unlock()

	r.maybeSave()
	return id, false, nil
}

// Tag adds tags to the proc metadata atomically.
func (r *Registry) Tag(id ProcID, add []string) error {
	r.mu.Lock()
	p := r.byID[id]
	if p == nil {
		r.mu.Unlock()
		return osErrNotFound(id)
	}
	old := toSet(p.Meta.Tags)
	for _, t := range norm(add) {
		if _, ok := old[t]; ok {
			continue
		}
		old[t] = struct{}{}
		if _, ok := r.byTag[t]; !ok {
			r.byTag[t] = make(map[ProcID]struct{})
		}
		r.byTag[t][id] = struct{}{}
	}
	p.Meta.Tags = setToSlice(old)
	r.mu.Unlock()

	r.maybeSave()
	return nil
}

func (r *Registry) Untag(id ProcID, remove []string) error {
	r.mu.Lock()
	p := r.byID[id]
	if p == nil {
		r.mu.Unlock()
		return osErrNotFound(id)
	}
	rm := toSet(norm(remove))
	newSet := make(map[string]struct{})
	for _, t := range p.Meta.Tags {
		if _, kill := rm[t]; kill {
			delete(r.byTag[t], id)
			continue
		}
		newSet[t] = struct{}{}
	}
	p.Meta.Tags = setToSlice(newSet)
	r.mu.Unlock()

	r.maybeSave()
	return nil
}

func (r *Registry) GroupAssign(id ProcID, groups []string) error {
	r.mu.Lock()
	p := r.byID[id]
	if p == nil {
		r.mu.Unlock()
		return osErrNotFound(id)
	}
	old := toSet(p.Meta.Groups)
	for _, g := range norm(groups) {
		if _, ok := old[g]; ok {
			continue
		}
		old[g] = struct{}{}
		if _, ok := r.byGroup[g]; !ok {
			r.byGroup[g] = make(map[ProcID]struct{})
		}
		r.byGroup[g][id] = struct{}{}
	}
	p.Meta.Groups = setToSlice(old)
	r.mu.Unlock()

	r.maybeSave()
	return nil
}

func (r *Registry) GroupRemove(id ProcID, groups []string) error {
	r.mu.Lock()
	p := r.byID[id]
	if p == nil {
		r.mu.Unlock()
		return osErrNotFound(id)
	}
	rm := toSet(norm(groups))
	newSet := make(map[string]struct{})
	for _, g := range p.Meta.Groups {
		if _, kill := rm[g]; kill {
			delete(r.byGroup[g], id)
			continue
		}
		newSet[g] = struct{}{}
	}
	p.Meta.Groups = setToSlice(newSet)
	r.mu.Unlock()

	r.maybeSave()
	return nil
}

// Remove deletes an entry by ID.
func (r *Registry) Remove(id ProcID) bool {
	r.mu.Lock()
	p := r.byID[id]
	if p == nil {
		r.mu.Unlock()
		return false
	}
	delete(r.byID, id)
	delete(r.byPID, p.PID)
	for _, t := range p.Meta.Tags {
		delete(r.byTag[t], id)
		if len(r.byTag[t]) == 0 {
			delete(r.byTag, t)
		}
	}
	for _, g := range p.Meta.Groups {
		delete(r.byGroup[g], id)
		if len(r.byGroup[g]) == 0 {
			delete(r.byGroup, g)
		}
	}
	r.mu.Unlock()

	r.maybeSave()
	return true
}

// SetAlive updates alive flag (and occasionally lastSeen) for the given process.
func (r *Registry) SetAlive(id ProcID, alive bool) bool {
	r.mu.Lock()

	p := r.byID[id]
	if p == nil {
		r.mu.Unlock()
		return false
	}

	changed := false
	if p.Alive != alive {
		p.Alive = alive
		changed = true
	}
	if alive {
		now := now()
		if p.LastSeen.IsZero() || now.Sub(p.LastSeen) >= r.lastSeenInterval {
			p.LastSeen = now
			changed = true
		}
	}
	r.mu.Unlock()

	if changed {
		r.maybeSave()
	}
	return changed
}

// RenameTag renames a tag across all processes and returns affected count.
func (r *Registry) RenameTag(from, to string) int {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" || from == to {
		return 0
	}

	r.mu.Lock()
	count := r.renameTagLocked(from, to)
	r.mu.Unlock()

	if count > 0 {
		r.maybeSave()
	}
	return count
}

func (r *Registry) renameTagLocked(from, to string) int {
	ids := r.byTag[from]
	if len(ids) == 0 {
		return 0
	}
	if _, ok := r.byTag[to]; !ok {
		r.byTag[to] = make(map[ProcID]struct{})
	}
	count := 0
	for id := range ids {
		p := r.byID[id]
		if p == nil {
			continue
		}
		set := toSet(p.Meta.Tags)
		if _, ok := set[from]; !ok {
			continue
		}
		delete(set, from)
		set[to] = struct{}{}
		p.Meta.Tags = setToSlice(set)
		delete(r.byTag[from], id)
		r.byTag[to][id] = struct{}{}
		count++
	}
	if len(r.byTag[from]) == 0 {
		delete(r.byTag, from)
	}
	return count
}

// RenameGroup renames a group label across all processes and returns affected count.
func (r *Registry) RenameGroup(from, to string) int {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" || from == to {
		return 0
	}

	r.mu.Lock()
	count := r.renameGroupLocked(from, to)
	r.mu.Unlock()

	if count > 0 {
		r.maybeSave()
	}
	return count
}

func (r *Registry) renameGroupLocked(from, to string) int {
	ids := r.byGroup[from]
	if len(ids) == 0 {
		return 0
	}
	if _, ok := r.byGroup[to]; !ok {
		r.byGroup[to] = make(map[ProcID]struct{})
	}
	count := 0
	for id := range ids {
		p := r.byID[id]
		if p == nil {
			continue
		}
		set := toSet(p.Meta.Groups)
		if _, ok := set[from]; !ok {
			continue
		}
		delete(set, from)
		set[to] = struct{}{}
		p.Meta.Groups = setToSlice(set)
		delete(r.byGroup[from], id)
		r.byGroup[to][id] = struct{}{}
		count++
	}
	if len(r.byGroup[from]) == 0 {
		delete(r.byGroup, from)
	}
	return count
}

// Reset clears the registry and resets the ID counter.
func (r *Registry) Reset() {
	r.mu.Lock()
	r.nextID = 1
	r.byID = make(map[ProcID]*Proc)
	r.byPID = make(map[int]ProcID)
	r.byTag = make(map[string]map[ProcID]struct{})
	r.byGroup = make(map[string]map[ProcID]struct{})
	r.mu.Unlock()

	r.maybeSave()
}

// Get returns a copy of a Proc by ID.
func (r *Registry) Get(id ProcID) (Proc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p := r.byID[id]
	if p == nil {
		return Proc{}, false
	}
	cp := *p
	return cp, true
}

// List returns matching processes, sorted by ID asc.
func (r *Registry) List(f ListFilter) []Proc {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]ProcID, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}

	if len(f.IDs) > 0 {
		set := make(map[ProcID]struct{}, len(f.IDs))
		for _, id := range f.IDs {
			set[id] = struct{}{}
		}
		ids = filterIDs(ids, func(id ProcID) bool {
			_, ok := set[id]
			return ok
		})
	}
	if len(f.PIDs) > 0 {
		pidSet := make(map[int]struct{}, len(f.PIDs))
		for _, p := range f.PIDs {
			pidSet[p] = struct{}{}
		}
		ids = filterIDs(ids, func(id ProcID) bool {
			_, ok := pidSet[r.byID[id].PID]
			return ok
		})
	}

	if len(f.TagsAny) > 0 {
		tagSet := make(map[ProcID]struct{})
		for _, t := range f.TagsAny {
			for id := range r.byTag[t] {
				tagSet[id] = struct{}{}
			}
		}
		ids = filterIDs(ids, func(id ProcID) bool {
			_, ok := tagSet[id]
			return ok
		})
	}
	if len(f.TagsAll) > 0 {
		ids = filterIDs(ids, func(id ProcID) bool {
			p := r.byID[id]
			have := toSet(p.Meta.Tags)
			for _, t := range f.TagsAll {
				if _, ok := have[t]; !ok {
					return false
				}
			}
			return true
		})
	}

	if len(f.GroupsAny) > 0 {
		groupSet := make(map[ProcID]struct{})
		for _, g := range f.GroupsAny {
			for id := range r.byGroup[g] {
				groupSet[id] = struct{}{}
			}
		}
		ids = filterIDs(ids, func(id ProcID) bool {
			_, ok := groupSet[id]
			return ok
		})
	}
	if len(f.GroupsAll) > 0 {
		ids = filterIDs(ids, func(id ProcID) bool {
			p := r.byID[id]
			have := toSet(p.Meta.Groups)
			for _, g := range f.GroupsAll {
				if _, ok := have[g]; !ok {
					return false
				}
			}
			return true
		})
	}

	if f.AliveOnly {
		ids = filterIDs(ids, func(id ProcID) bool {
			return r.byID[id].Alive
		})
	}

	if s := strings.TrimSpace(f.TextSearch); s != "" {
		ids = filterIDs(ids, func(id ProcID) bool {
			return strings.Contains(r.byID[id].Cmd, s)
		})
	}

	out := make([]Proc, 0, len(ids))
	for _, id := range ids {
		cp := *r.byID[id]
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// maybeSave performs a best-effort snapshot write if a path is configured.
func (r *Registry) maybeSave() {
	if r.SnapshotPath == "" {
		return
	}
	if err := r.saveSnapshot(r.SnapshotPath); err != nil {
		log.Printf("registry snapshot failed: %v", err)
	}
}

// --- helpers ---

func norm(xs []string) []string {
	out := make([]string, 0, len(xs))
	seen := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

func toSet(xs []string) map[string]struct{} {
	m := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		m[x] = struct{}{}
	}
	return m
}

func setToSlice(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func filterIDs(ids []ProcID, keep func(ProcID) bool) []ProcID {
	dst := ids[:0]
	for _, id := range ids {
		if keep(id) {
			dst = append(dst, id)
		}
	}
	return dst
}

func osErrNotFound(id ProcID) error {
	return fmt.Errorf("proc %d not found", id)
}

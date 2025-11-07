package registry

import "time"

type strset map[string]struct{}

func (s strset) add(v string) {
	s[v] = struct{}{}
}

func (s strset) has(v string) bool {
	_, ok := s[v]
	return ok
}

func now() time.Time {
	return time.Now().UTC()
}

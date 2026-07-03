package service

import "sort"

type activeShowsTracker struct {
	counts map[string]int
	names  []string
}

func newActiveShowsTracker() *activeShowsTracker {
	return &activeShowsTracker{counts: make(map[string]int)}
}

func (t *activeShowsTracker) start(name string) {
	if name == "" {
		return
	}
	t.counts[name]++
	t.rebuild()
}

func (t *activeShowsTracker) done(name string) {
	if name == "" {
		return
	}
	if t.counts[name] <= 1 {
		delete(t.counts, name)
	} else {
		t.counts[name]--
	}
	t.rebuild()
}

func (t *activeShowsTracker) rebuild() {
	names := make([]string, 0, len(t.counts))
	for name := range t.counts {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 8 {
		names = names[:8]
	}
	t.names = names
}

func (t *activeShowsTracker) list() []string {
	if len(t.names) == 0 {
		return nil
	}
	out := make([]string, len(t.names))
	copy(out, t.names)
	return out
}

func (t *activeShowsTracker) primary() string {
	if len(t.names) == 0 {
		return ""
	}
	return t.names[0]
}

func applyActiveShows(state *migrateState, tracker *activeShowsTracker) {
	if tracker == nil {
		return
	}
	state.progress.ActiveShows = tracker.list()
}

package client

import (
	"sync"
	"time"
)

const eventCap = 100

type Event struct {
	Time  int64  `json:"time"`
	Dir   string `json:"dir"`
	Kind  string `json:"kind"`
	Peer  string `json:"peer"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	State string `json:"state"`
	FID   string `json:"fid,omitempty"`
	Pct   int    `json:"pct,omitempty"`
	Done  int64  `json:"done,omitempty"`
}

type Store struct {
	mu      sync.Mutex
	events  []*Event
	pending map[string]*Event
	connUp  bool
}

func NewStore() *Store {
	return &Store{pending: make(map[string]*Event)}
}

func (s *Store) SetConnected(up bool) {
	s.mu.Lock()
	s.connUp = up
	s.mu.Unlock()
}

func (s *Store) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connUp
}

func (s *Store) Add(e *Event) {
	e.Time = time.Now().UnixMilli()
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.FID != "" && e.State != StateDone {
		s.pending[e.FID] = e
	}
	if e.State == StateDone || e.State == StateFailed {
		delete(s.pending, e.FID)
	}
	s.events = append(s.events, e)
	if len(s.events) > eventCap {
		s.events = s.events[len(s.events)-eventCap:]
	}
}

const (
	StateDone     = "done"
	StateFailed   = "failed"
	StateOffered  = "offered"
	StateSending  = "sending"
	StateReceived = "received"
)

func (s *Store) Snapshot(events bool) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]any{"connected": s.connUp}
	if !events {
		return out
	}
	evs := make([]*Event, len(s.events))
	for i, e := range s.events {
		c := *e
		evs[i] = &c
	}
	for i, j := 0, len(evs)-1; i < j; i, j = i+1, j-1 {
		evs[i], evs[j] = evs[j], evs[i]
	}
	out["events"] = evs
	return out
}

func (s *Store) UpdateProgress(fid string, done, total int64, pctOverride ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.pending[fid]; ok {
		if len(pctOverride) > 0 {
			e.Pct = pctOverride[0]
		} else if total > 0 {
			e.Pct = int(done * 100 / total)
		}
		e.Done = done
	}
}

func (s *Store) Mark(fid, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.pending[fid]; ok {
		e.State = state
		delete(s.pending, fid)
	}
}

func (s *Store) UpdateEvent(e *Event, done, total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if total > 0 {
		e.Pct = int(done * 100 / total)
		e.Done = done
	}
}

func (s *Store) SetState(e *Event, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.State = state
}

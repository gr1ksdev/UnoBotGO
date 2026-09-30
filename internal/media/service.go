// Package media caches Telegram photos independently from ranking reads.
package media

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type Source interface {
	Photo(context.Context, string, int64) ([]byte, string, error)
}
type key struct {
	kind string
	id   int64
}
type entry struct {
	key     key
	data    []byte
	mime    string
	expires time.Time
	pending bool
}
type Service struct {
	mu      sync.Mutex
	source  Source
	entries map[key]*list.Element
	lru     *list.List
	jobs    chan key
	bytes   int
	now     func() time.Time
}

func New(ctx context.Context, source Source) *Service {
	s := &Service{source: source, entries: map[key]*list.Element{}, lru: list.New(), jobs: make(chan key, 64), now: time.Now}
	// One shared ticker bounds requests even with four concurrent workers.
	ticker := time.NewTicker(350 * time.Millisecond)
	go func() { <-ctx.Done(); ticker.Stop() }()
	for i := 0; i < 4; i++ {
		go s.work(ctx, ticker.C)
	}
	return s
}
func (s *Service) Get(kind string, id int64) ([]byte, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := key{kind, id}
	if e := s.entries[k]; e != nil {
		v := e.Value.(*entry)
		s.lru.MoveToFront(e)
		if v.pending || v.expires.After(s.now()) {
			return v.data, v.mime, v.pending
		}
		s.remove(e)
	}
	select {
	case s.jobs <- k:
		e := s.lru.PushFront(&entry{key: k, pending: true})
		s.entries[k] = e
		s.trim()
		return nil, "", true
	default:
		return nil, "", true
	}
}
func (s *Service) remove(e *list.Element) {
	v := e.Value.(*entry)
	s.bytes -= len(v.data)
	delete(s.entries, v.key)
	s.lru.Remove(e)
}
func (s *Service) trim() {
	for len(s.entries) > 2000 || s.bytes > 64<<20 {
		s.remove(s.lru.Back())
	}
}
func (s *Service) work(ctx context.Context, ticks <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case k := <-s.jobs:
			select {
			case <-ctx.Done():
				return
			case <-ticks:
			}
			req, cancel := context.WithTimeout(ctx, 10*time.Second)
			data, mime, err := s.source.Photo(req, k.kind, k.id)
			cancel()
			ttl := 6 * time.Hour
			if len(data) == 0 {
				ttl = time.Hour
			}
			if err != nil {
				data = nil
				ttl = time.Minute
			}
			s.mu.Lock()
			if e := s.entries[k]; e != nil {
				v := e.Value.(*entry)
				s.bytes -= len(v.data)
				v.data = data
				v.mime = mime
				v.pending = false
				v.expires = s.now().Add(ttl)
				s.bytes += len(data)
				s.trim()
			}
			s.mu.Unlock()
		}
	}
}

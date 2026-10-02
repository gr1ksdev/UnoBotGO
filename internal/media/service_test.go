package media

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockSource struct {
	calls atomic.Int64
	fn    func(ctx context.Context, kind string, id int64) ([]byte, string, error)
}

func (m *mockSource) Photo(ctx context.Context, kind string, id int64) ([]byte, string, error) {
	m.calls.Add(1)
	if m.fn != nil {
		return m.fn(ctx, kind, id)
	}
	return []byte("fake-photo"), "image/png", nil
}

func TestMediaService_LifecycleAndCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := &mockSource{
		fn: func(_ context.Context, kind string, id int64) ([]byte, string, error) {
			if id == 404 {
				return nil, "", nil // missing photo
			}
			if id == 500 {
				return nil, "", errors.New("upstream failure")
			}
			return []byte("avatar-bytes"), "image/jpeg", nil
		},
	}

	currentTime := time.Now()
	s := New(ctx, mock)
	s.now = func() time.Time { return currentTime }

	// First call: pending
	data, mime, pending := s.Get("user", 1)
	if !pending || data != nil || mime != "" {
		t.Fatalf("expected pending on first request, got pending=%v, data=%v, mime=%q", pending, data, mime)
	}

	// Wait for worker to fetch with timeout
	deadline := time.Now().Add(2 * time.Second)
	var loaded bool
	for time.Now().Before(deadline) {
		data, mime, pending = s.Get("user", 1)
		if !pending && len(data) > 0 {
			loaded = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !loaded {
		t.Fatalf("expected photo to load, got pending=%v, data=%v", pending, data)
	}
	if string(data) != "avatar-bytes" || mime != "image/jpeg" {
		t.Fatalf("unexpected data: %q, mime: %q", string(data), mime)
	}

	// Repeated call should hit cache without calling source again
	prevCalls := mock.calls.Load()
	data2, mime2, pending2 := s.Get("user", 1)
	if pending2 || string(data2) != "avatar-bytes" || mime2 != "image/jpeg" {
		t.Fatalf("cache hit failed")
	}
	if mock.calls.Load() != prevCalls {
		t.Fatalf("expected no extra calls to source, had %d, now %d", prevCalls, mock.calls.Load())
	}

	// Test negative cache (missing photo)
	_, _, pending = s.Get("user", 404)
	if !pending {
		t.Fatal("expected pending for 404 on first request")
	}
	deadline = time.Now().Add(2 * time.Second)
	loaded = false
	for time.Now().Before(deadline) {
		data, mime, pending = s.Get("user", 404)
		if !pending {
			loaded = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !loaded || len(data) > 0 {
		t.Fatalf("expected negative cache (empty data, pending=false), got pending=%v, data=%v", pending, data)
	}

	// Test error caching (500)
	_, _, pending = s.Get("user", 500)
	if !pending {
		t.Fatal("expected pending for 500 on first request")
	}
	deadline = time.Now().Add(2 * time.Second)
	loaded = false
	for time.Now().Before(deadline) {
		data, mime, pending = s.Get("user", 500)
		if !pending {
			loaded = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !loaded || len(data) > 0 {
		t.Fatalf("expected error cache (empty data, pending=false), got pending=%v, data=%v", pending, data)
	}

	// Test TTL expiration
	currentTime = currentTime.Add(7 * time.Hour)
	_, _, pending = s.Get("user", 1)
	if !pending {
		t.Fatalf("expected entry to be evicted/re-requested after 7 hours")
	}
}

func TestMediaService_LRUTrim(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := &mockSource{
		fn: func(_ context.Context, _ string, _ int64) ([]byte, string, error) {
			return make([]byte, 1024), "image/png", nil
		},
	}
	s := New(ctx, mock)

	// Manually populate cache to test trim logic
	s.mu.Lock()
	for i := int64(1); i <= 2010; i++ {
		k := key{kind: "user", id: i}
		e := s.lru.PushFront(&entry{
			key:     k,
			data:    make([]byte, 10),
			mime:    "image/png",
			expires: time.Now().Add(time.Hour),
		})
		s.entries[k] = e
		s.bytes += 10
	}
	s.trim()
	count := len(s.entries)
	s.mu.Unlock()

	if count > 2000 {
		t.Fatalf("expected trim to reduce entries <= 2000, got %d", count)
	}
}

func TestMediaService_Concurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := &mockSource{
		fn: func(_ context.Context, _ string, _ int64) ([]byte, string, error) {
			time.Sleep(2 * time.Millisecond)
			return []byte("data"), "image/png", nil
		},
	}
	s := New(ctx, mock)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				s.Get("user", id%5)
			}
		}(int64(i))
	}
	wg.Wait()
}

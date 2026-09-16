package telegram

import (
	"sync"
	"time"
)

type updateDeduper struct {
	mu      sync.Mutex
	seen    map[int]time.Time
	pending map[int]time.Time
	now     func() time.Time
	ttl     time.Duration
	limit   int
}

func newUpdateDeduper(now func() time.Time) *updateDeduper {
	if now == nil {
		now = time.Now
	}
	return &updateDeduper{seen: make(map[int]time.Time), pending: make(map[int]time.Time), now: now, ttl: 24 * time.Hour, limit: 10000}
}

func (d *updateDeduper) reserve(id int) bool {
	if id <= 0 {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cleanupLocked()
	if _, ok := d.seen[id]; ok {
		return false
	}
	if _, ok := d.pending[id]; ok {
		return false
	}
	d.pending[id] = d.now()
	return true
}
func (d *updateDeduper) commit(id int) {
	if id <= 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.pending, id)
	d.seen[id] = d.now()
	d.trimLocked()
}
func (d *updateDeduper) release(id int) {
	if id <= 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.pending, id)
}
func (d *updateDeduper) cleanupLocked() {
	cutoff := d.now().Add(-d.ttl)
	for id, t := range d.seen {
		if t.Before(cutoff) {
			delete(d.seen, id)
		}
	}
	for id, t := range d.pending {
		if t.Before(cutoff) {
			delete(d.pending, id)
		}
	}
}
func (d *updateDeduper) trimLocked() {
	for len(d.seen) > d.limit {
		var oldestID int
		var oldest time.Time
		for id, t := range d.seen {
			if oldestID == 0 || t.Before(oldest) {
				oldestID = id
				oldest = t
			}
		}
		delete(d.seen, oldestID)
	}
}

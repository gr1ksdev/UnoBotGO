package game

import (
	"context"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
	"sync"
	"time"
)

// Subscribe coalesces invalidations. Every subscriber recovers an authoritative
// projection, so missed/coalesced events cannot lose state or reveal hidden hands.
func (s *Service) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.manager.indexMu.Lock()
	s.manager.subscribers[ch] = struct{}{}
	s.manager.indexMu.Unlock()
	return ch, func() { s.manager.indexMu.Lock(); delete(s.manager.subscribers, ch); s.manager.indexMu.Unlock() }
}
func (s *Service) Signal() {
	s.manager.indexMu.RLock()
	defer s.manager.indexMu.RUnlock()
	for ch := range s.manager.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

type Finalizer struct {
	Service    *Service
	Repository ranking.Repository
	Notify     func(context.Context, ranking.Result)
	OnFailure  func(string, error)
	mu         sync.Mutex
}

// Commit is the only preparation/persistence/acknowledgement path for both adapters.
// Serializing commits also serializes notifications against concurrent retries.
func (f *Finalizer) Commit(ctx context.Context, input ranking.Result) (ranking.Commit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result, err := ranking.Prepare(input)
	if err != nil {
		if f.OnFailure != nil {
			f.OnFailure(input.GameID, err)
		}
		return ranking.Commit{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	commit, err := f.Repository.RecordCompletedGame(ctx, result)
	if err != nil {
		if f.OnFailure != nil {
			f.OnFailure(input.GameID, err)
		}
		return commit, err
	}
	f.Service.AcknowledgeResult(uno.GameID(result.GameID))
	f.Service.Signal()
	if commit.Scored && !commit.AlreadyPersisted && f.Notify != nil {
		f.Notify(ctx, result)
	}
	return commit, nil
}
func (f *Finalizer) Retry(ctx context.Context) error {
	var first error
	for _, result := range f.Service.PendingResults() {
		if _, err := f.Commit(ctx, result); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Run retries retained immutable results with bounded backoff; shutdown never acknowledges failures.
func (f *Finalizer) Run(ctx context.Context) {
	delay := time.Second
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if err := f.Retry(ctx); err != nil {
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		} else {
			delay = time.Second
		}
	}
}

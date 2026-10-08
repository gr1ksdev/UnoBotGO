package game

import (
	"context"
	"errors"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
	"sync"
	"testing"
	"time"
)

type retryRepo struct {
	mu     sync.Mutex
	fail   bool
	stored map[string]bool
	writes int
	calls  int
}

func (r *retryRepo) RecordCompletedGame(_ context.Context, v ranking.Result) (ranking.Commit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.fail {
		return ranking.Commit{}, errors.New("offline")
	}
	if r.stored == nil {
		r.stored = map[string]bool{}
	}
	old := r.stored[v.GameID]
	r.stored[v.GameID] = true
	if !old {
		r.writes++
	}
	return ranking.Commit{Scored: true, AlreadyPersisted: old}, nil
}
func pendingClosure(t *testing.T) (*Service, ranking.Result) {
	t.Helper()
	s := testService(t)
	v := create(t, s, 42, 1, uno.BotRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.StartGame})
	out := act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.LeaveGame})
	return s, *out.Completed
}
func TestFinalizerRetainsRetriesAndSerializesDuplicates(t *testing.T) {
	s, result := pendingClosure(t)
	repo := &retryRepo{fail: true}
	notifications := 0
	f := &Finalizer{Service: s, Repository: repo, Notify: func(context.Context, ranking.Result) { notifications++ }}
	if _, err := f.Commit(t.Context(), result); err == nil || len(s.PendingResults()) != 1 || notifications != 0 {
		t.Fatal("failed commit acknowledged")
	}
	repo.fail = false
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := f.Commit(t.Context(), result); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if repo.writes != 1 || notifications != 1 || len(s.PendingResults()) != 0 {
		t.Fatal("duplicate score or notification", repo.writes, notifications)
	}
}
func TestAutomaticPendingRetry(t *testing.T) {
	s, _ := pendingClosure(t)
	repo := &retryRepo{fail: true}
	f := &Finalizer{Service: s, Repository: repo}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go f.Run(ctx)
	firstDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(firstDeadline) {
		repo.mu.Lock()
		calls := repo.calls
		repo.mu.Unlock()
		if calls > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(s.PendingResults()) != 1 {
		t.Fatal("failed automatic commit lost result")
	}
	repo.mu.Lock()
	repo.fail = false
	repo.mu.Unlock()
	deadline := time.Now().Add(4 * time.Second)
	for len(s.PendingResults()) > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(s.PendingResults()) != 0 {
		t.Fatal("automatic retry did not commit")
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.writes != 1 {
		t.Fatal(repo.writes)
	}
}

func TestWebAppNormalTwoPlayerFinishAndFinalActionRetry(t *testing.T) {
	s, v := rankingGame(t, 2)
	joinView, _ := s.PublicView(t.Context(), v.GameID)
	if _, err := s.ApplyWeb(t.Context(), Actor{PlayerID: 1, ChatID: 42}, v.GameID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: joinView.Revision}, "web-start-action"); err != nil {
		t.Fatal(err)
	}
	for turn := 0; turn < 100; turn++ {
		view, err := s.PublicView(t.Context(), v.GameID)
		if err != nil {
			t.Fatal(err)
		}
		actor := Actor{PlayerID: view.CurrentTurn, ChatID: 42}
		hand, err := s.PlayerView(t.Context(), actor, v.GameID)
		if err != nil {
			t.Fatal(err)
		}
		action := uno.Action{Type: uno.PlayCard, PlayerID: actor.PlayerID, Revision: view.Revision, CardID: hand.Hand[0].Card.ID}
		request := string(action.CardID) + "-web-action"
		out, err := s.ApplyWeb(t.Context(), actor, v.GameID, action, request)
		if err != nil {
			t.Fatal(err)
		}
		if out.Completed == nil {
			continue
		}
		if !out.View.Closed || out.View.CurrentTurn != 0 || !out.View.TurnStarted.IsZero() {
			t.Fatal("finished game retained turn")
		}
		prepared, err := ranking.Prepare(*out.Completed)
		if err != nil || prepared.Origin != "webapp" || len(prepared.Players) != 2 {
			t.Fatal(err, prepared)
		}
		for _, player := range prepared.Players {
			expected, err := ranking.Score(prepared.RankingSystem, 2, player.Position)
			if err != nil || player.Score != expected {
				t.Fatal("WebApp policy differs from Inline", prepared)
			}
		}
		retry, err := s.ApplyWeb(t.Context(), actor, v.GameID, action, request)
		if err != nil || !retry.View.Closed || retry.Completed != nil || len(s.PendingResults()) != 1 {
			t.Fatal("final action retry duplicated result", err, retry)
		}
		return
	}
	t.Fatal("game failed to finish")
}

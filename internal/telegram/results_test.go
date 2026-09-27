package telegram

import (
	"context"
	"errors"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
	"testing"
)

type resultRepo struct {
	calls   int
	err     error
	entered chan struct{}
	release chan struct{}
}

func (r *resultRepo) RecordCompletedGame(_ context.Context, result ranking.Result) (ranking.Commit, error) {
	r.calls++
	if r.entered != nil {
		close(r.entered)
		<-r.release
	}
	return ranking.Commit{}, r.err
}
func closureOutcome(t *testing.T, svc *game.Service) game.Outcome {
	t.Helper()
	ctx := t.Context()
	actor := game.Actor{ChatID: 42, PlayerID: 1}
	out, err := svc.Create(ctx, actor, game.CreateRequest{Rules: uno.BotRules()})
	if err != nil {
		t.Fatal(err)
	}
	v := out.View
	apply := func(id uno.PlayerID, kind uno.ActionType) game.Outcome {
		t.Helper()
		actor.PlayerID = id
		out, err := svc.Apply(ctx, actor, v.GameID, uno.Action{PlayerID: id, Type: kind, Revision: v.Revision})
		if err != nil {
			t.Fatal(err)
		}
		v = out.View
		return out
	}
	apply(1, uno.JoinGame)
	apply(2, uno.JoinGame)
	apply(1, uno.StartGame)
	return apply(1, uno.LeaveGame)
}
func TestSynchronousCommitAndFailureRetention(t *testing.T) {
	svc, _ := game.NewService(game.WithHistoryLimit(0))
	b := New(newMockBotAPI(), svc, nil, nil, 0, nil)
	out := closureOutcome(t, svc)
	repo := &resultRepo{entered: make(chan struct{}), release: make(chan struct{})}
	b.SetResultRepository(repo)
	done := make(chan struct{})
	go func() { b.finalizeOutcome(t.Context(), out); close(done) }()
	<-repo.entered
	select {
	case <-done:
		t.Fatal("finalization returned before commit")
	default:
	}
	if len(svc.PendingResults()) != 1 {
		t.Fatal("pending removed before commit")
	}
	// Game service remains available; no game lock is held while DB is blocked.
	if _, err := svc.Create(t.Context(), game.Actor{ChatID: 43, PlayerID: 3}, game.CreateRequest{Rules: uno.BotRules()}); err != nil {
		t.Fatal(err)
	}
	close(repo.release)
	<-done
	if len(svc.PendingResults()) != 0 {
		t.Fatal("commit not acknowledged")
	}
	out = closureOutcome(t, svc)
	repo = &resultRepo{err: errors.New("DB offline")}
	b.SetResultRepository(repo)
	b.finalizeOutcome(t.Context(), out)
	if len(svc.PendingResults()) != 1 {
		t.Fatal("failed result lost")
	}
	repo.err = nil
	if err := b.RetryPendingResults(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(svc.PendingResults()) != 0 {
		t.Fatal("retry failed")
	}
	b.finalizeOutcome(t.Context(), game.Outcome{})
	if repo.calls != 2 {
		t.Fatal("unfinished action used database")
	}
}

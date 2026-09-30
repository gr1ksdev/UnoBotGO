package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
)

type resultRepo struct {
	calls   int
	commit  ranking.Commit
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
	return r.commit, r.err
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

func TestRenderPoints_FormattingAndEscaping(t *testing.T) {
	now := time.Now().UTC()
	legacyResult := ranking.Result{
		GameID:        "game-legacy",
		ChatID:        100,
		RankingSystem: groups.Legacy,
		PolicyVersion: ranking.PlacementPolicyV1,
		FinishedAt:    now,
		Players: []ranking.Player{
			{UserID: 2, DisplayName: "Bob", Position: 2, WentOut: false, FinalStatus: "playing", Score: 0},
			{UserID: 1, DisplayName: "<Alice & Bob>", Position: 1, WentOut: true, FinalStatus: "went_out", Score: 100},
			{UserID: 3, DisplayName: "Charlie", Position: 0, WentOut: false, FinalStatus: "left", Score: 0},
			{UserID: 4, DisplayName: "", Position: 0, WentOut: false, FinalStatus: "left", Score: 0},
		},
	}

	text := renderPoints(legacyResult)
	expectedLines := []string{
		"🏁 Partida encerrada",
		"",
		"🥇 &lt;Alice &amp; Bob&gt; · +1 pt",
		"🥈 Bob · +0 pts",
		"Charlie · fora do ranking",
		"4 · fora do ranking",
	}
	expected := strings.Join(expectedLines, "\n")
	if text != expected {
		t.Fatalf("unexpected legacy rendered points:\nwant:\n%s\ngot:\n%s", expected, text)
	}

	updatedResult := ranking.Result{
		GameID:        "game-updated",
		ChatID:        100,
		RankingSystem: groups.Updated,
		PolicyVersion: ranking.PlacementPolicyV1,
		FinishedAt:    now,
		Players: []ranking.Player{
			{UserID: 2, DisplayName: "Bob", Position: 2, WentOut: false, FinalStatus: "playing", Score: 500},
			{UserID: 1, DisplayName: "Alice", Position: 1, WentOut: true, FinalStatus: "went_out", Score: 1000},
			{UserID: 3, DisplayName: "Charlie", Position: 3, WentOut: false, FinalStatus: "playing", Score: 0},
			{UserID: 5, DisplayName: "Dave", Position: 0, WentOut: false, FinalStatus: "left", Score: 0},
		},
	}

	textUpdated := renderPoints(updatedResult)
	expectedUpdatedLines := []string{
		"🏁 Partida encerrada",
		"",
		"🥇 Alice · +10,00 pts",
		"🥈 Bob · +5,00 pts",
		"🥉 Charlie · +0,00 pts",
		"Dave · fora do ranking",
	}
	expectedUpdated := strings.Join(expectedUpdatedLines, "\n")
	if textUpdated != expectedUpdated {
		t.Fatalf("unexpected updated rendered points:\nwant:\n%s\ngot:\n%s", expectedUpdated, textUpdated)
	}
}

func TestFinalizeOutcome_NotificationOrder(t *testing.T) {
	svc, _ := game.NewService(game.WithHistoryLimit(0))
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, 0, nil)

	ctx := t.Context()
	actor := game.Actor{ChatID: 42, PlayerID: 1}
	out, err := svc.Create(ctx, actor, game.CreateRequest{Rules: uno.BotRules()})
	if err != nil {
		t.Fatal(err)
	}
	gameID := out.View.GameID
	_ = svc.ObservePlayer(ctx, game.Actor{ChatID: 42, PlayerID: 1}, gameID, "Alice", "alice")
	_ = svc.ObservePlayer(ctx, game.Actor{ChatID: 42, PlayerID: 2}, gameID, "Bob", "bob")

	apply := func(id uno.PlayerID, kind uno.ActionType) game.Outcome {
		t.Helper()
		actor.PlayerID = id
		res, err := svc.Apply(ctx, actor, gameID, uno.Action{PlayerID: id, Type: kind, Revision: out.View.Revision})
		if err != nil {
			t.Fatal(err)
		}
		out = res
		return res
	}
	apply(1, uno.JoinGame)
	apply(2, uno.JoinGame)
	apply(1, uno.StartGame)

	repo := &resultRepo{commit: ranking.Commit{Scored: true, AlreadyPersisted: false}}
	b.SetResultRepository(repo)

	// Player 1 leaves -> game ends
	b.SetRankingService(&ranking.Service{Repository: rankingReadFunc(func(context.Context, int64, time.Time) (ranking.GroupRanking, error) {
		return ranking.GroupRanking{System: groups.Legacy}, nil
	})})
	b.cmdHandler.handleSair(ctx, 1, 42)

	api.mu.Lock()
	msgs := api.SentMessages
	api.mu.Unlock()

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (closure then points), got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Text, "Partida encerrada") {
		t.Fatalf("first message should be game closure, got: %s", msgs[0].Text)
	}
	if !strings.Contains(msgs[1].Text, "🏆 Ranking do grupo") {
		t.Fatalf("second message should be points notification, got: %s", msgs[1].Text)
	}
}

func TestFinalizeOutcome_UnscoredOrAlreadyPersisted(t *testing.T) {
	t.Run("unscored commit does not notify", func(t *testing.T) {
		svc, _ := game.NewService(game.WithHistoryLimit(0))
		api := newMockBotAPI()
		b := New(api, svc, nil, nil, 0, nil)

		ctx := t.Context()
		actor := game.Actor{ChatID: 42, PlayerID: 1}
		out, _ := svc.Create(ctx, actor, game.CreateRequest{Rules: uno.BotRules()})
		gameID := out.View.GameID

		apply := func(id uno.PlayerID, kind uno.ActionType) game.Outcome {
			actor.PlayerID = id
			res, _ := svc.Apply(ctx, actor, gameID, uno.Action{PlayerID: id, Type: kind, Revision: out.View.Revision})
			out = res
			return res
		}
		apply(1, uno.JoinGame)
		apply(2, uno.JoinGame)
		apply(1, uno.StartGame)

		repo := &resultRepo{commit: ranking.Commit{Scored: false, AlreadyPersisted: false}}
		b.SetResultRepository(repo)

		b.cmdHandler.handleSair(ctx, 1, 42)

		api.mu.Lock()
		msgs := api.SentMessages
		api.mu.Unlock()

		if len(msgs) != 1 {
			t.Fatalf("expected only 1 message (closure), got %d", len(msgs))
		}
		if !strings.Contains(msgs[0].Text, "Partida encerrada") {
			t.Fatalf("unexpected message: %s", msgs[0].Text)
		}
	})

	t.Run("already persisted commit does not notify", func(t *testing.T) {
		svc, _ := game.NewService(game.WithHistoryLimit(0))
		api := newMockBotAPI()
		b := New(api, svc, nil, nil, 0, nil)

		ctx := t.Context()
		actor := game.Actor{ChatID: 42, PlayerID: 1}
		out, _ := svc.Create(ctx, actor, game.CreateRequest{Rules: uno.BotRules()})
		gameID := out.View.GameID

		apply := func(id uno.PlayerID, kind uno.ActionType) game.Outcome {
			actor.PlayerID = id
			res, _ := svc.Apply(ctx, actor, gameID, uno.Action{PlayerID: id, Type: kind, Revision: out.View.Revision})
			out = res
			return res
		}
		apply(1, uno.JoinGame)
		apply(2, uno.JoinGame)
		apply(1, uno.StartGame)

		repo := &resultRepo{commit: ranking.Commit{Scored: true, AlreadyPersisted: true}}
		b.SetResultRepository(repo)

		b.cmdHandler.handleSair(ctx, 1, 42)

		api.mu.Lock()
		msgs := api.SentMessages
		api.mu.Unlock()

		if len(msgs) != 1 {
			t.Fatalf("expected only 1 message (closure), got %d", len(msgs))
		}
	})
}

func TestRetryPendingResults_NotifiesOnNewCommit(t *testing.T) {
	svc, _ := game.NewService(game.WithHistoryLimit(0))
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, 0, nil)

	out := closureOutcome(t, svc)
	repo := &resultRepo{err: errors.New("db temporarily unavailable")}
	b.SetResultRepository(repo)

	// Finalize fails persistence
	notify := b.finalizeOutcome(t.Context(), out)
	if notify != nil {
		t.Fatal("expected nil notify on persistence failure")
	}
	if len(svc.PendingResults()) != 1 {
		t.Fatal("expected 1 pending result")
	}

	// Retry succeeds with scored=true
	repo.err = nil
	repo.commit = ranking.Commit{Scored: true, AlreadyPersisted: false}
	if err := b.RetryPendingResults(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(svc.PendingResults()) != 0 {
		t.Fatal("expected pending result cleared")
	}

	api.mu.Lock()
	msgs := api.SentMessages
	api.mu.Unlock()

	if len(msgs) != 2 {
		t.Fatalf("expected points and ranking availability notification from retry, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Text, "🏁 Partida encerrada") {
		t.Fatalf("expected ranking text in retry notification, got: %s", msgs[0].Text)
	}

	// Unconfigured repository returns error
	bUnconfigured := New(newMockBotAPI(), svc, nil, nil, 0, nil)
	if err := bUnconfigured.RetryPendingResults(t.Context()); err == nil {
		t.Fatal("expected error when repository not configured")
	}
}

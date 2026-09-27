package game

import (
	"github.com/malbs/UnoGoBot/internal/uno"
	"testing"
)

func TestFinalResultSurvivesHistoryAndReset(t *testing.T) {
	s := testService(t, WithHistoryLimit(0))
	v := create(t, s, 42, 1, uno.BotRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	if err := s.ObservePlayer(t.Context(), Actor{PlayerID: 1, ChatID: 42}, v.GameID, "🎮 Gabriel", "user"); err != nil {
		t.Fatal(err)
	}
	start := act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.StartGame})
	if start.Completed != nil || len(s.PendingResults()) != 0 {
		t.Fatal("unfinished result persisted")
	}
	end := act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.LeaveGame})
	if end.Completed == nil || end.Completed.FinishReason != "departure" || end.Completed.Players[0].Position != 0 {
		t.Fatal("departure fabricated placement", end.Completed)
	}
	if end.Completed.Players[0].DisplayName != "🎮 Gabriel" || end.Completed.Players[0].LeaveCount != 1 {
		t.Fatal("metadata missing")
	}
	if err := end.Completed.Validate(); err != nil {
		t.Fatal(err)
	}
	end.Completed.Players[0].DisplayName = "mutated"
	pending := s.PendingResults()
	if len(pending) != 1 || pending[0].Players[0].DisplayName != "🎮 Gabriel" {
		t.Fatal("result mutable or evicted")
	}
	s.ResetChat(t.Context(), Actor{PlayerID: 1, ChatID: 42, ChatAdmin: true})
	if len(s.PendingResults()) != 1 {
		t.Fatal("reset dropped uncommitted result")
	}
	s.AcknowledgeResult(v.GameID)
	if len(s.PendingResults()) != 0 {
		t.Fatal("committed result retained")
	}
}
func TestCancellationDoesNotCreateResult(t *testing.T) {
	s := testService(t)
	v := create(t, s, 42, 1, uno.BotRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.StartGame})
	out := act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.CancelGame})
	if out.Completed != nil || len(s.PendingResults()) != 0 {
		t.Fatal("cancelled counted")
	}
}

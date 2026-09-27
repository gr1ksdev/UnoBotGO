package game

import (
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/uno"
	"testing"
)

func TestGroupConfigSnapshot(t *testing.T) {
	s := testService(t)
	c := groups.Defaults(42)
	c.RankingSystem = groups.Updated
	out, err := s.Create(t.Context(), Actor{PlayerID: 1, ChatID: 42}, CreateRequest{Rules: uno.BotRules(), GroupConfig: c.Snapshot()})
	if err != nil {
		t.Fatal(err)
	}
	c.RankingSystem = groups.Legacy
	c.Revision++
	view := out.View
	join(t, s, view, 1)
	join(t, s, view, 2)
	act(t, s, view.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.SetRules, Rules: uno.CaseiroRules()})
	started := act(t, s, view.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.StartGame}).View
	if started.GroupConfig.RankingSystem != groups.Updated || started.GroupConfig.ConfigRevision != 1 {
		t.Fatal("snapshot changed")
	}
	if !started.Rules.AllowSwapHands {
		t.Fatal("lobby mode did not survive start")
	}
}

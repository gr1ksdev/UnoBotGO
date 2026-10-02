//go:build debugcards

package game

import (
	"errors"
	"github.com/malbs/UnoGoBot/internal/uno"
	"reflect"
	"testing"
	"time"
)

func TestDebugGiveCardAuthorizationAndPublication(t *testing.T) {
	s := testService(t)
	v := create(t, s, -100, 1, uno.CaseiroRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: -100}, uno.Action{Type: uno.StartGame, DealerID: 2})
	entry := s.manager.byID[v.GameID].entry
	started := time.Now().Add(-time.Minute)
	entry.turnStarted = started
	for _, actor := range []Actor{{PlayerID: 1, ChatID: -100}, {PlayerID: 2, ChatID: -100, ChatAdmin: true}, {PlayerID: DebugCardsUserID}, {PlayerID: DebugCardsUserID, ChatID: -200}} {
		before := entry.engine.Snapshot()
		_, _, err := s.GiveCard(t.Context(), actor, v.GameID, 2, uno.NoColor, uno.SwapHands)
		if !errors.Is(err, ErrForbidden) || !reflect.DeepEqual(before, entry.engine.Snapshot()) {
			t.Fatal(err)
		}
	}
	before := entry.engine.Snapshot()
	out, c, err := s.GiveCard(t.Context(), Actor{PlayerID: DebugCardsUserID, ChatID: -100}, v.GameID, 2, uno.NoColor, uno.SwapHands)
	if err != nil {
		t.Fatal(err)
	}
	if c.Rank != uno.SwapHands || out.View.Revision != before.Revision+1 || !entry.turnStarted.Equal(started) || out.View.CurrentTurn != before.CurrentPlayerID {
		t.Fatal(out, c)
	}
	pv, err := s.PlayerView(t.Context(), Actor{PlayerID: 2}, v.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pv.Hand) != 8 || pv.Hand[7].Card != c {
		t.Fatal(pv.Hand)
	}
	_, err = s.Apply(t.Context(), Actor{PlayerID: 1}, v.GameID, uno.Action{Type: uno.DrawCard, PlayerID: 1, Revision: before.Revision})
	if !errors.Is(err, uno.ErrStaleRevision) {
		t.Fatal(err)
	}
	assertIndexes(t, s)
}

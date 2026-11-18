//go:build debugcards

package game

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
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

	// Only an administrator of the game's chat may use the debug command.
	for _, actor := range []Actor{
		{PlayerID: 1, ChatID: -100},
		{PlayerID: 2, ChatID: -100},
		{PlayerID: 1},
		{PlayerID: 1, ChatID: -200, ChatAdmin: true},
	} {
		before := entry.engine.Snapshot()
		_, _, err := s.GiveCard(t.Context(), actor, v.GameID, 2, uno.NoColor, uno.SwapHands)
		if !errors.Is(err, ErrForbidden) || !reflect.DeepEqual(before, entry.engine.Snapshot()) {
			t.Fatal(err)
		}
	}

	before := entry.engine.Snapshot()
	out, c, err := s.GiveCard(t.Context(), Actor{PlayerID: 1, ChatID: -100, ChatAdmin: true}, v.GameID, 2, uno.NoColor, uno.SwapHands)
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
	// A stale revision from before the debug grant is still rejected.
	_, err = s.Apply(t.Context(), Actor{PlayerID: 1}, v.GameID, uno.Action{Type: uno.DrawCard, PlayerID: 1, Revision: before.Revision})
	if !errors.Is(err, uno.ErrStaleRevision) {
		t.Fatal(err)
	}
	assertIndexes(t, s)
}

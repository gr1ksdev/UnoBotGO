package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestServiceSwapHandsAuthorizationViewsAndTimeout(t *testing.T) {
	s := testService(t)
	s.manager.newGame = func(id uno.GameID, rules uno.Rules) (*uno.Game, error) {
		deck := uno.ClassicDeck()
		// Dealer 2 deals the first card to player 1.
		deck[0] = uno.Card{ID: "swap", Rank: uno.SwapHands}
		return uno.NewGame(id, rules, uno.WithDeck(deck), uno.WithShuffler(func([]uno.CardID) {}))
	}
	v := create(t, s, -100, 1, uno.CaseiroRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: -100}, uno.Action{Type: uno.StartGame, DealerID: 2})
	before1, err := s.PlayerView(t.Context(), Actor{PlayerID: 1}, v.GameID)
	if err != nil {
		t.Fatal(err)
	}
	before2, err := s.PlayerView(t.Context(), Actor{PlayerID: 2}, v.GameID)
	if err != nil {
		t.Fatal(err)
	}
	played := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.PlayCard, CardID: "swap"})
	if played.View.Phase != uno.ChoosingPlayer || played.View.PlayerChooserID != 1 || played.View.ColorChooserID != 0 {
		t.Fatal(played.View)
	}
	rev := played.View.Revision
	denied(t, s, v.GameID, Actor{PlayerID: 2}, uno.Action{Type: uno.ChoosePlayer, PlayerID: 1, TargetID: 2, Revision: rev}, ErrForbidden)
	denied(t, s, v.GameID, Actor{PlayerID: 1, ChatID: -200}, uno.Action{Type: uno.ChoosePlayer, PlayerID: 1, TargetID: 2, Revision: rev}, ErrForbidden)
	denied(t, s, v.GameID, Actor{PlayerID: 2}, uno.Action{Type: uno.ChoosePlayer, PlayerID: 2, TargetID: 1, Revision: rev}, uno.ErrNotYourTurn)
	entry := s.manager.byID[v.GameID].entry
	entry.turnStarted = time.Now().Add(-time.Hour)
	if candidates := s.ExpiredTurns(t.Context(), time.Second); len(candidates) != 0 {
		t.Fatal("choice was eligible for timeout")
	}
	if _, applied := s.AutoSkipTurn(t.Context(), ExpiredTurn{GameID: v.GameID, ChatID: -100, PlayerID: 1, Revision: rev}, time.Second); applied {
		t.Fatal("timeout skipped choice")
	}
	chosen := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.ChoosePlayer, TargetID: 2})
	if chosen.View.Phase != uno.TakingTurn || chosen.View.PlayerChooserID != 0 || chosen.View.CurrentTurn != 2 || chosen.View.ActiveColor != played.View.ActiveColor {
		t.Fatal(chosen.View)
	}
	after1, err := s.PlayerView(t.Context(), Actor{PlayerID: 1}, v.GameID)
	if err != nil {
		t.Fatal(err)
	}
	after2, err := s.PlayerView(t.Context(), Actor{PlayerID: 2}, v.GameID)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(hand []CardView) []uno.CardID {
		var result []uno.CardID
		for _, c := range hand {
			result = append(result, c.Card.ID)
		}
		return result
	}
	if !slices.Equal(ids(after1.Hand), ids(before2.Hand)) || !slices.Equal(ids(after2.Hand), ids(before1.Hand[1:])) {
		t.Fatal("private views did not reflect exchange")
	}
	raw, err := json.Marshal(chosen.View)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"Hand"`, `"Cards"`, `"DrawPile"`} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatal("public view exposed private state")
		}
	}
	denied(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.ChoosePlayer, PlayerID: 1, TargetID: 2, Revision: rev}, uno.ErrStaleRevision)
	assertIndexes(t, s)
}

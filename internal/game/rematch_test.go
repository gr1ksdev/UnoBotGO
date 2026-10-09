package game

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func closedForRematch(t *testing.T) (*Service, PublicGameView) {
	t.Helper()
	s := testService(t)
	v := create(t, s, 42, 1, uno.ClassicRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.StartGame})
	end := act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.LeaveGame})
	return s, end.View
}
func vote(t *testing.T, s *Service, v PublicGameView, player uno.PlayerID, accept bool, request string) PublicGameView {
	t.Helper()
	result, err := s.VoteRematch(t.Context(), Actor{PlayerID: player, ChatID: v.ChatID}, v.GameID, v.Revision, accept, request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestRematchPendingResultAndFixedMembership(t *testing.T) {
	s, v := closedForRematch(t)
	original := s.PendingResults()
	if _, err := s.VoteRematch(t.Context(), Actor{PlayerID: 1, ChatID: 42}, v.GameID, v.Revision, true, "pending-01"); !errors.Is(err, ErrResultPending) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, s.PendingResults()) {
		t.Fatal("pending result mutated")
	}
	if !reflect.DeepEqual(v.Rematch.Required, []uno.PlayerID{1, 2}) {
		t.Fatal("departure excluded from consensus")
	}
	s.AcknowledgeResult(v.GameID)
	a := vote(t, s, v, 1, true, "accept-01")
	if a.Rematch.NextGameID != "" || len(a.Rematch.Accepted) != 1 {
		t.Fatal("single vote started game")
	}
	// Disconnect has no mutation path: snapshot retains required group and vote.
	recovered, _ := s.PublicView(t.Context(), v.GameID)
	if !reflect.DeepEqual(a.Rematch, recovered.Rematch) {
		t.Fatal("recovery lost consensus")
	}
	revoked := vote(t, s, v, 1, false, "decline-01")
	if len(revoked.Rematch.Accepted) != 0 || len(revoked.Rematch.Required) != 2 {
		t.Fatal("decline shrank group")
	}
	retry := vote(t, s, v, 1, true, "accept-01")
	if len(retry.Rematch.Accepted) != 0 {
		t.Fatal("duplicate old request reapplied vote")
	}
	vote(t, s, v, 1, true, "accept-02")
	end := vote(t, s, v, 2, true, "accept-03")
	if end.Rematch.NextGameID == "" || end.Rematch.NextGameID == v.GameID {
		t.Fatal("missing new ID")
	}
	fresh, err := s.PublicView(t.Context(), end.Rematch.NextGameID)
	if err != nil || fresh.Closed || fresh.Phase != uno.TakingTurn || len(fresh.Players) != 2 || fresh.GroupConfig != v.GroupConfig || fresh.Rules != v.Rules {
		t.Fatal("new engine contract", fresh, err)
	}
	if fresh.TopCard == nil || fresh.GameID == v.GameID {
		t.Fatal("round not dealt")
	}
	for _, player := range []uno.PlayerID{1, 2} {
		pv, err := s.PlayerView(t.Context(), Actor{PlayerID: player}, fresh.GameID)
		if err != nil || len(pv.Hand) != 7 || pv.PlayerID != player {
			t.Fatal("private new hand", err)
		}
	}
	old, _ := s.PublicView(t.Context(), v.GameID)
	if !old.Closed || !reflect.DeepEqual(old.Placements, v.Placements) {
		t.Fatal("previous result lost")
	}
	assertIndexes(t, s)
}
func TestConcurrentRematchVotesStartExactlyOnce(t *testing.T) {
	s, v := closedForRematch(t)
	s.AcknowledgeResult(v.GameID)
	var factoryMu sync.Mutex
	creations := 0
	factory := s.manager.newGame
	s.manager.newGame = func(id uno.GameID, rules uno.Rules) (*uno.Game, error) {
		factoryMu.Lock()
		defer factoryMu.Unlock()
		creations++
		return factory(id, rules)
	}
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := s.VoteRematch(t.Context(), Actor{PlayerID: uno.PlayerID(i%2 + 1), ChatID: 42}, v.GameID, v.Revision, true, fmt.Sprintf("request-%02d", i))
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if creations != 1 {
		t.Fatalf("created %d rounds", creations)
	}
	before, _ := s.PublicView(t.Context(), v.GameID)
	vote(t, s, v, 1, false, "late-decline")
	after, _ := s.PublicView(t.Context(), v.GameID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("old vote changed published round")
	}
	assertIndexes(t, s)
}
func TestRematchAuthorizationStaleRevisionAndOccupiedChat(t *testing.T) {
	s, v := closedForRematch(t)
	s.AcknowledgeResult(v.GameID)
	for _, actor := range []Actor{{PlayerID: 3, ChatID: 42}, {PlayerID: 1, ChatID: 43}} {
		if _, err := s.VoteRematch(t.Context(), actor, v.GameID, v.Revision, true, "forged-01"); !errors.Is(err, ErrForbidden) {
			t.Fatal(err)
		}
	}
	if _, err := s.VoteRematch(t.Context(), Actor{PlayerID: 1, ChatID: 42}, v.GameID, v.Revision-1, true, "stale-01"); !errors.Is(err, uno.ErrStaleRevision) {
		t.Fatal(err)
	}
	vote(t, s, v, 1, true, "accept-01")
	other := create(t, s, 42, 9, uno.ClassicRules())
	if _, err := s.VoteRematch(t.Context(), Actor{PlayerID: 2, ChatID: 42}, v.GameID, v.Revision, true, "accept-02"); !errors.Is(err, ErrChatOccupied) {
		t.Fatal(err)
	}
	chat, _ := s.FindChatGame(t.Context(), 42)
	if chat.GameID != other.GameID {
		t.Fatal("overwrote active chat")
	}
	old, _ := s.PublicView(t.Context(), v.GameID)
	if old.Rematch.NextGameID != "" {
		t.Fatal("published conflicting game")
	}
	assertIndexes(t, s)
}

func TestTenPlayerRematchRequiresEveryOriginalParticipant(t *testing.T) {
	s := testService(t)
	v := create(t, s, 42, 1, uno.ClassicRules())
	for player := uno.PlayerID(1); player <= 10; player++ {
		join(t, s, v, player)
	}
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 42}, uno.Action{Type: uno.StartGame})
	var closed PublicGameView
	for player := uno.PlayerID(1); player <= 9; player++ {
		closed = act(t, s, v.GameID, Actor{PlayerID: player, ChatID: 42}, uno.Action{Type: uno.LeaveGame}).View
	}
	if !closed.Closed || len(closed.Rematch.Required) != 10 {
		t.Fatal("original group changed")
	}
	s.AcknowledgeResult(v.GameID)
	var wg sync.WaitGroup
	for player := uno.PlayerID(1); player <= 9; player++ {
		wg.Add(1)
		go func(player uno.PlayerID) {
			defer wg.Done()
			_, err := s.VoteRematch(t.Context(), Actor{PlayerID: player, ChatID: 42}, v.GameID, closed.Revision, true, fmt.Sprintf("ten-vote-%02d", player))
			if err != nil {
				t.Error(err)
			}
		}(player)
	}
	wg.Wait()
	still, _ := s.PublicView(t.Context(), v.GameID)
	if len(still.Rematch.Accepted) != 9 || still.Rematch.NextGameID != "" {
		t.Fatal("missing tenth vote ignored")
	}
	all := vote(t, s, closed, 10, true, "ten-vote-10")
	fresh, _ := s.PublicView(t.Context(), all.Rematch.NextGameID)
	if len(fresh.Players) != 10 || fresh.Phase != uno.TakingTurn {
		t.Fatal("ten-player round not started")
	}
	cards := map[uno.CardID]bool{}
	for player := uno.PlayerID(1); player <= 10; player++ {
		hand, err := s.PlayerView(t.Context(), Actor{PlayerID: player}, fresh.GameID)
		if err != nil || len(hand.Hand) != 7 {
			t.Fatal(err)
		}
		for _, card := range hand.Hand {
			if cards[card.Card.ID] {
				t.Fatal("duplicated private card")
			}
			cards[card.Card.ID] = true
		}
	}
	// Public snapshots are copies: a caller cannot change the fixed consensus.
	still.Rematch.Required[0] = 999
	again, _ := s.PublicView(t.Context(), v.GameID)
	if again.Rematch.Required[0] != 1 {
		t.Fatal("mutable consensus escaped")
	}
	assertIndexes(t, s)
}

func TestRematchCreationFailureCanRetryWithoutDuplicateRound(t *testing.T) {
	s, v := closedForRematch(t)
	s.AcknowledgeResult(v.GameID)
	vote(t, s, v, 1, true, "accept-first")
	factory := s.manager.newGame
	s.manager.newGame = func(uno.GameID, uno.Rules) (*uno.Game, error) { return nil, errors.New("test failure") }
	if _, err := s.VoteRematch(t.Context(), Actor{PlayerID: 2, ChatID: 42}, v.GameID, v.Revision, true, "retry-last-vote"); err == nil {
		t.Fatal("factory failure hidden")
	}
	old, _ := s.PublicView(t.Context(), v.GameID)
	if old.Rematch.NextGameID != "" || len(old.Rematch.Accepted) != 2 {
		t.Fatal("partial round published")
	}
	s.manager.newGame = factory
	all := vote(t, s, v, 2, true, "retry-last-vote")
	if all.Rematch.NextGameID == "" {
		t.Fatal("could not retry")
	}
	again := vote(t, s, v, 2, true, "retry-last-vote")
	if again.Rematch.NextGameID != all.Rematch.NextGameID {
		t.Fatal("retry started another round")
	}
}

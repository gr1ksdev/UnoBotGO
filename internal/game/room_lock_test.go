package game

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestRoomLockLifecycleAndAdmission(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "lobby", true: "active"}[active], func(t *testing.T) {
			s := testService(t)
			v := create(t, s, 10, 99, uno.BotRules()) // Owner is not a player.
			if v.Locked {
				t.Fatal("new room locked")
			}
			join(t, s, v, 1)
			join(t, s, v, 2)
			if active {
				act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 10}, uno.Action{Type: uno.StartGame})
			}
			entry := s.manager.byID[v.GameID].entry
			before := entry.engine.Snapshot()
			deadline := entry.turnStarted
			owner := Actor{PlayerID: 99, ChatID: 10}
			for _, locked := range []bool{true, true, false, false, true} {
				old := entry.locked
				view, changed, err := s.SetLocked(t.Context(), owner, v.GameID, locked)
				if err != nil || view.Locked != locked || changed != (old != locked) {
					t.Fatalf("lock result %v %v %v", view.Locked, changed, err)
				}
				summary, _ := s.FindChatGame(t.Context(), 10)
				if summary.Locked != locked {
					t.Fatal("stale lock projection")
				}
				if !reflect.DeepEqual(before, entry.engine.Snapshot()) || !entry.turnStarted.Equal(deadline) {
					t.Fatal("lock changed engine or timeout")
				}
				for _, other := range []Actor{{PlayerID: 1, ChatID: 10}, {PlayerID: 1, ChatID: 10, ChatAdmin: true}, {PlayerID: 99, ChatID: 11}} {
					if _, _, err := s.SetLocked(t.Context(), other, v.GameID, !locked); !errors.Is(err, ErrForbidden) {
						t.Fatalf("non-owner authorized: %v", err)
					}
				}
			}
			denied(t, s, v.GameID, Actor{PlayerID: 3, ChatID: 10}, uno.Action{Type: uno.JoinGame, PlayerID: 3, Revision: before.Revision}, ErrRoomLocked)
			if !reflect.DeepEqual(before, entry.engine.Snapshot()) {
				t.Fatal("rejected join changed engine")
			}
			if active {
				act(t, s, v.GameID, Actor{PlayerID: before.CurrentPlayerID, ChatID: 10}, uno.Action{Type: uno.DrawCard})
			}
			if _, _, err := s.SetLocked(t.Context(), owner, v.GameID, false); err != nil {
				t.Fatal(err)
			}
			join(t, s, v, 3)
			if _, _, err := s.SetLocked(t.Context(), owner, v.GameID, true); err != nil {
				t.Fatal(err)
			}
			act(t, s, v.GameID, owner, uno.Action{Type: uno.CancelGame})
			for _, locked := range []bool{false, true} {
				if _, _, err := s.SetLocked(t.Context(), owner, v.GameID, locked); !errors.Is(err, ErrGameClosed) {
					t.Fatalf("closed room revived: %v", err)
				}
			}
			if entry.engine != nil {
				t.Fatal("cancel retained runtime")
			}
			fresh := create(t, s, 10, 99, uno.BotRules())
			if fresh.Locked {
				t.Fatal("lock leaked to new room")
			}
			assertIndexes(t, s)
		})
	}
}

func TestRoomLockConcurrentJoin(t *testing.T) {
	for range 80 {
		s := testService(t)
		v := create(t, s, 10, 99, uno.BotRules())
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		var lockErr, joinErr error
		go func() {
			defer wg.Done()
			<-start
			_, _, lockErr = s.SetLocked(context.Background(), Actor{PlayerID: 99, ChatID: 10}, v.GameID, true)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, joinErr = s.Apply(context.Background(), Actor{PlayerID: 1, ChatID: 10}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: v.Revision})
		}()
		close(start)
		wg.Wait()
		if lockErr != nil || (joinErr != nil && !errors.Is(joinErr, ErrRoomLocked)) {
			t.Fatalf("lock=%v join=%v", lockErr, joinErr)
		}
		after, _ := s.PublicView(t.Context(), v.GameID)
		wantPlayers := 0
		if joinErr == nil {
			wantPlayers = 1
		}
		if !after.Locked || len(after.Players) != wantPlayers {
			t.Fatalf("non-atomic result: %+v", after)
		}
		denied(t, s, v.GameID, Actor{PlayerID: 2, ChatID: 10}, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: after.Revision}, ErrRoomLocked)
		assertIndexes(t, s)
	}
}

func TestRoomLockTimeoutResetAndFinalization(t *testing.T) {
	s := testService(t)
	v := create(t, s, 10, 99, uno.BotRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 10}, uno.Action{Type: uno.StartGame})
	entry := s.manager.byID[v.GameID].entry
	entry.turnStarted = time.Now().Add(-time.Hour)
	deadline := entry.turnStarted
	candidates := s.ExpiredTurns(t.Context(), time.Minute)
	if len(candidates) != 1 {
		t.Fatal("missing timeout")
	}
	owner := Actor{PlayerID: 99, ChatID: 10}
	if _, _, err := s.SetLocked(t.Context(), owner, v.GameID, true); err != nil {
		t.Fatal(err)
	}
	if !entry.turnStarted.Equal(deadline) {
		t.Fatal("lock reset deadline")
	}
	if _, applied := s.AutoSkipTurn(t.Context(), candidates[0], time.Minute); !applied {
		t.Fatal("lock invalidated timer candidate")
	}
	joinDenied := entry.engine.Snapshot()
	denied(t, s, v.GameID, Actor{PlayerID: 3, ChatID: 10}, uno.Action{Type: uno.JoinGame, PlayerID: 3, Revision: joinDenied.Revision}, ErrRoomLocked)
	if _, err := s.ResetChat(t.Context(), owner); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetLocked(t.Context(), owner, v.GameID, false); !errors.Is(err, ErrGameNotFound) {
		t.Fatal(err)
	}
	fresh := create(t, s, 10, 99, uno.BotRules())
	if fresh.Locked {
		t.Fatal("reset leaked lock")
	}
	// A normal departure finalizes the session; locking must not resurrect it.
	join(t, s, fresh, 1)
	join(t, s, fresh, 2)
	act(t, s, fresh.GameID, Actor{PlayerID: 1, ChatID: 10}, uno.Action{Type: uno.StartGame})
	if _, _, err := s.SetLocked(t.Context(), owner, fresh.GameID, true); err != nil {
		t.Fatal(err)
	}
	out := act(t, s, fresh.GameID, Actor{PlayerID: 1, ChatID: 10}, uno.Action{Type: uno.LeaveGame})
	if !out.View.Closed {
		t.Fatal("not finalized")
	}
	if _, _, err := s.SetLocked(t.Context(), owner, fresh.GameID, false); !errors.Is(err, ErrGameClosed) {
		t.Fatal(err)
	}
	assertIndexes(t, s)
}

func TestLateJoinPreservesTurnDeadline(t *testing.T) {
	s := testService(t)
	v := create(t, s, 10, 99, uno.CaseiroRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 10}, uno.Action{Type: uno.StartGame})
	entry := s.manager.byID[v.GameID].entry
	entry.turnStarted = time.Now().Add(-time.Hour)
	deadline := entry.turnStarted
	join(t, s, v, 3)
	if !entry.turnStarted.Equal(deadline) {
		t.Fatal("late join reset deadline")
	}
}

func TestRoomLockNaturalCompletionAndOwnerTransfer(t *testing.T) {
	s := testService(t)
	v := position(t, s, uno.Rules{EndPolicy: uno.Placements}, false)
	if _, _, err := s.SetLocked(t.Context(), Actor{PlayerID: 1, ChatID: v.ChatID}, v.GameID, true); err != nil {
		t.Fatal(err)
	}
	out := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.PlayCard, CardID: "a"})
	if !out.View.Locked || out.View.OwnerID != 2 {
		t.Fatal("lock or owner lost")
	}
	if _, _, err := s.SetLocked(t.Context(), Actor{PlayerID: 1, ChatID: v.ChatID}, v.GameID, false); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, _, err := s.SetLocked(t.Context(), Actor{PlayerID: 2, ChatID: v.ChatID}, v.GameID, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetLocked(t.Context(), Actor{PlayerID: 2, ChatID: v.ChatID}, v.GameID, true); err != nil {
		t.Fatal(err)
	}
	out = act(t, s, v.GameID, Actor{PlayerID: 2}, uno.Action{Type: uno.PlayCard, CardID: "b"})
	if !out.View.Closed || out.View.CloseReason != Completed || len(out.View.Placements) != 3 {
		t.Fatal("completion changed")
	}
	for _, locked := range []bool{false, true} {
		if _, _, err := s.SetLocked(t.Context(), Actor{PlayerID: 2, ChatID: v.ChatID}, v.GameID, locked); !errors.Is(err, ErrGameClosed) {
			t.Fatal(err)
		}
	}
	if !out.View.Locked {
		t.Fatal("final summary lost session metadata")
	}
	assertIndexes(t, s)
}

func TestJoinPrecedenceWithRoomLock(t *testing.T) {
	s := testService(t)
	rules := uno.BotRules()
	v := create(t, s, 10, 99, rules)

	// Build a valid state with P1 placed, and P2, P3, P4 active in game
	state := uno.State{
		ID:              v.GameID,
		Rules:           rules,
		Phase:           uno.TakingTurn,
		Direction:       1,
		DealerID:        4,
		CurrentPlayerID: 2,
		ActiveColor:     uno.Red,
		Placements:      []uno.Placement{{PlayerID: 1, Position: 1, WentOut: true}},
		Order:           []uno.PlayerID{2, 3, 4},
	}
	add := func(c uno.Card) uno.CardID {
		c.ID = uno.CardID(fmt.Sprintf("card_%d", len(state.Cards)+1))
		state.Cards = append(state.Cards, c)
		return c.ID
	}
	// P1 has WentOut (hand is nil)
	state.Players = append(state.Players, uno.Player{ID: 1, Status: uno.WentOut})
	// P2, P3, P4 have 2 cards each
	for id := uno.PlayerID(2); id <= 4; id++ {
		p := uno.Player{ID: id, Status: uno.Playing}
		p.Hand = append(p.Hand, add(uno.Card{Color: uno.Red, Rank: uno.One}), add(uno.Card{Color: uno.Blue, Rank: uno.Two}))
		state.Players = append(state.Players, p)
	}
	state.DiscardPile = []uno.CardID{add(uno.Card{Color: uno.Red, Rank: uno.Five})}
	for range 24 {
		state.DrawPile = append(state.DrawPile, add(uno.Card{Color: uno.Blue, Rank: uno.Nine}))
	}

	eng, err := uno.Restore(state, func([]uno.CardID) {})
	if err != nil {
		t.Fatalf("failed to restore test state: %v", err)
	}

	entry := s.manager.byID[v.GameID].entry
	entry.mu.Lock()
	entry.engine = eng
	entry.mu.Unlock()

	// Verify P1 has placement
	curView, _ := s.PublicView(t.Context(), v.GameID)
	if len(curView.Placements) != 1 || curView.Placements[0].PlayerID != 1 {
		t.Fatalf("expected P1 placement, got %+v", curView.Placements)
	}

	// P2 leaves the game via LeaveGame
	act(t, s, v.GameID, Actor{PlayerID: 2, ChatID: 10}, uno.Action{Type: uno.LeaveGame, PlayerID: 2})

	// Now lock the room
	owner := Actor{PlayerID: 99, ChatID: 10}
	_, _, err = s.SetLocked(t.Context(), owner, v.GameID, true)
	if err != nil {
		t.Fatal(err)
	}

	curView, _ = s.PublicView(t.Context(), v.GameID)

	// Precedence 1: P1 (finished/placed) attempts JoinGame on locked room
	// MUST return uno.ErrAlreadyFinished (NOT ErrRoomLocked)
	_, errP1 := s.Apply(t.Context(), Actor{PlayerID: 1, ChatID: 10}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: curView.Revision})
	if !errors.Is(errP1, uno.ErrAlreadyFinished) {
		t.Fatalf("expected ErrAlreadyFinished for placed player on locked room, got: %v", errP1)
	}

	// Precedence 2: P3 (active playing) attempts JoinGame on locked room
	// MUST return uno.ErrAlreadyJoined (NOT ErrRoomLocked)
	_, errP3 := s.Apply(t.Context(), Actor{PlayerID: 3, ChatID: 10}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 3, Revision: curView.Revision})
	if !errors.Is(errP3, uno.ErrAlreadyJoined) {
		t.Fatalf("expected ErrAlreadyJoined for active player on locked room, got: %v", errP3)
	}

	// Precedence 3: P2 (left without placement) attempts JoinGame on locked room
	// MUST return ErrRoomLocked
	_, errP2 := s.Apply(t.Context(), Actor{PlayerID: 2, ChatID: 10}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: curView.Revision})
	if !errors.Is(errP2, ErrRoomLocked) {
		t.Fatalf("expected ErrRoomLocked for departed player on locked room, got: %v", errP2)
	}

	// Precedence 3b: Brand new player P5 attempts JoinGame on locked room
	// MUST return ErrRoomLocked
	_, errP5 := s.Apply(t.Context(), Actor{PlayerID: 5, ChatID: 10}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 5, Revision: curView.Revision})
	if !errors.Is(errP5, ErrRoomLocked) {
		t.Fatalf("expected ErrRoomLocked for new player on locked room, got: %v", errP5)
	}

	// Unlock room
	_, _, err = s.SetLocked(t.Context(), owner, v.GameID, false)
	if err != nil {
		t.Fatal(err)
	}

	curView, _ = s.PublicView(t.Context(), v.GameID)

	// Now P2 (who left) re-enters the unlocked room -> MUST succeed!
	outP2, errP2Unlock := s.Apply(t.Context(), Actor{PlayerID: 2, ChatID: 10}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: curView.Revision})
	if errP2Unlock != nil {
		t.Fatalf("expected departed player to re-enter unlocked room, got: %v", errP2Unlock)
	}
	if outP2.View.Revision <= curView.Revision {
		t.Fatal("revision should advance on re-entry")
	}

	// But P1 (who finished) STILL cannot enter unlocked room
	_, errP1Unlock := s.Apply(t.Context(), Actor{PlayerID: 1, ChatID: 10}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: outP2.View.Revision})
	if !errors.Is(errP1Unlock, uno.ErrAlreadyFinished) {
		t.Fatalf("expected ErrAlreadyFinished for placed player on unlocked room, got: %v", errP1Unlock)
	}
}

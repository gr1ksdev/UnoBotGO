package game

import (
	"context"
	"errors"
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

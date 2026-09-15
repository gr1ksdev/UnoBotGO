package game

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

// Release all workers through a barrier; timeout is only a deadlock watchdog.
func concurrent(t *testing.T, n int, work func(int) error) []error {
	t.Helper()
	start := make(chan struct{})
	result := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() { <-start; result <- work(i) }()
	}
	close(start)
	errs := make([]error, 0, n)
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for range n {
		select {
		case err := <-result:
			errs = append(errs, err)
		case <-timer.C:
			t.Fatal("deadlock")
		}
	}
	return errs
}
func oneAccepted(t *testing.T, errs []error, rejected error) {
	t.Helper()
	n := 0
	for _, err := range errs {
		if err == nil {
			n++
		} else if !errors.Is(err, rejected) {
			t.Fatal(err)
		}
	}
	if n != 1 {
		t.Fatalf("accepted %d actions", n)
	}
}

func TestConcurrentCreateSameChat(t *testing.T) {
	s := testService(t)
	errs := concurrent(t, 32, func(i int) error {
		_, err := s.Create(t.Context(), Actor{PlayerID: uno.PlayerID(i + 1), ChatID: 1}, CreateRequest{})
		return err
	})
	oneAccepted(t, errs, ErrChatOccupied)
	assertIndexes(t, s)
}
func TestConcurrentSameRevision(t *testing.T) {
	s := testService(t)
	v := position(t, s, uno.BotRules(), false)
	errs := concurrent(t, 32, func(int) error {
		_, err := s.Apply(t.Context(), Actor{PlayerID: 1}, v.GameID, uno.Action{Type: uno.DrawCard, PlayerID: 1, Revision: v.Revision})
		return err
	})
	oneAccepted(t, errs, uno.ErrStaleRevision)
	got, _ := s.PublicView(t.Context(), v.GameID)
	if got.Revision != v.Revision+1 || got.Players[0].CardCount != 2 {
		t.Fatal(got)
	}
	assertIndexes(t, s)
}
func TestConcurrentGamesAndReaders(t *testing.T) {
	s := testService(t)
	const n = 12
	games := make([]PublicGameView, n)
	for i := range games {
		games[i] = create(t, s, ChatID(i+1), 99, uno.ClassicRules())
	}
	errs := concurrent(t, n, func(i int) error {
		v := games[i]
		before, err := s.PublicView(t.Context(), v.GameID)
		if err != nil {
			return err
		}
		_, err = s.Apply(t.Context(), Actor{PlayerID: 1, ChatID: v.ChatID}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: before.Revision})
		if err != nil {
			return err
		}
		list, err := s.FindPlayerGames(t.Context(), Actor{PlayerID: 1})
		if err != nil {
			return err
		}
		for _, item := range list {
			if item.Phase != uno.Lobby || item.Revision%2 != 1 {
				return errors.New("partially published membership")
			}
		}
		hand, err := s.PlayerView(t.Context(), Actor{PlayerID: 1}, v.GameID)
		if err != nil {
			return err
		}
		if len(hand.Hand) != 0 {
			return errors.New("lobby hand")
		}
		_, err = s.Apply(t.Context(), Actor{PlayerID: 1, ChatID: v.ChatID}, v.GameID, uno.Action{Type: uno.LeaveGame, PlayerID: 1, Revision: before.Revision + 1})
		if err != nil {
			return err
		}
		return nil
	})
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertIndexes(t, s)
}

func await(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock")
	}
}

func TestPerGameIsolationAndCancellationAfterAcceptance(t *testing.T) {
	s := testService(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	s.manager.newGame = func(id uno.GameID, rules uno.Rules) (*uno.Game, error) {
		return uno.NewGame(id, rules, uno.WithShuffler(func([]uno.CardID) { close(entered); <-release }))
	}
	a := create(t, s, 1, 99, uno.ClassicRules())
	join(t, s, a, 1)
	joined := join(t, s, a, 2)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := s.Apply(ctx, Actor{PlayerID: 99, ChatID: 1}, a.GameID, uno.Action{Type: uno.StartGame, PlayerID: 99, Revision: joined.View.Revision})
		done <- err
	}()
	await(t, entered)
	// A is inside engine Apply with its entry lock. B and index lookups must finish.
	independent := make(chan struct{})
	go func() {
		defer close(independent)
		b, err := s.Create(t.Context(), Actor{PlayerID: 99, ChatID: 2}, CreateRequest{})
		if err != nil {
			t.Error(err)
			return
		}
		_, err = s.Apply(t.Context(), Actor{PlayerID: 1, ChatID: 2}, b.View.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 1})
		if err != nil {
			t.Error(err)
		}
		_, err = s.FindChatGame(t.Context(), 1)
		if err != nil {
			t.Error(err)
		}
	}()
	await(t, independent)
	cancel()
	unblock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal("accepted action must publish", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("start deadlock")
	}
	got, _ := s.PublicView(t.Context(), a.GameID)
	if got.Revision != joined.View.Revision+1 || got.Phase == uno.Lobby {
		t.Fatal(got)
	}
	assertIndexes(t, s)
}

// Observe the initial context check without timing assumptions. Returning its
// captured result guarantees Apply proceeds to the deliberately locked entry.
type observedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *observedContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}

func TestCancelledContextAfterWaitingForGame(t *testing.T) {
	s := testService(t)
	v := create(t, s, 1, 99, uno.ClassicRules())
	e := s.manager.byID[v.GameID].entry
	e.mu.Lock()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	observed := &observedContext{Context: ctx, checked: make(chan struct{})}
	go func() {
		_, err := s.Apply(observed, Actor{PlayerID: 1, ChatID: 1}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 1})
		done <- err
	}()
	<-observed.checked
	cancel()
	e.mu.Unlock()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	got, _ := s.PublicView(t.Context(), v.GameID)
	if got.Revision != 0 {
		t.Fatal("cancelled mutation")
	}
	assertIndexes(t, s)
}

func TestConcurrentCloseAndRecreate(t *testing.T) {
	for range 20 {
		s := testService(t)
		v := create(t, s, 1, 99, uno.ClassicRules())
		errs := concurrent(t, 2, func(i int) error {
			if i == 0 {
				_, err := s.Apply(t.Context(), Actor{PlayerID: 99, ChatID: 1}, v.GameID, uno.Action{Type: uno.CancelGame, PlayerID: 99})
				return err
			}
			_, err := s.Create(t.Context(), Actor{PlayerID: 99, ChatID: 1}, CreateRequest{})
			return err
		})
		for _, err := range errs {
			if err != nil && !errors.Is(err, ErrChatOccupied) {
				t.Fatal(err)
			}
		}
		assertIndexes(t, s)
	}
}

func TestViewsDuringPlacementAndClosure(t *testing.T) {
	s := testService(t)
	v := position(t, s, uno.BotRules(), false)
	errs := concurrent(t, 16, func(worker int) error {
		if worker == 0 {
			out, err := s.Apply(t.Context(), Actor{PlayerID: 1}, v.GameID, uno.Action{Type: uno.PlayCard, PlayerID: 1, CardID: "a", Revision: 10})
			if err != nil {
				return err
			}
			_, err = s.Apply(t.Context(), Actor{PlayerID: 2}, v.GameID, uno.Action{Type: uno.PlayCard, PlayerID: 2, CardID: "b", Revision: out.View.Revision})
			return err
		}
		for range 40 {
			got, err := s.PublicView(t.Context(), v.GameID)
			if err != nil {
				return err
			}
			switch got.Revision {
			case 10:
				if got.OwnerID != 1 || len(got.Placements) != 0 {
					return errors.New("partial initial view")
				}
			case 11:
				if got.OwnerID != 2 || len(got.Placements) != 1 || got.Players[0].Active {
					return errors.New("partial placement view")
				}
			case 12:
				if !got.Closed || got.OwnerID != 0 || len(got.Placements) != 3 {
					return errors.New("partial final view")
				}
			default:
				return errors.New("unexpected revision")
			}
			list, err := s.FindPlayerGames(t.Context(), Actor{PlayerID: 1})
			if err != nil {
				return err
			}
			if len(list) > 0 && list[0].Revision != 10 {
				return errors.New("finisher still routed")
			}
			hand, err := s.PlayerView(t.Context(), Actor{PlayerID: 1}, v.GameID)
			if err != nil && !errors.Is(err, ErrNotParticipant) && !errors.Is(err, ErrGameClosed) {
				return err
			}
			if err == nil {
				if hand.Public.Revision != 10 || len(hand.Hand) != 1 {
					return errors.New("incoherent private view")
				}
				hand.Hand[0].Card.ID = "caller mutation"
			}
			got.Players[0].ID = 999
			got.TopCard.ID = "caller mutation"
		}
		return nil
	})
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertIndexes(t, s)
}

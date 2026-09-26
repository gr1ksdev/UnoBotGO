//go:build debugcards

package uno

import (
	"math"
	"slices"
)

// GiveCard transfers an existing drawable card for debugging. Like Apply, it
// requires serialized access; the application must authorize the caller.
func (g *Game) GiveCard(revision uint64, target PlayerID, color Color, rank Rank) (Result, Card, error) {
	if revision != g.state.Revision {
		return Result{}, Card{}, ErrStaleRevision
	}
	if revision == math.MaxUint64 {
		return Result{}, Card{}, ErrInvalidState
	}
	if g.state.Phase != TakingTurn || g.state.DrawCounter != 0 || g.state.PendingBluff != nil {
		return Result{}, Card{}, ErrInvalidAction
	}
	if !(Card{ID: "validate", Color: color, Rank: rank}).valid() {
		return Result{}, Card{}, ErrInvalidAction
	}
	if rank == SwapHands && !g.state.Rules.AllowSwapHands {
		return Result{}, Card{}, ErrInvalidRules
	}
	s := g.state.clone()
	player := s.player(target)
	if player == nil || player.Status != Playing {
		return Result{}, Card{}, ErrUnknownPlayer
	}
	find := func(ids []CardID) (int, Card) {
		for i, id := range ids {
			c, ok := s.card(id)
			if ok && c.Color == color && c.Rank == rank {
				return i, c
			}
		}
		return -1, Card{}
	}
	index, granted := find(s.DrawPile)
	if index >= 0 {
		s.DrawPile = slices.Delete(s.DrawPile, index, index+1)
	} else {
		index, granted = find(s.DiscardPile[:len(s.DiscardPile)-1])
		if index < 0 {
			return Result{}, Card{}, ErrCardNotFound
		}
		s.DiscardPile = slices.Delete(s.DiscardPile, index, index+1)
	}
	player.Hand = append(player.Hand, granted.ID)
	s.Revision++
	if err := s.Validate(); err != nil {
		return Result{}, Card{}, err
	}
	g.state = s
	return Result{Revision: s.Revision, Events: []Event{{Type: CardsDrawn, PlayerID: target, Count: 1}}}, granted, nil
}

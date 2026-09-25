package uno

import (
	"fmt"
	"slices"
)

type Phase uint8

const (
	Lobby Phase = iota
	TakingTurn
	ChoosingColor
	Finished
	ChoosingPlayer
)

type FinishReason string

const (
	FinishedNormally       FinishReason = "completed"
	FinishedByDeparture    FinishReason = "departure"
	FinishedByCancellation FinishReason = "cancelled"
)

// ColorChoice records the pre-play evidence needed by a future challenge flow.
// Initial Wild color choice retains the chooser's turn; played Wild advances it.
type ColorChoice struct {
	Actor         PlayerID
	Target        PlayerID
	PreviousColor Color
	DrawCount     int
	Initial       bool
	Bluffing      bool
}

type BluffInfo struct {
	Actor    PlayerID
	Target   PlayerID
	Bluffing bool
}

// State is a serializable snapshot, with no locks, clock or Telegram types.
// Cards is immutable after starting; lobby rule changes may rebuild the default
// inventory. Piles/hands contain physical IDs.
// Order contains active seats only; Players retains historical participants.
type State struct {
	ID              GameID
	Revision        uint64
	Rules           Rules
	Phase           Phase
	CustomDeck      bool // Explicit WithDeck inventory; preserved across lobby rule changes.
	Cards           []Card
	DrawPile        []CardID
	DiscardPile     []CardID
	Players         []Player
	Order           []PlayerID
	DealerID        PlayerID
	CurrentPlayerID PlayerID
	Direction       int
	ActiveColor     Color
	DrawnCardID     CardID
	DrawCounter     int
	Pending         *ColorChoice
	PendingBluff    *BluffInfo
	Placements      []Placement
	FinishReason    FinishReason
}

func (s State) clone() State {
	s.Cards = slices.Clone(s.Cards)
	s.DrawPile = slices.Clone(s.DrawPile)
	s.DiscardPile = slices.Clone(s.DiscardPile)
	s.Players = slices.Clone(s.Players)
	for i := range s.Players {
		s.Players[i].Hand = slices.Clone(s.Players[i].Hand)
	}
	s.Order = slices.Clone(s.Order)
	s.Placements = slices.Clone(s.Placements)
	if s.Pending != nil {
		pending := *s.Pending
		s.Pending = &pending
	}
	if s.PendingBluff != nil {
		pb := *s.PendingBluff
		s.PendingBluff = &pb
	}
	return s
}

func (s *State) player(id PlayerID) *Player {
	for i := range s.Players {
		if s.Players[i].ID == id {
			return &s.Players[i]
		}
	}
	return nil
}

func (s *State) card(id CardID) (Card, bool) {
	for _, c := range s.Cards {
		if c.ID == id {
			return c, true
		}
	}
	return Card{}, false
}

func (s *State) next(id PlayerID, steps int) PlayerID {
	i := slices.Index(s.Order, id)
	n := len(s.Order)
	if i < 0 || n == 0 {
		return 0
	}
	return s.Order[((i+steps*s.Direction)%n+n)%n]
}

// Validate checks structural invariants for tests, debug and recovery.
func (s State) Validate() error {
	bad := func(message string) error { return fmt.Errorf("%w: %s", ErrInvalidState, message) }
	if s.ID == "" || s.Phase > ChoosingPlayer || (s.Direction != 1 && s.Direction != -1) || s.DrawCounter < 0 {
		return bad("identity, phase, direction or draw counter")
	}
	if s.Rules.EndPolicy > Placements {
		return bad("end policy")
	}
	if s.PendingBluff != nil && (s.PendingBluff.Actor <= 0 || s.PendingBluff.Target <= 0) {
		return bad("pending bluff identity")
	}
	if len(s.Cards) == 0 || len(s.Players) > 10 {
		return bad("inventory or participant limit")
	}
	inventory := make(map[CardID]Card, len(s.Cards))
	for _, c := range s.Cards {
		if _, exists := inventory[c.ID]; exists || !c.valid() || (c.Rank == SwapHands && !s.Rules.AllowSwapHands) {
			return bad("invalid or duplicate physical card")
		}
		inventory[c.ID] = c
	}
	seen := make(map[CardID]bool, len(inventory))
	checkCards := func(ids []CardID) bool {
		for _, id := range ids {
			if _, ok := inventory[id]; !ok || seen[id] {
				return false
			}
			seen[id] = true
		}
		return true
	}
	if !checkCards(s.DrawPile) || !checkCards(s.DiscardPile) {
		return bad("pile ownership")
	}
	players := make(map[PlayerID]Player, len(s.Players))
	for _, p := range s.Players {
		if _, exists := players[p.ID]; exists || p.ID <= 0 || p.Status > Left {
			return bad("player identity/status")
		}
		if p.Status != Playing && len(p.Hand) != 0 {
			return bad("inactive player holds cards")
		}
		if !checkCards(p.Hand) {
			return bad("hand ownership")
		}
		players[p.ID] = p
	}
	if len(seen) != len(inventory) {
		return bad("missing physical card")
	}
	order := make(map[PlayerID]bool, len(s.Order))
	for _, id := range s.Order {
		p, ok := players[id]
		if !ok || p.Status != Playing || order[id] {
			return bad("seat order")
		}
		order[id] = true
	}
	for _, p := range players {
		if (p.Status == Playing) != order[p.ID] {
			return bad("missing active seat")
		}
	}
	placed := make(map[PlayerID]bool)
	for i, p := range s.Placements {
		player, exists := players[p.PlayerID]
		if !exists || placed[p.PlayerID] || p.Position != i+1 || (p.WentOut && player.Status != WentOut) || (!p.WentOut && (s.Phase != Finished || player.Status != Playing)) {
			return bad("placements")
		}
		if !p.WentOut && (i != len(s.Placements)-1 || len(s.Order) != 1 || s.Order[0] != p.PlayerID || s.FinishReason == FinishedByCancellation) {
			return bad("last remaining placement")
		}
		placed[p.PlayerID] = true
	}
	for _, p := range players {
		if p.Status == WentOut && !placed[p.ID] {
			return bad("unrecorded finisher")
		}
	}
	if s.Phase == Lobby {
		if s.CurrentPlayerID != 0 || s.DealerID != 0 || s.ActiveColor != NoColor || len(s.DiscardPile) != 0 || len(s.Placements) != 0 {
			return bad("lobby state")
		}
		for _, p := range players {
			if len(p.Hand) != 0 {
				return bad("lobby hand")
			}
		}
	}
	if s.Phase == TakingTurn || s.Phase == ChoosingColor || s.Phase == ChoosingPlayer {
		if len(s.Order) < 2 || !order[s.CurrentPlayerID] || len(s.DiscardPile) == 0 {
			return bad("active game")
		}
		if _, ok := players[s.DealerID]; !ok {
			return bad("dealer")
		}
		if s.Phase != ChoosingColor && !s.ActiveColor.valid() {
			return bad("active color")
		}
		for _, p := range players {
			if p.Status == Playing && len(p.Hand) == 0 && !(s.Phase == ChoosingColor && s.Pending != nil && s.Pending.Actor == p.ID) {
				return bad("empty active hand")
			}
		}
	}
	if s.Phase == Finished {
		if s.CurrentPlayerID != 0 || s.DrawnCardID != "" || s.Pending != nil {
			return bad("finished turn")
		}
		if s.FinishReason != FinishedNormally && s.FinishReason != FinishedByDeparture && s.FinishReason != FinishedByCancellation {
			return bad("finish reason")
		}
		if s.FinishReason == FinishedNormally && (len(s.Placements) == 0 || !s.Placements[0].WentOut || (s.Rules.EndPolicy == Placements && len(s.Order) != 1)) {
			return bad("completed outcome")
		}
		if s.FinishReason == FinishedByDeparture && len(s.Order) > 1 {
			return bad("departure outcome")
		}
	} else if s.FinishReason != "" {
		return bad("premature finish reason")
	}
	if s.Phase == ChoosingPlayer {
		top := inventory[s.DiscardPile[len(s.DiscardPile)-1]]
		if !s.Rules.AllowSwapHands || top.Rank != SwapHands || s.DrawCounter != 0 || s.PendingBluff != nil {
			return bad("pending player choice")
		}
	}
	if (s.Phase == ChoosingColor) != (s.Pending != nil) {
		return bad("pending color phase")
	}
	if p := s.Pending; p != nil {
		if p.Actor != s.CurrentPlayerID || !order[p.Actor] || !order[p.Target] || p.Target != s.next(p.Actor, 1) {
			return bad("pending actors")
		}
		top := inventory[s.DiscardPile[len(s.DiscardPile)-1]]
		if (top.Rank != Wild && top.Rank != WildDrawFour) || (p.DrawCount != 0 && p.DrawCount != 4) || (top.Rank == WildDrawFour) != (p.DrawCount == 4) {
			return bad("pending wild")
		}
		if p.Initial && (top.Rank != Wild || p.PreviousColor != NoColor) {
			return bad("initial wild")
		}
		if !p.Initial && !p.PreviousColor.valid() {
			return bad("previous color")
		}
		if s.ActiveColor != p.PreviousColor {
			return bad("pending active color")
		}
	}
	if s.DrawnCardID != "" {
		p, ok := players[s.CurrentPlayerID]
		if s.Phase != TakingTurn || !ok || !slices.Contains(p.Hand, s.DrawnCardID) {
			return bad("drawn card")
		}
	}
	if len(s.DiscardPile) != 0 && s.Phase != ChoosingColor {
		top := inventory[s.DiscardPile[len(s.DiscardPile)-1]]
		if top.Rank >= Wild && !s.ActiveColor.valid() && !(s.Phase == Finished && s.FinishReason != FinishedNormally && s.ActiveColor == NoColor && top.Rank == Wild) {
			return bad("wild active color")
		}
		if top.Rank < Wild && s.ActiveColor != top.Color {
			return bad("top color")
		}
	}
	return nil
}

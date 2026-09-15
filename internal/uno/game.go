package uno

import (
	"fmt"
	"math"
	"slices"
)

// Game is owned by one caller at a time. The application manager must serialize
// Apply and Snapshot for a given game; independent games share no mutable state.
type Game struct {
	state   State
	shuffle Shuffler
}

type setup struct {
	deck    []Card
	shuffle Shuffler
}
type Option func(*setup)

// WithDeck supplies an exact inventory, including its draw order. Combine with
// WithShuffler for deterministic play. The input is copied by NewGame.
func WithDeck(cards []Card) Option         { return func(s *setup) { s.deck = slices.Clone(cards) } }
func WithShuffler(shuffle Shuffler) Option { return func(s *setup) { s.shuffle = shuffle } }

func NewGame(id GameID, rules Rules, options ...Option) (*Game, error) {
	if rules.EndPolicy > Placements {
		return nil, ErrInvalidRules
	}
	cfg := setup{deck: ClassicDeck(), shuffle: randomShuffle}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.shuffle == nil {
		cfg.shuffle = randomShuffle
	}
	s := State{ID: id, Rules: rules, Direction: 1, Cards: slices.Clone(cfg.deck)}
	for _, card := range s.Cards {
		s.DrawPile = append(s.DrawPile, card.ID)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &Game{state: s, shuffle: cfg.shuffle}, nil
}

// Restore validates and deep-copies a trusted server-side snapshot. Never accept
// snapshots from players. Supplying nil restores the standard shuffler.
func Restore(state State, shuffle Shuffler) (*Game, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	if shuffle == nil {
		shuffle = randomShuffle
	}
	return &Game{state: state.clone(), shuffle: shuffle}, nil
}

func (g *Game) Snapshot() State { return g.state.clone() }

// Apply commits exactly one revision after a successful, fully validated action.
// Rejected actions return a zero Result and leave the entire state untouched.
func (g *Game) Apply(a Action) (Result, error) {
	if g.state.Phase == Finished {
		return Result{}, ErrGameFinished
	}
	if a.Revision != g.state.Revision {
		return Result{}, ErrStaleRevision
	}
	if g.state.Revision == math.MaxUint64 {
		return Result{}, ErrInvalidState
	}
	if a.PlayerID <= 0 || a.Type < JoinGame || a.Type > CancelGame {
		return Result{}, ErrInvalidAction
	}
	if (a.Type != PlayCard && a.CardID != "") || (a.Type != ChooseColor && a.Color != NoColor) || (a.Type != StartGame && a.DealerID != 0) {
		return Result{}, ErrInvalidAction
	}
	s := g.state.clone()
	if a.Type != JoinGame {
		p := s.player(a.PlayerID)
		if p == nil || p.Status != Playing {
			return Result{}, ErrUnknownPlayer
		}
	}
	var events []Event
	var err error
	switch a.Type {
	case JoinGame:
		err = g.join(&s, a.PlayerID, &events)
	case LeaveGame:
		err = g.leave(&s, a.PlayerID, &events)
	case StartGame:
		err = g.start(&s, a.DealerID, &events)
	case CancelGame:
		finish(&s, FinishedByCancellation, &events)
	default:
		err = g.takeAction(&s, a, &events)
	}
	if err != nil {
		return Result{}, err
	}
	s.Revision++
	if err := s.Validate(); err != nil {
		return Result{}, err
	}
	g.state = s
	return Result{Revision: s.Revision, Events: events}, nil
}

func (g *Game) join(s *State, id PlayerID, events *[]Event) error {
	if s.player(id) != nil {
		return ErrAlreadyJoined
	}
	if len(s.Players) >= 10 {
		return ErrPlayerLimit
	}
	if s.Phase != Lobby && !s.Rules.AllowLateJoin {
		return ErrLobbyClosed
	}
	p := Player{ID: id}
	if s.Phase != Lobby {
		cards, err := g.draw(s, 7)
		if err != nil {
			return err
		}
		p.Hand = cards
	}
	s.Players = append(s.Players, p)
	if s.Phase == Lobby {
		s.Order = append(s.Order, id)
	} else {
		// In forward play insert before current; reversed play after current.
		i := slices.Index(s.Order, s.CurrentPlayerID)
		if s.Direction == -1 {
			i++
		}
		s.Order = slices.Insert(s.Order, i, id)
	}
	*events = append(*events, Event{Type: PlayerJoined, PlayerID: id})
	if len(p.Hand) != 0 {
		*events = append(*events, Event{Type: CardsDrawn, PlayerID: id, Count: len(p.Hand)})
	}
	return nil
}

func (g *Game) leave(s *State, id PlayerID, events *[]Event) error {
	if s.Pending != nil && s.Pending.Actor == id {
		return ErrColorChoiceRequired
	}
	p := s.player(id)
	next := s.next(id, 1)
	if len(p.Hand) > 0 {
		// Insert below the top; removed players' cards remain in the inventory.
		i := len(s.DiscardPile) - 1
		s.DiscardPile = slices.Insert(s.DiscardPile, i, p.Hand...)
	}
	p.Hand = nil
	p.Status = Left
	s.Order = slices.Delete(s.Order, slices.Index(s.Order, id), slices.Index(s.Order, id)+1)
	*events = append(*events, Event{Type: PlayerLeft, PlayerID: id})
	if s.Phase == Lobby {
		return nil
	}
	if len(s.Order) <= 1 {
		if len(s.Order) == 1 {
			recordLast(s)
		}
		finish(s, FinishedByDeparture, events)
		return nil
	}
	if s.Pending != nil && s.Pending.Target == id {
		s.Pending.Target = s.next(s.Pending.Actor, 1)
	}
	if s.CurrentPlayerID == id {
		changeTurn(s, next, events)
	}
	return nil
}

func (g *Game) start(s *State, dealer PlayerID, events *[]Event) error {
	if s.Phase != Lobby {
		return ErrGameStarted
	}
	if len(s.Order) < 2 {
		return ErrNotEnoughPlayers
	}
	if !slices.Contains(s.Order, dealer) {
		return ErrUnknownPlayer
	}
	if len(s.DrawPile) < len(s.Order)*7+1 {
		return ErrDeckEmpty
	}
	g.shuffle(s.DrawPile)
	// Round-robin dealing starts to the dealer's left.
	for range 7 {
		for i := 1; i <= len(s.Order); i++ {
			cards, err := g.draw(s, 1)
			if err != nil {
				return err
			}
			p := s.player(s.next(dealer, i))
			p.Hand = append(p.Hand, cards...)
		}
	}
	// A finite search handles custom decks containing only +4 after dealing.
	var top Card
	found := false
	skipped := false
	for i, id := range s.DrawPile {
		c, _ := s.card(id)
		if c.Rank != WildDrawFour {
			top = c
			skipped = i > 0
			s.DrawPile = slices.Delete(s.DrawPile, i, i+1)
			found = true
			break
		}
	}
	if !found {
		return ErrDeckEmpty
	}
	// Rejected initial +4 cards remain in the draw pile; reshuffle if any were
	// bypassed so they aren't repeatedly exposed as the very next draw.
	// Exact injected order is otherwise preserved.
	if skipped {
		g.shuffle(s.DrawPile)
	}
	s.DiscardPile = []CardID{top.ID}
	s.DealerID = dealer
	s.CurrentPlayerID = s.next(dealer, 1)
	s.Phase = TakingTurn
	s.ActiveColor = top.Color
	*events = append(*events, Event{Type: GameStarted})
	for _, id := range s.Order {
		*events = append(*events, Event{Type: CardsDrawn, PlayerID: id, Count: 7})
	}
	switch top.Rank {
	case Reverse:
		s.Direction = -1
		s.CurrentPlayerID = dealer
		*events = append(*events, Event{Type: DirectionChanged, Direction: -1})
	case Skip:
		*events = append(*events, Event{Type: PlayerSkipped, PlayerID: s.CurrentPlayerID})
		s.CurrentPlayerID = s.next(s.CurrentPlayerID, 1)
	case DrawTwo:
		if err := g.penalty(s, s.CurrentPlayerID, 2, events); err != nil {
			return err
		}
		s.CurrentPlayerID = s.next(s.CurrentPlayerID, 1)
	case Wild:
		s.Phase = ChoosingColor
		s.Pending = &ColorChoice{Actor: s.CurrentPlayerID, Target: s.next(s.CurrentPlayerID, 1), Initial: true}
		*events = append(*events, Event{Type: ColorChoiceRequired, PlayerID: s.CurrentPlayerID})
	}
	*events = append(*events, Event{Type: TurnChanged, PlayerID: s.CurrentPlayerID})
	return nil
}

func (g *Game) takeAction(s *State, a Action, events *[]Event) error {
	if s.Phase == Lobby {
		return ErrGameNotStarted
	}
	if a.Type == ChooseColor && s.Pending == nil {
		return ErrNoColorChoice
	}
	if a.PlayerID != s.CurrentPlayerID {
		return ErrNotYourTurn
	}
	if s.Pending != nil {
		if a.Type != ChooseColor {
			return ErrColorChoiceRequired
		}
		return g.choose(s, a.Color, events)
	}
	switch a.Type {
	case PlayCard:
		return g.play(s, a.CardID, events)
	case DrawCard:
		if s.DrawnCardID != "" {
			return ErrAlreadyDrawn
		}
		cards, err := g.draw(s, 1)
		if err != nil {
			return err
		}
		p := s.player(a.PlayerID)
		p.Hand = append(p.Hand, cards...)
		s.DrawnCardID = cards[0]
		*events = append(*events, Event{Type: CardsDrawn, PlayerID: p.ID, Count: 1})
		if playable(s, p.ID, cards[0]) != nil {
			changeTurn(s, s.next(p.ID, 1), events)
		}
		return nil
	case PassTurn:
		if s.DrawnCardID == "" {
			return ErrCannotPass
		}
		changeTurn(s, s.next(a.PlayerID, 1), events)
		return nil
	}
	return ErrInvalidAction
}

// CanPlay applies the same validation used by Apply, without mutation.
func (g *Game) CanPlay(player PlayerID, card CardID) error { return playable(&g.state, player, card) }

func playable(s *State, player PlayerID, id CardID) error {
	if s.Phase == Finished {
		return ErrGameFinished
	}
	if s.Phase == Lobby {
		return ErrGameNotStarted
	}
	p := s.player(player)
	if p == nil || p.Status != Playing {
		return ErrUnknownPlayer
	}
	if player != s.CurrentPlayerID {
		return ErrNotYourTurn
	}
	if s.Pending != nil {
		return ErrColorChoiceRequired
	}
	card, ok := s.card(id)
	if !ok {
		return ErrCardNotFound
	}
	if !slices.Contains(p.Hand, id) {
		return ErrCardNotOwned
	}
	if s.DrawnCardID != "" && s.DrawnCardID != id {
		return ErrCardNotPlayable
	}
	if card.Rank == WildDrawFour {
		hand := make([]Card, 0, len(p.Hand))
		for _, id := range p.Hand {
			c, _ := s.card(id)
			hand = append(hand, c)
		}
		if !CanPlayDrawFour(hand, s.ActiveColor) {
			return ErrCardNotPlayable
		}
		return nil
	}
	if card.Rank == Wild {
		return nil
	}
	top, _ := s.card(s.DiscardPile[len(s.DiscardPile)-1])
	if card.Color == s.ActiveColor || card.Rank == top.Rank {
		return nil
	}
	return ErrCardNotPlayable
}

func (g *Game) play(s *State, id CardID, events *[]Event) error {
	actor := s.CurrentPlayerID
	if err := playable(s, actor, id); err != nil {
		return err
	}
	card, _ := s.card(id)
	p := s.player(actor)
	i := slices.Index(p.Hand, id)
	p.Hand = slices.Delete(p.Hand, i, i+1)
	s.DiscardPile = append(s.DiscardPile, id)
	s.DrawnCardID = ""
	*events = append(*events, Event{Type: CardPlayed, PlayerID: actor, CardID: id})
	if len(p.Hand) == 1 {
		*events = append(*events, Event{Type: UnoAnnounced, PlayerID: actor})
	}
	if card.Rank >= Wild {
		count := 0
		if card.Rank == WildDrawFour {
			count = 4
		}
		s.Pending = &ColorChoice{Actor: actor, Target: s.next(actor, 1), PreviousColor: s.ActiveColor, DrawCount: count}
		s.Phase = ChoosingColor
		*events = append(*events, Event{Type: ColorChoiceRequired, PlayerID: actor})
		return nil
	}
	s.ActiveColor = card.Color
	next := s.next(actor, 1)
	switch card.Rank {
	case Skip:
		*events = append(*events, Event{Type: PlayerSkipped, PlayerID: next})
		next = s.next(actor, 2)
	case Reverse:
		if len(s.Order) == 2 {
			*events = append(*events, Event{Type: PlayerSkipped, PlayerID: next})
			next = actor
		} else {
			s.Direction *= -1
			*events = append(*events, Event{Type: DirectionChanged, Direction: s.Direction})
			next = s.next(actor, 1)
		}
	case DrawTwo:
		if err := g.penalty(s, next, 2, events); err != nil {
			return err
		}
		next = s.next(actor, 2)
	}
	completePlay(s, actor, next, events)
	return nil
}

func (g *Game) choose(s *State, color Color, events *[]Event) error {
	if !color.valid() {
		return ErrInvalidColor
	}
	pending := *s.Pending
	s.ActiveColor = color
	s.Pending = nil
	s.Phase = TakingTurn
	*events = append(*events, Event{Type: ColorChosen, PlayerID: pending.Actor, Color: color})
	if pending.Initial {
		return nil
	}
	next := pending.Target
	if pending.DrawCount != 0 {
		if err := g.penalty(s, next, pending.DrawCount, events); err != nil {
			return err
		}
		next = s.next(next, 1)
	}
	completePlay(s, pending.Actor, next, events)
	return nil
}

func (g *Game) penalty(s *State, player PlayerID, count int, events *[]Event) error {
	cards, err := g.draw(s, count)
	if err != nil {
		return err
	}
	p := s.player(player)
	if p == nil {
		return fmt.Errorf("%w: penalty target", ErrInvalidState)
	}
	p.Hand = append(p.Hand, cards...)
	*events = append(*events, Event{Type: CardsDrawn, PlayerID: player, Count: count}, Event{Type: PlayerSkipped, PlayerID: player})
	return nil
}

func completePlay(s *State, actor, next PlayerID, events *[]Event) {
	p := s.player(actor)
	if len(p.Hand) == 0 {
		fallback := s.next(actor, 1)
		p.Status = WentOut
		s.Placements = append(s.Placements, Placement{PlayerID: actor, Position: len(s.Placements) + 1, WentOut: true})
		*events = append(*events, Event{Type: PlayerWon, PlayerID: actor, Position: len(s.Placements)})
		i := slices.Index(s.Order, actor)
		s.Order = slices.Delete(s.Order, i, i+1)
		if s.Rules.EndPolicy == FirstWinner || len(s.Order) == 1 {
			if s.Rules.EndPolicy == Placements {
				recordLast(s)
			}
			finish(s, FinishedNormally, events)
			return
		}
		if next == actor {
			next = fallback
		}
	}
	changeTurn(s, next, events)
}

func recordLast(s *State) {
	s.Placements = append(s.Placements, Placement{PlayerID: s.Order[0], Position: len(s.Placements) + 1})
}

func changeTurn(s *State, player PlayerID, events *[]Event) {
	s.CurrentPlayerID = player
	s.DrawnCardID = ""
	*events = append(*events, Event{Type: TurnChanged, PlayerID: player})
}

func finish(s *State, reason FinishReason, events *[]Event) {
	s.Phase = Finished
	s.CurrentPlayerID = 0
	s.DrawnCardID = ""
	s.Pending = nil
	s.FinishReason = reason
	*events = append(*events, Event{Type: GameFinished, Reason: reason})
}

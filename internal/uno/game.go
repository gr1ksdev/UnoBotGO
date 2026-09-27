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
	customDeck bool
	deck       []Card
	shuffle    Shuffler
}
type Option func(*setup)

// WithDeck supplies an exact inventory, including its draw order. Combine with
// WithShuffler for deterministic play. The input is copied by NewGame.
func WithDeck(cards []Card) Option {
	return func(s *setup) {
		s.deck = slices.Clone(cards)
		s.customDeck = true
	}
}
func WithShuffler(shuffle Shuffler) Option { return func(s *setup) { s.shuffle = shuffle } }

func NewGame(id GameID, rules Rules, options ...Option) (*Game, error) {
	if rules.EndPolicy > Placements {
		return nil, ErrInvalidRules
	}
	cfg := setup{deck: deckForRules(rules), shuffle: randomShuffle}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.shuffle == nil {
		cfg.shuffle = randomShuffle
	}
	s := State{ID: id, Rules: rules, CustomDeck: cfg.customDeck, Direction: 1, Cards: slices.Clone(cfg.deck)}
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
	if a.PlayerID <= 0 || a.Type < JoinGame || a.Type > ChoosePlayer {
		return Result{}, ErrInvalidAction
	}
	if (a.Type != PlayCard && a.CardID != "") || (a.Type != ChooseColor && a.Color != NoColor) || (a.Type != StartGame && a.DealerID != 0) || (a.Type != ChoosePlayer && a.TargetID != 0) {
		return Result{}, ErrInvalidAction
	}
	s := g.state.clone()
	if a.Type != JoinGame && a.Type != StartGame && a.Type != CancelGame && a.Type != SetRules {
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
	case SetRules:
		err = g.setRules(&s, a, &events)
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

func (g *Game) setRules(s *State, a Action, events *[]Event) error {
	if s.Phase != Lobby {
		return ErrGameStarted
	}
	if a.Rules.EndPolicy > Placements {
		return ErrInvalidRules
	}
	if !s.CustomDeck {
		s.Cards = deckForRules(a.Rules)
		s.DrawPile = make([]CardID, 0, len(s.Cards))
		for _, card := range s.Cards {
			s.DrawPile = append(s.DrawPile, card.ID)
		}
	} else if !a.Rules.AllowSwapHands {
		for _, card := range s.Cards {
			if card.Rank == SwapHands {
				return ErrInvalidRules
			}
		}
	}
	s.Rules = a.Rules
	*events = append(*events, Event{Type: RulesChanged, PlayerID: a.PlayerID})
	return nil
}

func (g *Game) join(s *State, id PlayerID, events *[]Event) error {
	if s.HasPlacement(id) {
		return ErrAlreadyFinished
	}
	if p := s.player(id); p != nil && p.Status == Playing {
		return ErrAlreadyJoined
	}
	if s.player(id) == nil && len(s.Players) >= 10 {
		return ErrPlayerLimit
	}
	if s.Phase != Lobby && !s.Rules.AllowLateJoin {
		return ErrLobbyClosed
	}
	var p *Player
	if existing := s.player(id); existing != nil {
		p = existing
		p.Status = Playing
		p.Hand = nil
	} else {
		s.Players = append(s.Players, Player{ID: id, Status: Playing})
		p = &s.Players[len(s.Players)-1]
	}
	if s.Phase != Lobby {
		cards, err := g.draw(s, 7)
		if err != nil {
			return err
		}
		p.Hand = cards
	}
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
	if s.Phase == ChoosingPlayer && s.CurrentPlayerID == id {
		return ErrPlayerChoiceRequired
	}
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
	if s.PendingBluff != nil && (s.PendingBluff.Actor == id || s.PendingBluff.Target == id) {
		s.PendingBluff = nil
		s.DrawFourChallengeable = false
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
		if s.Rules.NumberedStart && c.Rank >= Skip {
			continue
		}
		if c.Rank != WildDrawFour && c.Rank != SwapHands {
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
		if s.Rules.StackDrawTwo {
			s.DrawCounter = 2
		} else {
			if err := g.penalty(s, s.CurrentPlayerID, 2, events); err != nil {
				return err
			}
			s.CurrentPlayerID = s.next(s.CurrentPlayerID, 1)
		}
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
	if s.Phase == ChoosingPlayer {
		if a.Type != ChoosePlayer {
			return ErrPlayerChoiceRequired
		}
		return g.swapHands(s, a.TargetID, events)
	}
	if s.Pending != nil {
		if a.Type != ChooseColor {
			return ErrColorChoiceRequired
		}
		return g.choose(s, a.Color, events)
	}
	if a.Type == SkipTurn {
		if s.Phase != TakingTurn {
			return ErrInvalidAction
		}
		*events = append(*events, Event{Type: PlayerSkipped, PlayerID: a.PlayerID})
		changeTurn(s, s.next(a.PlayerID, 1), events)
		return nil
	}
	switch a.Type {
	case PlayCard:
		return g.play(s, a.CardID, events)
	case DrawCard:
		s.PendingBluff = nil
		s.DrawFourChallengeable = false
		if s.DrawnCardID != "" {
			return ErrAlreadyDrawn
		}
		if s.DrawCounter > 0 {
			count := s.DrawCounter
			cards, err := g.draw(s, count)
			if err != nil {
				return err
			}
			p := s.player(a.PlayerID)
			p.Hand = append(p.Hand, cards...)
			s.DrawCounter = 0
			*events = append(*events, Event{Type: CardsDrawn, PlayerID: p.ID, Count: count})
			changeTurn(s, s.next(p.ID, 1), events)
			return nil
		}
		cards, err := g.draw(s, 1)
		if err != nil {
			return err
		}
		p := s.player(a.PlayerID)
		p.Hand = append(p.Hand, cards...)
		s.DrawnCardID = cards[0]
		*events = append(*events, Event{Type: CardsDrawn, PlayerID: p.ID, Count: 1})
		if !s.Rules.FreePlayAfterDraw && playable(s, p.ID, cards[0]) != nil {
			changeTurn(s, s.next(p.ID, 1), events)
		}
		return nil
	case CallBluff:
		if s.PendingBluff == nil || !s.DrawFourChallengeable || s.PendingBluff.Target != a.PlayerID || s.DrawCounter == 0 {
			return ErrInvalidAction
		}
		return g.bluff(s, a.PlayerID, events)
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
	if s.Phase == ChoosingPlayer {
		return ErrPlayerChoiceRequired
	}
	card, ok := s.card(id)
	if !ok {
		return ErrCardNotFound
	}
	if !slices.Contains(p.Hand, id) {
		return ErrCardNotOwned
	}
	if !s.Rules.FreePlayAfterDraw && s.DrawnCardID != "" && s.DrawnCardID != id {
		return ErrCardNotPlayable
	}
	if card.Rank == SwapHands && (!s.Rules.AllowSwapHands || len(p.Hand) == 1) {
		return ErrCardNotPlayable
	}
	if s.Rules.NoWildFinish && len(p.Hand) == 1 && card.Rank >= Wild {
		return ErrCardNotPlayable
	}
	if s.DrawCounter > 0 {
		top, _ := s.card(s.DiscardPile[len(s.DiscardPile)-1])
		if card.Rank == WildDrawFour {
			if top.Rank == WildDrawFour && s.Rules.StackWildDrawFour {
				return nil
			}
			if top.Rank == DrawTwo && s.Rules.StackWildDrawFourOnTwo {
				return nil
			}
			return ErrCardNotPlayable
		}
		if card.Rank == DrawTwo {
			if top.Rank == WildDrawFour {
				if s.Rules.StackDrawTwoOnWildFour && card.Color == s.ActiveColor {
					return nil
				}
				return ErrCardNotPlayable
			}
			return nil
		}
		return ErrCardNotPlayable
	}
	top, _ := s.card(s.DiscardPile[len(s.DiscardPile)-1])
	if s.Rules.NoWildOnWild && top.Rank >= Wild && card.Rank >= Wild {
		return ErrCardNotPlayable
	}
	if card.Rank == WildDrawFour {
		if !s.Rules.AllowWildDrawFourAlways {
			hand := make([]Card, 0, len(p.Hand))
			for _, id := range p.Hand {
				c, _ := s.card(id)
				hand = append(hand, c)
			}
			if !CanPlayDrawFour(hand, s.ActiveColor) {
				return ErrCardNotPlayable
			}
		}
		return nil
	}
	if card.Rank == Wild || card.Rank == SwapHands {
		return nil
	}
	if card.Color == s.ActiveColor || card.Rank == top.Rank {
		return nil
	}
	return ErrCardNotPlayable
}

func (g *Game) play(s *State, id CardID, events *[]Event) error {
	s.PendingBluff = nil
	s.DrawFourChallengeable = false
	actor := s.CurrentPlayerID
	if err := playable(s, actor, id); err != nil {
		return err
	}
	card, _ := s.card(id)
	top, _ := s.card(s.DiscardPile[len(s.DiscardPile)-1])
	stackedOnDrawTwo := s.DrawCounter > 0 && top.Rank == DrawTwo && s.Rules.StackWildDrawFourOnTwo
	p := s.player(actor)
	i := slices.Index(p.Hand, id)
	p.Hand = slices.Delete(p.Hand, i, i+1)
	s.DiscardPile = append(s.DiscardPile, id)
	s.DrawnCardID = ""
	*events = append(*events, Event{Type: CardPlayed, PlayerID: actor, CardID: id})
	if card.Rank == SwapHands {
		s.Phase = ChoosingPlayer
		*events = append(*events, Event{Type: PlayerChoiceRequired, PlayerID: actor})
		return nil
	}
	if len(p.Hand) == 1 {
		*events = append(*events, Event{Type: UnoAnnounced, PlayerID: actor})
	}
	if card.Rank >= Wild {
		count := 0
		bluffing := false
		challengeable := false
		if card.Rank == WildDrawFour {
			count = 4
			if stackedOnDrawTwo {
				challengeable = false
				bluffing = false
			} else {
				challengeable = true
				for _, hid := range p.Hand {
					c, ok := s.card(hid)
					if ok && c.Color == s.ActiveColor {
						bluffing = true
						break
					}
				}
			}
		}
		s.Pending = &ColorChoice{
			Actor:                 actor,
			Target:                s.next(actor, 1),
			PreviousColor:         s.ActiveColor,
			DrawCount:             count,
			Bluffing:              bluffing,
			DrawFourChallengeable: challengeable,
		}
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
		if s.Rules.StackDrawTwo {
			s.DrawCounter += 2
			next = s.next(actor, 1)
		} else {
			if err := g.penalty(s, next, 2, events); err != nil {
				return err
			}
			next = s.next(actor, 2)
		}
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
		if s.Rules.StackWildDrawFour || s.Rules.StackWildDrawFourOnTwo || s.Rules.StackDrawTwoOnWildFour {
			s.DrawCounter += pending.DrawCount
			if pending.DrawFourChallengeable {
				s.DrawFourChallengeable = true
				s.PendingBluff = &BluffInfo{Actor: pending.Actor, Target: next, Bluffing: pending.Bluffing}
			} else {
				s.DrawFourChallengeable = false
				s.PendingBluff = nil
			}
		} else {
			if err := g.penalty(s, next, pending.DrawCount, events); err != nil {
				return err
			}
			next = s.next(next, 1)
		}
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
	s.PendingBluff = nil
	s.DrawFourChallengeable = false
	s.FinishReason = reason
	*events = append(*events, Event{Type: GameFinished, Reason: reason})
}

func (g *Game) bluff(s *State, challenger PlayerID, events *[]Event) error {
	bluff := *s.PendingBluff
	s.PendingBluff = nil
	s.DrawFourChallengeable = false
	if bluff.Bluffing {
		count := s.DrawCounter
		cards, err := g.draw(s, count)
		if err != nil {
			return err
		}
		bluffer := s.player(bluff.Actor)
		if bluffer != nil {
			bluffer.Hand = append(bluffer.Hand, cards...)
		}
		s.DrawCounter = 0
		*events = append(*events,
			Event{Type: BluffCalled, PlayerID: challenger, TargetID: bluff.Actor, Success: true, Count: count},
			Event{Type: CardsDrawn, PlayerID: bluff.Actor, Count: count},
		)
	} else {
		count := s.DrawCounter + 2
		cards, err := g.draw(s, count)
		if err != nil {
			return err
		}
		p := s.player(challenger)
		p.Hand = append(p.Hand, cards...)
		s.DrawCounter = 0
		*events = append(*events,
			Event{Type: BluffCalled, PlayerID: challenger, TargetID: bluff.Actor, Success: false, Count: count},
			Event{Type: CardsDrawn, PlayerID: challenger, Count: count},
		)
	}
	next := s.next(challenger, 1)
	changeTurn(s, next, events)
	return nil
}

// swapHands commits only after the chooser and target have been validated.
func (g *Game) swapHands(s *State, target PlayerID, events *[]Event) error {
	actor := s.CurrentPlayerID
	other := s.player(target)
	if target == actor || other == nil || other.Status != Playing {
		return ErrInvalidSwapTarget
	}
	player := s.player(actor)
	player.Hand, other.Hand = other.Hand, player.Hand
	s.Phase = TakingTurn
	*events = append(*events, Event{Type: HandsSwapped, PlayerID: actor, TargetID: target})
	for _, p := range []*Player{player, other} {
		if len(p.Hand) == 1 {
			*events = append(*events, Event{Type: UnoAnnounced, PlayerID: p.ID})
		}
	}
	changeTurn(s, s.next(actor, 1), events)
	return nil
}

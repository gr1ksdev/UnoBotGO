package uno

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func noShuffle([]CardID)               {}
func card(color Color, rank Rank) Card { return Card{Color: color, Rank: rank} }

// scenario builds a validated exact state. Repeated appearances get different IDs.
func scenario(t *testing.T, rules Rules, hands [][]Card, top Card, deck []Card) *Game {
	t.Helper()
	s := State{ID: "test", Rules: rules, Phase: TakingTurn, Direction: 1, DealerID: PlayerID(len(hands)), CurrentPlayerID: 1, ActiveColor: top.Color}
	if top.Rank >= Wild {
		s.ActiveColor = Red
	}
	add := func(c Card) CardID {
		c.ID = CardID(fmt.Sprintf("c%d", len(s.Cards)+1))
		s.Cards = append(s.Cards, c)
		return c.ID
	}
	for i, hand := range hands {
		p := Player{ID: PlayerID(i + 1)}
		for _, c := range hand {
			p.Hand = append(p.Hand, add(c))
		}
		s.Players = append(s.Players, p)
		s.Order = append(s.Order, p.ID)
	}
	s.DiscardPile = []CardID{add(top)}
	if deck == nil {
		for range 24 {
			deck = append(deck, card(Blue, Nine))
		}
	}
	for _, c := range deck {
		s.DrawPile = append(s.DrawPile, add(c))
	}
	g, err := Restore(s, noShuffle)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func apply(t *testing.T, g *Game, a Action) Result {
	t.Helper()
	a.Revision = g.Snapshot().Revision
	r, err := g.Apply(a)
	if err != nil {
		t.Fatalf("action %+v: %v", a, err)
	}
	if err := g.Snapshot().Validate(); err != nil {
		t.Fatal(err)
	}
	if r.Revision != a.Revision+1 {
		t.Fatalf("revision %d", r.Revision)
	}
	return r
}

func rejected(t *testing.T, g *Game, a Action, want error) {
	t.Helper()
	before := g.Snapshot()
	r, err := g.Apply(a)
	if !errors.Is(err, want) {
		t.Fatalf("action %+v: got %v, want %v", a, err, want)
	}
	if r.Revision != 0 || len(r.Events) != 0 {
		t.Fatalf("rejected action emitted result %+v", r)
	}
	if !reflect.DeepEqual(before, g.Snapshot()) {
		t.Fatal("rejected action mutated state")
	}
}

func hasEvent(r Result, kind EventType, player PlayerID) bool {
	return slices.ContainsFunc(r.Events, func(e Event) bool { return e.Type == kind && e.PlayerID == player })
}

func TestClassicDeck(t *testing.T) {
	deck := ClassicDeck()
	if len(deck) != 108 {
		t.Fatal(len(deck))
	}
	counts := make(map[[2]uint8]int)
	ids := make(map[CardID]bool)
	for _, c := range deck {
		if !c.valid() || ids[c.ID] {
			t.Fatalf("invalid card %+v", c)
		}
		ids[c.ID] = true
		counts[[2]uint8{uint8(c.Color), uint8(c.Rank)}]++
	}
	for color := Red; color <= Yellow; color++ {
		for rank := Zero; rank <= DrawTwo; rank++ {
			want := 2
			if rank == Zero {
				want = 1
			}
			if counts[[2]uint8{uint8(color), uint8(rank)}] != want {
				t.Fatalf("composition %d/%d", color, rank)
			}
		}
	}
	for _, rank := range []Rank{Wild, WildDrawFour} {
		if counts[[2]uint8{0, uint8(rank)}] != 4 {
			t.Fatal("wild count")
		}
	}
}

func TestStartAndDeal(t *testing.T) {
	deck := ClassicDeck()
	deck[21], deck[25] = deck[25], deck[21]
	g, err := NewGame("round", ClassicRules(), WithDeck(deck), WithShuffler(noShuffle))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []PlayerID{1, 2, 3} {
		apply(t, g, Action{Type: JoinGame, PlayerID: id})
	}
	r := apply(t, g, Action{Type: StartGame, PlayerID: 1, DealerID: 3})
	s := g.Snapshot()
	if !hasEvent(r, GameStarted, 0) || s.CurrentPlayerID != 1 {
		t.Fatal(s.CurrentPlayerID, r)
	}
	if len(s.DrawPile) != 86 || len(s.DiscardPile) != 1 {
		t.Fatal("deal count")
	}
	for i, p := range s.Players {
		if len(p.Hand) != 7 || p.Hand[0] != s.Cards[i].ID {
			t.Fatal("not round-robin")
		}
	}
	if s.DrawPile[0] != s.Cards[22].ID {
		t.Fatal("draw order")
	}
}

func TestInitialActionCards(t *testing.T) {
	for _, tt := range []struct {
		name      string
		rank      Rank
		current   PlayerID
		direction int
		penalty   int
		color     bool
	}{
		{"number", Five, 1, 1, 0, false}, {"skip", Skip, 2, 1, 0, false},
		{"reverse", Reverse, 3, -1, 0, false}, {"draw two", DrawTwo, 2, 1, 2, false},
		{"wild", Wild, 1, 1, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deck := make([]Card, 0, 40)
			for range 21 {
				deck = append(deck, card(Blue, Nine))
			}
			color := Red
			if tt.rank == Wild {
				color = NoColor
			}
			deck = append(deck, card(color, tt.rank))
			for range 12 {
				deck = append(deck, card(Green, One))
			}
			for i := range deck {
				deck[i].ID = CardID(fmt.Sprintf("d%d", i))
			}
			g, err := NewGame("initial", ClassicRules(), WithDeck(deck), WithShuffler(noShuffle))
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []PlayerID{1, 2, 3} {
				apply(t, g, Action{Type: JoinGame, PlayerID: id})
			}
			apply(t, g, Action{Type: StartGame, PlayerID: 1, DealerID: 3})
			s := g.Snapshot()
			if s.CurrentPlayerID != tt.current || s.Direction != tt.direction || len(s.Players[0].Hand) != 7+tt.penalty {
				t.Fatalf("state %+v", s)
			}
			if tt.color {
				apply(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Yellow})
				if g.Snapshot().CurrentPlayerID != 1 || g.Snapshot().ActiveColor != Yellow {
					t.Fatal("initial wild advanced turn")
				}
			}
		})
	}
}

func TestInitialDrawFourReturnedAndImpossibleStart(t *testing.T) {
	for _, onlyFour := range []bool{false, true} {
		t.Run(fmt.Sprint(onlyFour), func(t *testing.T) {
			var deck []Card
			for range 14 {
				deck = append(deck, card(Red, One))
			}
			deck = append(deck, card(NoColor, WildDrawFour))
			if !onlyFour {
				deck = append(deck, card(Yellow, Two))
			}
			for i := range deck {
				deck[i].ID = CardID(fmt.Sprint(i + 1))
			}
			g, err := NewGame("initial", ClassicRules(), WithDeck(deck), WithShuffler(noShuffle))
			if err != nil {
				t.Fatal(err)
			}
			apply(t, g, Action{Type: JoinGame, PlayerID: 1})
			apply(t, g, Action{Type: JoinGame, PlayerID: 2})
			a := Action{Type: StartGame, PlayerID: 1, DealerID: 2, Revision: g.Snapshot().Revision}
			if onlyFour {
				rejected(t, g, a, ErrDeckEmpty)
				return
			}
			apply(t, g, a)
			if !slices.Contains(g.Snapshot().DrawPile, deck[14].ID) || g.Snapshot().DiscardPile[0] != deck[15].ID {
				t.Fatal("initial +4 lost")
			}
		})
	}
}

func TestPlayRules(t *testing.T) {
	for _, tt := range []struct {
		name      string
		play      Card
		top       Card
		players   int
		reversed  bool
		next      PlayerID
		direction int
		penalty   int
		fail      bool
	}{
		{"color", card(Red, One), card(Red, Five), 3, false, 2, 1, 0, false},
		{"rank", card(Blue, Five), card(Red, Five), 3, false, 2, 1, 0, false},
		{"invalid", card(Blue, One), card(Red, Five), 3, false, 1, 1, 0, true},
		{"skip", card(Red, Skip), card(Red, Five), 3, false, 3, 1, 0, false},
		{"reverse", card(Red, Reverse), card(Red, Five), 3, false, 3, -1, 0, false},
		{"reverse twice", card(Red, Reverse), card(Red, Five), 3, true, 2, 1, 0, false},
		{"reverse two", card(Red, Reverse), card(Red, Five), 2, false, 1, 1, 0, false},
		{"skip two", card(Red, Skip), card(Red, Five), 2, false, 1, 1, 0, false},
		{"draw two", card(Red, DrawTwo), card(Red, Five), 3, false, 3, 1, 2, false},
		{"draw two duel", card(Red, DrawTwo), card(Red, Five), 2, false, 1, 1, 2, false},
		{"draw two reversed", card(Red, DrawTwo), card(Red, Five), 3, true, 2, -1, 2, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hands := [][]Card{{tt.play, card(Green, Nine)}}
			for i := 1; i < tt.players; i++ {
				hands = append(hands, []Card{card(Yellow, One)})
			}
			g := scenario(t, ClassicRules(), hands, tt.top, nil)
			if tt.reversed {
				g.state.Direction = -1
			}
			id := g.Snapshot().Players[0].Hand[0]
			if tt.fail {
				rejected(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: id}, ErrCardNotPlayable)
				return
			}
			r := apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: id})
			s := g.Snapshot()
			if s.CurrentPlayerID != tt.next || s.Direction != tt.direction {
				t.Fatalf("turn/direction %d/%d", s.CurrentPlayerID, s.Direction)
			}
			if !hasEvent(r, UnoAnnounced, 1) {
				t.Fatal("missing automatic UNO")
			}
			if tt.penalty > 0 {
				target := 1
				if tt.reversed {
					target = tt.players - 1
				}
				if len(s.Players[target].Hand) != 1+tt.penalty {
					t.Fatal("penalty count")
				}
			}
		})
	}
}

func TestWildAndDrawFour(t *testing.T) {
	for _, rank := range []Rank{Wild, WildDrawFour} {
		for _, size := range []int{2, 3} {
			t.Run(fmt.Sprintf("%d/%d", rank, size), func(t *testing.T) {
				hands := [][]Card{{card(NoColor, rank), card(Green, Five)}, {card(Blue, One)}}
				if size == 3 {
					hands = append(hands, []Card{card(Yellow, One)})
				}
				g := scenario(t, ClassicRules(), hands, card(Red, Five), nil)
				apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.Snapshot().Players[0].Hand[0]})
				pending := g.Snapshot()
				if pending.Phase != ChoosingColor || pending.Pending.Actor != 1 || pending.Pending.Target != 2 || pending.Pending.PreviousColor != Red {
					t.Fatal("pending evidence")
				}
				rejected(t, g, Action{Type: DrawCard, PlayerID: 1, Revision: pending.Revision}, ErrColorChoiceRequired)
				rejected(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: NoColor, Revision: pending.Revision}, ErrInvalidColor)
				apply(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Yellow})
				s := g.Snapshot()
				next := PlayerID(2)
				if rank == WildDrawFour {
					next = PlayerID(size%2 + 1)
					if size == 3 {
						next = 3
					}
					if len(s.Players[1].Hand) != 5 {
						t.Fatal("+4 count")
					}
				}
				if s.CurrentPlayerID != next || s.ActiveColor != Yellow {
					t.Fatal("wild turn/color")
				}
				c, _ := s.card(s.DiscardPile[len(s.DiscardPile)-1])
				if c.Color != NoColor {
					t.Fatal("physical wild mutated")
				}
			})
		}
	}
}

func TestDrawFourRestriction(t *testing.T) {
	for _, tt := range []struct {
		name  string
		extra Card
		want  bool
	}{
		{"matching color", card(Red, One), false}, {"matching action color", card(Red, Skip), false},
		{"only matching rank", card(Blue, Five), true}, {"another wild", card(NoColor, Wild), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hand := []Card{card(NoColor, WildDrawFour), tt.extra}
			if CanPlayDrawFour(hand, Red) != tt.want {
				t.Fatal("pure restriction")
			}
			g := scenario(t, ClassicRules(), [][]Card{hand, {card(Blue, One)}}, card(Red, Five), nil)
			before := g.Snapshot()
			id := before.Players[0].Hand[0]
			err := g.CanPlay(1, id)
			if (err == nil) != tt.want || !reflect.DeepEqual(before, g.Snapshot()) {
				t.Fatal("query mutation or disagreement")
			}
			if tt.want {
				apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: id})
			} else {
				rejected(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: id}, ErrCardNotPlayable)
			}
		})
	}
	if CanPlayDrawFour(nil, NoColor) {
		t.Fatal("invalid active color")
	}
}

func TestDrawAndPass(t *testing.T) {
	for _, playableDraw := range []bool{false, true} {
		t.Run(fmt.Sprint(playableDraw), func(t *testing.T) {
			drawn := card(Blue, One)
			if playableDraw {
				drawn = card(Red, Two)
			}
			g := scenario(t, ClassicRules(), [][]Card{{card(Red, One)}, {card(Blue, Five)}}, card(Red, Five), []Card{drawn})
			rejected(t, g, Action{Type: PassTurn, PlayerID: 1}, ErrCannotPass)
			oldCard := g.Snapshot().Players[0].Hand[0]
			apply(t, g, Action{Type: DrawCard, PlayerID: 1})
			s := g.Snapshot()
			if !playableDraw {
				if s.CurrentPlayerID != 2 || s.DrawnCardID != "" {
					t.Fatal("unplayable draw retained turn")
				}
				return
			}
			if s.CurrentPlayerID != 1 {
				t.Fatal("playable draw advanced turn")
			}
			rejected(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: oldCard, Revision: s.Revision}, ErrCardNotPlayable)
			rejected(t, g, Action{Type: DrawCard, PlayerID: 1, Revision: s.Revision}, ErrAlreadyDrawn)
			clone, err := Restore(s, noShuffle)
			if err != nil {
				t.Fatal(err)
			}
			apply(t, clone, Action{Type: PassTurn, PlayerID: 1})
			if clone.Snapshot().CurrentPlayerID != 2 {
				t.Fatal("pass")
			}
			apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: s.DrawnCardID})
			if g.Snapshot().CurrentPlayerID != 2 {
				t.Fatal("drawn play")
			}
		})
	}
}

func TestDrawnFourChecksWholeHand(t *testing.T) {
	g := scenario(t, ClassicRules(), [][]Card{{card(Red, One)}, {card(Blue, Five)}}, card(Red, Five), []Card{card(NoColor, WildDrawFour)})
	apply(t, g, Action{Type: DrawCard, PlayerID: 1})
	if g.Snapshot().CurrentPlayerID != 2 {
		t.Fatal("drawn illegal +4 should end turn")
	}
}

func TestSecurityAndRevision(t *testing.T) {
	g := scenario(t, ClassicRules(), [][]Card{{card(Red, One), card(Red, Two)}, {card(Blue, Five)}}, card(Red, Five), nil)
	s := g.Snapshot()
	for _, tt := range []struct {
		name   string
		action Action
		want   error
	}{
		{"wrong turn", Action{Type: PlayCard, PlayerID: 2, CardID: s.Players[1].Hand[0]}, ErrNotYourTurn},
		{"unknown player", Action{Type: DrawCard, PlayerID: 99}, ErrUnknownPlayer},
		{"foreign card", Action{Type: PlayCard, PlayerID: 1, CardID: s.Players[1].Hand[0]}, ErrCardNotOwned},
		{"missing card", Action{Type: PlayCard, PlayerID: 1, CardID: "missing"}, ErrCardNotFound},
		{"no pending color", Action{Type: ChooseColor, PlayerID: 1, Color: Blue}, ErrNoColorChoice},
		{"invalid payload", Action{Type: DrawCard, PlayerID: 1, CardID: "x"}, ErrInvalidAction},
		{"unknown action", Action{Type: ActionType(255), PlayerID: 1}, ErrInvalidAction},
		{"future revision", Action{Type: DrawCard, PlayerID: 1, Revision: 2}, ErrStaleRevision},
	} {
		t.Run(tt.name, func(t *testing.T) { rejected(t, g, tt.action, tt.want) })
	}
	// Two requests rendered at revision 10: only the first can commit.
	g.state.Revision = 10
	a := Action{Type: PlayCard, PlayerID: 1, CardID: s.Players[0].Hand[0], Revision: 10}
	if _, err := g.Apply(a); err != nil {
		t.Fatal(err)
	}
	rejected(t, g, a, ErrStaleRevision)
	if g.Snapshot().Revision != 11 {
		t.Fatal("duplicate revision")
	}
}

func TestVictoryAndFinalPenalties(t *testing.T) {
	for _, rank := range []Rank{One, Skip, Reverse, DrawTwo, Wild, WildDrawFour} {
		t.Run(fmt.Sprint(rank), func(t *testing.T) {
			color := Red
			if rank >= Wild {
				color = NoColor
			}
			g := scenario(t, ClassicRules(), [][]Card{{card(color, rank)}, {card(Blue, One)}, {card(Green, One)}}, card(Red, Five), nil)
			r := apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.Snapshot().Players[0].Hand[0]})
			if rank >= Wild {
				if g.Snapshot().Phase != ChoosingColor || hasEvent(r, PlayerWon, 1) {
					t.Fatal("wild won before choice")
				}
				r = apply(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Blue})
			}
			s := g.Snapshot()
			if s.Phase != Finished || len(s.Placements) != 1 || !s.Placements[0].WentOut || s.Placements[0].PlayerID != 1 {
				t.Fatal("winner")
			}
			if !hasEvent(r, GameFinished, 0) || hasEvent(r, TurnChanged, 2) {
				t.Fatal("events after finish")
			}
			penalty := 0
			if rank == DrawTwo {
				penalty = 2
			}
			if rank == WildDrawFour {
				penalty = 4
			}
			if len(s.Players[1].Hand) != 1+penalty {
				t.Fatal("final penalty")
			}
			rejected(t, g, Action{Type: DrawCard, PlayerID: 2, Revision: s.Revision}, ErrGameFinished)
		})
	}
}

func TestSnapshotsAndSerialization(t *testing.T) {
	g := scenario(t, BotRules(), [][]Card{{card(NoColor, Wild), card(Blue, One)}, {card(Green, One)}}, card(Red, Five), nil)
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.Snapshot().Players[0].Hand[0]})
	original := g.Snapshot()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded State
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(decoded, noShuffle)
	if err != nil {
		t.Fatal(err)
	}
	decoded.Pending.Actor = 99
	decoded.Players[0].Hand[0] = "bad"
	decoded.Cards[0].ID = "bad"
	decoded.Order[0] = 99
	decoded.DrawPile[0] = "bad"
	decoded.DiscardPile[0] = "bad"
	if !reflect.DeepEqual(original, restored.Snapshot()) {
		t.Fatal("restore retained aliases")
	}
	copy := g.Snapshot()
	copy.Pending.Actor = 99
	copy.Players[0].Hand[0] = "bad"
	copy.Cards[0].Color = Yellow
	if !reflect.DeepEqual(original, g.Snapshot()) {
		t.Fatal("snapshot retained aliases")
	}
	apply(t, restored, Action{Type: ChooseColor, PlayerID: 1, Color: Yellow})
	if !reflect.DeepEqual(original, g.Snapshot()) {
		t.Fatal("restored game mutated original")
	}
}

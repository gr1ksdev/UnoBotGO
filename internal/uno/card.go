package uno

import "fmt"

type GameID string
type PlayerID int64
type CardID string

type Color uint8

const (
	NoColor Color = iota
	Red
	Blue
	Green
	Yellow
)

func (c Color) valid() bool { return c >= Red && c <= Yellow }

type Rank uint8

const (
	Zero Rank = iota
	One
	Two
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Skip
	Reverse
	DrawTwo
	Wild
	WildDrawFour
	SwapHands
)

type Card struct {
	ID    CardID
	Color Color
	Rank  Rank
}

func (c Card) valid() bool {
	return c.ID != "" && c.Rank <= SwapHands &&
		((c.Rank >= Wild && c.Color == NoColor) || (c.Rank < Wild && c.Color.valid()))
}

// ClassicDeck returns 108 distinct physical cards, in a stable order.
func ClassicDeck() []Card {
	cards := make([]Card, 0, 108)
	add := func(color Color, rank Rank) {
		cards = append(cards, Card{ID: CardID(fmt.Sprintf("c%03d", len(cards)+1)), Color: color, Rank: rank})
	}
	for color := Red; color <= Yellow; color++ {
		for rank := Zero; rank <= DrawTwo; rank++ {
			add(color, rank)
			if rank != Zero {
				add(color, rank)
			}
		}
	}
	for rank := Wild; rank <= WildDrawFour; rank++ {
		for range 4 {
			add(NoColor, rank)
		}
	}
	return cards
}

// deckForRules preserves classic physical IDs and adds one house-only card.
func deckForRules(rules Rules) []Card {
	cards := ClassicDeck()
	if rules.AllowSwapHands {
		cards = append(cards, Card{ID: "c109", Color: NoColor, Rank: SwapHands})
	}
	return cards
}

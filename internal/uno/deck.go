package uno

import "math/rand/v2"

// Shuffler permutes IDs in place. It must preserve the inventory, not retain
// the slice, and must not reenter Game. Nil selects the standard random shuffle.
type Shuffler func([]CardID)

func randomShuffle(ids []CardID) {
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
}

// draw preflights the entire amount before changing the candidate state.
func (g *Game) draw(s *State, amount int) ([]CardID, error) {
	available := len(s.DrawPile)
	if len(s.DiscardPile) > 1 {
		available += len(s.DiscardPile) - 1
	}
	if available < amount {
		return nil, ErrDeckEmpty
	}
	cards := make([]CardID, 0, amount)
	for range amount {
		if len(s.DrawPile) == 0 {
			top := s.DiscardPile[len(s.DiscardPile)-1]
			s.DrawPile = append([]CardID(nil), s.DiscardPile[:len(s.DiscardPile)-1]...)
			s.DiscardPile = []CardID{top}
			g.shuffle(s.DrawPile)
		}
		// The first element is the next card, also for injected decks.
		cards = append(cards, s.DrawPile[0])
		s.DrawPile = s.DrawPile[1:]
	}
	return cards, nil
}

package simulation

import (
	"math/rand/v2"

	"github.com/malbs/UnoGoBot/internal/uno"
)

type strategy struct {
	rng *rand.Rand
}

func (s strategy) decide(game *uno.Game, state uno.State) uno.Action {
	action := uno.Action{PlayerID: state.CurrentPlayerID, Revision: state.Revision}
	if state.Phase == uno.ChoosingPlayer {
		action.Type = uno.ChoosePlayer
		// Prefer the smallest active hand; seat order breaks ties deterministically.
		smallest := int(^uint(0) >> 1)
		for _, id := range state.Order {
			if id == state.CurrentPlayerID {
				continue
			}
			if player := statePlayer(state, id); player != nil && len(player.Hand) < smallest {
				action.TargetID, smallest = id, len(player.Hand)
			}
		}
		return action
	}
	if state.Phase == uno.ChoosingColor {
		action.Type = uno.ChooseColor
		action.Color = preferredColor(state, state.CurrentPlayerID)
		return action
	}

	playable := playableCards(game, state)
	if state.PendingBluff != nil && state.PendingBluff.Target == state.CurrentPlayerID {
		if card, ok := choosePenaltyResponse(playable, state); ok {
			action.Type = uno.PlayCard
			action.CardID = card.ID
			return action
		}
		action.Type = uno.CallBluff
		return action
	}

	if len(playable) != 0 {
		card := playable[s.rng.IntN(len(playable))]
		action.Type = uno.PlayCard
		action.CardID = card.ID
		return action
	}
	if state.DrawnCardID != "" {
		action.Type = uno.PassTurn
		return action
	}
	action.Type = uno.DrawCard
	return action
}

func playableCards(game *uno.Game, state uno.State) []uno.Card {
	player := statePlayer(state, state.CurrentPlayerID)
	if player == nil {
		return nil
	}
	cards := make([]uno.Card, 0, len(player.Hand))
	for _, id := range player.Hand {
		if game.CanPlay(player.ID, id) != nil {
			continue
		}
		if card, ok := stateCard(state, id); ok {
			cards = append(cards, card)
		}
	}
	return cards
}

func choosePenaltyResponse(cards []uno.Card, state uno.State) (uno.Card, bool) {
	if state.DrawCounter == 0 {
		return uno.Card{}, false
	}
	for _, card := range cards {
		if card.Rank == uno.DrawTwo || card.Rank == uno.WildDrawFour {
			return card, true
		}
	}
	return uno.Card{}, false
}

func preferredColor(state uno.State, playerID uno.PlayerID) uno.Color {
	counts := [5]int{}
	player := statePlayer(state, playerID)
	if player != nil {
		for _, id := range player.Hand {
			card, ok := stateCard(state, id)
			if ok && card.Color >= uno.Red && card.Color <= uno.Yellow {
				counts[card.Color]++
			}
		}
	}
	best := uno.Red
	for color := uno.Blue; color <= uno.Yellow; color++ {
		if counts[color] > counts[best] {
			best = color
		}
	}
	return best
}

func statePlayer(state uno.State, id uno.PlayerID) *uno.Player {
	for i := range state.Players {
		if state.Players[i].ID == id {
			return &state.Players[i]
		}
	}
	return nil
}

func stateCard(state uno.State, id uno.CardID) (uno.Card, bool) {
	for _, card := range state.Cards {
		if card.ID == id {
			return card, true
		}
	}
	return uno.Card{}, false
}

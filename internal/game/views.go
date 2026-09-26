package game

import (
	"slices"

	"github.com/malbs/UnoGoBot/internal/uno"
)

type ChatID int64

type PublicPlayer struct {
	ID        uno.PlayerID
	Status    uno.PlayerStatus
	CardCount int
	Active    bool
}

type CloseReason string

const (
	Completed CloseReason = "completed"
	Departure CloseReason = "departure"
	Cancelled CloseReason = "cancelled"
)

// PublicGameView has no hands, draw pile or full inventory. OwnerID is metadata,
// not a participant; Players contains only users explicitly registered via Join.
type PublicGameView struct {
	GameID          uno.GameID
	ChatID          ChatID
	ChatName        string
	CreatorID       uno.PlayerID
	OwnerID         uno.PlayerID
	Locked          bool
	Revision        uint64
	Phase           uno.Phase
	Rules           uno.Rules
	CurrentTurn     uno.PlayerID
	Direction       int
	ActiveColor     uno.Color
	TopCard         *uno.Card
	PlayerChooserID uno.PlayerID
	ColorChooserID  uno.PlayerID
	DrawCounter     int
	Players         []PublicPlayer
	Order           []uno.PlayerID
	Placements      []uno.Placement
	Closed          bool
	CloseReason     CloseReason
	CanCallBluff    bool
}

type CardView struct {
	Card     uno.Card
	Playable bool
}

type PlayerGameView struct {
	Public      PublicGameView
	PlayerID    uno.PlayerID
	Hand        []CardView
	DrawnCardID uno.CardID
}

// GameSummary is an index projection published atomically with membership.
type GameSummary struct {
	GameID   uno.GameID
	ChatID   ChatID
	ChatName string
	OwnerID  uno.PlayerID
	Locked   bool
	Revision uint64
	Phase    uno.Phase
}

func (v PublicGameView) clone() PublicGameView {
	v.Players = slices.Clone(v.Players)
	v.Order = slices.Clone(v.Order)
	v.Placements = slices.Clone(v.Placements)
	if v.TopCard != nil {
		card := *v.TopCard
		v.TopCard = &card
	}
	return v
}

func (v PublicGameView) summary() GameSummary {
	return GameSummary{GameID: v.GameID, ChatID: v.ChatID, ChatName: v.ChatName, OwnerID: v.OwnerID, Locked: v.Locked, Revision: v.Revision, Phase: v.Phase}
}

func publicView(entry *managedGame, state uno.State) PublicGameView {
	v := PublicGameView{
		GameID: state.ID, ChatID: entry.chatID, ChatName: entry.chatName,
		CreatorID: entry.creatorID, OwnerID: entry.ownerID, Locked: entry.locked, Revision: state.Revision,
		Phase: state.Phase, Rules: state.Rules, CurrentTurn: state.CurrentPlayerID,
		Direction: state.Direction, ActiveColor: state.ActiveColor,
		DrawCounter: state.DrawCounter,
		Order:       slices.Clone(state.Order), Placements: slices.Clone(state.Placements),
		Closed: state.Phase == uno.Finished, CloseReason: CloseReason(state.FinishReason),
		Players: make([]PublicPlayer, 0, len(state.Players)),
	}
	for _, p := range state.Players {
		v.Players = append(v.Players, PublicPlayer{ID: p.ID, Status: p.Status, CardCount: len(p.Hand), Active: !v.Closed && p.Status == uno.Playing})
	}
	if len(state.DiscardPile) > 0 {
		top := state.DiscardPile[len(state.DiscardPile)-1]
		for _, c := range state.Cards {
			if c.ID == top {
				card := c
				v.TopCard = &card
				break
			}
		}
	}
	if state.Phase == uno.ChoosingPlayer {
		v.PlayerChooserID = state.CurrentPlayerID
	}
	if state.Pending != nil {
		v.ColorChooserID = state.Pending.Actor
	}
	if state.PendingBluff != nil && state.DrawFourChallengeable && state.CurrentPlayerID == state.PendingBluff.Target && state.DrawCounter > 0 {
		v.CanCallBluff = true
	}
	return v
}

// Called only with entry.mu held, so snapshot and CanPlay describe one revision.
func playerView(entry *managedGame, state uno.State, id uno.PlayerID) (PlayerGameView, error) {
	var player *uno.Player
	for i := range state.Players {
		if state.Players[i].ID == id {
			player = &state.Players[i]
			break
		}
	}
	if player == nil || player.Status != uno.Playing {
		return PlayerGameView{}, ErrNotParticipant
	}
	v := PlayerGameView{Public: publicView(entry, state), PlayerID: id, Hand: make([]CardView, 0, len(player.Hand))}
	catalog := make(map[uno.CardID]uno.Card, len(state.Cards))
	for _, c := range state.Cards {
		catalog[c.ID] = c
	}
	for _, id := range player.Hand {
		v.Hand = append(v.Hand, CardView{Card: catalog[id], Playable: entry.engine.CanPlay(player.ID, id) == nil})
	}
	if state.CurrentPlayerID == id {
		v.DrawnCardID = state.DrawnCardID
	}
	return v, nil
}

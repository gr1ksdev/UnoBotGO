package telegram

import (
	"context"
	"sync"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
)

// currentGameTracker remembers which game a player is currently playing.
//
// The inline menu is opened with an empty query, so the bot cannot read the
// game from the query text. It resolves the game from this tracker instead,
// the same way the upstream bot keeps a single "current game" per player.
// The tracker is updated whenever the player acts in a group.
type currentGameTracker struct {
	mu     sync.Mutex
	byUser map[uno.PlayerID]uno.GameID
}

func newCurrentGameTracker() *currentGameTracker {
	return &currentGameTracker{byUser: make(map[uno.PlayerID]uno.GameID)}
}

// Set records the game the player is dealing with.
func (t *currentGameTracker) Set(userID uno.PlayerID, gameID uno.GameID) {
	if t == nil || userID <= 0 || gameID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byUser[userID] = gameID
}

// Get returns the remembered game, if any.
func (t *currentGameTracker) Get(userID uno.PlayerID) (uno.GameID, bool) {
	if t == nil || userID <= 0 {
		return "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	id, ok := t.byUser[userID]
	return id, ok
}

// resolve picks the game whose hand the inline menu must show.
//
// Preference order: the remembered current game, then the first active game.
// The fallback keeps a single-game player working even without any prior
// interaction.
func (t *currentGameTracker) resolve(ctx context.Context, service *game.Service, actorID uno.PlayerID) (uno.GameID, bool) {
	// The remembered game wins, even after it closed: the player must see
	// that the game is over instead of an empty menu.
	if id, ok := t.Get(actorID); ok {
		return id, true
	}
	games, err := service.FindPlayerGames(ctx, game.Actor{PlayerID: actorID})
	if err != nil || len(games) == 0 {
		return "", false
	}
	return games[0].GameID, true
}

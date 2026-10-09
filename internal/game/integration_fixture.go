//go:build integration

package game

import (
	"github.com/malbs/UnoGoBot/internal/uno"
	"math/rand"
	"sync"
)

// NewDeterministicService is a test-only standard engine with seeded shuffling.
// It changes neither the deck nor rules; production builds do not expose it.
func NewDeterministicService(seed int64) *Service {
	svc, _ := NewService()
	random := rand.New(rand.NewSource(seed))
	var mu sync.Mutex
	svc.manager.newGame = func(id uno.GameID, rules uno.Rules) (*uno.Game, error) {
		return uno.NewGame(id, rules, uno.WithShuffler(func(cards []uno.CardID) {
			mu.Lock()
			defer mu.Unlock()
			random.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
		}))
	}
	return svc
}

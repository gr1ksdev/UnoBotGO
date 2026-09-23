package game

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

// The lock order is entry.mu -> indexMu. Lookup releases indexMu before waiting
// for entry.mu. No engine operation, ID generation, factory or I/O under indexMu.
type manager struct {
	indexMu      sync.RWMutex
	byID         map[uno.GameID]indexRecord
	byChat       map[ChatID]uno.GameID
	byPlayer     map[uno.PlayerID]map[uno.GameID]struct{}
	history      []uno.GameID
	historyLimit int
	newID        func() (uno.GameID, error)
	newGame      func(uno.GameID, uno.Rules) (*uno.Game, error)
}

type indexRecord struct {
	entry   *managedGame
	summary GameSummary // accessed only under indexMu
}

type managedGame struct {
	mu          sync.Mutex
	engine      *uno.Game // unique runtime owner; nil once closed
	chatID      ChatID
	chatName    string
	creatorID   uno.PlayerID
	ownerID     uno.PlayerID
	turnStarted time.Time
	final       *PublicGameView // public projection only, accessed under mu
}

func newManager(limit int) *manager {
	return &manager{
		byID: make(map[uno.GameID]indexRecord), byChat: make(map[ChatID]uno.GameID),
		byPlayer: make(map[uno.PlayerID]map[uno.GameID]struct{}), historyLimit: limit,
		newID:   randomID,
		newGame: func(id uno.GameID, rules uno.Rules) (*uno.Game, error) { return uno.NewGame(id, rules) },
	}
}

func randomID() (uno.GameID, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate game ID: %w", err)
	}
	return uno.GameID(hex.EncodeToString(raw[:])), nil
}

func (m *manager) create(ctx context.Context, actor Actor, req CreateRequest) (Outcome, error) {
	id, err := m.newID()
	if err != nil {
		return Outcome{}, err
	}
	if id == "" {
		return Outcome{}, ErrInvalidArgument
	}
	engine, err := m.newGame(id, req.Rules)
	if err != nil {
		return Outcome{}, err
	}
	entry := &managedGame{engine: engine, chatID: actor.ChatID, chatName: req.ChatName, creatorID: actor.PlayerID, ownerID: actor.PlayerID, turnStarted: time.Now()}
	view := publicView(entry, engine.Snapshot())
	// Entry is still private to this call. Creation publishes only an empty lobby.
	m.indexMu.Lock()
	defer m.indexMu.Unlock()
	if err := ctx.Err(); err != nil {
		return Outcome{}, err
	}
	if _, exists := m.byChat[actor.ChatID]; exists {
		return Outcome{}, ErrChatOccupied
	}
	if _, exists := m.byID[id]; exists {
		return Outcome{}, ErrIDConflict
	}
	m.byID[id] = indexRecord{entry: entry, summary: view.summary()}
	m.byChat[actor.ChatID] = id
	return Outcome{View: view}, nil
}

// lockGame returns a locked entry. Caller must unlock it. Already acquired
// references remain safe even if their closed summary is subsequently evicted.
func (m *manager) lockGame(ctx context.Context, id uno.GameID) (*managedGame, error) {
	m.indexMu.RLock()
	record, ok := m.byID[id]
	m.indexMu.RUnlock()
	if !ok {
		return nil, ErrGameNotFound
	}
	entry := record.entry
	entry.mu.Lock()
	if err := ctx.Err(); err != nil {
		entry.mu.Unlock()
		return nil, err
	}
	return entry, nil
}

// publish completes every successful engine action while the caller holds
// entry.mu. No cancellation checks after Apply: an accepted action must publish.
func (m *manager) publish(entry *managedGame, before, after uno.State, result uno.Result) Outcome {
	if after.Phase == uno.Finished {
		entry.turnStarted = time.Time{}
	} else if before.CurrentPlayerID != after.CurrentPlayerID || before.Phase != after.Phase {
		entry.turnStarted = time.Now()
	}
	transferOwner(entry, before, after)
	view := publicView(entry, after)
	if view.Closed {
		final := view.clone()
		entry.final = &final
		entry.engine = nil // discard hands and full runtime on closure
	}
	m.indexMu.Lock()
	defer m.indexMu.Unlock()
	// At most ten registered players per game. These maps are routing projections.
	for _, p := range before.Players {
		if games := m.byPlayer[p.ID]; games != nil {
			delete(games, after.ID)
			if len(games) == 0 {
				delete(m.byPlayer, p.ID)
			}
		}
	}
	if !view.Closed {
		for _, p := range after.Players {
			if p.Status != uno.Playing {
				continue
			}
			if m.byPlayer[p.ID] == nil {
				m.byPlayer[p.ID] = make(map[uno.GameID]struct{})
			}
			m.byPlayer[p.ID][after.ID] = struct{}{}
		}
	}
	m.byID[after.ID] = indexRecord{entry: entry, summary: view.summary()}
	if view.Closed {
		delete(m.byChat, entry.chatID)
		m.history = append(m.history, after.ID)
		if len(m.history) > m.historyLimit {
			delete(m.byID, m.history[0])
			m.history = slices.Delete(m.history, 0, 1)
		}
	}
	return Outcome{View: view, Events: slices.Clone(result.Events)}
}

func transferOwner(entry *managedGame, before, after uno.State) {
	if after.Phase == uno.Finished {
		entry.ownerID = 0
		return
	}
	wasActive, isActive := false, false
	for _, p := range before.Players {
		if p.ID == entry.ownerID {
			wasActive = p.Status == uno.Playing
		}
	}
	for _, p := range after.Players {
		if p.ID == entry.ownerID {
			isActive = p.Status == uno.Playing
		}
	}
	if !wasActive || isActive || len(after.Order) == 0 {
		return
	}
	if after.CurrentPlayerID != 0 {
		entry.ownerID = after.CurrentPlayerID
	} else {
		entry.ownerID = after.Order[0]
	}
}

func (m *manager) findChat(ctx context.Context, chat ChatID) (GameSummary, error) {
	m.indexMu.RLock()
	defer m.indexMu.RUnlock()
	if err := ctx.Err(); err != nil {
		return GameSummary{}, err
	}
	id, ok := m.byChat[chat]
	if !ok {
		return GameSummary{}, ErrNoActiveGame
	}
	return m.byID[id].summary, nil
}

func (m *manager) findPlayer(ctx context.Context, id uno.PlayerID) ([]GameSummary, error) {
	m.indexMu.RLock()
	if err := ctx.Err(); err != nil {
		m.indexMu.RUnlock()
		return nil, err
	}
	games := m.byPlayer[id]
	result := make([]GameSummary, 0, len(games))
	for gameID := range games {
		result = append(result, m.byID[gameID].summary)
	}
	m.indexMu.RUnlock()
	slices.SortFunc(result, func(a, b GameSummary) int {
		if a.ChatID < b.ChatID {
			return -1
		}
		if a.ChatID > b.ChatID {
			return 1
		}
		if a.GameID < b.GameID {
			return -1
		}
		if a.GameID > b.GameID {
			return 1
		}
		return 0
	})
	return result, nil
}

func (m *manager) expiredTurns(ctx context.Context, timeout time.Duration, now time.Time) []ExpiredTurn {
	m.indexMu.RLock()
	entries := make([]*managedGame, 0, len(m.byID))
	for _, record := range m.byID {
		entries = append(entries, record.entry)
	}
	m.indexMu.RUnlock()
	results := make([]ExpiredTurn, 0)
	for _, entry := range entries {
		if ctx.Err() != nil {
			break
		}
		entry.mu.Lock()
		if entry.final != nil || entry.engine == nil || entry.turnStarted.IsZero() || now.Sub(entry.turnStarted) < timeout {
			entry.mu.Unlock()
			continue
		}
		before := entry.engine.Snapshot()
		if before.Phase != uno.TakingTurn || before.CurrentPlayerID == 0 {
			entry.mu.Unlock()
			continue
		}
		results = append(results, ExpiredTurn{GameID: before.ID, ChatID: entry.chatID, PlayerID: before.CurrentPlayerID, Revision: before.Revision})
		entry.mu.Unlock()
	}
	return results
}

func (m *manager) skipTurn(ctx context.Context, candidate ExpiredTurn, timeout time.Duration, now time.Time) (Outcome, bool) {
	entry, err := m.lockGame(ctx, candidate.GameID)
	if err != nil {
		return Outcome{}, false
	}
	defer entry.mu.Unlock()
	if entry.final != nil || entry.engine == nil || entry.chatID != candidate.ChatID || entry.turnStarted.IsZero() || now.Sub(entry.turnStarted) < timeout {
		return Outcome{}, false
	}
	before := entry.engine.Snapshot()
	if before.Phase != uno.TakingTurn || before.CurrentPlayerID == 0 || before.CurrentPlayerID != candidate.PlayerID || before.Revision != candidate.Revision || ctx.Err() != nil {
		return Outcome{}, false
	}
	result, err := entry.engine.Apply(uno.Action{Type: uno.SkipTurn, PlayerID: candidate.PlayerID, Revision: candidate.Revision})
	if err != nil {
		return Outcome{}, false
	}
	return m.publish(entry, before, entry.engine.Snapshot(), result), true
}

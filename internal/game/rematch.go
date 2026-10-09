package game

import (
	"context"
	"errors"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

var ErrResultPending = errors.New("previous result awaits commit")

// VoteRematch belongs to the closed game, never to the engine action stream.
// Membership is fixed to ALL registered participants, including departures.
// Disconnect does not revoke a vote or remove a participant. An explicit decline
// revokes only that person's vote; returning may accept again. After publication
// votes on the previous ID are immutable and cannot start another round.
func (s *Service) VoteRematch(ctx context.Context, actor Actor, id uno.GameID, revision uint64, accept bool, requestID string) (PublicGameView, error) {
	if len(requestID) < 8 || len(requestID) > 128 {
		return PublicGameView{}, ErrInvalidArgument
	}
	if err := checkContext(ctx); err != nil {
		return PublicGameView{}, err
	}
	e, err := s.manager.lockGame(ctx, id)
	if err != nil {
		return PublicGameView{}, err
	}
	defer e.mu.Unlock()
	if e.final == nil || e.final.Rematch == nil {
		return PublicGameView{}, ErrGameClosed
	}
	if actor.ChatID != e.chatID || actor.PlayerID <= 0 {
		return PublicGameView{}, ErrForbidden
	}
	member := false
	for _, p := range e.final.Players {
		if p.ID == actor.PlayerID {
			member = true
		}
	}
	if !member {
		return PublicGameView{}, ErrForbidden
	}
	if revision != e.final.Revision {
		return PublicGameView{}, uno.ErrStaleRevision
	}
	key := receiptKey{actor.PlayerID, requestID}
	if old, exists := e.rematchReceipts[key]; exists {
		if old != accept {
			return PublicGameView{}, ErrInvalidArgument
		}
		return e.final.clone(), nil
	}
	if len(e.rematchReceipts) >= 8192 {
		return PublicGameView{}, ErrInvalidArgument
	}
	r := e.final.Rematch
	if r.NextGameID != "" {
		return e.final.clone(), nil
	}
	m := s.manager
	m.indexMu.RLock()
	_, pending := m.pendingResults[id]
	m.indexMu.RUnlock()
	if pending {
		return PublicGameView{}, ErrResultPending
	}
	if e.rematchVotes == nil {
		e.rematchVotes = make(map[uno.PlayerID]bool)
	}
	if e.rematchVotes[actor.PlayerID] != accept {
		e.rematchVotes[actor.PlayerID] = accept
		r.Revision++
	}
	r.Accepted = nil
	for _, player := range r.Required {
		if e.rematchVotes[player] {
			r.Accepted = append(r.Accepted, player)
		}
	}
	r.Ready = true
	if e.rematchReceipts == nil {
		e.rematchReceipts = make(map[receiptKey]bool)
	}
	e.rematchReceipts[key] = accept
	defer func() {
		if r.NextGameID == "" && len(r.Accepted) == len(r.Required) {
			delete(e.rematchReceipts, key)
		}
	}()
	// Always signal recorded votes, even if creating the next game fails.
	defer s.Signal()
	if len(r.Accepted) != len(r.Required) {
		return e.final.clone(), nil
	}
	nextID, err := m.newID()
	if err != nil {
		return PublicGameView{}, err
	}
	if nextID == "" {
		return PublicGameView{}, ErrInvalidArgument
	}
	engine, err := m.newGame(nextID, e.final.Rules)
	if err != nil {
		return PublicGameView{}, err
	}
	next := &managedGame{engine: engine, chatID: e.chatID, chatName: e.chatName, creatorID: r.Required[0], ownerID: r.Required[0], groupConfig: e.groupConfig, locked: e.locked, origin: "webapp", participants: make(map[uno.PlayerID]participantHistory), turnStarted: time.Now()}
	// Prepare the entire round privately. Nothing becomes observable until the
	// index transaction below. No I/O or factory is called under indexMu.
	for _, player := range r.Required {
		metadata := e.participants[player]
		next.participants[player] = participantHistory{DisplayName: metadata.DisplayName, Username: metadata.Username, LastSeenAt: metadata.LastSeenAt}
		if _, err := engine.Apply(uno.Action{Type: uno.JoinGame, PlayerID: player, Revision: engine.Snapshot().Revision}); err != nil {
			return PublicGameView{}, err
		}
	}
	if _, err := engine.Apply(uno.Action{Type: uno.StartGame, PlayerID: r.Required[0], DealerID: r.Required[0], Revision: engine.Snapshot().Revision}); err != nil {
		return PublicGameView{}, err
	}
	next.startedAt = time.Now().UTC()
	view := publicView(next, engine.Snapshot())
	m.indexMu.Lock()
	defer m.indexMu.Unlock()
	if err := ctx.Err(); err != nil {
		return PublicGameView{}, err
	}
	if _, occupied := m.byChat[e.chatID]; occupied {
		return PublicGameView{}, ErrChatOccupied
	}
	if _, duplicate := m.byID[nextID]; duplicate {
		return PublicGameView{}, ErrIDConflict
	}
	if _, pending := m.pendingResults[id]; pending {
		return PublicGameView{}, ErrResultPending
	}
	m.byID[nextID] = indexRecord{entry: next, summary: view.summary()}
	m.byChat[e.chatID] = nextID
	for _, player := range r.Required {
		if m.byPlayer[player] == nil {
			m.byPlayer[player] = make(map[uno.GameID]struct{})
		}
		m.byPlayer[player][nextID] = struct{}{}
	}
	r.NextGameID = nextID
	r.Revision++
	return e.final.clone(), nil
}

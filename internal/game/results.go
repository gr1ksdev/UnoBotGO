package game

import (
	"context"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
	"strconv"
	"time"
)

type participantHistory struct {
	DisplayName, Username    string
	JoinedAfterStart         bool
	LeaveCount, ReentryCount int
}

// ObservePlayer updates public metadata in memory only. The trusted adapter
// provides the authenticated actor and observed Telegram names.
func (s *Service) ObservePlayer(ctx context.Context, actor Actor, id uno.GameID, name, username string) error {
	entry, err := s.manager.lockGame(ctx, id)
	if err != nil {
		return err
	}
	defer entry.mu.Unlock()
	if actor.PlayerID <= 0 || actor.ChatID != entry.chatID {
		return ErrForbidden
	}
	if entry.final != nil {
		return ErrGameClosed
	}
	if entry.participants == nil {
		entry.participants = map[uno.PlayerID]participantHistory{}
	}
	metadata := entry.participants[actor.PlayerID]
	metadata.DisplayName = name
	metadata.Username = username
	entry.participants[actor.PlayerID] = metadata
	return nil
}

// observeLifecycle is called under entry.mu after an accepted engine mutation.
func observeLifecycle(entry *managedGame, before, after uno.State, events []uno.Event) {
	if entry.participants == nil {
		entry.participants = map[uno.PlayerID]participantHistory{}
	}
	for _, e := range events {
		if e.Type == uno.GameStarted {
			entry.startedAt = time.Now().UTC()
		}
		p := entry.participants[e.PlayerID]
		switch e.Type {
		case uno.PlayerJoined:
			if old := before.Player(e.PlayerID); old != nil && old.Status == uno.Left {
				p.ReentryCount++
			}
			if before.Phase != uno.Lobby {
				p.JoinedAfterStart = true
			}
			entry.participants[e.PlayerID] = p
		case uno.PlayerLeft:
			p.LeaveCount++
			entry.participants[e.PlayerID] = p
		}
	}
}

func finalResult(entry *managedGame, state uno.State) *ranking.Result {
	if state.Phase != uno.Finished || state.FinishReason == uno.FinishedByCancellation || entry.startedAt.IsZero() {
		return nil
	}
	mode := groups.Classic
	if state.Rules.AllowSwapHands || state.Rules.StackWildDrawFourOnTwo || state.Rules.StackDrawTwoOnWildFour {
		mode = groups.Caseiro
	}
	r := ranking.Result{GameID: string(state.ID), ChatID: int64(entry.chatID), GameMode: mode, RankingSystem: entry.groupConfig.RankingSystem, ConfigRevision: entry.groupConfig.ConfigRevision, StartedAt: entry.startedAt, FinishedAt: time.Now().UTC(), FinalRevision: state.Revision, FinishReason: string(state.FinishReason)}
	for _, player := range state.Players {
		h := entry.participants[player.ID]
		name := h.DisplayName
		if name == "" {
			name = strconv.FormatInt(int64(player.ID), 10)
		}
		status := "playing"
		if player.Status == uno.Left {
			status = "left"
		}
		if player.Status == uno.WentOut {
			status = "went_out"
		}
		p := ranking.Player{UserID: int64(player.ID), DisplayName: name, Username: h.Username, FinalStatus: status, JoinedAfterStart: h.JoinedAfterStart, LeaveCount: h.LeaveCount, ReentryCount: h.ReentryCount}
		for _, placed := range state.Placements {
			if placed.PlayerID == player.ID {
				p.Position = placed.Position
				p.WentOut = placed.WentOut
			}
		}
		r.Players = append(r.Players, p)
	}
	return &r
}

// PendingResults is independent of bounded public history; no worker/finalization
// queue exists. The closing adapter synchronously persists, then acknowledges.
func (s *Service) PendingResults() []ranking.Result {
	s.manager.indexMu.RLock()
	defer s.manager.indexMu.RUnlock()
	result := make([]ranking.Result, 0, len(s.manager.pendingResults))
	for _, r := range s.manager.pendingResults {
		result = append(result, r.Clone())
	}
	return result
}

// AcknowledgeResult is only for the trusted finalization adapter after COMMIT.
func (s *Service) AcknowledgeResult(id uno.GameID) {
	s.manager.indexMu.Lock()
	defer s.manager.indexMu.Unlock()
	delete(s.manager.pendingResults, id)
}

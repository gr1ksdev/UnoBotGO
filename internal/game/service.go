package game

import (
	"context"
	"slices"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

// Actor must come from a trusted adapter, not from a player-controlled payload.
// A role in a lobby does not grant participation or access to private hands.
type Actor struct {
	PlayerID uno.PlayerID
	ChatID   ChatID
}

type CreateRequest struct {
	ChatName string
	Rules    uno.Rules
}

type Outcome struct {
	View   PublicGameView
	Events []uno.Event
}

type config struct{ historyLimit int }
type Option func(*config)

// WithHistoryLimit bounds retained closed public summaries. Zero disables
// retention; a negative limit is rejected by NewService. Default is 100.
func WithHistoryLimit(limit int) Option { return func(c *config) { c.historyLimit = limit } }

// Service is safe for concurrent use. Construct it with NewService; do not copy.
// Its manager is private so adapters cannot bypass authorization or obtain games.
type Service struct{ manager *manager }

func NewService(options ...Option) (*Service, error) {
	cfg := config{historyLimit: 100}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.historyLimit < 0 {
		return nil, ErrInvalidArgument
	}
	return &Service{manager: newManager(cfg.historyLimit)}, nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidArgument
	}
	return ctx.Err()
}

func (s *Service) Create(ctx context.Context, actor Actor, req CreateRequest) (Outcome, error) {
	if err := checkContext(ctx); err != nil {
		return Outcome{}, err
	}
	if actor.PlayerID <= 0 || actor.ChatID == 0 {
		return Outcome{}, ErrInvalidArgument
	}
	return s.manager.create(ctx, actor, req)
}

func (s *Service) Apply(ctx context.Context, actor Actor, id uno.GameID, action uno.Action) (Outcome, error) {
	if err := checkContext(ctx); err != nil {
		return Outcome{}, err
	}
	if actor.PlayerID <= 0 || id == "" {
		return Outcome{}, ErrInvalidArgument
	}
	if action.PlayerID != actor.PlayerID {
		return Outcome{}, ErrForbidden
	}
	entry, err := s.manager.lockGame(ctx, id)
	if err != nil {
		return Outcome{}, err
	}
	defer entry.mu.Unlock()
	if entry.final != nil {
		return Outcome{}, ErrGameClosed
	}
	if err := authorize(entry, actor, action.Type); err != nil {
		return Outcome{}, err
	}
	before := entry.engine.Snapshot()
	if action.Revision != before.Revision {
		return Outcome{}, uno.ErrStaleRevision
	}
	if action.Type == uno.StartGame && before.Phase == uno.Lobby {
		if len(before.Order) < 2 {
			return Outcome{}, uno.ErrNotEnoughPlayers
		}
		if action.DealerID == 0 {
			action.DealerID = before.Order[0]
		}
		if !slices.Contains(before.Order, action.DealerID) {
			return Outcome{}, uno.ErrUnknownPlayer
		}
	}
	// Recheck after authorization/projection, immediately before the mutation.
	if err := ctx.Err(); err != nil {
		return Outcome{}, err
	}
	result, err := entry.engine.Apply(action)
	if err != nil {
		return Outcome{}, err
	}
	return s.manager.publish(entry, before, entry.engine.Snapshot(), result), nil
}

// Pure application checks, executed while the entry is locked. No role lookup I/O.
func authorize(entry *managedGame, actor Actor, kind uno.ActionType) error {
	if actor.ChatID != 0 && actor.ChatID != entry.chatID {
		return ErrForbidden
	}
	switch kind {
	case uno.JoinGame, uno.LeaveGame, uno.StartGame:
		if actor.ChatID == 0 {
			return ErrForbidden
		}
	case uno.CancelGame:
		if actor.ChatID == 0 || actor.PlayerID != entry.ownerID {
			return ErrForbidden
		}
	case uno.PlayCard, uno.DrawCard, uno.PassTurn, uno.ChooseColor, uno.ChallengeDrawFour:
		// Inline actions have no chat context. The engine still validates the actor.
	default:
		return uno.ErrInvalidAction
	}
	return nil
}

func (s *Service) PublicView(ctx context.Context, id uno.GameID) (PublicGameView, error) {
	if err := checkContext(ctx); err != nil {
		return PublicGameView{}, err
	}
	if id == "" {
		return PublicGameView{}, ErrInvalidArgument
	}
	entry, err := s.manager.lockGame(ctx, id)
	if err != nil {
		return PublicGameView{}, err
	}
	defer entry.mu.Unlock()
	if entry.final != nil {
		return entry.final.clone(), nil
	}
	return publicView(entry, entry.engine.Snapshot()), nil
}

// PlayerView intentionally has no target player: only the authenticated actor's
// own hand is accessible. Even an owner cannot inspect another player's hand.
func (s *Service) PlayerView(ctx context.Context, actor Actor, id uno.GameID) (PlayerGameView, error) {
	if err := checkContext(ctx); err != nil {
		return PlayerGameView{}, err
	}
	if actor.PlayerID <= 0 || id == "" {
		return PlayerGameView{}, ErrInvalidArgument
	}
	entry, err := s.manager.lockGame(ctx, id)
	if err != nil {
		return PlayerGameView{}, err
	}
	defer entry.mu.Unlock()
	if entry.final != nil {
		return PlayerGameView{}, ErrGameClosed
	}
	if actor.ChatID != 0 && actor.ChatID != entry.chatID {
		return PlayerGameView{}, ErrForbidden
	}
	return playerView(entry, entry.engine.Snapshot(), actor.PlayerID)
}

func (s *Service) FindChatGame(ctx context.Context, chat ChatID) (GameSummary, error) {
	if err := checkContext(ctx); err != nil {
		return GameSummary{}, err
	}
	if chat == 0 {
		return GameSummary{}, ErrInvalidArgument
	}
	return s.manager.findChat(ctx, chat)
}

// FindPlayerGames lists participation across chats, never ownership. ChatID in
// actor is intentionally not a filter and never selects an implicit current game.
func (s *Service) FindPlayerGames(ctx context.Context, actor Actor) ([]GameSummary, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if actor.PlayerID <= 0 {
		return nil, ErrInvalidArgument
	}
	return s.manager.findPlayer(ctx, actor.PlayerID)
}

// AutoSkipExpired advances turns that exceeded timeout. It is intended for a
// single application-level scheduler; no clock or goroutine is part of uno.Game.
func (s *Service) AutoSkipExpired(ctx context.Context, timeout time.Duration) []Outcome {
	if ctx == nil || timeout <= 0 {
		return nil
	}
	now := time.Now()
	return s.manager.skipExpired(ctx, timeout, now)
}

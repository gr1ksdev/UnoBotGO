package game

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
)

// TurnPolicy drives the skip-with-penalty timer. Initial is the full waiting
// time of a turn, Min is the floor for the automatic timer and Penalty is the
// amount removed from a player's waiting time on every skip.
type TurnPolicy struct {
	Initial time.Duration
	Min     time.Duration
	Penalty time.Duration
}

// DefaultTurnPolicy matches the documented product values: 120s per turn,
// 20s lost per skip and a 15s floor for the automatic timer.
var DefaultTurnPolicy = TurnPolicy{Initial: 2 * time.Minute, Min: 15 * time.Second, Penalty: 20 * time.Second}

// effectiveTimeout derives the current deadline from the base timeout and the
// number of skips charged to the player. The floor never exceeds the base, so
// short timeouts used in tests keep working as before.
func (p TurnPolicy) effectiveTimeout(base time.Duration, skips int) time.Duration {
	if base <= 0 {
		return 0
	}
	floor := p.Min
	if floor > base {
		floor = base
	}
	wait := base - time.Duration(skips)*p.Penalty
	if wait < 0 {
		wait = 0
	}
	if wait < floor {
		wait = floor
	}
	return wait
}

// ErrSkipTooEarly is returned when a non-current player requests a skip before
// the current player's deadline. Wait is rounded up to a whole second by the
// adapter before being shown.
type ErrSkipTooEarly struct{ Wait time.Duration }

func (e *ErrSkipTooEarly) Error() string {
	return fmt.Sprintf("game: skip too early; wait %s", e.Wait.Round(time.Second))
}

// SkipNotice is carried by Outcome so adapters can announce a penalty without
// re-reading state under a second lock.
type SkipNotice struct {
	PlayerID  uno.PlayerID
	Remaining time.Duration
}

// Actor must come from a trusted adapter, not from a player-controlled payload.
// A role in a lobby does not grant participation or access to private hands.
type Actor struct {
	PlayerID uno.PlayerID
	ChatID   ChatID
	// ChatAdmin is asserted only by a trusted adapter after checking the
	// platform role. User-controlled payloads must never set it directly.
	ChatAdmin bool
}

type CreateRequest struct {
	ChatName     string
	Rules        uno.Rules
	GroupConfig  groups.Snapshot
	ReplaceLobby bool
}

type Outcome struct {
	Completed      *ranking.Result
	View           PublicGameView
	Events         []uno.Event
	ReplacedGameID uno.GameID
	Skip           *SkipNotice // set only when a penalty skip was applied
}

type ResetResult struct {
	GameIDs        []uno.GameID
	RemovedActive  bool
	RemovedHistory int
}

type config struct {
	historyLimit int
	policy       TurnPolicy
}
type Option func(*config)

// WithHistoryLimit bounds retained closed public summaries. Zero disables
// retention; a negative limit is rejected by NewService. Default is 100.
func WithHistoryLimit(limit int) Option { return func(c *config) { c.historyLimit = limit } }

// WithTurnPolicy configures the skip timer. All fields must be positive.
func WithTurnPolicy(policy TurnPolicy) Option { return func(c *config) { c.policy = policy } }

// Service is safe for concurrent use. Construct it with NewService; do not copy.
// Its manager is private so adapters cannot bypass authorization or obtain games.
type Service struct {
	manager *manager
	policy  TurnPolicy
}

func NewService(options ...Option) (*Service, error) {
	cfg := config{historyLimit: 100, policy: DefaultTurnPolicy}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}
	if cfg.historyLimit < 0 {
		return nil, ErrInvalidArgument
	}
	if cfg.policy.Initial <= 0 || cfg.policy.Min <= 0 || cfg.policy.Penalty <= 0 {
		return nil, ErrInvalidArgument
	}
	return &Service{manager: newManager(cfg.historyLimit, cfg.policy), policy: cfg.policy}, nil
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
	if req.GroupConfig.RankingSystem == "" {
		req.GroupConfig = groups.Defaults(int64(actor.ChatID)).Snapshot()
	}
	if !req.GroupConfig.RankingSystem.Valid() || req.GroupConfig.ConfigRevision <= 0 {
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
	if action.Type == uno.JoinGame {
		if before.HasPlacement(action.PlayerID) {
			return Outcome{}, uno.ErrAlreadyFinished
		}
		if p := before.Player(action.PlayerID); p != nil && p.Status == uno.Playing {
			return Outcome{}, uno.ErrAlreadyJoined
		}
		if entry.locked {
			return Outcome{}, ErrRoomLocked
		}
	}
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
	// A manual skip is only allowed after the current player's deadline, except
	// for the current player themselves. The check lives under the game lock so
	// the decision and the mutation cannot race with another action.
	if action.Type == uno.PenaltySkip {
		if err := s.checkSkipTiming(entry, before, actor); err != nil {
			return Outcome{}, err
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
	if action.Type == uno.PenaltySkip {
		s.manager.recordSkip(entry, before.CurrentPlayerID)
		outcome := s.manager.publish(entry, before, entry.engine.Snapshot(), result)
		outcome.Skip = &SkipNotice{PlayerID: before.CurrentPlayerID, Remaining: s.policy.effectiveTimeout(s.policy.Initial, entry.skipCount[before.CurrentPlayerID])}
		return outcome, nil
	}
	// Any other accepted action clears the penalty accumulated by the requester.
	s.manager.clearSkip(entry, action.PlayerID)
	return s.manager.publish(entry, before, entry.engine.Snapshot(), result), nil
}

func (s *Service) checkSkipTiming(entry *managedGame, before uno.State, actor Actor) error {
	if before.Phase != uno.TakingTurn || before.Pending != nil || before.CurrentPlayerID == 0 {
		return uno.ErrInvalidAction
	}
	if actor.PlayerID == before.CurrentPlayerID {
		return nil
	}
	effective := s.policy.effectiveTimeout(s.policy.Initial, entry.skipCount[before.CurrentPlayerID])
	elapsed := time.Since(entry.turnStarted)
	if elapsed < effective {
		return &ErrSkipTooEarly{Wait: effective - elapsed}
	}
	return nil
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
		if actor.ChatID == 0 || (!actor.ChatAdmin && actor.PlayerID != entry.ownerID && actor.PlayerID != entry.creatorID) {
			return ErrForbidden
		}
	case uno.PlayCard, uno.DrawCard, uno.PassTurn, uno.ChooseColor, uno.ChoosePlayer, uno.KeepHand, uno.CallBluff:
		// Inline actions have no chat context. The engine still validates the actor.
	case uno.ForceSkipTurn, uno.KickPlayer:
		// Administrative actions over the running game: the owner of the game
		// or an administrator of the chat.
		if actor.ChatID == 0 || (!actor.ChatAdmin && actor.PlayerID != entry.ownerID) {
			return ErrForbidden
		}
	case uno.PenaltySkip:
		// Any active participant may request a skip. Admins have no shortcut:
		// the waiting-time rule is enforced by checkSkipTiming for everyone.
		if actor.ChatID == 0 {
			return ErrForbidden
		}
		player := entry.engine.Snapshot().Player(actor.PlayerID)
		if player == nil || player.Status != uno.Playing {
			return ErrForbidden
		}
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

// ExpiredTurn identifies a particular revision, not a future turn in the chat.
// Only a trusted scheduler should submit these candidates, never player input.
type ExpiredTurn struct {
	GameID   uno.GameID
	ChatID   ChatID
	PlayerID uno.PlayerID
	Revision uint64
}

// ExpiredTurns discovers candidates without changing any game. Adapters must
// enqueue their execution in the same chat queue as player actions.
func (s *Service) ExpiredTurns(ctx context.Context, timeout time.Duration) []ExpiredTurn {
	if ctx == nil || timeout <= 0 {
		return nil
	}
	return s.manager.expiredTurns(ctx, timeout, time.Now())
}

// AutoSkipTurn revalidates a candidate and its deadline under the game lock.
// Obsolete candidates return applied=false and must not produce a notification.
func (s *Service) AutoSkipTurn(ctx context.Context, candidate ExpiredTurn, timeout time.Duration) (outcome Outcome, applied bool) {
	if ctx == nil || timeout <= 0 {
		return Outcome{}, false
	}
	return s.manager.skipTurn(ctx, candidate, timeout, time.Now())
}

// ResetChat is an administrative recovery operation. It removes all active and
// retained game state for one chat. Authorization accepts the current game
// owner or a chat administrator verified by a trusted adapter.
func (s *Service) ResetChat(ctx context.Context, actor Actor) (ResetResult, error) {
	if err := checkContext(ctx); err != nil {
		return ResetResult{}, err
	}
	if actor.PlayerID <= 0 || actor.ChatID == 0 {
		return ResetResult{}, ErrInvalidArgument
	}
	return s.manager.resetChat(ctx, actor)
}

// SetLocked changes admission policy under the same lock as JoinGame. It does
// not mutate engine revision, turn deadlines or participants. Owners may be observers.
func (s *Service) SetLocked(ctx context.Context, actor Actor, id uno.GameID, locked bool) (PublicGameView, bool, error) {
	if err := checkContext(ctx); err != nil {
		return PublicGameView{}, false, err
	}
	if actor.PlayerID <= 0 || actor.ChatID == 0 || id == "" {
		return PublicGameView{}, false, ErrInvalidArgument
	}
	entry, err := s.manager.lockGame(ctx, id)
	if err != nil {
		return PublicGameView{}, false, err
	}
	defer entry.mu.Unlock()
	if entry.final != nil {
		return PublicGameView{}, false, ErrGameClosed
	}
	if actor.ChatID != entry.chatID || actor.PlayerID != entry.ownerID {
		return PublicGameView{}, false, ErrForbidden
	}
	if err := ctx.Err(); err != nil {
		return PublicGameView{}, false, err
	}
	changed := entry.locked != locked
	entry.locked = locked
	view := publicView(entry, entry.engine.Snapshot())
	s.manager.indexMu.Lock()
	s.manager.byID[id] = indexRecord{entry: entry, summary: view.summary()}
	s.manager.indexMu.Unlock()
	return view, changed, nil
}

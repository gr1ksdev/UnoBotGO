package telegram

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
)

var (
	ErrInvalidCursor = errors.New("invalid or expired cursor")
)

type ConsumeStatus int

const (
	ConsumeOK ConsumeStatus = iota
	ConsumeNotFound
	ConsumeExpired
	ConsumeUserMismatch
	ConsumeAlreadyConsumed
	ConsumeInvalidType
)

type ActionToken struct {
	Token         string
	UserID        uno.PlayerID
	GameID        uno.GameID
	ChatID        game.ChatID
	Action        uno.Action
	CreatedAt     time.Time
	ExpiresAt     time.Time
	Consumed      bool
	ResultSummary string
}

type CursorKind int

const (
	CursorKindHand CursorKind = iota + 1
	CursorKindGames
)

type CursorToken struct {
	Token     string
	UserID    uno.PlayerID
	Kind      CursorKind
	GameID    uno.GameID
	Revision  uint64
	Offset    int
	CreatedAt time.Time
	ExpiresAt time.Time
}

type TokenStore struct {
	mu          sync.Mutex
	globalLimit int
	userLimit   int
	tokens      map[string]any // *ActionToken or *CursorToken
	order       []string       // FIFO
	userTokens  map[uno.PlayerID][]string
	now         func() time.Time
	randReader  io.Reader
}

func NewTokenStore(globalLimit, userLimit int, now func() time.Time, randReader io.Reader) *TokenStore {
	if globalLimit <= 0 {
		globalLimit = 20000
	}
	if userLimit <= 0 {
		userLimit = 512
	}
	if now == nil {
		now = time.Now
	}
	if randReader == nil {
		randReader = rand.Reader
	}
	return &TokenStore{
		globalLimit: globalLimit,
		userLimit:   userLimit,
		tokens:      make(map[string]any),
		order:       make([]string, 0),
		userTokens:  make(map[uno.PlayerID][]string),
		now:         now,
		randReader:  randReader,
	}
}

func (s *TokenStore) generateTokenString() (string, error) {
	var buf [16]byte
	if _, err := io.ReadFull(s.randReader, buf[:]); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func (s *TokenStore) cleanupExpiredOpportunistic(currentTime time.Time) {
	// Clean up up to 10 expired tokens from the front of the FIFO queue
	cleaned := 0
	for len(s.order) > 0 && cleaned < 10 {
		oldest := s.order[0]
		tok, ok := s.tokens[oldest]
		if !ok {
			s.order = s.order[1:]
			continue
		}
		var exp time.Time
		switch t := tok.(type) {
		case *ActionToken:
			exp = t.ExpiresAt
		case *CursorToken:
			exp = t.ExpiresAt
		}
		if currentTime.After(exp) {
			s.deleteTokenLocked(oldest)
			cleaned++
		} else {
			break
		}
	}
}

func (s *TokenStore) deleteTokenLocked(tokenStr string) {
	tok, ok := s.tokens[tokenStr]
	if !ok {
		return
	}
	delete(s.tokens, tokenStr)

	// Remove from order
	if idx := slices.Index(s.order, tokenStr); idx >= 0 {
		s.order = slices.Delete(s.order, idx, idx+1)
	}

	// Remove from userTokens
	var uid uno.PlayerID
	switch t := tok.(type) {
	case *ActionToken:
		uid = t.UserID
	case *CursorToken:
		uid = t.UserID
	}
	if ulist, exists := s.userTokens[uid]; exists {
		if idx := slices.Index(ulist, tokenStr); idx >= 0 {
			s.userTokens[uid] = slices.Delete(ulist, idx, idx+1)
		}
		if len(s.userTokens[uid]) == 0 {
			delete(s.userTokens, uid)
		}
	}
}

func (s *TokenStore) evictIfNecessaryLocked(uid uno.PlayerID) {
	// User limit eviction
	if len(s.userTokens[uid]) >= s.userLimit {
		oldestUserTok := s.userTokens[uid][0]
		s.deleteTokenLocked(oldestUserTok)
	}

	// Global limit eviction
	if len(s.tokens) >= s.globalLimit && len(s.order) > 0 {
		oldestGlobalTok := s.order[0]
		s.deleteTokenLocked(oldestGlobalTok)
	}
}

// CreateActionToken registers an unpredictable token for a player's action.
func (s *TokenStore) CreateActionToken(
	userID uno.PlayerID,
	gameID uno.GameID,
	chatID game.ChatID,
	action uno.Action,
	ttl time.Duration,
) (string, error) {
	tokStr, err := s.generateTokenString()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.cleanupExpiredOpportunistic(now)
	s.evictIfNecessaryLocked(userID)

	tok := &ActionToken{
		Token:     tokStr,
		UserID:    userID,
		GameID:    gameID,
		ChatID:    chatID,
		Action:    action,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	s.tokens[tokStr] = tok
	s.order = append(s.order, tokStr)
	s.userTokens[userID] = append(s.userTokens[userID], tokStr)

	return tokStr, nil
}

// CreateCursorToken registers an unpredictable cursor token for pagination.
func (s *TokenStore) CreateCursorToken(
	userID uno.PlayerID,
	kind CursorKind,
	gameID uno.GameID,
	revision uint64,
	offset int,
	ttl time.Duration,
) (string, error) {
	tokStr, err := s.generateTokenString()
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.cleanupExpiredOpportunistic(now)
	s.evictIfNecessaryLocked(userID)

	tok := &CursorToken{
		Token:     tokStr,
		UserID:    userID,
		Kind:      kind,
		GameID:    gameID,
		Revision:  revision,
		Offset:    offset,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	s.tokens[tokStr] = tok
	s.order = append(s.order, tokStr)
	s.userTokens[userID] = append(s.userTokens[userID], tokStr)

	return tokStr, nil
}

// ConsumeAction atomically consumes an action token. Only one caller will obtain ConsumeOK.
func (s *TokenStore) ConsumeAction(tokenStr string, actorID uno.PlayerID) (ActionToken, ConsumeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokVal, ok := s.tokens[tokenStr]
	if !ok {
		return ActionToken{}, ConsumeNotFound
	}

	tok, ok := tokVal.(*ActionToken)
	if !ok {
		return ActionToken{}, ConsumeInvalidType
	}

	if s.now().After(tok.ExpiresAt) {
		return ActionToken{}, ConsumeExpired
	}

	if tok.UserID != actorID {
		return ActionToken{}, ConsumeUserMismatch
	}

	if tok.Consumed {
		res := *tok
		return res, ConsumeAlreadyConsumed
	}

	tok.Consumed = true
	res := *tok
	return res, ConsumeOK
}

// SetActionResult records the public outcome of an action token and wipes private action details.
func (s *TokenStore) SetActionResult(tokenStr string, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tokVal, ok := s.tokens[tokenStr]; ok {
		if tok, ok := tokVal.(*ActionToken); ok {
			tok.ResultSummary = summary
			tok.Action.CardID = ""
		}
	}
}

// GetActionStatus queries the status of an action token without consuming it.
func (s *TokenStore) GetActionStatus(tokenStr string) (summary string, consumed bool, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokVal, ok := s.tokens[tokenStr]
	if !ok {
		return "", false, false
	}
	tok, ok := tokVal.(*ActionToken)
	if !ok {
		return "", false, false
	}
	return tok.ResultSummary, tok.Consumed, true
}

// GetCursor returns the cursor token if valid, non-expired and matching actorID.
func (s *TokenStore) GetCursor(tokenStr string, actorID uno.PlayerID) (CursorToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokVal, ok := s.tokens[tokenStr]
	if !ok {
		return CursorToken{}, ErrInvalidCursor
	}

	cursor, ok := tokVal.(*CursorToken)
	if !ok {
		return CursorToken{}, ErrInvalidCursor
	}

	if s.now().After(cursor.ExpiresAt) {
		return CursorToken{}, ErrInvalidCursor
	}

	if cursor.UserID != actorID {
		return CursorToken{}, ErrInvalidCursor
	}

	return *cursor, nil
}

// InvalidateGame invalidates all tokens associated with a given GameID.
func (s *TokenStore) InvalidateGame(gameID uno.GameID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	toDelete := make([]string, 0)
	for k, v := range s.tokens {
		switch t := v.(type) {
		case *ActionToken:
			if t.GameID == gameID {
				toDelete = append(toDelete, k)
			}
		case *CursorToken:
			if t.GameID == gameID {
				toDelete = append(toDelete, k)
			}
		}
	}
	for _, k := range toDelete {
		s.deleteTokenLocked(k)
	}
}

// InvalidateUserGame invalidates all tokens associated with a given user and game.
func (s *TokenStore) InvalidateUserGame(gameID uno.GameID, userID uno.PlayerID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	toDelete := make([]string, 0)
	for k, v := range s.tokens {
		switch t := v.(type) {
		case *ActionToken:
			if t.GameID == gameID && t.UserID == userID {
				toDelete = append(toDelete, k)
			}
		case *CursorToken:
			if t.GameID == gameID && t.UserID == userID {
				toDelete = append(toDelete, k)
			}
		}
	}
	for _, k := range toDelete {
		s.deleteTokenLocked(k)
	}
}

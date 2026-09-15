package game

import "errors"

var (
	ErrInvalidArgument = errors.New("invalid application argument")
	ErrForbidden       = errors.New("operation not authorized")
	ErrGameNotFound    = errors.New("game not found")
	ErrNoActiveGame    = errors.New("no active game in chat")
	ErrChatOccupied    = errors.New("chat already has an open game")
	ErrGameClosed      = errors.New("game closed")
	ErrNotParticipant  = errors.New("not an active participant")
	ErrIDConflict      = errors.New("game ID collision")
)

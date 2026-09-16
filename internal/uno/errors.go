package uno

import "errors"

var (
	ErrInvalidState        = errors.New("invalid game state")
	ErrInvalidAction       = errors.New("invalid action")
	ErrInvalidRules        = errors.New("invalid rules")
	ErrUnknownPlayer       = errors.New("unknown player")
	ErrAlreadyJoined       = errors.New("player already registered")
	ErrPlayerLimit         = errors.New("player limit reached")
	ErrNotEnoughPlayers    = errors.New("not enough players")
	ErrLobbyClosed         = errors.New("late join disabled")
	ErrGameNotStarted      = errors.New("game not started")
	ErrGameStarted         = errors.New("game already started")
	ErrGameFinished        = errors.New("game finished")
	ErrNotYourTurn         = errors.New("not your turn")
	ErrCardNotOwned        = errors.New("card not owned")
	ErrCardNotPlayable     = errors.New("card not playable")
	ErrCardNotFound        = errors.New("card not found")
	ErrStaleRevision       = errors.New("stale revision")
	ErrColorChoiceRequired = errors.New("color choice required")
	ErrNoColorChoice       = errors.New("no color choice pending")
	ErrInvalidColor        = errors.New("invalid color")
	ErrAlreadyDrawn        = errors.New("already drawn this turn")
	ErrCannotPass          = errors.New("draw before passing")
	ErrDeckEmpty           = errors.New("not enough drawable cards")
	ErrNoChallenge         = errors.New("no draw four challenge pending")
)

package uno

type ActionType uint8

const (
	JoinGame ActionType = iota + 1
	LeaveGame
	StartGame
	PlayCard
	DrawCard
	PassTurn
	ChooseColor
	CancelGame
	SkipTurn
	CallBluff
	SetRules
	ChoosePlayer
	KeepHand
	// ForceSkipTurn advances the turn of the current player. The requester
	// may be an administrator who is not playing.
	ForceSkipTurn
	// KickPlayer removes another active participant. TargetID names the
	// participant to remove.
	KickPlayer
	// PenaltySkip advances the turn after drawing one card for the skipped
	// player. Any active participant may request it; the application enforces
	// the waiting-time rule before submitting the action.
	PenaltySkip
)

// Action uses only domain IDs. The application authenticates PlayerID and
// authorizes management actions. PlayerID is the real requester, not proof of
// participation. StartGame and CancelGame do not require requester membership;
// player actions still require an active participant. The engine checks rules.
// DealerID is required by StartGame and must identify an active player.
// Revision is mandatory for every action, including lobby mutations.
type Action struct {
	Type     ActionType
	PlayerID PlayerID
	Revision uint64
	CardID   CardID
	Color    Color
	TargetID PlayerID // ChoosePlayer or KickPlayer only; another active participant.
	DealerID PlayerID
	Rules    Rules
}

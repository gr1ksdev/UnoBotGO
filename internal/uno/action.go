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
	ChallengeDrawFour
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
	DealerID PlayerID
}

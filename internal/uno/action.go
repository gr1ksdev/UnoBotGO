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
)

// Action uses only domain IDs. The application authenticates PlayerID and
// authorizes management actions; the engine validates membership and rules.
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

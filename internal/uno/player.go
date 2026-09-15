package uno

type PlayerStatus uint8

const (
	Playing PlayerStatus = iota
	WentOut
	Left
)

type Player struct {
	ID     PlayerID
	Hand   []CardID
	Status PlayerStatus
}

type Placement struct {
	PlayerID PlayerID
	Position int
	WentOut  bool
}

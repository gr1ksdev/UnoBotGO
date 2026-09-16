package uno

// EndPolicy is independent of card matching and action-card effects.
type EndPolicy uint8

const (
	FirstWinner EndPolicy = iota
	Placements
)

type Rules struct {
	EndPolicy     EndPolicy
	AllowLateJoin bool
	NumberedStart bool
}

func ClassicRules() Rules { return Rules{EndPolicy: FirstWinner} }
func BotRules() Rules     { return Rules{EndPolicy: Placements, AllowLateJoin: true, NumberedStart: true} }

// CanPlayDrawFour is the official color restriction, independent of turn/UI.
// Evaluate the entire hand, including after a voluntary draw.
func CanPlayDrawFour(hand []Card, activeColor Color) bool {
	if !activeColor.valid() {
		return false
	}
	for _, c := range hand {
		if c.Color == activeColor {
			return false
		}
	}
	return true
}

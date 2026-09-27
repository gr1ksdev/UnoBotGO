package uno

// EndPolicy is independent of card matching and action-card effects.
type EndPolicy uint8

const (
	FirstWinner EndPolicy = iota
	Placements
)

type Rules struct {
	EndPolicy         EndPolicy
	AllowLateJoin     bool
	NumberedStart     bool
	StackDrawTwo      bool
	StackWildDrawFour bool
	// Caseiro permits a Wild Draw Four to answer a pending Draw Two and a
	// Draw Two of the chosen colour to answer a pending Wild Draw Four.
	StackWildDrawFourOnTwo  bool
	StackDrawTwoOnWildFour  bool
	NoWildFinish            bool
	NoWildOnWild            bool
	AllowWildDrawFourAlways bool
	AllowSwapHands          bool
	FreePlayAfterDraw       bool
}

func ClassicRules() Rules { return Rules{EndPolicy: FirstWinner} }
func BotRules() Rules {
	return Rules{
		EndPolicy:               Placements,
		AllowLateJoin:           true,
		NumberedStart:           true,
		StackDrawTwo:            true,
		StackWildDrawFour:       true,
		NoWildFinish:            true,
		NoWildOnWild:            true,
		AllowWildDrawFourAlways: true,
		FreePlayAfterDraw:       true,
	}
}

// CaseiroRules preserves the homologated bot lifecycle while enabling the
// V1 responses for accumulated penalties and the house-only hand swap card.
func CaseiroRules() Rules {
	r := BotRules()
	r.StackWildDrawFour = false
	r.StackWildDrawFourOnTwo = true
	r.StackDrawTwoOnWildFour = true
	r.AllowSwapHands = true
	return r
}

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

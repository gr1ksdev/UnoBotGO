package uno

type EventType string

const (
	PlayerJoined        EventType = "player_joined"
	PlayerLeft          EventType = "player_left"
	GameStarted         EventType = "game_started"
	CardPlayed          EventType = "card_played"
	CardsDrawn          EventType = "cards_drawn"
	PlayerSkipped       EventType = "player_skipped"
	DirectionChanged    EventType = "direction_changed"
	ColorChoiceRequired EventType = "color_choice_required"
	ColorChosen         EventType = "color_chosen"
	TurnChanged         EventType = "turn_changed"
	UnoAnnounced        EventType = "uno_announced"
	PlayerWon           EventType = "player_won"
	GameFinished        EventType = "game_finished"
)

// Event contains public facts only. CardsDrawn never exposes private card IDs.
type Event struct {
	Type      EventType
	PlayerID  PlayerID
	CardID    CardID
	Count     int
	Color     Color
	Direction int
	Position  int
	Reason    FinishReason
}

type Result struct {
	Revision uint64
	Events   []Event
}

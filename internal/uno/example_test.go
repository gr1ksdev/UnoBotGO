package uno_test

import (
	"fmt"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func ExampleGame() {
	g, err := uno.NewGame("round-1", uno.ClassicRules(),
		uno.WithShuffler(func([]uno.CardID) {}))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, id := range []uno.PlayerID{1, 2} {
		_, err = g.Apply(uno.Action{Type: uno.JoinGame, PlayerID: id, Revision: g.Snapshot().Revision})
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	result, err := g.Apply(uno.Action{Type: uno.StartGame, PlayerID: 1, DealerID: 2, Revision: 2})
	if err != nil {
		fmt.Println(err)
		return
	}
	state := g.Snapshot()
	fmt.Println(result.Revision, len(state.Players[0].Hand), len(state.Players[1].Hand))
	// Output: 3 7 7
}

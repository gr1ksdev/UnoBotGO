// Package uno implements UNO card rules without any transport or persistence
// dependency. Game is a single-owner engine: callers must serialize access to
// the same instance. Different games can be used independently.
//
// Every accepted action increments Revision exactly once. Errors leave the
// serializable state unchanged. Snapshot and Restore deep-copy all slices and
// pending state. Snapshot contains private hands and must stay server-side.
// Applications should expose player-specific views, not entire snapshots.
//
// ClassicRules ends at the first player who empties their hand. BotRules keeps
// playing for placements and permits late joining. Both use Classic card effects,
// automatic UNO announcements and strict Wild Draw Four color validation.
//
// A deterministic game can be created without starting a bot:
//
//	g, err := uno.NewGame("round-1", uno.ClassicRules(),
//	    uno.WithShuffler(func(ids []uno.CardID) {}))
//	if err != nil { /* handle error */ }
//	result, err := g.Apply(uno.Action{
//	    Type: uno.JoinGame, PlayerID: 1, Revision: 0,
//	})
//
// See docs/v2-rules.md for policies, limits and intentionally deferred features.
package uno

// Package game is the application boundary around the single-owner UNO engine.
// Service authenticates no credentials: Actor must be supplied by a trusted
// adapter. It authorizes lobby management separately from participation and
// exposes only public projections or the authenticated participant's own hand.
//
// Each runtime game has a private mutex. Index lookups release the index lock
// before waiting for that mutex. Accepted actions publish routing projections
// under the index lock while still holding the game lock. Engine execution never
// holds the index lock, and no external I/O is performed under either lock.
//
// Context cancellation is checked before mutation, including after lock waits.
// Once the engine accepts an action, publication completes even if cancellation
// occurs. Closed games release their runtime and retain only bounded public
// summaries. No persistence, Telegram integration or inline tokens are provided.
package game

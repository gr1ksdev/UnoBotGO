package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"net/http"
	"net/url"
	"strconv"

	"time"
)

type liveCommand struct {
	GameID    uno.GameID `json:"game_id"`
	RequestID string     `json:"request_id"`
	Revision  uint64     `json:"expected_revision"`
	Type      string     `json:"action"`
	CardID    uno.CardID `json:"card_id"`
	Color     uno.Color  `json:"color"`
	Target    string     `json:"target"`
}
type seat struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Me       bool   `json:"me"`
	Current  bool   `json:"current"`
	Active   bool   `json:"active"`
	Position int    `json:"position,omitempty"`
}
type publicAward struct {
	Key      string  `json:"key"`
	Score    *string `json:"score_units"`
	Position int     `json:"position"`
}
type liveView struct {
	Awards      []publicAward    `json:"awards"`
	ID          uno.GameID       `json:"game_id"`
	Revision    uint64           `json:"revision"`
	Phase       uno.Phase        `json:"phase"`
	Group       string           `json:"group"`
	Mode        string           `json:"mode"`
	System      string           `json:"system"`
	Players     []seat           `json:"players"`
	Hand        []game.CardView  `json:"hand"`
	Top         *uno.Card        `json:"top"`
	ActiveColor uno.Color        `json:"active_color"`
	Direction   int              `json:"direction"`
	CloseReason game.CloseReason `json:"close_reason"`
	Closed      bool             `json:"closed"`
	Owner       bool             `json:"owner"`
	MyTurn      bool             `json:"my_turn"`
	Drawn       uno.CardID       `json:"drawn_card_id"`
	CanBluff    bool             `json:"can_bluff"`
	Deadline    *time.Time       `json:"deadline"`
	ServerTime  time.Time        `json:"server_time"`
	Result      any              `json:"result"`
}

func (a *API) member(ctx context.Context, id uno.GameID, user int64) (game.PublicGameView, error) {
	if a.Games == nil {
		return game.PublicGameView{}, game.ErrForbidden
	}
	v, err := a.Games.PublicView(ctx, id)
	if err != nil {
		return v, err
	}
	for _, p := range v.Players {
		if int64(p.ID) == user {
			return v, nil
		}
	}
	return game.PublicGameView{}, game.ErrForbidden
}
func (a *API) project(ctx context.Context, id uno.GameID, user int64) (liveView, error) {
	v, err := a.member(ctx, id, user)
	if err != nil {
		return liveView{}, err
	}
	out := liveView{ID: id, Revision: v.Revision, Phase: v.Phase, Group: v.ChatName, Mode: "classic", System: string(v.GroupConfig.RankingSystem), Players: []seat{}, Hand: []game.CardView{}, Top: v.TopCard, ActiveColor: v.ActiveColor, Direction: v.Direction, CloseReason: v.CloseReason, Closed: v.Closed, Owner: int64(v.OwnerID) == user, MyTurn: int64(v.CurrentTurn) == user, CanBluff: v.CanCallBluff && int64(v.CurrentTurn) == user, ServerTime: a.now()}
	if v.Rules.AllowSwapHands {
		out.Mode = "caseiro"
	}
	if a.TurnTimeout > 0 && !v.TurnStarted.IsZero() && !v.Closed {
		d := v.TurnStarted.Add(a.TurnTimeout)
		out.Deadline = &d
	}
	for _, p := range v.Players {
		name := p.Name
		if name == "" {
			name = "Jogador"
		}
		s := seat{Key: a.References.key("seat", int64(p.ID)), Name: name, Count: p.CardCount, Me: int64(p.ID) == user, Current: p.ID == v.CurrentTurn, Active: p.Active}
		for _, pl := range v.Placements {
			if pl.PlayerID == p.ID {
				s.Position = pl.Position
			}
		}
		out.Players = append(out.Players, s)
	}
	if !v.Closed {
		pv, e := a.Games.PlayerView(ctx, game.Actor{PlayerID: uno.PlayerID(user)}, id)
		if e == nil {
			out.Hand = pv.Hand
			out.Drawn = pv.DrawnCardID
		} else if !errors.Is(e, game.ErrNotParticipant) {
			return out, e
		}
	}
	if v.Closed && a.Profiles != nil {
		readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
		result, e := a.Profiles.ReadResult(readCtx, string(id), user)
		readCancel()
		if e == nil && len(result) > 0 {
			out.Result = result[0]
			for _, award := range result[0].Awards {
				out.Awards = append(out.Awards, publicAward{Key: a.References.key("seat", award.UserID), Score: award.Score, Position: award.Position})
			}
		}
	}
	return out, nil
}
func (a *API) rooms(w http.ResponseWriter, r *http.Request) {
	if a.Games == nil {
		apiError(w, 503, "unavailable")
		return
	}
	user, _ := UserIDFromContext(r.Context())
	games, err := a.Games.FindPlayerGames(r.Context(), game.Actor{PlayerID: uno.PlayerID(user)})
	if err != nil {
		apiError(w, 503, "unavailable")
		return
	}
	out := []map[string]any{}
	for _, g := range games {
		out = append(out, map[string]any{"game_id": g.GameID, "group": g.ChatName, "phase": g.Phase})
	}
	writeJSON(w, 200, out)
}
func (a *API) snapshot(w http.ResponseWriter, r *http.Request) {
	user, _ := UserIDFromContext(r.Context())
	view, err := a.project(r.Context(), uno.GameID(r.PathValue("id")), user)
	if err != nil {
		apiError(w, 403, "forbidden")
		return
	}
	writeJSON(w, 200, view)
}
func (a *API) command(ctx context.Context, user int64, c liveCommand) error {
	v, err := a.member(ctx, c.GameID, user)
	if err != nil {
		return err
	}
	kinds := map[string]uno.ActionType{"start": uno.StartGame, "play": uno.PlayCard, "draw": uno.DrawCard, "pass": uno.PassTurn, "color": uno.ChooseColor, "target": uno.ChoosePlayer, "keep": uno.KeepHand, "bluff": uno.CallBluff, "leave": uno.LeaveGame, "cancel": uno.CancelGame}
	kind, ok := kinds[c.Type]
	if !ok {
		return uno.ErrInvalidAction
	}
	action := uno.Action{Type: kind, PlayerID: uno.PlayerID(user), Revision: c.Revision, CardID: c.CardID, Color: c.Color}
	if c.Target != "" {
		for _, p := range v.Players {
			if a.References.key("seat", int64(p.ID)) == c.Target {
				action.TargetID = p.ID
			}
		}
		if action.TargetID == 0 {
			return game.ErrForbidden
		}
	}
	outcome, err := a.Games.ApplyWeb(ctx, game.Actor{PlayerID: uno.PlayerID(user), ChatID: v.ChatID}, c.GameID, action, c.RequestID)
	if err != nil {
		return err
	}
	if outcome.Completed != nil && a.Finalizer != nil {
		_, _ = a.Finalizer.Commit(ctx, *outcome.Completed)
	}
	return nil
}
func (a *API) live(w http.ResponseWriter, r *http.Request) {
	if a.Games == nil || a.References == nil {
		apiError(w, 503, "unavailable")
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(20 << 10)
	lifecycle := a.Lifecycle
	if lifecycle == nil {
		lifecycle = context.Background()
	}
	ctx, cancel := context.WithCancel(lifecycle)
	defer cancel()
	authCtx, authCancel := context.WithTimeout(ctx, 5*time.Second)
	var auth struct {
		InitData string     `json:"init_data"`
		GameID   uno.GameID `json:"game_id"`
	}
	err = wsjson.Read(authCtx, conn, &auth)
	authCancel()
	if err != nil {
		return
	}
	user, err := ValidateInitData(auth.InitData, a.Token, a.now(), a.MaxAge)
	if err != nil {
		conn.Close(websocket.StatusPolicyViolation, "unauthorized")
		return
	}
	if _, err := a.member(ctx, auth.GameID, user); err != nil {
		conn.Close(websocket.StatusPolicyViolation, "forbidden")
		return
	}
	a.wsMu.Lock()
	if a.wsUsers[user] >= 3 {
		a.wsMu.Unlock()
		conn.Close(websocket.StatusTryAgainLater, "busy")
		return
	}
	a.wsUsers[user]++
	a.wsMu.Unlock()
	defer func() {
		a.wsMu.Lock()
		a.wsUsers[user]--
		if a.wsUsers[user] == 0 {
			delete(a.wsUsers, user)
		}
		a.wsMu.Unlock()
	}()
	changes, unsubscribe := a.Games.Subscribe()
	defer unsubscribe()
	messages := make(chan json.RawMessage, 1)
	go func() {
		defer cancel()
		for {
			_, data, e := conn.Read(ctx)
			if e != nil {
				return
			}
			select {
			case messages <- data:
			case <-ctx.Done():
				return
			}
		}
	}()
	// Bound session lifetime; reconnect requires fresh validation of Telegram credentials.
	values, _ := url.ParseQuery(auth.InitData)
	authDate, _ := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	remainingAuth := time.Unix(authDate, 0).Add(a.MaxAge).Sub(a.now())
	if remainingAuth <= 0 {
		return
	}
	expiry := time.NewTimer(remainingAuth)
	defer expiry.Stop()
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	terminalUnconfirmed := false
	send := func(kind, request, reason string) bool {
		v, e := a.project(ctx, auth.GameID, user)
		if e != nil {
			return false
		}
		terminalUnconfirmed = v.Closed && v.Result == nil && v.CloseReason != game.Cancelled
		writeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		return wsjson.Write(writeCtx, conn, map[string]any{"type": kind, "request_id": request, "reason": reason, "view": v}) == nil
	}
	if !send("snapshot", "", "") {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-expiry.C:
			return
		case <-heartbeat.C:
			pingCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			e := conn.Ping(pingCtx)
			stop()
			if e != nil {
				return
			}
			// A confirmed terminal snapshot may precede a temporarily unavailable DB
			// read. Recover only its score status; live turns remain event-driven.
			if terminalUnconfirmed && !send("snapshot", "", "") {
				return
			}
		case <-changes:
			if !send("snapshot", "", "") {
				return
			}
		case raw := <-messages:
			var c liveCommand
			reason := ""
			kind := "accepted"
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&c) != nil || c.GameID != auth.GameID || !a.allow(user) {
				reason = "invalid_request"
			} else if e := a.command(ctx, user, c); e != nil {
				reason = "action_rejected"
				if errors.Is(e, uno.ErrStaleRevision) {
					reason = "stale_revision"
				}
			}
			if reason != "" {
				kind = "rejected"
			}
			if !send(kind, c.RequestID, reason) {
				return
			}
		}
	}
}

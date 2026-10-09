package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/uno"
)

type RoomGroupRepository interface {
	WebAppGroups(context.Context, int64) ([]groups.Config, error)
	GetOrCreateGroupConfig(context.Context, int64) (groups.Config, error)
	ObserveGroupTitle(context.Context, int64, string) error
}
type roomReceiptKey struct {
	User    int64
	Request string
}
type roomReceipt struct {
	Chat int64
	Mode string
	ID   uno.GameID
}

func decodeRoom(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		apiError(w, 400, "invalid_request")
		return false
	}
	return true
}
func (a *API) groupOption(ctx context.Context, user int64, c groups.Config) (map[string]any, error) {
	if a.VerifyRoomGroup == nil {
		return nil, game.ErrForbidden
	}
	title, err := a.VerifyRoomGroup(ctx, c.ChatID, user)
	if err != nil {
		return nil, err
	}
	ref, err := a.References.seal(reference{Kind: "room_group", ID: c.ChatID, Expires: a.now().Add(time.Hour).Unix()})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ref": ref, "title": title, "system": c.RankingSystem}, nil
}
func (a *API) roomGroups(w http.ResponseWriter, r *http.Request) {
	if a.RoomGroups == nil || a.VerifyRoomGroup == nil {
		apiError(w, 503, "unavailable")
		return
	}
	user, _ := UserIDFromContext(r.Context())
	candidates, err := a.RoomGroups.WebAppGroups(r.Context(), user)
	if err != nil {
		apiError(w, 503, "unavailable")
		return
	}
	out := []map[string]any{}
	for _, c := range candidates {
		if item, e := a.groupOption(r.Context(), user, c); e == nil {
			out = append(out, item)
		}
	}
	writeJSON(w, 200, out)
}
func (a *API) resolveRoomGroup(w http.ResponseWriter, r *http.Request) {
	if a.RoomGroups == nil || a.ResolveRoomGroup == nil {
		apiError(w, 503, "unavailable")
		return
	}
	var body struct {
		Username string `json:"username"`
	}
	if !decodeRoom(w, r, &body) {
		return
	}
	user, _ := UserIDFromContext(r.Context())
	id, title, err := a.ResolveRoomGroup(r.Context(), body.Username, user)
	if err != nil || id >= 0 {
		apiError(w, 403, "group_not_verified")
		return
	}
	c, err := a.RoomGroups.GetOrCreateGroupConfig(r.Context(), id)
	if err != nil {
		apiError(w, 503, "unavailable")
		return
	}
	_ = a.RoomGroups.ObserveGroupTitle(r.Context(), id, title)
	item, err := a.groupOption(r.Context(), user, c)
	if err != nil {
		apiError(w, 403, "group_not_verified")
		return
	}
	writeJSON(w, 200, item)
}
func (a *API) createRoom(w http.ResponseWriter, r *http.Request) {
	if a.Games == nil || a.RoomGroups == nil || a.VerifyRoomGroup == nil {
		apiError(w, 503, "unavailable")
		return
	}
	var body struct {
		Group   string `json:"group_ref"`
		Mode    string `json:"mode"`
		Request string `json:"request_id"`
	}
	if !decodeRoom(w, r, &body) {
		return
	}
	if len(body.Request) < 8 || len(body.Request) > 110 || !groups.Mode(body.Mode).Valid() {
		apiError(w, 400, "invalid_request")
		return
	}
	ref, err := a.References.open(body.Group, a.now())
	if err != nil || ref.Kind != "room_group" || ref.ID >= 0 {
		apiError(w, 403, "group_not_verified")
		return
	}
	user, _ := UserIDFromContext(r.Context())
	title, err := a.VerifyRoomGroup(r.Context(), ref.ID, user)
	if err != nil {
		apiError(w, 403, "group_not_verified")
		return
	}
	cfg, err := a.RoomGroups.GetOrCreateGroupConfig(r.Context(), ref.ID)
	if err != nil {
		apiError(w, 503, "unavailable")
		return
	}
	_ = a.RoomGroups.ObserveGroupTitle(r.Context(), ref.ID, title)
	a.roomMu.Lock()
	defer a.roomMu.Unlock()
	key := roomReceiptKey{user, body.Request}
	receipt, exists := a.createdRooms[key]
	if exists && (receipt.Chat != ref.ID || receipt.Mode != body.Mode) {
		apiError(w, 409, "request_conflict")
		return
	}
	actor := game.Actor{PlayerID: uno.PlayerID(user), ChatID: game.ChatID(ref.ID)}
	if !exists {
		if len(a.createdRooms) >= 2048 {
			apiError(w, 429, "busy")
			return
		}
		rules := uno.BotRules()
		if body.Mode == "caseiro" {
			rules = uno.CaseiroRules()
		}
		out, e := a.Games.Create(r.Context(), actor, game.CreateRequest{ChatName: title, Rules: rules, GroupConfig: cfg.Snapshot()})
		if e != nil {
			apiError(w, 409, "group_has_room")
			return
		}
		receipt = roomReceipt{Chat: ref.ID, Mode: body.Mode, ID: out.View.GameID}
		a.createdRooms[key] = receipt
	}
	v, err := a.Games.PublicView(r.Context(), receipt.ID)
	if err != nil {
		apiError(w, 409, "room_closed")
		return
	}
	if !v.Closed {
		joined := false
		for _, p := range v.Players {
			if p.ID == actor.PlayerID {
				joined = true
			}
		}
		if !joined {
			_, err = a.Games.ApplyWeb(r.Context(), actor, receipt.ID, uno.Action{Type: uno.JoinGame, PlayerID: actor.PlayerID, Revision: v.Revision}, body.Request+"-join")
			if err != nil && !errors.Is(err, uno.ErrAlreadyJoined) {
				apiError(w, 409, "admission_failed")
				return
			}
		}
		name, username := roomIdentity(r)
		_ = a.Games.ObservePlayer(r.Context(), actor, receipt.ID, name, username)
	}
	view, err := a.project(r.Context(), receipt.ID, user)
	if err != nil {
		apiError(w, 403, "forbidden")
		return
	}
	writeJSON(w, 200, view)
}
func roomIdentity(r *http.Request) (string, string) {
	raw, _ := strings.CutPrefix(r.Header.Get("Authorization"), "tma ")
	v, _ := url.ParseQuery(raw)
	var u struct {
		First    string `json:"first_name"`
		Last     string `json:"last_name"`
		Username string `json:"username"`
	}
	_ = json.Unmarshal([]byte(v.Get("user")), &u)
	name := strings.TrimSpace(u.First + " " + u.Last)
	if name == "" {
		name = "Jogador"
	}
	return name, u.Username
}
func (a *API) roomInvite(w http.ResponseWriter, r *http.Request) {
	user, _ := UserIDFromContext(r.Context())
	v, err := a.member(r.Context(), uno.GameID(r.PathValue("id")), user)
	if err != nil || v.Closed {
		apiError(w, 403, "forbidden")
		return
	}
	if a.BotUsername == nil || a.BotUsername() == "" {
		apiError(w, 503, "bot_starting")
		return
	}
	token, err := a.References.seal(reference{Kind: "invite", Game: string(v.GameID), Expires: a.now().Add(7 * 24 * time.Hour).Unix()})
	if err != nil {
		apiError(w, 503, "unavailable")
		return
	}
	link := "https://t.me/" + a.BotUsername() + "/ranking?startapp=join_" + token
	writeJSON(w, 200, map[string]string{"url": link, "token": token})
}
func (a *API) invited(r *http.Request) (game.PublicGameView, int64, error) {
	user, _ := UserIDFromContext(r.Context())
	ref, err := a.References.open(r.PathValue("token"), a.now())
	if err != nil || ref.Kind != "invite" || ref.Game == "" || a.Games == nil || a.VerifyRoomGroup == nil {
		return game.PublicGameView{}, user, game.ErrForbidden
	}
	v, err := a.Games.PublicView(r.Context(), uno.GameID(ref.Game))
	if err != nil {
		return v, user, err
	}
	if _, err = a.VerifyRoomGroup(r.Context(), int64(v.ChatID), user); err != nil {
		return v, user, game.ErrForbidden
	}
	return v, user, nil
}
func (a *API) previewInvite(w http.ResponseWriter, r *http.Request) {
	v, user, err := a.invited(r)
	if err != nil {
		apiError(w, 403, "invite_unavailable")
		return
	}
	players := []seat{}
	canReenter := false
	joined := false
	count := 0
	for _, p := range v.Players {
		if p.Active {
			count++
		}
		if int64(p.ID) == user && p.Status == uno.Left {
			canReenter = true
		}
		if int64(p.ID) == user && p.Active {
			joined = true
		}
		name := p.Name
		if name == "" {
			name = "Jogador"
		}
		players = append(players, seat{Key: a.References.key("seat", int64(p.ID)), Name: name, Me: int64(p.ID) == user, Active: p.Active})
	}
	mode := "classic"
	if v.Rules.AllowSwapHands {
		mode = "caseiro"
	}
	writeJSON(w, 200, map[string]any{"game_id": v.GameID, "revision": v.Revision, "group": v.ChatName, "mode": mode, "system": v.GroupConfig.RankingSystem, "players": players, "capacity": 10, "player_count": count, "owner_key": a.References.key("seat", int64(v.OwnerID)), "joined": joined, "can_join": !v.Closed && !v.Locked && (len(v.Players) < 10 || canReenter) && (v.Phase == uno.Lobby || v.Rules.AllowLateJoin), "closed": v.Closed})
}
func (a *API) joinInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Request  string `json:"request_id"`
		Revision uint64 `json:"expected_revision"`
	}
	if !decodeRoom(w, r, &body) {
		return
	}
	v, user, err := a.invited(r)
	if err != nil {
		apiError(w, 403, "invite_unavailable")
		return
	}
	actor := game.Actor{PlayerID: uno.PlayerID(user), ChatID: v.ChatID}
	_, err = a.Games.ApplyWeb(r.Context(), actor, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: actor.PlayerID, Revision: body.Revision}, body.Request)
	if err != nil && !errors.Is(err, uno.ErrAlreadyJoined) {
		apiError(w, 409, "admission_failed")
		return
	}
	name, username := roomIdentity(r)
	_ = a.Games.ObservePlayer(r.Context(), actor, v.GameID, name, username)
	a.Games.Signal()
	view, err := a.project(r.Context(), v.GameID, user)
	if err != nil {
		apiError(w, 403, "forbidden")
		return
	}
	writeJSON(w, 200, view)
}

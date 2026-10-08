package httpapi

import (
	"encoding/json"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (a *API) configuration(w http.ResponseWriter, r *http.Request) {
	name := ""
	if a.BotUsername != nil {
		name = a.BotUsername()
	}
	if name == "" {
		apiError(w, 503, "bot_starting")
		return
	}
	writeJSON(w, 200, map[string]string{"bot_username": name})
}
func (a *API) profile(w http.ResponseWriter, r *http.Request) {
	if a.Profiles == nil {
		apiError(w, 503, "unavailable")
		return
	}
	user, _ := UserIDFromContext(r.Context())
	var before *ranking.HistoryKey
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		ref, err := a.References.open(raw, a.now())
		if err != nil || ref.Kind != "history" || ref.ID != user {
			apiError(w, 400, "invalid_cursor")
			return
		}
		if ref.After == nil || ref.After.Activity.IsZero() || ref.After.Name == "" {
			apiError(w, 400, "invalid_cursor")
			return
		}
		before = &ranking.HistoryKey{Finished: ref.After.Activity, ID: ref.After.Name}
	}
	data, err := a.Profiles.ReadProfile(r.Context(), user, before)
	if err != nil {
		apiError(w, 503, "unavailable")
		return
	}
	if len(data.History) > 20 {
		data.History = data.History[:20]
		data.Next, err = a.References.seal(reference{Kind: "history", ID: user, After: &ranking.PageKey{Activity: data.History[19].Finished, Name: data.History[19].ID}, Expires: a.now().Add(time.Hour).Unix()})
		if err != nil {
			apiError(w, 503, "unavailable")
			return
		}
	}
	raw, _ := strings.CutPrefix(r.Header.Get("Authorization"), "tma ")
	values, _ := url.ParseQuery(raw)
	var metadata struct {
		First    string `json:"first_name"`
		Last     string `json:"last_name"`
		Username string `json:"username"`
	}
	_ = json.Unmarshal([]byte(values.Get("user")), &metadata)
	name := strings.TrimSpace(metadata.First + " " + metadata.Last)
	if name == "" {
		name = metadata.Username
	}
	if name == "" {
		name = "Jogador"
	}
	identity, err := a.makeItem(ranking.GlobalRow{ID: user, Name: name}, "user", ranking.GlobalRequest{System: groups.Updated, Month: a.Rankings.CurrentMonth()})
	if err != nil {
		apiError(w, 503, "unavailable")
		return
	}
	writeJSON(w, 200, struct {
		ranking.Profile
		Identity item `json:"identity"`
	}{data, identity})
}
func (a *API) position(w http.ResponseWriter, r *http.Request) {
	user, _ := UserIDFromContext(r.Context())
	system := groups.RankingSystem(r.URL.Query().Get("system"))
	if system == "" {
		system = groups.Updated
	}
	req := ranking.GlobalRequest{Kind: "players", System: system, Month: a.Rankings.CurrentMonth(), Limit: 1, LookupID: user}
	page, err := a.Rankings.List(r.Context(), req)
	if err != nil {
		apiError(w, 503, "unavailable")
		return
	}
	var mine *item
	if len(page.Rows) > 0 {
		v, err := a.makeItem(page.Rows[0], "user", req)
		if err != nil {
			apiError(w, 503, "unavailable")
			return
		}
		mine = &v
	}
	writeJSON(w, 200, map[string]any{"entry": mine, "month_start": ranking.MonthDateString(req.Month), "month_name": ranking.MonthName(req.Month), "system": system})
}

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

type Media interface {
	Get(kind string, id int64) ([]byte, string, bool)
}

type PrivacyChecker interface {
	IsEntityAnonymous(ctx context.Context, kind string, id int64) (bool, error)
}

type UserPrivacyStore interface {
	GetUserRankingPrivacy(ctx context.Context, userID int64) (bool, error)
	SetUserRankingPrivacy(ctx context.Context, userID int64, private bool) error
}

type API struct {
	Rankings    *ranking.GlobalService
	References  *References
	Media       Media
	Privacy     PrivacyChecker
	UserPrivacy UserPrivacyStore
	Token       string
	MaxAge      time.Duration
	Now         func() time.Time
	mu          sync.Mutex
	clients     map[int64]bucket
	slots       chan struct{}
}
type bucket struct {
	count   int
	expires time.Time
}

func (a *API) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// Handler owns its limiter state; requests never receive upstream errors or IDs.
func (a *API) Handler() http.Handler {
	a.clients = make(map[int64]bucket)
	a.slots = make(chan struct{}, 32)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/rankings/groups", func(w http.ResponseWriter, r *http.Request) { a.list(w, r, "groups") })
	mux.HandleFunc("GET /api/v1/rankings/players", func(w http.ResponseWriter, r *http.Request) { a.list(w, r, "players") })
	mux.HandleFunc("GET /api/v1/rankings/groups/{ref}", func(w http.ResponseWriter, r *http.Request) { a.list(w, r, "detail") })
	mux.HandleFunc("GET /api/v1/media/{ref}", a.avatar)
	mux.HandleFunc("GET /api/v1/me/privacy", a.getMyPrivacy)
	mux.HandleFunc("PUT /api/v1/me/privacy", a.putMyPrivacy)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "tma ")
		id, err := ValidateInitData(raw, a.Token, a.now(), a.MaxAge)
		if !ok || err != nil {
			apiError(w, 401, "unauthorized")
			return
		}
		if !a.allow(id) {
			w.Header().Set("Retry-After", "60")
			apiError(w, 429, "busy")
			return
		}
		select {
		case a.slots <- struct{}{}:
			defer func() { <-a.slots }()
		default:
			apiError(w, 503, "busy")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		ctx = WithUserID(ctx, id)
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (a *API) allow(id int64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	if len(a.clients) >= 10000 {
		for id, b := range a.clients {
			if !b.expires.After(now) {
				delete(a.clients, id)
			}
		}
	}
	b, ok := a.clients[id]
	if !ok && len(a.clients) >= 10000 {
		return false
	}
	if !b.expires.After(now) {
		b = bucket{expires: now.Add(time.Minute)}
	}
	b.count++
	a.clients[id] = b
	return b.count <= 240
}

type item struct {
	Position  int64  `json:"position"`
	Key       string `json:"key"`
	GroupRef  string `json:"group_ref,omitempty"`
	Name      string `json:"name"`
	MaskedID  string `json:"masked_id,omitempty"`
	Score     string `json:"score_units"`
	Avatar    string `json:"avatar_url,omitempty"`
	Anonymous bool   `json:"anonymous,omitempty"`
}
type response struct {
	Month      string               `json:"month_start"`
	MonthName  string               `json:"month_name"`
	Ends       time.Time            `json:"month_ends_at"`
	ServerTime time.Time            `json:"server_time"`
	Timezone   string               `json:"timezone"`
	System     groups.RankingSystem `json:"system"`
	Items      []item               `json:"items"`
	Group      *item                `json:"group,omitempty"`
	Next       string               `json:"next_cursor,omitempty"`
}

func (a *API) makeItem(row ranking.GlobalRow, kind string, req ranking.GlobalRequest) (item, error) {
	if row.Anonymous {
		name := "Anônimo"
		if kind == "group" {
			name = "Grupo anônimo"
		}
		v := item{
			Position:  row.Position,
			Key:       a.References.key(kind, row.ID),
			Name:      name,
			Score:     strconv.FormatInt(int64(row.Score), 10),
			Anonymous: true,
		}
		if kind == "group" {
			ref := reference{Kind: "detail", ID: row.ID, System: string(req.System), Month: ranking.MonthDateString(req.Month), Expires: a.now().Add(time.Hour).Unix()}
			var err error
			v.GroupRef, err = a.References.seal(ref)
			if err != nil {
				return item{}, err
			}
		}
		return v, nil
	}
	ref := reference{Kind: kind, ID: row.ID, System: string(req.System), Month: ranking.MonthDateString(req.Month), Expires: a.now().Add(time.Hour).Unix()}
	avatar, err := a.References.seal(ref)
	if err != nil {
		return item{}, err
	}
	v := item{Position: row.Position, Key: a.References.key(kind, row.ID), Name: row.Name, MaskedID: "ID " + ranking.MaskID(row.ID), Score: strconv.FormatInt(int64(row.Score), 10), Avatar: "/api/v1/media/" + avatar}
	if kind == "group" {
		ref.Kind = "detail"
		v.GroupRef, err = a.References.seal(ref)
	}
	return v, err
}
func (a *API) list(w http.ResponseWriter, r *http.Request, kind string) {
	q := r.URL.Query()
	system := q.Get("system")
	if system == "" {
		system = "updated"
	}
	req := ranking.GlobalRequest{Kind: kind, System: groups.RankingSystem(system), Month: a.Rankings.CurrentMonth(), Limit: 50}
	if q.Has("month") || q.Has("month_start") {
		apiError(w, 400, "current_month_only")
		return
	}
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			apiError(w, 400, "invalid_limit")
			return
		}
		req.Limit = n
	}
	month := ranking.MonthDateString(req.Month)
	if kind == "detail" {
		ref, err := a.References.open(r.PathValue("ref"), a.now())
		if err != nil || ref.Kind != "detail" || ref.System != system {
			apiError(w, 404, "not_found")
			return
		}
		if ref.Month != month {
			apiError(w, 409, "ranking_period_changed")
			return
		}
		req.GroupID = ref.ID
	}
	if raw := q.Get("cursor"); raw != "" {
		ref, err := a.References.open(raw, a.now())
		if err != nil || ref.Kind != "cursor/"+kind || ref.System != system || ref.ID != req.GroupID || ref.After == nil {
			apiError(w, 400, "invalid_cursor")
			return
		}
		if ref.Month != month {
			apiError(w, 409, "ranking_period_changed")
			return
		}
		req.After = ref.After
	}
	page, err := a.Rankings.List(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ranking.ErrInvalid):
			apiError(w, 400, "invalid_request")
		case errors.Is(err, ranking.ErrPeriodChanged):
			apiError(w, 409, "ranking_period_changed")
		case errors.Is(err, ranking.ErrNotFound):
			apiError(w, 404, "not_found")
		default:
			apiError(w, 503, "unavailable")
		}
		return
	}
	out := response{Month: month, MonthName: ranking.MonthName(page.Month), Ends: page.Month.AddDate(0, 1, 0), ServerTime: a.now(), Timezone: "America/Sao_Paulo", System: req.System, Items: []item{}}
	rowKind := "user"
	if kind == "groups" {
		rowKind = "group"
	}
	for _, row := range page.Rows {
		v, err := a.makeItem(row, rowKind, req)
		if err != nil {
			apiError(w, 500, "unavailable")
			return
		}
		out.Items = append(out.Items, v)
	}
	if page.Group != nil && kind == "detail" {
		v, err := a.makeItem(*page.Group, "group", req)
		if err != nil {
			apiError(w, 500, "unavailable")
			return
		}
		out.Group = &v
	}
	if page.More && len(page.Rows) > 0 {
		key := page.Rows[len(page.Rows)-1].Key()
		out.Next, err = a.References.seal(reference{Kind: "cursor/" + kind, ID: req.GroupID, System: system, Month: month, Expires: a.now().Add(5 * time.Minute).Unix(), After: &key})
		if err != nil {
			apiError(w, 500, "unavailable")
			return
		}
	}
	writeJSON(w, 200, out)
}
func (a *API) avatar(w http.ResponseWriter, r *http.Request) {
	ref, err := a.References.open(r.PathValue("ref"), a.now())
	if err != nil || (ref.Kind != "group" && ref.Kind != "user") {
		apiError(w, 404, "not_found")
		return
	}
	if a.Privacy != nil {
		if anon, err := a.Privacy.IsEntityAnonymous(r.Context(), ref.Kind, ref.ID); err == nil && anon {
			w.WriteHeader(204)
			return
		}
	}
	if a.Media == nil {
		w.WriteHeader(204)
		return
	}
	data, mime, pending := a.Media.Get(ref.Kind, ref.ID)
	if pending {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(202)
		return
	}
	if len(data) == 0 {
		w.WriteHeader(204)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.Write(data)
}

func (a *API) getMyPrivacy(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok || a.UserPrivacy == nil {
		apiError(w, 401, "unauthorized")
		return
	}
	anon, err := a.UserPrivacy.GetUserRankingPrivacy(r.Context(), userID)
	if err != nil {
		apiError(w, 500, "unavailable")
		return
	}
	writeJSON(w, 200, map[string]bool{"anonymous": anon})
}

func (a *API) putMyPrivacy(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok || a.UserPrivacy == nil {
		apiError(w, 401, "unauthorized")
		return
	}
	var req struct {
		Anonymous bool `json:"anonymous"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, 400, "invalid_request")
		return
	}
	if err := a.UserPrivacy.SetUserRankingPrivacy(r.Context(), userID, req.Anonymous); err != nil {
		apiError(w, 500, "unavailable")
		return
	}
	writeJSON(w, 200, map[string]bool{"anonymous": req.Anonymous})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

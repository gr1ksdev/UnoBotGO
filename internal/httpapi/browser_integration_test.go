//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/storage/postgres"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/malbs/UnoGoBot/web"
)

// Opt-in browser test: real HTTP/WebSocket/engine/transactions, isolated database
// schema, two test identities signed with a fictional bot token. No live Telegram.
func TestTwoBrowserWebAppGame(t *testing.T) {
	if os.Getenv("UNO_BROWSER_E2E") != "1" {
		t.Skip("set UNO_BROWSER_E2E=1 and TEST_DATABASE_URL for real browser integration")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Minute)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "webapp_e2e_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	store, err := postgres.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	svc := game.NewDeterministicService(11)
	refs, _ := NewReferences(make([]byte, 32))
	var notifications atomic.Int64
	committed := make(chan ranking.Result, 1)
	finalizer := &game.Finalizer{Service: svc, Repository: store, Notify: func(_ context.Context, result ranking.Result) { notifications.Add(1); committed <- result.Clone() }}
	go finalizer.Run(ctx)
	const token = "fictional-e2e-bot-token"
	a := &API{Games: svc, References: refs, Token: token, MaxAge: time.Hour, TurnTimeout: time.Minute,
		Lifecycle: ctx, Profiles: store, Privacy: store, UserPrivacy: store, Finalizer: finalizer,
		Rankings: &ranking.GlobalService{Repository: store}, BotUsername: func() string { return "TestOnlyBot" }}
	if _, err = store.GetOrCreateGroupConfig(ctx, -42); err != nil {
		t.Fatal(err)
	}
	if err = store.ObserveGroupUser(ctx, groups.KnownUser{ChatID: -42, UserID: 12345, DisplayName: "Conta A", LastSeenAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	a.RoomGroups = store
	a.VerifyRoomGroup = func(_ context.Context, chat, user int64) (string, error) {
		if chat != -42 || (user != 12345 && user != 23456) {
			return "", groups.ErrForbidden
		}
		return "Sala WebApp E2E", nil
	}
	a.ResolveRoomGroup = func(_ context.Context, username string, user int64) (int64, string, error) {
		if username != "@e2e_group" || user != 12345 {
			return 0, "", groups.ErrForbidden
		}
		return -42, "Sala WebApp E2E", nil
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", a.Handler())
	mux.Handle("/", Static(web.Files()))
	server := httptest.NewServer(Security(mux))
	defer server.Close()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	outputDir := os.Getenv("UNO_E2E_OUTPUT")
	if outputDir == "" {
		outputDir = ".reports/partida-pixi/integration"
	}
	auth1 := signed(token, time.Now(), map[string]string{"user": `{"id":12345,"first_name":"Conta A"}`})
	auth2 := signed(token, time.Now(), map[string]string{"user": `{"id":23456,"first_name":"Conta B"}`})
	cmd := exec.CommandContext(ctx, "node", "web/scripts/game-e2e.mjs")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "UNO_E2E_BACKEND="+server.URL, "UNO_E2E_AUTH_A="+auth1, "UNO_E2E_AUTH_B="+auth2)
	output, err := cmd.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatal(err)
	}
	browserReport, err := os.ReadFile(filepath.Join(root, outputDir, "browser.json"))
	if err != nil {
		t.Fatal(err)
	}
	var browserResult struct {
		GameID string `json:"game_id"`
	}
	if err = json.Unmarshal(browserReport, &browserResult); err != nil || browserResult.GameID == "" {
		t.Fatal(err)
	}
	id := uno.GameID(browserResult.GameID)
	result, err := store.ReadResult(ctx, string(id), 12345)
	if err != nil || len(result) != 1 || result[0].Status != "scored" {
		t.Fatal("unconfirmed result", err, result)
	}
	before, err := store.ReadProfile(ctx, 12345, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Reprepare the exact engine result and retry the canonical transaction.
	original := <-committed
	for range 2 {
		c, e := finalizer.Commit(ctx, original)
		if e != nil || !c.AlreadyPersisted {
			t.Fatal("duplicate finalization", e, c)
		}
	}
	after, e := store.ReadProfile(ctx, 12345, nil)
	if e != nil || after.Stats[0] != before.Stats[0] || len(after.History) != len(before.History) {
		t.Fatal("retry duplicated profile", e, after)
	}
	var count int
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM "+schema+".completed_games").Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicated result", count, err)
	}
	if len(before.History) != 1 || before.Stats[0].Games != 1 || notifications.Load() != 1 {
		t.Fatal("duplicated history/notification", before, notifications.Load())
	}
	page, err := store.ReadGlobalRanking(ctx, ranking.GlobalRequest{Kind: "players", System: groups.Updated, Month: ranking.MonthStart(time.Now()), Limit: 10})
	if err != nil || len(page.Rows) != 2 {
		t.Fatal("ranking not updated", err, page)
	}
	expected := map[int64]ranking.Units{}
	for _, player := range original.Players {
		expected[player.UserID] = player.Score
	}
	for _, row := range page.Rows {
		if row.Score != expected[row.ID] {
			t.Fatal("ranking differs from canonical score", row)
		}
	}
	groupPage, e := store.ReadGlobalRanking(ctx, ranking.GlobalRequest{Kind: "groups", System: groups.Updated, Month: ranking.MonthStart(time.Now()), Limit: 10})
	if e != nil || len(groupPage.Rows) != 1 || groupPage.Rows[0].Score != expected[12345]+expected[23456] {
		t.Fatal("group score", e, groupPage)
	}
	second, e := store.ReadProfile(ctx, 23456, nil)
	if e != nil || len(second.History) != 1 || second.Stats[0].Games != 1 {
		t.Fatal("second account history", e, second)
	}
	report, _ := json.MarshalIndent(map[string]any{"completed_games": count, "notifications": notifications.Load(), "history_entries": len(before.History), "ranking_players": len(page.Rows), "score_committed": true, "duplicate_finalization_retries": 2, "ranking_groups": len(groupPage.Rows)}, "", "  ")
	if err = os.WriteFile(filepath.Join(root, outputDir, "backend.json"), report, 0644); err != nil {
		t.Fatal(err)
	}
}

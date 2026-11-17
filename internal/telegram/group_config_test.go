package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/mymmrac/telego"
)

type configRepo struct {
	config groups.Config
	err    error
	calls  int
}

func (r *configRepo) GetOrCreateGroupConfig(context.Context, int64) (groups.Config, error) {
	r.calls++
	return r.config, r.err
}
func (r *configRepo) SetCommandLanguage(context.Context, int64, groups.CommandLanguage) (groups.Config, error) {
	panic("unexpected SetCommandLanguage call")
}
func (r *configRepo) ReadCommandLanguage(context.Context, int64) (groups.CommandLanguage, error) {
	return groups.CommandsPT, nil
}
func (r *configRepo) SetRankingSystem(context.Context, int64, groups.RankingSystem) (groups.Config, error) {
	panic("unexpected SetRankingSystem call")
}
func (r *configRepo) SetInstalledBy(context.Context, int64, int64) (groups.Config, error) {
	panic("unexpected SetInstalledBy call")
}
func (r *configRepo) ObserveGroupTitle(context.Context, int64, string) error {
	return nil
}

// The house mode is the only mode: neither the group default nor a command
// argument may change it.
func TestNovoAlwaysUsesHouseRules(t *testing.T) {
	for _, tt := range []struct {
		cmd string
	}{
		{"/novo"},
		{"/novo classico"},
		{"/novo caseiro"},
	} {
		t.Run(tt.cmd, func(t *testing.T) {
			svc, _ := game.NewService()
			h := NewCommandHandler(newMockBotAPI(), svc, NewRenderer(nil), nil, "unobot", nil)
			r := &configRepo{config: groups.Defaults(42)}
			r.config.RankingSystem = groups.Updated
			h.groupConfigs = r
			h.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: 42, Type: "group"}, From: &telego.User{ID: 1}, Text: strings.Replace(tt.cmd, "/novo", "/novo@unobot", 1)})
			summary, err := svc.FindChatGame(t.Context(), 42)
			if err != nil {
				t.Fatal(err)
			}
			v, err := svc.PublicView(t.Context(), summary.GameID)
			if err != nil {
				t.Fatal(err)
			}
			if !v.Rules.AllowSwapHands || v.GroupConfig.RankingSystem != groups.Updated {
				t.Fatalf("wrong rules: %+v", v)
			}
			if r.calls != 1 {
				t.Fatal("unexpected DB reads")
			}
			// A later gameplay command must work even if persistence is unavailable.
			r.err = errors.New("offline")
			h.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: 42, Type: "group"}, From: &telego.User{ID: 1}, Text: "/entrar@unobot"})
			v, _ = svc.PublicView(t.Context(), summary.GameID)
			if len(v.Players) != 1 || r.calls != 1 {
				t.Fatal("join depended on DB")
			}
		})
	}
}

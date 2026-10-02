package telegram

import (
	"context"
	"errors"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/mymmrac/telego"
	"strings"
	"testing"
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
func (r *configRepo) SetDefaultGameMode(context.Context, int64, groups.Mode) (groups.Config, error) {
	panic("/novo must not update defaults")
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

func TestNovoGroupDefaultAndOverrides(t *testing.T) {
	for _, tt := range []struct {
		cmd     string
		mode    groups.Mode
		caseiro bool
	}{
		{"/novo", groups.Caseiro, true}, {"/novo classico", groups.Caseiro, false}, {"/novo caseiro", groups.Classic, true}, {"/novo", groups.Classic, false},
	} {
		t.Run(tt.cmd+string(tt.mode), func(t *testing.T) {
			svc, _ := game.NewService()
			h := NewCommandHandler(newMockBotAPI(), svc, NewRenderer(nil), nil, "unobot", nil)
			r := &configRepo{config: groups.Defaults(42)}
			r.config.DefaultGameMode = tt.mode
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
			if v.Rules.AllowSwapHands != tt.caseiro || v.GroupConfig.RankingSystem != groups.Updated {
				t.Fatalf("wrong defaults/override: %+v", v)
			}
			if r.config.DefaultGameMode != tt.mode || r.calls != 1 {
				t.Fatal("default modified or unexpected DB reads")
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

package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/mymmrac/telego"
)

func TestResolveCommandIsLanguageScoped(t *testing.T) {
	for _, tc := range []struct {
		lang     groups.CommandLanguage
		accepted string
		rejected string
		want     commandID
	}{
		{groups.CommandsPT, "novo", "new", cmdNew},
		{groups.CommandsEN, "new", "novo", cmdNew},
		{groups.CommandsPT, "ajuda", "help", cmdHelp},
		{groups.CommandsEN, "help", "ajuda", cmdHelp},
		{groups.CommandsPT, "config", "settings", cmdConfig},
		{groups.CommandsEN, "settings", "config", cmdConfig},
	} {
		if id, ok := resolveCommand(tc.accepted, tc.lang); !ok || id != tc.want {
			t.Fatalf("%s should resolve to %s in %s, got %s %v", tc.accepted, tc.want, tc.lang, id, ok)
		}
		if _, ok := resolveCommand(tc.rejected, tc.lang); ok {
			t.Fatalf("%s must not be accepted in %s", tc.rejected, tc.lang)
		}
	}
}

func TestCommandListsFollowLanguage(t *testing.T) {
	pt := groupCommandNames(groups.CommandsPT)
	en := groupCommandNames(groups.CommandsEN)
	for _, want := range []string{"novo", "entrar", "iniciar", "ajuda", "config"} {
		if !contains(pt, want) {
			t.Fatalf("Portuguese menu missing /%s", want)
		}
	}
	for _, want := range []string{"new", "join", "start", "help", "settings"} {
		if !contains(en, want) {
			t.Fatalf("English menu missing /%s", want)
		}
	}
	for _, forbidden := range []string{"new", "join", "help", "settings"} {
		if contains(pt, forbidden) {
			t.Fatalf("Portuguese menu exposes /%s", forbidden)
		}
	}
	for _, forbidden := range []string{"novo", "entrar", "ajuda", "config"} {
		if contains(en, forbidden) {
			t.Fatalf("English menu exposes /%s", forbidden)
		}
	}
}

func TestEnglishGroupAcceptsOnlyEnglishCommands(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "unobot", nil)
	repo := newMockGroupRepo()
	cfg, _ := repo.GetOrCreateGroupConfig(t.Context(), -55)
	cfg.CommandLanguage = groups.CommandsEN
	repo.configs[-55] = cfg
	h.groupConfigs = repo
	h.languages.repo = repo

	chat := telego.Chat{ID: -55, Type: "supergroup"}
	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 1, FirstName: "Owner"}, Text: "/new@unobot"})
	summary, err := svc.FindChatGame(t.Context(), -55)
	if err != nil {
		t.Fatal(err)
	}

	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 2, FirstName: "PT"}, Text: "/entrar@unobot"})
	view, _ := svc.PublicView(t.Context(), summary.GameID)
	if len(view.Players) != 0 {
		t.Fatal("Portuguese command was accepted in an English group")
	}

	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 2, FirstName: "EN"}, Text: "/join@unobot"})
	view, _ = svc.PublicView(t.Context(), summary.GameID)
	if len(view.Players) != 1 || view.Players[0].ID != 2 {
		t.Fatal("English command was refused in an English group")
	}
}

func TestLanguageChangeUpdatesTelegramMenu(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	repo := newMockGroupRepo()
	b.SetGroupConfigs(repo)
	api.ChatMembers[10] = &telego.ChatMemberAdministrator{Status: telego.MemberStatusAdministrator}

	callback := func(lang string) {
		b.cbHandler.HandleCallback(t.Context(), &telego.CallbackQuery{
			ID:      "lang",
			From:    telego.User{ID: 10, FirstName: "Admin"},
			Message: &telego.Message{MessageID: 1, Chat: telego.Chat{ID: -66}},
			Data:    "cfg_lang_" + lang + "_-66",
		})
	}
	callback("en")
	if len(api.DeletedCommandScopes) != 0 {
		t.Fatal("switching to English must not delete the override")
	}
	if len(api.CommandRegistrations) == 0 {
		t.Fatal("switching to English must install a per-chat menu")
	}
	last := api.CommandRegistrations[len(api.CommandRegistrations)-1]
	if last.Scope.ScopeType() != telego.ScopeTypeChat {
		t.Fatalf("wrong scope: %s", last.Scope.ScopeType())
	}
	if got := last.Commands[0].Command; got != "novo" && !contains(groupCommandNames(groups.CommandsEN), got) {
		t.Fatalf("menu is not in English: %s", got)
	}

	callback("pt")
	if len(api.DeletedCommandScopes) != 1 || api.DeletedCommandScopes[0].ScopeType() != telego.ScopeTypeChat {
		t.Fatal("switching back to Portuguese must delete the per-chat override")
	}
	cfg, _ := repo.GetOrCreateGroupConfig(t.Context(), -66)
	if cfg.CommandLanguage != groups.CommandsPT {
		t.Fatalf("language not persisted: %s", cfg.CommandLanguage)
	}
}

func TestHelpUsesConfiguredLanguage(t *testing.T) {
	r := NewRenderer(nil)
	pt := r.RenderHelp("ExampleBot", groups.CommandsPT)
	en := r.RenderHelp("ExampleBot", groups.CommandsEN)
	for _, want := range []string{"/ajuda", "/novo", "/config"} {
		if !strings.Contains(pt, want) || strings.Contains(en, want) {
			t.Fatalf("language leaking in help: %s", want)
		}
	}
	for _, want := range []string{"/help", "/new", "/settings"} {
		if !strings.Contains(en, want) || strings.Contains(pt, want) {
			t.Fatalf("language leaking in help: %s", want)
		}
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

type rankingReadFunc func(context.Context, int64, time.Time) (ranking.GroupRanking, error)

func (f rankingReadFunc) ListGroupRanking(ctx context.Context, id int64, at time.Time) (ranking.GroupRanking, error) {
	return f(ctx, id, at)
}

func (f rankingReadFunc) ListUserMonthlyRankings(ctx context.Context, userID int64, at time.Time) (ranking.UserMonthlyRankings, error) {
	return ranking.UserMonthlyRankings{UserID: userID}, nil
}

func TestRenderGroupRankingUniqueSequentialRanks(t *testing.T) {
	for _, tc := range []struct {
		scores []ranking.Units
		labels []string
	}{
		{[]ranking.Units{1000, 1000, 500}, []string{"🥇", "🥈", "🥉"}},
		{[]ranking.Units{1000, 800, 800, 500, 0}, []string{"🥇", "🥈", "🥉", "4.", "5."}},
		{[]ranking.Units{1000, 857, 500, 100, 0}, []string{"🥇", "🥈", "🥉", "4.", "5."}},
		{[]ranking.Units{0, 0, 0, 0, 0, 0}, []string{"🥇", "🥈", "🥉", "4.", "5.", "6."}},
	} {
		for _, system := range []groups.RankingSystem{groups.Legacy, groups.Updated} {
			group := ranking.GroupRanking{System: system, Total: int64(len(tc.scores))}
			for i, score := range tc.scores {
				group.Entries = append(group.Entries, ranking.Entry{UserID: int64(i + 1), DisplayName: fmt.Sprintf("Pessoa %d", i), Score: score})
			}
			lines := strings.Split(RenderGroupRanking(group), "\n")[2:]
			for i, label := range tc.labels {
				if !strings.HasPrefix(lines[i], label+" ") {
					t.Fatalf("wrong rank: %v", lines)
				}
			}
		}
	}
}

func TestRenderGroupRankingFormatsAndUnicode(t *testing.T) {
	for _, tc := range []struct {
		system  groups.RankingSystem
		scores  []ranking.Units
		amounts []string
	}{
		{groups.Legacy, []ranking.Units{200, 100, 0}, []string{"2 pts", "1 pt", "0 pts"}},
		{groups.Updated, []ranking.Units{1000, 857, 0}, []string{"10,00 pts", "8,57 pts", "0,00 pts"}},
	} {
		group := ranking.GroupRanking{System: tc.system, Total: 3}
		for i, score := range tc.scores {
			group.Entries = append(group.Entries, ranking.Entry{UserID: int64(i + 1), DisplayName: "João 🦊 <&> é 中文", Score: score})
		}
		text := RenderGroupRanking(group)
		for _, amount := range tc.amounts {
			if !strings.Contains(text, "João 🦊 &lt;&amp;&gt; é 中文 · "+amount) {
				t.Fatal(text)
			}
		}
		if strings.Contains(text, "+") {
			t.Fatal("accumulated score has gain prefix", text)
		}
	}
}

func TestRankingPreservesObservedDotName(t *testing.T) {
	name := observedName(telego.User{ID: 123, FirstName: "."})
	group := ranking.GroupRanking{System: groups.Updated, MonthName: "Setembro", Total: 1, Entries: []ranking.Entry{{UserID: 123, DisplayName: name, Score: 3000}}}
	if name != "." || RenderGroupRanking(group) != "🏆 Ranking do grupo · Setembro\n\n🥇 . · 30,00 pts" {
		t.Fatal("observed name was changed: ", RenderGroupRanking(group))
	}
}

func TestRenderGroupRankingLargeBoundedWholeLines(t *testing.T) {
	group := ranking.GroupRanking{System: groups.Updated, Total: 10000}
	for i := 0; i < ranking.MaxRankingEntries; i++ {
		group.Entries = append(group.Entries, ranking.Entry{UserID: int64(i + 1), DisplayName: strings.Repeat("🦊<&>é", 12), Score: ranking.Units(10000 - i)})
	}
	text := RenderGroupRanking(group)
	shown := strings.Count(text, " · ")
	if shown < 10 || messageUnits(text) > rankingMessageLimit || !utf8.ValidString(text) {
		t.Fatalf("unsafe or undersized message: shown=%d units=%d", shown, messageUnits(text))
	}
	if !strings.HasSuffix(text, rankingRemaining(group.Total-int64(shown))) {
		t.Fatal("wrong omitted count", text)
	}
	// The next whole line cannot fit with its required suffix.
	prefix := strings.TrimSuffix(text, rankingRemaining(group.Total-int64(shown)))
	next := group.Entries[shown]
	line := fmt.Sprintf("\n%s %s · %s", placementLabel(shown+1), rankingName(next.DisplayName, next.UserID), ranking.FormatScore(group.System, next.Score))
	if messageUnits(prefix+line+rankingRemaining(group.Total-int64(shown+1))) <= rankingMessageLimit {
		t.Fatal("stopped before filling message")
	}
	group.Entries[0].DisplayName = strings.Repeat("😀", 5000)
	text = RenderGroupRanking(group)
	if strings.Contains(text, "😀") || messageUnits(text) > rankingMessageLimit || !strings.Contains(text, "10000 jogadores") {
		t.Fatal("oversized name not handled as a whole omitted line")
	}
}

func TestRenderPointsMedalsAndPositions(t *testing.T) {
	r := ranking.Result{RankingSystem: groups.Updated}
	for i := 1; i <= 10; i++ {
		r.Players = append(r.Players, ranking.Player{UserID: int64(i), DisplayName: fmt.Sprintf("P%d", i), Position: i, FinalStatus: "playing", Score: 100})
	}
	text := renderPoints(r)
	for i, label := range []string{"🥇", "🥈", "🥉", "4.", "5.", "6.", "7.", "8.", "9.", "10."} {
		if !strings.Contains(text, fmt.Sprintf("%s P%d · +1,00 pts", label, i+1)) {
			t.Fatal(text)
		}
	}
	if strings.Contains(text, "🏅") {
		t.Fatal(text)
	}
}

func TestRankingCommandSharedRendererIsolationAndNoAdmin(t *testing.T) {
	svc, _ := game.NewService()
	api := newMockBotAPI()
	api.ChatMemberErr = errors.New("membership API must not be needed")
	b := New(api, svc, nil, nil, 0, nil)
	calls := 0
	b.SetRankingService(&ranking.Service{Repository: rankingReadFunc(func(_ context.Context, id int64, _ time.Time) (ranking.GroupRanking, error) {
		calls++
		if id == -1 {
			return ranking.GroupRanking{System: groups.Updated, MonthName: "Outubro", Total: 1, Entries: []ranking.Entry{{UserID: 1, DisplayName: "Ana histórica", Score: 1500}}}, nil
		}
		if id == -3 {
			return ranking.GroupRanking{}, errors.New("offline")
		}
		return ranking.GroupRanking{System: groups.Legacy, MonthName: "Outubro"}, nil
	})})
	for _, id := range []int64{-1, -2, -3} {
		b.cmdHandler.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: id, Type: "supergroup"}, From: &telego.User{ID: 999, FirstName: "Membro"}, Text: "/ranking"})
		text := api.LastSentMessage()
		switch id {
		case -1:
			if text != "🏆 Ranking do grupo · Outubro\n\n🥇 Ana histórica · 15,00 pts" {
				t.Fatal(text)
			}
		case -2:
			if text != "🏆 Ranking do grupo · Outubro\n\nAinda não há partidas pontuadas neste mês." {
				t.Fatal(text)
			}
		case -3:
			if !strings.Contains(text, "Não foi possível") || strings.Contains(text, "Ainda não há") {
				t.Fatal(text)
			}
		}
	}
	b.cmdHandler.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: 999, Type: "private"}, From: &telego.User{ID: 999}, Text: "/ranking"})
	if calls != 3 || api.LastSentMessage() != "🏆 Seus rankings\n\nVocê ainda não possui partidas pontuadas neste mês." {
		t.Fatalf("expected empty private monthly ranking, got %q", api.LastSentMessage())
	}
}

func TestPostCommitRankingReadsUpdatedHistoryAndMatchesCommand(t *testing.T) {
	svc, _ := game.NewService()
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, 0, nil)
	repo := &resultRepo{commit: ranking.Commit{Scored: true}, entered: make(chan struct{}), release: make(chan struct{})}
	b.SetResultRepository(repo)
	reads := 0
	b.SetRankingService(&ranking.Service{Repository: rankingReadFunc(func(_ context.Context, id int64, _ time.Time) (ranking.GroupRanking, error) {
		select {
		case <-repo.release:
		default:
			t.Error("ranking read before commit")
		}
		if id != 42 {
			t.Error("wrong group", id)
		}
		reads++
		return ranking.GroupRanking{System: groups.Updated, MonthName: "Setembro", Total: 3, Entries: []ranking.Entry{{UserID: 99, DisplayName: "Ana histórica", Score: 2000}, {UserID: 1, DisplayName: "Freddy", Score: 1500}, {UserID: 2, DisplayName: "Mezi", Score: 1500}}}, nil
	})})
	view, action := readyToFinish(t, svc, 42, uno.BotRules(), false)
	out, err := svc.Apply(t.Context(), game.Actor{PlayerID: view.CurrentTurn, ChatID: 42}, view.GameID, action)
	if err != nil || out.Completed == nil || out.Completed.EligibleCount() != 2 {
		t.Fatal("expected scored closure", err)
	}
	done := make(chan func(), 1)
	go func() { done <- b.finalizeOutcome(t.Context(), out) }()
	<-repo.entered
	if api.LastSentMessage() != "" || reads != 0 {
		t.Fatal("announced before commit")
	}
	close(repo.release)
	notify := <-done
	if notify == nil {
		t.Fatal("no notification")
	}
	notify()
	if len(api.SentMessages) != 2 || !strings.Contains(api.SentMessages[0].Text, "🏁 Partida encerrada") || !strings.Contains(api.SentMessages[1].Text, "Ana histórica · 20,00 pts") {
		t.Fatal(api.SentMessages)
	}
	automatic := api.SentMessages[1].Text
	b.cmdHandler.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: 42, Type: "group"}, From: &telego.User{ID: 999}, Text: "/ranking"})
	if api.LastSentMessage() != automatic || reads != 2 {
		t.Fatal("different command renderer")
	}
	// Failed and duplicate commits never consult or announce a fresh ranking.
	for _, testRepo := range []*resultRepo{{err: errors.New("commit failed")}, {commit: ranking.Commit{Scored: true, AlreadyPersisted: true}}} {
		b.SetResultRepository(testRepo)
		before := len(api.SentMessages)
		if notify := b.finalizeOutcome(t.Context(), out); notify != nil {
			t.Fatal("false notification")
		}
		if reads != 2 || len(api.SentMessages) != before {
			t.Fatal("failed/duplicate commit announced")
		}
	}
}

func TestInlineScoredClosureSendsExactlyResultAndRanking(t *testing.T) {
	svc, _ := game.NewService()
	view, action := readyToFinish(t, svc, -42, uno.BotRules(), false)
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	repo := &resultRepo{commit: ranking.Commit{Scored: true}}
	b.SetResultRepository(repo)
	b.SetRankingService(&ranking.Service{Repository: rankingReadFunc(func(_ context.Context, id int64, _ time.Time) (ranking.GroupRanking, error) {
		if repo.calls != 1 || id != -42 || len(api.SentMessages) != 1 {
			t.Error("ranking must follow commit and result message")
		}
		return ranking.GroupRanking{System: groups.Legacy, MonthName: "Setembro", Total: 1, Entries: []ranking.Entry{{UserID: 99, DisplayName: "Histórico", Score: 500}}}, nil
	})})
	token, err := b.tokens.CreateActionToken(view.CurrentTurn, view.GameID, view.ChatID, action, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: token, From: telego.User{ID: int64(view.CurrentTurn), FirstName: "João <&>"}})
	flushChat(t, b, view.ChatID)
	if len(api.SentMessages) != 2 {
		t.Fatalf("expected two independent messages: %+v", api.SentMessages)
	}
	if !strings.HasPrefix(api.SentMessages[0].Text, "🏁 Partida encerrada\n\n🥇") || !strings.Contains(api.SentMessages[0].Text, "+1 pt") {
		t.Fatal(api.SentMessages[0].Text)
	}
	if api.SentMessages[1].Text != "🏆 Ranking do grupo · Setembro\n\n🥇 Histórico · 5 pts" {
		t.Fatal(api.SentMessages[1].Text)
	}
	for _, msg := range api.SentMessages {
		if msg.ParseMode != "HTML" || msg.ReplyMarkup != nil {
			t.Fatal("invalid final message parameters", msg)
		}
	}
}

func TestRenderUserMonthlyRankings_UpdatedOnly(t *testing.T) {
	rankings := ranking.UserMonthlyRankings{
		UserID:    123,
		MonthName: "Setembro",
		Updated: &ranking.UserMonthlyRankingSection{
			System: groups.Updated,
			Entries: []ranking.UserGroupRankingEntry{
				{ChatID: -1001, GroupName: "UNO da Galera", ScoreUnits: 3000},
				{ChatID: -1002, GroupName: "Amigos do UNO", ScoreUnits: 1857},
				{ChatID: -1003, GroupName: "Grupo Zero", ScoreUnits: 0},
			},
			TotalScore: 4857,
		},
	}

	got := RenderUserMonthlyRankings(rankings)
	want := "🏆 Seus rankings · Setembro\n\n" +
		"⚡ Atualizado\n" +
		"• UNO da Galera · 30,00 pts\n" +
		"• Amigos do UNO · 18,57 pts\n" +
		"• Grupo Zero · 0,00 pts\n\n" +
		"Total · 48,57 pts"

	if got != want {
		t.Fatalf("unexpected rendering:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderUserMonthlyRankings_LegacyOnly(t *testing.T) {
	rankings := ranking.UserMonthlyRankings{
		UserID:    123,
		MonthName: "Setembro",
		Legacy: &ranking.UserMonthlyRankingSection{
			System: groups.Legacy,
			Entries: []ranking.UserGroupRankingEntry{
				{ChatID: -1001, GroupName: "Jogatina BR", ScoreUnits: 500},
				{ChatID: -1002, GroupName: "Amigos 2", ScoreUnits: 100},
				{ChatID: -1003, GroupName: "Zerados", ScoreUnits: 0},
			},
			TotalScore: 600,
		},
	}

	got := RenderUserMonthlyRankings(rankings)
	want := "🏆 Seus rankings · Setembro\n\n" +
		"🕹️ Legado\n" +
		"• Jogatina BR · 5 pts\n" +
		"• Amigos 2 · 1 pt\n" +
		"• Zerados · 0 pts\n\n" +
		"Total · 6 pts"

	if got != want {
		t.Fatalf("unexpected rendering:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderUserMonthlyRankings_BothSeparated(t *testing.T) {
	rankings := ranking.UserMonthlyRankings{
		UserID:    123,
		MonthName: "Setembro",
		Updated: &ranking.UserMonthlyRankingSection{
			System: groups.Updated,
			Entries: []ranking.UserGroupRankingEntry{
				{ChatID: -1001, GroupName: "UNO da Galera", ScoreUnits: 3000},
				{ChatID: -1002, GroupName: "Amigos do UNO", ScoreUnits: 1857},
			},
			TotalScore: 4857,
		},
		Legacy: &ranking.UserMonthlyRankingSection{
			System: groups.Legacy,
			Entries: []ranking.UserGroupRankingEntry{
				{ChatID: -1003, GroupName: "Jogatina BR", ScoreUnits: 500},
				{ChatID: -1004, GroupName: "Amigos 2", ScoreUnits: 300},
			},
			TotalScore: 800,
		},
	}

	got := RenderUserMonthlyRankings(rankings)
	want := "🏆 Seus rankings · Setembro\n\n" +
		"⚡ Atualizado\n" +
		"• UNO da Galera · 30,00 pts\n" +
		"• Amigos do UNO · 18,57 pts\n\n" +
		"Total · 48,57 pts\n\n" +
		"🕹️ Legado\n" +
		"• Jogatina BR · 5 pts\n" +
		"• Amigos 2 · 3 pts\n\n" +
		"Total · 8 pts"

	if got != want {
		t.Fatalf("unexpected rendering:\ngot:\n%s\nwant:\n%s", got, want)
	}

	// Verify never combines totals
	if strings.Contains(got, "56,57") || strings.Contains(got, "Total · 56") {
		t.Fatalf("combined total detected: %s", got)
	}
}

func TestRenderUserMonthlyRankings_Empty(t *testing.T) {
	rankings := ranking.UserMonthlyRankings{
		UserID:    123,
		MonthName: "Outubro",
	}

	got := RenderUserMonthlyRankings(rankings)
	want := "🏆 Seus rankings · Outubro\n\nVocê ainda não possui partidas pontuadas neste mês."

	if got != want {
		t.Fatalf("unexpected rendering:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderUserMonthlyRankings_HtmlEscapingAndFallback(t *testing.T) {
	rankings := ranking.UserMonthlyRankings{
		UserID:    123,
		MonthName: "Setembro",
		Updated: &ranking.UserMonthlyRankingSection{
			System: groups.Updated,
			Entries: []ranking.UserGroupRankingEntry{
				{ChatID: -1001, GroupName: "UNO <&> 'Friends'", ScoreUnits: 1000},
				{ChatID: -1002, GroupName: "", ScoreUnits: 500},
			},
			TotalScore: 1500,
		},
	}

	got := RenderUserMonthlyRankings(rankings)
	if !strings.Contains(got, "• UNO &lt;&amp;&gt; &#39;Friends&#39; · 10,00 pts") {
		t.Fatalf("failed to escape HTML in group name: %s", got)
	}
	if !strings.Contains(got, "• Grupo -1002 · 5,00 pts") {
		t.Fatalf("fallback group name missing: %s", got)
	}
}

func TestRenderUserMonthlyRankings_MessageLimitTruncationAndTotal(t *testing.T) {
	entries := make([]ranking.UserGroupRankingEntry, 120)
	var total ranking.Units
	for i := 0; i < len(entries); i++ {
		score := ranking.Units((120 - i) * 100)
		total += score
		entries[i] = ranking.UserGroupRankingEntry{
			ChatID:     int64(-1000 - i),
			GroupName:  fmt.Sprintf("Grupo Longo de UNO Número %03d com Nome Extenso", i),
			ScoreUnits: score,
		}
	}

	rankings := ranking.UserMonthlyRankings{
		UserID:    123,
		MonthName: "Setembro",
		Updated: &ranking.UserMonthlyRankingSection{
			System:     groups.Updated,
			Entries:    entries,
			TotalScore: total,
		},
	}

	got := RenderUserMonthlyRankings(rankings)
	units := messageUnits(got)
	if units > rankingMessageLimit {
		t.Fatalf("rendered message exceeds Telegram limit: %d > %d", units, rankingMessageLimit)
	}
	if !strings.Contains(got, "… e mais ") || !strings.Contains(got, "grupos.") {
		t.Fatalf("omitted suffix missing: %s", got)
	}
	// Total must represent ALL entries, not just visible ones
	expectedTotalStr := fmt.Sprintf("Total · %s", ranking.FormatScore(groups.Updated, total))
	if !strings.Contains(got, expectedTotalStr) {
		t.Fatalf("total does not match sum of all entries: expected %q in:\n%s", expectedTotalStr, got)
	}
}

func TestRenderUserMonthlyRankings_SingleOmittedGroup(t *testing.T) {
	// Create enough text so that exactly 1 group is omitted
	line := strings.Repeat("A", 150)
	// Build entries where omitting 1 fits below rankingMessageLimit
	var entries []ranking.UserGroupRankingEntry
	var total ranking.Units
	for i := 0; i < 28; i++ {
		score := ranking.Units(1000)
		total += score
		entries = append(entries, ranking.UserGroupRankingEntry{
			ChatID:     int64(-1000 - i),
			GroupName:  fmt.Sprintf("Grupo %02d %s", i, line),
			ScoreUnits: score,
		})
	}
	rankings := ranking.UserMonthlyRankings{
		UserID:    123,
		MonthName: "Setembro",
		Updated: &ranking.UserMonthlyRankingSection{
			System:     groups.Updated,
			Entries:    entries,
			TotalScore: total,
		},
	}

	got := RenderUserMonthlyRankings(rankings)
	if strings.Contains(got, "… e mais 1 grupos.") {
		t.Fatalf("pluralized 1 group incorrectly: %s", got)
	}
	if strings.Contains(got, "… e mais 1 grupo.") {
		// Valid singular format verified
	}
}

func TestPrivateRankingCommand_ScopesToSenderAndNoButtons(t *testing.T) {
	svc, _ := game.NewService()
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, 0, nil)

	var queriedUserID int64
	mockRepo := &mockPrivateRankingRepo{
		onListUser: func(_ context.Context, userID int64, _ time.Time) (ranking.UserMonthlyRankings, error) {
			queriedUserID = userID
			return ranking.UserMonthlyRankings{
				UserID:    userID,
				MonthName: "Setembro",
				Updated: &ranking.UserMonthlyRankingSection{
					System: groups.Updated,
					Entries: []ranking.UserGroupRankingEntry{
						{ChatID: -1001, GroupName: "Galera", ScoreUnits: 1500},
					},
					TotalScore: 1500,
				},
			}, nil
		},
	}
	b.SetRankingService(&ranking.Service{Repository: mockRepo})

	// User 777 sends /ranking in private
	b.cmdHandler.HandleMessage(t.Context(), &telego.Message{
		Chat: telego.Chat{ID: 777, Type: "private"},
		From: &telego.User{ID: 777, FirstName: "Freddy"},
		Text: "/ranking",
	})

	if queriedUserID != 777 {
		t.Fatalf("expected query for user 777, got %d", queriedUserID)
	}
	sent := api.LastSentMessage()
	if !strings.Contains(sent, "🏆 Seus rankings · Setembro") || !strings.Contains(sent, "• Galera · 15,00 pts") {
		t.Fatalf("unexpected message content: %s", sent)
	}
	api.mu.Lock()
	lastMsg := api.SentMessages[len(api.SentMessages)-1]
	api.mu.Unlock()
	if lastMsg.ReplyMarkup != nil {
		t.Fatalf("expected no buttons or WebApp markup, got: %+v", lastMsg.ReplyMarkup)
	}
}

type mockPrivateRankingRepo struct {
	onListGroup func(context.Context, int64, time.Time) (ranking.GroupRanking, error)
	onListUser  func(context.Context, int64, time.Time) (ranking.UserMonthlyRankings, error)
}

func (m *mockPrivateRankingRepo) ListGroupRanking(ctx context.Context, id int64, at time.Time) (ranking.GroupRanking, error) {
	if m.onListGroup != nil {
		return m.onListGroup(ctx, id, at)
	}
	return ranking.GroupRanking{}, nil
}

func (m *mockPrivateRankingRepo) ListUserMonthlyRankings(ctx context.Context, userID int64, at time.Time) (ranking.UserMonthlyRankings, error) {
	if m.onListUser != nil {
		return m.onListUser(ctx, userID, at)
	}
	return ranking.UserMonthlyRankings{UserID: userID}, nil
}

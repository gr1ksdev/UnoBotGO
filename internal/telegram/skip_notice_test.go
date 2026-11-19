package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestAutoSkipPostsOnlyTheSkipNotice(t *testing.T) {
	svc, _ := game.NewService()
	view := createStartedGame(t, svc, -101, uno.BotRules())
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	b.SetTurnTimeout(time.Nanosecond)
	defer b.dispatcher.Stop(time.Second)

	candidate := svc.ExpiredTurns(t.Context(), time.Nanosecond)[0]
	if !b.enqueueAutoSkip(candidate) {
		t.Fatal("timeout admission failed")
	}
	flushChat(t, b, view.ChatID)

	last := plainGameplay(api.LastSentMessage())
	t.Logf("skip notice:\n%s", last)
	for _, want := range []string{"foi pulado", "Tempo restante:", "Vez de:"} {
		if !strings.Contains(last, want) {
			t.Fatalf("skip notice missing %q: %s", want, last)
		}
	}
	if strings.Contains(last, "🃏 Topo:") || strings.Contains(last, "👥") {
		t.Fatalf("skip notice leaked the detailed state: %s", last)
	}
}

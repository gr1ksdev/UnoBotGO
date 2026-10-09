package telegram

import (
	"context"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/mymmrac/telego"
	"regexp"
	"strings"
	"time"
)

var roomUsername = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{3,31}$`)

type RoomDirectory struct {
	Bot   *telego.Bot
	BotID int64
}

func (d RoomDirectory) Verify(ctx context.Context, chat, user int64) (string, error) {
	if chat >= 0 || user <= 0 {
		return "", groups.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	info, err := d.Bot.GetChat(ctx, &telego.GetChatParams{ChatID: telego.ChatID{ID: chat}})
	if err != nil || (info.Type != "group" && info.Type != "supergroup") {
		return "", groups.ErrForbidden
	}
	for _, id := range []int64{user, d.BotID} {
		member, e := d.Bot.GetChatMember(ctx, &telego.GetChatMemberParams{ChatID: telego.ChatID{ID: chat}, UserID: id})
		if e != nil || !parseMembership(member).Member {
			return "", groups.ErrForbidden
		}
	}
	return info.Title, nil
}
func (d RoomDirectory) Resolve(ctx context.Context, username string, user int64) (int64, string, error) {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if !roomUsername.MatchString(username) {
		return 0, "", groups.ErrForbidden
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	info, err := d.Bot.GetChat(ctx, &telego.GetChatParams{ChatID: telego.ChatID{Username: "@" + username}})
	if err != nil {
		return 0, "", groups.ErrForbidden
	}
	title, err := d.Verify(ctx, info.ID, user)
	return info.ID, title, err
}

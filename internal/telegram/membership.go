package telegram

import (
	"context"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/mymmrac/telego"
)

// lookupMembershipAPI inspects a user's membership in a chat via the Telegram Bot API.
func lookupMembershipAPI(ctx context.Context, bot BotAPI, chatID, userID int64) (groups.Membership, error) {
	if chatID == 0 || userID <= 0 {
		return groups.Membership{}, groups.ErrInvalid
	}
	if bot == nil {
		return groups.Membership{}, groups.ErrForbidden
	}
	roleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	member, err := bot.GetChatMember(roleCtx, &telego.GetChatMemberParams{
		ChatID: telego.ChatID{ID: chatID},
		UserID: userID,
	})
	if err != nil {
		return groups.Membership{}, err
	}
	return parseMembership(member), nil
}

// parseMembership translates telego.ChatMember into a normalized groups.Membership.
func parseMembership(member telego.ChatMember) groups.Membership {
	if member == nil {
		return groups.Membership{}
	}
	switch member.MemberStatus() {
	case telego.MemberStatusCreator, telego.MemberStatusAdministrator:
		return groups.Membership{Admin: true, Member: true}
	case telego.MemberStatusMember:
		return groups.Membership{Admin: false, Member: true}
	case telego.MemberStatusRestricted:
		if r, ok := member.(*telego.ChatMemberRestricted); ok {
			return groups.Membership{Admin: false, Member: r.IsMember}
		}
		return groups.Membership{Admin: false, Member: true}
	case telego.MemberStatusLeft, telego.MemberStatusBanned:
		return groups.Membership{Admin: false, Member: false}
	default:
		return groups.Membership{}
	}
}

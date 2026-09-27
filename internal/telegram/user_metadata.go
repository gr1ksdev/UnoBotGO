package telegram

import (
	"github.com/mymmrac/telego"
	"strings"
)

func observedName(user telego.User) string {
	return strings.TrimSpace(user.FirstName + " " + user.LastName)
}

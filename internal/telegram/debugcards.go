//go:build debugcards

package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func parseDebugCard(args []string) (uno.Color, uno.Rank, bool) {
	if len(args) == 1 {
		switch strings.ToLower(args[0]) {
		case "troca":
			return uno.NoColor, uno.SwapHands, true
		case "coringa":
			return uno.NoColor, uno.Wild, true
		case "+4":
			return uno.NoColor, uno.WildDrawFour, true
		}
	}
	if len(args) != 2 {
		return 0, 0, false
	}
	colors := map[string]uno.Color{"vermelho": uno.Red, "azul": uno.Blue, "verde": uno.Green, "amarelo": uno.Yellow}
	color, ok := colors[strings.ToLower(args[0])]
	if !ok {
		return 0, 0, false
	}
	value := strings.ToLower(args[1])
	if len(value) == 1 && value[0] >= '0' && value[0] <= '9' {
		return color, uno.Rank(value[0] - '0'), true
	}
	switch value {
	case "+2":
		return color, uno.DrawTwo, true
	case "pular":
		return color, uno.Skip, true
	case "inverter":
		return color, uno.Reverse, true
	}
	return 0, 0, false
}

func (h *CommandHandler) handleDebugCommand(ctx context.Context, msg *telego.Message, command string, fields []string) bool {
	if command != "dar" {
		return false
	}
	if uno.PlayerID(msg.From.ID) != game.DebugCardsUserID {
		return true
	}
	color, rank, ok := parseDebugCard(fields[1:])
	if !ok {
		h.reply(ctx, msg.Chat.ID, "Uso: /dar troca, /dar coringa, /dar +4 ou /dar &lt;vermelho|azul|verde|amarelo&gt; &lt;0..9|+2|pular|inverter&gt;. Responda à mensagem do destinatário; sem resposta, você recebe a carta.", nil)
		return true
	}
	target := msg.From
	if reply := msg.ReplyToMessage; reply != nil {
		if reply.From == nil || reply.From.IsBot || reply.SenderChat != nil || reply.Chat.ID != msg.Chat.ID {
			h.reply(ctx, msg.Chat.ID, "⚠️ Responda à mensagem de um jogador identificável deste grupo.", nil)
			return true
		}
		target = reply.From
	}
	summary, err := h.service.FindChatGame(ctx, game.ChatID(msg.Chat.ID))
	if err != nil {
		h.reply(ctx, msg.Chat.ID, "⚠️ Não há partida disponível neste grupo.", nil)
		return true
	}
	outcome, card, err := h.service.GiveCard(ctx, game.Actor{PlayerID: uno.PlayerID(msg.From.ID), ChatID: game.ChatID(msg.Chat.ID)}, summary.GameID, uno.PlayerID(target.ID), color, rank)
	if err != nil {
		text := "⚠️ Não foi possível entregar a carta."
		switch {
		case errors.Is(err, uno.ErrCardNotFound):
			text = "⚠️ Não há cópia disponível no monte ou descarte."
		case errors.Is(err, uno.ErrUnknownPlayer):
			text = "⚠️ O destinatário não é um participante ativo."
		case errors.Is(err, uno.ErrInvalidRules):
			text = "⚠️ Essa carta não está habilitada neste modo."
		case errors.Is(err, uno.ErrInvalidAction):
			text = "⚠️ Use durante um turno normal, sem penalidade, blefe ou escolha pendente."
		}
		h.reply(ctx, msg.Chat.ID, text, nil)
		return true
	}
	h.renderer.userCache.Put(uno.PlayerID(target.ID), target.FirstName, target.Username)
	h.reply(ctx, msg.Chat.ID, fmt.Sprintf("🃏 %s recebeu <b>%s</b>.", h.renderer.PlayerLink(uno.PlayerID(target.ID), outcome.View), CardRepr(card)), makeGameButtons(outcome.View))
	return true
}

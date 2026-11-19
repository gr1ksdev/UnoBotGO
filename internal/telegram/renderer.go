package telegram

import (
	"fmt"
	"html"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
)

type UserCache struct {
	mu    sync.RWMutex
	users map[uno.PlayerID]string
	order []uno.PlayerID
	limit int
}

func NewUserCache(limit int) *UserCache {
	if limit <= 0 {
		limit = 10000
	}
	return &UserCache{
		users: make(map[uno.PlayerID]string),
		order: make([]uno.PlayerID, 0),
		limit: limit,
	}
}

func (c *UserCache) Put(id uno.PlayerID, firstName, username string) {
	if id <= 0 {
		return
	}
	name := strings.TrimSpace(firstName)
	if name == "" {
		name = fmt.Sprintf("Jogador %d", id)
	}
	if username != "" {
		name = fmt.Sprintf("%s (@%s)", name, username)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.users[id]; !exists {
		if len(c.users) >= c.limit && len(c.order) > 0 {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.users, oldest)
		}
		c.order = append(c.order, id)
	}
	c.users[id] = name
}

func (c *UserCache) GetRawName(id uno.PlayerID) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if name, ok := c.users[id]; ok {
		return name
	}
	return fmt.Sprintf("Jogador %d", id)
}

// CardRepr returns a human-friendly string for a card, e.g. "❤️ 7" or "🌈+4 Coringa".
func CardRepr(c uno.Card) string {
	colorIcon := ColorIcon(c.Color)
	rankName := RankName(c.Rank)

	if c.Rank >= uno.Wild {
		return rankName
	}
	return fmt.Sprintf("%s %s", colorIcon, rankName)
}

func ColorIcon(color uno.Color) string {
	switch color {
	case uno.Red:
		return "❤️"
	case uno.Blue:
		return "💙"
	case uno.Green:
		return "💚"
	case uno.Yellow:
		return "💛"
	default:
		return "⬛️"
	}
}

func ColorNamePT(color uno.Color) string {
	switch color {
	case uno.Red:
		return "Vermelho"
	case uno.Blue:
		return "Azul"
	case uno.Green:
		return "Verde"
	case uno.Yellow:
		return "Amarelo"
	default:
		return "Nenhuma"
	}
}

func RankName(rank uno.Rank) string {
	switch rank {
	case uno.Zero:
		return "0"
	case uno.One:
		return "1"
	case uno.Two:
		return "2"
	case uno.Three:
		return "3"
	case uno.Four:
		return "4"
	case uno.Five:
		return "5"
	case uno.Six:
		return "6"
	case uno.Seven:
		return "7"
	case uno.Eight:
		return "8"
	case uno.Nine:
		return "9"
	case uno.DrawTwo:
		return "+2 (Comprar 2)"
	case uno.Reverse:
		return "🔄 Inverter"
	case uno.Skip:
		return "🚫 Pular"
	case uno.SwapHands:
		return "🔀 Trocar cartas"
	case uno.Wild:
		return "🌈 Coringa"
	case uno.WildDrawFour:
		return "🌈+4 Coringa Comprar 4"
	default:
		return "?"
	}
}

type Renderer struct {
	botID     atomic.Int64
	userCache *UserCache
}

func NewRenderer(cache *UserCache) *Renderer {
	if cache == nil {
		cache = NewUserCache(10000)
	}
	return &Renderer{userCache: cache}
}

// SetBotID configures presentation identity after GetMe, before update ingress.
func (r *Renderer) SetBotID(id int64) { r.botID.Store(id) }

// PlayerLink keeps the displayed identity separate from the mention target.
// Events and confirmations must use the resulting view, not the previous turn.
func (r *Renderer) PlayerLink(id uno.PlayerID, view game.PublicGameView) string {
	name := html.EscapeString(r.userCache.GetRawName(id))
	target := r.botID.Load()
	if target <= 0 {
		return name
	}
	if !view.Closed {
		responsible := uno.PlayerID(0)
		switch view.Phase {
		case uno.TakingTurn:
			responsible = view.CurrentTurn
		case uno.ChoosingPlayer:
			responsible = view.PlayerChooserID
		case uno.ChoosingColor:
			responsible = view.ColorChooserID
		}
		if id > 0 && id == responsible {
			target = int64(id)
		}
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, target, name)
}

// RenderLobby returns formatted text for a newly created or joined lobby.
func (r *Renderer) RenderLobby(view game.PublicGameView, lang groups.CommandLanguage) string {
	var sb strings.Builder
	sb.WriteString("🎮 <b>Partida de UNO</b>\n\n")
	sb.WriteString(fmt.Sprintf("Responsável: %s\n\n", r.PlayerLink(view.OwnerID, view)))

	sb.WriteString(fmt.Sprintf("Jogadores inscritos (%d/10):\n", len(view.Players)))
	if len(view.Players) == 0 {
		sb.WriteString("<i>Nenhum jogador inscrito ainda.</i>\n")
	} else {
		for i, p := range view.Players {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.PlayerLink(p.ID, view)))
		}
	}

	if lang == groups.CommandsEN {
		if view.Locked {
			sb.WriteString("\n🔒 This game is locked and is not accepting new players.")
		} else {
			sb.WriteString(fmt.Sprintf("\nUse %s to join.", commandName(cmdJoin, lang)))
		}
		if len(view.Players) >= 2 {
			sb.WriteString(fmt.Sprintf("\nUse %s to start!", commandName(cmdStart, lang)))
		} else {
			sb.WriteString("\n(At least 2 players are needed to start)")
		}
	} else {
		if view.Locked {
			sb.WriteString("\n🔒 Esta partida está trancada e não aceita novos jogadores.")
		} else {
			sb.WriteString(fmt.Sprintf("\nUse %s para participar.", commandName(cmdJoin, lang)))
		}
		if len(view.Players) >= 2 {
			sb.WriteString(fmt.Sprintf("\nUse %s para começar!", commandName(cmdStart, lang)))
		} else {
			sb.WriteString("\n(Mínimo de 2 jogadores para iniciar)")
		}
	}

	return sb.String()
}

func placementLabel(position int) string {
	switch position {
	case 1:
		return "🥇"
	case 2:
		return "🥈"
	case 3:
		return "🥉"
	default:
		return fmt.Sprintf("%d.", position)
	}
}

func (r *Renderer) renderPlacements(view game.PublicGameView) string {
	lines := make([]string, 0, len(view.Placements))
	for _, pl := range view.Placements {
		lines = append(lines, fmt.Sprintf("%s %s", placementLabel(pl.Position), r.PlayerLink(pl.PlayerID, view)))
	}
	return strings.Join(lines, "\n")
}

// RenderPublicState displays the next decision and the actual turn sequence,
// starting at the current player and walking in the engine's direction.
// renderClosedState renders the final summary of a finished game.
func (r *Renderer) renderClosedState(view game.PublicGameView) string {
	var sb strings.Builder
	sb.WriteString("🏆 <b>Partida encerrada</b>")
	switch view.CloseReason {
	case game.Cancelled:
		sb.WriteString("\nCancelada pelo responsável.")
	case game.Departure:
		sb.WriteString("\nEncerrada após desistência de jogadores.")
	}
	if len(view.Placements) > 0 {
		sb.WriteString("\n\n" + r.renderPlacements(view))
	}
	return sb.String()
}

// RenderPublicState renders the detailed public state. It is requested on
// demand; the running flow only announces the action and the next turn.
func (r *Renderer) RenderPublicState(view game.PublicGameView) string {
	var sb strings.Builder
	if view.Closed || view.Phase == uno.Finished {
		return r.renderClosedState(view)
	}
	if view.TopCard != nil {
		sb.WriteString(fmt.Sprintf("🃏 Topo: <b>%s</b>\n", CardRepr(*view.TopCard)))
		if view.TopCard.Color == uno.NoColor && view.ActiveColor != uno.NoColor {
			sb.WriteString(fmt.Sprintf("🎨 Cor: %s <b>%s</b>\n", ColorIcon(view.ActiveColor), ColorNamePT(view.ActiveColor)))
		}
	}
	if view.DrawCounter > 0 {
		sb.WriteString(fmt.Sprintf("⚠️ Compra acumulada: %d cartas\n", view.DrawCounter))
	}
	if len(view.Placements) == 1 {
		sb.WriteString("🏅 Classificação: " + r.renderPlacements(view) + "\n")
	} else if len(view.Placements) > 1 {
		sb.WriteString("🏅 <b>Classificação</b>\n" + r.renderPlacements(view) + "\n")
	}
	sb.WriteString("\n")
	switch view.Phase {
	case uno.ChoosingPlayer:
		sb.WriteString(fmt.Sprintf("🔀 <b>Aguardando %s escolher com quem trocar ou manter a mão!</b>\n", r.PlayerLink(view.PlayerChooserID, view)))
	case uno.ChoosingColor:
		sb.WriteString(fmt.Sprintf("🎨 <b>Aguardando %s escolher a cor!</b>\n", r.PlayerLink(view.ColorChooserID, view)))
	default:
		if view.CurrentTurn > 0 {
			sb.WriteString(fmt.Sprintf("Vez de: %s\n", r.PlayerLink(view.CurrentTurn, view)))
		}
	}
	if order := r.renderOrder(view, view.CurrentTurn); order != "" {
		sb.WriteString(order)
	}
	return strings.TrimSpace(sb.String())
}

// renderOrder walks the active seats from startAt in the current direction and
// includes each player's public card count. Counts are public information; the
// cards themselves remain private to the inline hand menu.
func (r *Renderer) renderOrder(view game.PublicGameView, startAt uno.PlayerID) string {
	if len(view.Order) == 0 {
		return ""
	}
	parts := make([]string, 0, len(view.Order))
	start, direction := 0, 1
	if view.Direction < 0 {
		direction = -1
	}
	for i, id := range view.Order {
		if id == startAt {
			start = i
			break
		}
	}
	for step := range view.Order {
		pid := view.Order[(start+step*direction+len(view.Order))%len(view.Order)]
		for _, player := range view.Players {
			if player.ID != pid || !player.Active {
				continue
			}
			text := fmt.Sprintf("%s (%d)", r.PlayerLink(pid, view), player.CardCount)
			if player.CardCount == 1 {
				text += " ⚠️ <b>UNO!</b>"
			}
			parts = append(parts, text)
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "👥 " + strings.Join(parts, " → ")
}

// RenderTurn names the player whose decision is pending. A closed game has no
// pending decision and therefore no turn line.
func (r *Renderer) RenderTurn(view game.PublicGameView) string {
	if view.Closed {
		return ""
	}
	var responsible uno.PlayerID
	switch view.Phase {
	case uno.ChoosingColor:
		responsible = view.ColorChooserID
	case uno.ChoosingPlayer:
		responsible = view.PlayerChooserID
	default:
		responsible = view.CurrentTurn
	}
	if responsible <= 0 {
		return ""
	}
	return fmt.Sprintf("Vez de: %s", r.PlayerLink(responsible, view))
}

// RenderActionConfirmation renders the only message posted for an accepted
// action. The sticker posted by inline mode already shows the card, so the
// confirmation carries just whose decision is next. A bluff challenge has no
// sticker of its own, so its result is announced here instead. A finished game
// shows the final placements.
// RenderSkipNotice announces a penalty skip: who was skipped, how much waiting
// time they have left and whose turn it is. It never carries the top card or
// the order with card counts, which belong exclusively to /estado.
func (r *Renderer) RenderSkipNotice(actorID uno.PlayerID, action uno.Action, outcome game.Outcome) string {
	if outcome.View.Closed || outcome.View.Phase == uno.Finished || outcome.Skip == nil {
		return r.RenderActionConfirmation(actorID, action, outcome)
	}
	seconds := int(outcome.Skip.Remaining.Seconds())
	if outcome.Skip.Remaining > 0 && seconds == 0 {
		seconds = 1
	}
	text := fmt.Sprintf("⏰ %s foi pulado. Tempo restante: %ds.", r.PlayerLink(outcome.Skip.PlayerID, outcome.View), seconds)
	if turn := r.RenderTurn(outcome.View); turn != "" {
		text += "\n\n" + turn
	}
	return text
}

func (r *Renderer) RenderActionConfirmation(actorID uno.PlayerID, action uno.Action, outcome game.Outcome) string {
	var prefix string
	if action.Type == uno.CallBluff {
		for _, ev := range outcome.Events {
			if ev.Type != uno.BluffCalled {
				continue
			}
			target := r.PlayerLink(ev.TargetID, outcome.View)
			if ev.Success {
				prefix = fmt.Sprintf("Blefe pego! %s recebeu %d cartas!", target, ev.Count)
			} else {
				prefix = fmt.Sprintf("%s não blefou! %s recebeu %d cartas!", target, r.PlayerLink(actorID, outcome.View), ev.Count)
			}
			break
		}
	}
	if outcome.View.Closed || outcome.View.Phase == uno.Finished {
		final := r.renderClosedState(outcome.View)
		if prefix == "" {
			return final
		}
		return prefix + "\n\n" + final
	}
	turn := r.RenderTurn(outcome.View)
	if prefix == "" {
		return turn
	}
	return prefix + "\n\n" + turn
}

func (r *Renderer) RenderWelcome() string {
	return "👋 <b>Bem-vindo ao UnoBotGO!</b>\n\n" +
		"Jogue UNO com seus amigos diretamente nos grupos do Telegram. Crie partidas, escolha o modo de jogo e use sua mão pelo menu privado.\n\n" +
		"Use /ajuda para conhecer todos os comandos."
}

func (r *Renderer) RenderHelp(botUsername string, lang groups.CommandLanguage) string {
	var sb strings.Builder
	sb.WriteString("📖 <b>UnoBotGO — ")
	if lang == groups.CommandsEN {
		sb.WriteString("Commands")
	} else {
		sb.WriteString("Comandos")
	}
	sb.WriteString("</b>\n\n<blockquote>")

	if lang == groups.CommandsEN {
		sb.WriteString("<b>/start</b> — Shows the bot introduction in private.\n")
	} else {
		sb.WriteString("<b>/start</b> — Mostra a apresentação do bot no privado.\n")
	}
	orders := []commandID{cmdNew, cmdJoin, cmdClose, cmdOpen, cmdStart, cmdState, cmdRanking, cmdStats, cmdLeave, cmdSkip, cmdKick, cmdKill, cmdReset, cmdConfig, cmdHelp}
	for _, id := range orders {
		sb.WriteString(fmt.Sprintf("<b>%s</b> — %s\n", commandName(id, lang), commandDescriptions[lang][id]))
	}
	sb.WriteString("</blockquote>\n\n")

	if lang == groups.CommandsEN {
		sb.WriteString("In groups, every command must mention this bot. In private, the suffix is optional.\n")
		if botUsername != "" {
			sb.WriteString(fmt.Sprintf("Example: <code>/join@%s</code>\n\n", html.EscapeString(strings.TrimPrefix(botUsername, "@"))))
		}
		sb.WriteString("<b>How to play your cards:</b>\n")
		sb.WriteString("On your turn, tap the <b>Suas cartas</b> button or type in the chat:\n")
		if botUsername != "" {
			sb.WriteString(fmt.Sprintf("<code>@%s</code>\n\n", html.EscapeString(strings.TrimPrefix(botUsername, "@"))))
		} else {
			sb.WriteString("<code>@yourbot</code>\n\n")
		}
		sb.WriteString("Your private hand appears in the inline menu. Tap a playable (colored) card to play it! ")
		sb.WriteString("The 🔀 Swap card lets you choose another player or keep your hand, then a colour. ")
		sb.WriteString("The official confirmation and the updated state are posted in the group.")
	} else {
		sb.WriteString("Em grupos, todos os comandos precisam mencionar este bot. No privado, o sufixo é opcional.\n")
		if botUsername != "" {
			sb.WriteString(fmt.Sprintf("Exemplo: <code>/entrar@%s</code>\n\n", html.EscapeString(strings.TrimPrefix(botUsername, "@"))))
		}
		sb.WriteString("<b>Como jogar suas cartas:</b>\n")
		sb.WriteString("Quando for a sua vez, clique no botão <b>Suas cartas</b> ou digite no chat:\n")
		if botUsername != "" {
			sb.WriteString(fmt.Sprintf("<code>@%s</code>\n\n", html.EscapeString(strings.TrimPrefix(botUsername, "@"))))
		} else {
			sb.WriteString("<code>@seubot</code>\n\n")
		}
		sb.WriteString("Sua mão privada aparecerá no menu inline. Toque em uma carta jogável (colorida) para jogá-la! ")
		sb.WriteString("A carta 🔀 Trocar cartas permite escolher outro jogador ou manter a mão, e depois a cor. ")
		sb.WriteString("A confirmação oficial e o estado atualizado serão enviados no grupo.\n\n")
		sb.WriteString("🇧🇷 Esta é uma versão brasileira desenvolvida em Go (Golang), baseada no @unopybot.")
	}
	return sb.String()
}

func groupRankingSummary(system groups.RankingSystem) (string, string) {
	switch system {
	case groups.Updated:
		return "🏆 Colocação", "Os pontos seguem a posição final: quanto melhor, mais pontos."
	case groups.Legacy:
		fallthrough
	default:
		return "🏆 Vitória", "Quem não ficou em último recebe +1 ponto."
	}
}

// RenderGroupConfig formats the current group configuration for the /config command.
func (r *Renderer) RenderGroupConfig(config groups.Config) string {
	rankLabel := "Vitória"
	if config.RankingSystem == groups.Updated {
		rankLabel = "Colocação"
	}
	rankTitle, rankDesc := groupRankingSummary(config.RankingSystem)
	langLabel := "Português"
	if config.CommandLanguage == groups.CommandsEN {
		langLabel = "English"
	}

	return fmt.Sprintf("⚙️ <b>Configuração do Grupo</b>\n\n"+
		"<b>Modo de partida:</b> Caseiro\n"+
		"<b>Sistema de ranking:</b> %s\n"+
		"<b>Idioma dos comandos:</b> %s\n\n"+
		"<blockquote><b>%s</b>\n%s</blockquote>\n\n"+
		"<i>Selecione abaixo para alterar.\nAs mudanças afetarão apenas as próximas partidas criadas.</i>",
		rankLabel, langLabel, rankTitle, rankDesc)
}

// RenderPlayerStats formats the accumulated history of one player.
func (r *Renderer) RenderPlayerStats(stats ranking.PlayerStats, scope string) string {
	if !stats.HasHistory() {
		if scope == "" {
			return "📊 <b>Suas estatísticas</b>\n\nVocê ainda não tem partidas pontuadas."
		}
		return fmt.Sprintf("📊 <b>Suas estatísticas em %s</b>\n\nVocê ainda não tem partidas pontuadas aqui.", scope)
	}
	var sb strings.Builder
	sb.WriteString("📊 <b>Suas estatísticas")
	if scope != "" {
		sb.WriteString(" em " + scope)
	}
	sb.WriteString("</b>\n\n")
	sb.WriteString(fmt.Sprintf("Partidas pontuadas: %d\n", stats.Games))
	sb.WriteString(fmt.Sprintf("Vitórias: %d\n", stats.Wins))
	sb.WriteString(fmt.Sprintf("Pontos: %s\n", ranking.FormatScore(stats.System, stats.Score)))
	if stats.BestPosition > 0 {
		sb.WriteString(fmt.Sprintf("Melhor colocação: %dº\n", stats.BestPosition))
	}
	if stats.AvgPosition != "" {
		sb.WriteString(fmt.Sprintf("Colocação média: %s\n", stats.AvgPosition))
	}
	if stats.Rank > 0 && stats.Total > 0 {
		sb.WriteString(fmt.Sprintf("Posição no ranking: %d de %d\n", stats.Rank, stats.Total))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// RenderGroupWelcome formats the introductory message when the bot joins a group.
func (r *Renderer) RenderGroupWelcome(config groups.Config) string {
	rankLabel := "Vitória"
	if config.RankingSystem == groups.Updated {
		rankLabel = "Colocação"
	}
	return fmt.Sprintf("👋 <b>Olá! Obrigado por adicionar o UnoBotGO ao grupo!</b>\n\n"+
		"O bot já está pronto para jogar. As partidas usam o modo <b>Caseiro</b> e o ranking <b>%s</b>.\n\n"+
		"Use /novo para iniciar uma partida ou clique no botão abaixo para personalizar as opções do grupo.", rankLabel)
}

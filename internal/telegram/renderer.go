package telegram

import (
	"fmt"
	"html"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/malbs/UnoGoBot/internal/game"
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
func (r *Renderer) RenderLobby(view game.PublicGameView) string {
	var sb strings.Builder
	sb.WriteString("🎮 <b>Partida de UNO</b>\n\n")
	sb.WriteString(fmt.Sprintf("Responsável: %s\n", r.PlayerLink(view.OwnerID, view)))
	rulesDesc := "Clássico"
	if view.Rules.StackWildDrawFourOnTwo || view.Rules.StackDrawTwoOnWildFour {
		rulesDesc = "Caseiro"
	}
	sb.WriteString(fmt.Sprintf("Regras: %s\n\n", rulesDesc))

	sb.WriteString(fmt.Sprintf("Jogadores inscritos (%d/10):\n", len(view.Players)))
	if len(view.Players) == 0 {
		sb.WriteString("<i>Nenhum jogador inscrito ainda.</i>\n")
	} else {
		for i, p := range view.Players {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.PlayerLink(p.ID, view)))
		}
	}

	sb.WriteString("\nClique em /entrar para participar.")
	if len(view.Players) >= 2 {
		sb.WriteString("\nUse /iniciar para começar!")
	} else {
		sb.WriteString("\n(Mínimo de 2 jogadores para iniciar)")
	}

	return sb.String()
}

// RenderPublicState formats the full observable state of an active or finished game.
func (r *Renderer) RenderPublicState(view game.PublicGameView) string {
	var sb strings.Builder

	if view.Closed || view.Phase == uno.Finished {
		sb.WriteString("🏆 <b>Partida Encerrada!</b>\n\n")
		switch view.CloseReason {
		case game.Completed:
			sb.WriteString("A partida chegou ao fim.\n\n")
		case game.Cancelled:
			sb.WriteString("A partida foi cancelada pelo responsável.\n\n")
		case game.Departure:
			sb.WriteString("A partida foi encerrada após desistência de jogadores.\n\n")
		}

		if len(view.Placements) > 0 {
			sb.WriteString("<b>Colocações finais:</b>\n")
			for _, pl := range view.Placements {
				sb.WriteString(fmt.Sprintf("%dº lugar: %s\n", pl.Position, r.PlayerLink(pl.PlayerID, view)))
			}
		}
		return sb.String()
	}

	// Top card & active color
	if view.TopCard != nil {
		sb.WriteString(fmt.Sprintf("Carta no topo: <b>%s</b>\n", CardRepr(*view.TopCard)))
	}
	if view.ActiveColor != uno.NoColor {
		sb.WriteString(fmt.Sprintf("Cor ativa: %s <b>%s</b>\n", ColorIcon(view.ActiveColor), ColorNamePT(view.ActiveColor)))
	}

	// Direction
	dir := "➡️ Sentido horário"
	if view.Direction < 0 {
		dir = "⬅️ Sentido anti-horário (Invertido)"
	}
	sb.WriteString(fmt.Sprintf("Direção: %s\n\n", dir))

	if view.DrawCounter > 0 {
		sb.WriteString(fmt.Sprintf("⚠️ <b>Penalidade acumulada: comprar %d cartas!</b>\n\n", view.DrawCounter))
	}

	// Placements so far (if BotRules)
	if len(view.Placements) > 0 {
		sb.WriteString("<b>Colocações:</b>\n")
		for _, pl := range view.Placements {
			sb.WriteString(fmt.Sprintf("%dº: %s | ", pl.Position, r.PlayerLink(pl.PlayerID, view)))
		}
		sb.WriteString("\n\n")
	}

	// Players
	sb.WriteString("<b>Jogadores em jogo:</b>\n")
	playerParts := make([]string, 0, len(view.Order))
	for _, pid := range view.Order {
		var p *game.PublicPlayer
		for i := range view.Players {
			if view.Players[i].ID == pid {
				p = &view.Players[i]
				break
			}
		}
		if p == nil || !p.Active {
			continue
		}

		entry := r.PlayerLink(pid, view)
		if p.CardCount == 1 {
			entry += " ⚠️ <b>UNO!</b>"
		}
		if pid == view.CurrentTurn {
			entry = "👉 <b>" + entry + "</b>"
		}
		playerParts = append(playerParts, entry)
	}

	sep := " ➡️ "
	if view.Direction < 0 {
		sep = " ⬅️ "
	}
	sb.WriteString(strings.Join(playerParts, sep))
	sb.WriteString("\n\n")

	// Phase / Turn
	if view.Phase == uno.ChoosingColor {
		sb.WriteString(fmt.Sprintf("🎨 <b>Aguardando %s escolher a cor!</b>", r.PlayerLink(view.ColorChooserID, view)))
	} else if view.CurrentTurn > 0 {
		sb.WriteString(fmt.Sprintf("👉 Vez de: %s", r.PlayerLink(view.CurrentTurn, view)))
	}

	return sb.String()
}

// RenderActionConfirmation renders the confirmation of an accepted action to the group.
func (r *Renderer) RenderActionConfirmation(actorID uno.PlayerID, action uno.Action, outcome game.Outcome) string {
	var sb strings.Builder

	actorLink := r.PlayerLink(actorID, outcome.View)

	switch action.Type {
	case uno.PlayCard:
		if outcome.View.TopCard != nil {
			sb.WriteString(fmt.Sprintf("%s jogou <b>%s</b>!", actorLink, CardRepr(*outcome.View.TopCard)))
		} else {
			sb.WriteString(fmt.Sprintf("%s jogou uma carta!", actorLink))
		}
	case uno.DrawCard:
		count := 1
		for _, ev := range outcome.Events {
			if ev.Type == uno.CardsDrawn {
				count = ev.Count
				break
			}
		}
		cardWord := "carta"
		if count > 1 {
			cardWord = "cartas"
		}
		sb.WriteString(fmt.Sprintf("%s comprou %d %s.", actorLink, count, cardWord))
	case uno.PassTurn:
		sb.WriteString(fmt.Sprintf("%s passou a vez.", actorLink))
	case uno.ChooseColor:
		sb.WriteString(fmt.Sprintf("%s escolheu a cor %s <b>%s</b>!", actorLink, ColorIcon(action.Color), ColorNamePT(action.Color)))
	}

	// Check for special events
	for _, ev := range outcome.Events {
		switch ev.Type {
		case uno.DirectionChanged:
			sb.WriteString(" 🔄 O sentido do jogo foi invertido!")
		case uno.PlayerSkipped:
			if ev.PlayerID > 0 {
				sb.WriteString(fmt.Sprintf(" 🚫 %s foi pulado(a)!", r.PlayerLink(ev.PlayerID, outcome.View)))
			}
		case uno.PlayerWon:
			sb.WriteString(fmt.Sprintf("\n🎉 <b>%s bateu e garantiu o %dº lugar!</b>", r.PlayerLink(ev.PlayerID, outcome.View), ev.Position))
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(r.RenderPublicState(outcome.View))
	return sb.String()
}

// RenderHelp returns standard help text in Portuguese.
func (r *Renderer) RenderHelp(botUsername string) string {
	var sb strings.Builder
	sb.WriteString("📖 <b>UnoBotGO — Como Jogar</b>\n\n")
	sb.WriteString("UnoBotGO V2 permite jogar UNO diretamente em grupos pelo Telegram!\n\n")
	sb.WriteString("<b>Comandos principais (em grupos):</b>\n")
	sb.WriteString("/novo — Cria uma nova partida com regras clássicas e colocações\n")
	sb.WriteString("/entrar — Inscreve-se na partida aberta\n")
	sb.WriteString("/iniciar — Começa a partida (apenas o responsável)\n")
	sb.WriteString("/estado — Mostra o estado atual da partida\n")
	sb.WriteString("/sair — Sai da partida em andamento\n")
	sb.WriteString("/cancelar — Cancela a partida (apenas o responsável)\n")
	sb.WriteString("/ajuda — Exibe esta mensagem de ajuda\n\n")

	sb.WriteString("<b>Como jogar suas cartas:</b>\n")
	sb.WriteString("Quando for a sua vez, clique no botão <b>Suas cartas</b> ou digite no chat:\n")
	if botUsername != "" {
		sb.WriteString(fmt.Sprintf("<code>@%s</code>\n\n", botUsername))
	} else {
		sb.WriteString("<code>@seubot</code>\n\n")
	}
	sb.WriteString("Sua mão privada aparecerá no menu inline. Toque em uma carta jogável (colorida) para jogá-la! ")
	sb.WriteString("A confirmação oficial e o estado atualizado serão enviados no grupo da partida.")

	return sb.String()
}

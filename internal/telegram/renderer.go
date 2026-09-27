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

	if view.Locked {
		sb.WriteString("\n🔒 Esta partida está trancada e não aceita novos jogadores.")
	} else {
		sb.WriteString("\nClique em /entrar para participar.")
	}
	if len(view.Players) >= 2 {
		sb.WriteString("\nUse /iniciar para começar!")
	} else {
		sb.WriteString("\n(Mínimo de 2 jogadores para iniciar)")
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
		return fmt.Sprintf("%dº", position)
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
func (r *Renderer) RenderPublicState(view game.PublicGameView) string {
	var sb strings.Builder
	if view.Closed || view.Phase == uno.Finished {
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
		sb.WriteString(fmt.Sprintf("🔀 <b>Aguardando %s escolher um jogador para trocar cartas!</b>\n", r.PlayerLink(view.PlayerChooserID, view)))
	case uno.ChoosingColor:
		sb.WriteString(fmt.Sprintf("🎨 <b>Aguardando %s escolher a cor!</b>\n", r.PlayerLink(view.ColorChooserID, view)))
	default:
		if view.CurrentTurn > 0 {
			sb.WriteString(fmt.Sprintf("🎯 Vez: 👉 %s\n", r.PlayerLink(view.CurrentTurn, view)))
		}
	}
	parts := make([]string, 0, len(view.Order))
	start, direction := 0, 1
	if view.Direction < 0 {
		direction = -1
	}
	for i, id := range view.Order {
		if id == view.CurrentTurn {
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
			text := r.PlayerLink(pid, view)
			if player.CardCount == 1 {
				text += " ⚠️ <b>UNO!</b>"
			}
			parts = append(parts, text)
			break
		}
	}
	if len(parts) > 0 {
		sb.WriteString("👥 " + strings.Join(parts, " → "))
	}
	return strings.TrimSpace(sb.String())
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
	case uno.ChoosePlayer:
		sb.WriteString(fmt.Sprintf("🔀 %s trocou todas as cartas com %s!", actorLink, r.PlayerLink(action.TargetID, outcome.View)))
	case uno.ChooseColor:
		sb.WriteString(fmt.Sprintf("%s escolheu %s <b>%s</b>!", actorLink, ColorIcon(action.Color), ColorNamePT(action.Color)))
	case uno.CallBluff:
		var bluffEv *uno.Event
		for i := range outcome.Events {
			if outcome.Events[i].Type == uno.BluffCalled {
				bluffEv = &outcome.Events[i]
				break
			}
		}
		if bluffEv != nil {
			targetLink := r.PlayerLink(bluffEv.TargetID, outcome.View)
			if bluffEv.Success {
				sb.WriteString(fmt.Sprintf("Blefe pego! %s recebeu %d cartas!", targetLink, bluffEv.Count))
			} else {
				sb.WriteString(fmt.Sprintf("%s não blefou! %s recebeu %d cartas!", targetLink, actorLink, bluffEv.Count))
			}
		}
	}

	// Check for special events
	for _, ev := range outcome.Events {
		switch ev.Type {
		case uno.DirectionChanged:
			sb.WriteString("\n🔄 O sentido foi invertido.")
		case uno.PlayerSkipped:
			if ev.PlayerID > 0 {
				sb.WriteString(fmt.Sprintf("\n🚫 %s foi pulado.", r.PlayerLink(ev.PlayerID, outcome.View)))
			}
		case uno.PlayerWon:
			medal := placementLabel(ev.Position)
			if ev.Position > 3 {
				medal = "🏅"
			}
			sb.WriteString(fmt.Sprintf("\n%s <b>%s terminou em %dº lugar!</b>", medal, r.PlayerLink(ev.PlayerID, outcome.View), ev.Position))
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(r.RenderPublicState(outcome.View))
	return sb.String()
}

// RenderHelp returns standard help text in Portuguese.
func (r *Renderer) RenderWelcome() string {
	return "👋 <b>Bem-vindo ao UnoBotGO!</b>\n\n" +
		"Jogue UNO com seus amigos diretamente nos grupos do Telegram. Crie partidas, escolha o modo de jogo e use sua mão pelo menu privado.\n\n" +
		"Use /help para conhecer todos os comandos."
}

func (r *Renderer) RenderHelp(botUsername string) string {
	var sb strings.Builder
	sb.WriteString("📖 <b>UnoBotGO — Comandos</b>\n\n")
	sb.WriteString("<blockquote>")
	sb.WriteString("<b>/start</b> — Mostra a apresentação do bot no privado.\n")
	sb.WriteString("<b>/help</b> — Exibe esta ajuda. O comando /ajuda é um alias.\n")
	sb.WriteString("<b>/novo</b> — Cria uma partida no grupo.\n")
	sb.WriteString("<b>/entrar</b> — Entra na partida aberta ou em andamento.\n")
	sb.WriteString("<b>/trancar</b> — Impede novos jogadores de entrar.\n")
	sb.WriteString("<b>/destrancar</b> — Permite novas entradas.\n")
	sb.WriteString("<b>/iniciar</b> — Inicia a partida quando houver pelo menos dois jogadores.\n")
	sb.WriteString("<b>/estado</b> — Mostra o lobby ou o estado atual da partida.\n")
	sb.WriteString("<b>/sair</b> — Sai da partida em andamento.\n")
	sb.WriteString("<b>/cancelar</b> — Cancela a partida. O comando /kill é um alias.\n")
	sb.WriteString("<b>/reset</b> — Recupera o grupo e limpa sua partida e histórico.")
	sb.WriteString("</blockquote>\n\n")

	sb.WriteString("<b>Como jogar suas cartas:</b>\n")
	sb.WriteString("Quando for a sua vez, clique no botão <b>Suas cartas</b> ou digite no chat:\n")
	if botUsername != "" {
		sb.WriteString(fmt.Sprintf("<code>@%s</code>\n\n", html.EscapeString(strings.TrimPrefix(botUsername, "@"))))
	} else {
		sb.WriteString("<code>@seubot</code>\n\n")
	}
	sb.WriteString("Sua mão privada aparecerá no menu inline. Toque em uma carta jogável (colorida) para jogá-la! ")
	sb.WriteString("No modo caseiro, a carta 🔀 Trocar cartas permite escolher outro jogador em <b>Suas cartas</b> e trocar as mãos inteiras, mantendo a cor da mesa. ")
	sb.WriteString("A confirmação oficial e o estado atualizado serão enviados no grupo da partida.\n\n")
	sb.WriteString("🇧🇷 Esta é uma versão brasileira desenvolvida em Go (Golang), baseada no @unopybot.")

	return sb.String()
}

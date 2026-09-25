package simulation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

type PlayerStats struct {
	PlayerID          uno.PlayerID
	CardsPlayed       int
	DrawActions       int
	CardsDrawn        int
	PenaltiesReceived int
	UnoAnnouncements  int
	BluffsCalled      int
	BluffsCaught      int
	LargestHand       int
}

type Stats struct {
	GameplayActions  int
	CardsPlayed      int
	DrawActions      int
	CardsDrawn       int
	Skips            int
	Reverses         int
	HandSwaps        int
	Wilds            int
	WildDrawFours    int
	DrawTwos         int
	Stacks           int
	PenaltyEvents    int
	BluffsCalled     int
	BluffsCaught     int
	UnoAnnouncements int
	Players          map[uno.PlayerID]*PlayerStats
}

func CollectStats(result Result) Stats {
	stats := Stats{Players: make(map[uno.PlayerID]*PlayerStats, result.Config.Players)}
	for id := uno.PlayerID(1); id <= uno.PlayerID(result.Config.Players); id++ {
		stats.Players[id] = &PlayerStats{PlayerID: id}
	}
	for _, step := range result.Steps {
		for id, size := range step.After.HandSizes {
			player := ensurePlayerStats(&stats, id)
			if size > player.LargestHand {
				player.LargestHand = size
			}
		}
		if step.Setup || step.Error != "" {
			continue
		}
		stats.GameplayActions++
		player := ensurePlayerStats(&stats, step.Action.PlayerID)
		switch step.Action.Type {
		case uno.PlayCard:
			stats.CardsPlayed++
			player.CardsPlayed++
			if step.Before.DrawCounter > 0 {
				stats.Stacks++
			}
			if step.Card != nil {
				switch step.Card.Rank {
				case uno.Skip:
					stats.Skips++
				case uno.Reverse:
					stats.Reverses++
				case uno.DrawTwo:
					stats.DrawTwos++
				case uno.Wild:
					stats.Wilds++
				case uno.WildDrawFour:
					stats.WildDrawFours++
				}
			}
		case uno.DrawCard:
			stats.DrawActions++
			player.DrawActions++
			if step.Before.DrawCounter > 0 {
				stats.PenaltyEvents++
				player.PenaltiesReceived++
			}
		case uno.ChoosePlayer:
			stats.HandSwaps++
		case uno.CallBluff:
			stats.BluffsCalled++
			player.BluffsCalled++
		}
		for _, event := range step.Events {
			switch event.Type {
			case uno.CardsDrawn:
				stats.CardsDrawn += event.Count
				ensurePlayerStats(&stats, event.PlayerID).CardsDrawn += event.Count
			case uno.UnoAnnounced:
				stats.UnoAnnouncements++
				ensurePlayerStats(&stats, event.PlayerID).UnoAnnouncements++
			case uno.BluffCalled:
				stats.PenaltyEvents++
				recipient := event.PlayerID
				if event.Success {
					recipient = event.TargetID
				}
				ensurePlayerStats(&stats, recipient).PenaltiesReceived++
				if event.Success {
					stats.BluffsCaught++
					ensurePlayerStats(&stats, event.PlayerID).BluffsCaught++
				}
			}
		}
	}
	return stats
}

func ensurePlayerStats(stats *Stats, id uno.PlayerID) *PlayerStats {
	if stats.Players[id] == nil {
		stats.Players[id] = &PlayerStats{PlayerID: id}
	}
	return stats.Players[id]
}

func ExplainStep(step Step) []string {
	if step.Error != "" {
		return nil
	}
	player := PlayerName(step.Action.PlayerID)
	if step.Action.Type == uno.PlayCard && step.Card != nil {
		stack := ""
		if step.Before.DrawCounter > 0 {
			stack = fmt.Sprintf(" A penalidade foi empilhada sobre %d carta(s) pendente(s).", step.Before.DrawCounter)
		}
		switch step.Card.Rank {
		case uno.Skip:
			target := eventPlayer(step.Events, uno.PlayerSkipped)
			return []string{fmt.Sprintf("%s jogou %s: %s perdeu a vez.", player, CardName(*step.Card), PlayerName(target))}
		case uno.Reverse:
			if target := eventPlayer(step.Events, uno.PlayerSkipped); target != 0 {
				return []string{fmt.Sprintf("%s jogou %s: com dois jogadores, a reversão funciona como bloqueio e %s perdeu a vez.", player, CardName(*step.Card), PlayerName(target))}
			}
			direction := "horário"
			if step.After.Direction < 0 {
				direction = "anti-horário"
			}
			return []string{fmt.Sprintf("%s jogou %s: o sentido mudou para %s.", player, CardName(*step.Card), direction)}
		case uno.DrawTwo:
			return []string{fmt.Sprintf("%s jogou %s: a penalidade pendente passou a %d carta(s).%s", player, CardName(*step.Card), step.After.DrawCounter, stack)}
		case uno.SwapHands:
			return []string{fmt.Sprintf("%s jogou Trocar cartas e deve escolher outro jogador; a cor ativa permanece %s.", player, ColorName(step.After.ActiveColor))}
		case uno.Wild:
			return []string{fmt.Sprintf("%s jogou Coringa e deve escolher a nova cor ativa.", player)}
		case uno.WildDrawFour:
			return []string{fmt.Sprintf("%s jogou Coringa +4: após escolher a cor, o próximo jogador poderá responder conforme o modo ou receber a penalidade.%s", player, stack)}
		}
	}
	if step.Action.Type == uno.ChoosePlayer {
		return []string{fmt.Sprintf("%s trocou todas as cartas com %s; a cor ativa permanece %s.", player, PlayerName(step.Action.TargetID), ColorName(step.After.ActiveColor))}
	}
	if step.Action.Type == uno.ChooseColor {
		return []string{fmt.Sprintf("%s escolheu %s como nova cor ativa; a penalidade pendente agora é de %d carta(s).", player, ColorName(step.Action.Color), step.After.DrawCounter)}
	}
	if step.Action.Type == uno.DrawCard && step.Before.DrawCounter > 0 {
		count := eventCount(step.Events, uno.CardsDrawn)
		return []string{fmt.Sprintf("%s não conseguiu ou não pôde empilhar e recebeu a penalidade de %d carta(s); sua vez foi encerrada.", player, count)}
	}
	if step.Action.Type == uno.CallBluff {
		for _, event := range step.Events {
			if event.Type != uno.BluffCalled {
				continue
			}
			if event.Success {
				return []string{fmt.Sprintf("%s desafiou o +4 e acertou: %s tinha carta da cor anterior e comprou %d carta(s).", player, PlayerName(event.TargetID), event.Count)}
			}
			return []string{fmt.Sprintf("%s desafiou o +4 e errou: a jogada era válida e o desafiante comprou %d carta(s).", player, event.Count)}
		}
	}
	return nil
}

func RenderMarkdown(result Result) string {
	stats := CollectStats(result)
	var b strings.Builder
	b.WriteString("# Relatório da simulação UNO\n\n")
	fmt.Fprintf(&b, "- **Modo:** %s\n", result.Config.Mode.Label())
	fmt.Fprintf(&b, "- **Jogadores:** %d\n", result.Config.Players)
	fmt.Fprintf(&b, "- **Semente:** `%d`\n", result.Config.Seed)
	fmt.Fprintf(&b, "- **Dealer:** %s\n", PlayerName(result.DealerID))
	fmt.Fprintf(&b, "- **Início:** %s\n", result.StartedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- **Fim:** %s\n", result.FinishedAt.UTC().Format(time.RFC3339Nano))
	fmt.Fprintf(&b, "- **Tempo total da simulação:** %s\n", formatDuration(result.FinishedAt.Sub(result.StartedAt)))
	fmt.Fprintf(&b, "- **Estado final:** %s\n\n", completionLabel(result))

	b.WriteString("## Resultado\n\n")
	if len(result.FinalState.Placements) == 0 {
		b.WriteString("A partida não produziu colocações.\n\n")
	} else {
		b.WriteString("| Posição | Jogador | Saiu com vitória |\n|---:|---|:---:|\n")
		for _, placement := range result.FinalState.Placements {
			won := "não"
			if placement.WentOut {
				won = "sim"
			}
			fmt.Fprintf(&b, "| %d | %s | %s |\n", placement.Position, PlayerName(placement.PlayerID), won)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Estatísticas gerais\n\n")
	b.WriteString("| Métrica | Valor |\n|---|---:|\n")
	rows := [][2]any{
		{"Ações de jogo", stats.GameplayActions}, {"Cartas jogadas", stats.CardsPlayed},
		{"Ações de compra", stats.DrawActions}, {"Cartas compradas", stats.CardsDrawn},
		{"Bloqueios", stats.Skips}, {"Reversões", stats.Reverses}, {"Coringas", stats.Wilds},
		{"Trocas de mãos", stats.HandSwaps}, {"+2", stats.DrawTwos}, {"Coringas +4", stats.WildDrawFours}, {"Empilhamentos", stats.Stacks},
		{"Penalidades recebidas", stats.PenaltyEvents}, {"Blefes desafiados", stats.BluffsCalled},
		{"Blefes descobertos", stats.BluffsCaught}, {"Anúncios de UNO", stats.UnoAnnouncements},
	}
	for _, row := range rows {
		fmt.Fprintf(&b, "| %v | %v |\n", row[0], row[1])
	}
	b.WriteString("\n")

	b.WriteString("## Estatísticas por jogador\n\n")
	b.WriteString("| Jogador | Jogadas | Compras | Cartas compradas | Penalidades | UNO | Blefes | Maior mão |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	ids := make([]int, 0, len(stats.Players))
	for id := range stats.Players {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for _, rawID := range ids {
		player := stats.Players[uno.PlayerID(rawID)]
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d (%d certos) | %d |\n",
			PlayerName(player.PlayerID), player.CardsPlayed, player.DrawActions, player.CardsDrawn,
			player.PenaltiesReceived, player.UnoAnnouncements, player.BluffsCalled, player.BluffsCaught, player.LargestHand)
	}
	b.WriteString("\n")

	b.WriteString("## Histórico completo da partida\n\n")
	b.WriteString("O histórico abaixo inclui preparação, todas as jogadas, eventos emitidos pela engine e o estado público resultante de cada ação.\n\n")
	b.WriteString("| Etapa | Revisão | Tipo | Ação | Eventos | Estado após a ação |\n")
	b.WriteString("|---:|---:|---|---|---|---|\n")
	for _, step := range result.Steps {
		kind := "Jogada"
		if step.Setup {
			kind = "Preparação"
		}
		events := describeEvents(step.Events)
		if step.Error != "" {
			events = "ERRO: " + step.Error
		}
		fmt.Fprintf(&b, "| %d | %d → %d | %s | %s | %s | %s |\n",
			step.Number, step.Before.Revision, step.After.Revision, kind,
			DescribeStep(step), events, describeState(step.After))
	}
	b.WriteString("\n")

	b.WriteString("## Jogadas especiais explicadas\n\n")
	specials := 0
	for _, step := range result.Steps {
		for _, explanation := range ExplainStep(step) {
			specials++
			fmt.Fprintf(&b, "%d. **Ação %d, revisão %d:** %s\n", specials, step.Number, step.After.Revision, explanation)
		}
	}
	if specials == 0 {
		b.WriteString("Nenhuma jogada especial ocorreu nesta partida.\n")
	}
	b.WriteString("\n")

	b.WriteString("## Diagnóstico\n\n")
	if len(result.Diagnostics) == 0 {
		b.WriteString("Nenhum erro ou violação de estado foi detectado.\n")
	} else {
		for _, diagnostic := range result.Diagnostics {
			fmt.Fprintf(&b, "- `%s` na ação %d, revisão %d: %s\n", diagnostic.Kind, diagnostic.Step, diagnostic.Revision, diagnostic.Message)
		}
	}
	return b.String()
}

func DefaultReportPath(started time.Time, seed uint64) string {
	utc := started.UTC()
	name := fmt.Sprintf("partida_%s_%03d_seed-%d.md", utc.Format("2006-01-02_15-04-05"), utc.Nanosecond()/int(time.Millisecond), seed)
	return filepath.Join(".reports", "simulations", name)
}

func WriteReport(path string, result Result) error {
	if path == "" {
		path = DefaultReportPath(result.StartedAt, result.Config.Seed)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(RenderMarkdown(result)), 0o644)
}

func DescribeStep(step Step) string {
	player := PlayerName(step.Action.PlayerID)
	switch step.Action.Type {
	case uno.JoinGame:
		return player + " entrou na partida"
	case uno.StartGame:
		return player + " iniciou a partida"
	case uno.PlayCard:
		if step.Card != nil {
			return fmt.Sprintf("%s jogou %s", player, CardName(*step.Card))
		}
	case uno.DrawCard:
		return fmt.Sprintf("%s comprou %d carta(s)", player, eventCount(step.Events, uno.CardsDrawn))
	case uno.PassTurn:
		return player + " passou a vez"
	case uno.ChoosePlayer:
		return fmt.Sprintf("%s trocou todas as cartas com %s", player, PlayerName(step.Action.TargetID))
	case uno.ChooseColor:
		return fmt.Sprintf("%s escolheu %s", player, ColorName(step.Action.Color))
	case uno.CallBluff:
		return player + " desafiou o blefe"
	}
	return fmt.Sprintf("%s executou %s", player, ActionName(step.Action.Type))
}

func ActionName(action uno.ActionType) string {
	switch action {
	case uno.JoinGame:
		return "entrar"
	case uno.StartGame:
		return "iniciar"
	case uno.PlayCard:
		return "jogar carta"
	case uno.DrawCard:
		return "comprar"
	case uno.PassTurn:
		return "passar"
	case uno.ChoosePlayer:
		return "escolher jogador"
	case uno.ChooseColor:
		return "escolher cor"
	case uno.CallBluff:
		return "desafiar blefe"
	case uno.LeaveGame:
		return "sair"
	case uno.CancelGame:
		return "cancelar"
	case uno.SkipTurn:
		return "pular turno"
	case uno.SetRules:
		return "alterar regras"
	default:
		return fmt.Sprintf("ação %d", action)
	}
}

func PlayerName(id uno.PlayerID) string {
	if id <= 0 {
		return "—"
	}
	return fmt.Sprintf("Jogador %d", id)
}

func CardName(card uno.Card) string {
	switch card.Rank {
	case uno.Skip:
		return ColorName(card.Color) + " Bloqueio"
	case uno.Reverse:
		return ColorName(card.Color) + " Reversão"
	case uno.DrawTwo:
		return ColorName(card.Color) + " +2"
	case uno.SwapHands:
		return "Trocar cartas"
	case uno.Wild:
		return "Coringa"
	case uno.WildDrawFour:
		return "Coringa +4"
	default:
		return fmt.Sprintf("%s %d", ColorName(card.Color), card.Rank)
	}
}

func ColorName(color uno.Color) string {
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
		return "sem cor"
	}
}

func completionLabel(result Result) string {
	if result.Completed {
		return "concluída normalmente"
	}
	return "interrompida com diagnóstico"
}

func formatDuration(duration time.Duration) string {
	if duration < time.Millisecond {
		return duration.Round(time.Microsecond).String()
	}
	return duration.Round(time.Millisecond).String()
}

func describeEvents(events []uno.Event) string {
	if len(events) == 0 {
		return "nenhum"
	}
	descriptions := make([]string, 0, len(events))
	for _, event := range events {
		switch event.Type {
		case uno.PlayerJoined:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" entrou")
		case uno.PlayerLeft:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" saiu")
		case uno.GameStarted:
			descriptions = append(descriptions, "partida iniciada")
		case uno.CardPlayed:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" descartou a carta")
		case uno.CardsDrawn:
			descriptions = append(descriptions, fmt.Sprintf("%s comprou %d carta(s)", PlayerName(event.PlayerID), event.Count))
		case uno.PlayerSkipped:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" foi bloqueado")
		case uno.DirectionChanged:
			direction := "horário"
			if event.Direction < 0 {
				direction = "anti-horário"
			}
			descriptions = append(descriptions, "sentido alterado para "+direction)
		case uno.PlayerChoiceRequired:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" deve escolher um jogador")
		case uno.HandsSwapped:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" trocou todas as cartas com "+PlayerName(event.TargetID))
		case uno.ColorChoiceRequired:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" deve escolher uma cor")
		case uno.ColorChosen:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" escolheu "+ColorName(event.Color))
		case uno.TurnChanged:
			descriptions = append(descriptions, "vez de "+PlayerName(event.PlayerID))
		case uno.UnoAnnounced:
			descriptions = append(descriptions, PlayerName(event.PlayerID)+" anunciou UNO")
		case uno.PlayerWon:
			descriptions = append(descriptions, fmt.Sprintf("%s terminou em %dº", PlayerName(event.PlayerID), event.Position))
		case uno.GameFinished:
			descriptions = append(descriptions, "partida encerrada: "+string(event.Reason))
		case uno.BluffCalled:
			outcome := "não encontrou blefe"
			if event.Success {
				outcome = "encontrou blefe"
			}
			descriptions = append(descriptions, fmt.Sprintf("%s %s de %s (%d carta(s))", PlayerName(event.PlayerID), outcome, PlayerName(event.TargetID), event.Count))
		case uno.RulesChanged:
			descriptions = append(descriptions, "regras alteradas")
		default:
			descriptions = append(descriptions, string(event.Type))
		}
	}
	return strings.Join(descriptions, "; ")
}

func describeState(state StateSummary) string {
	parts := []string{"fase: " + phaseName(state.Phase)}
	if state.CurrentPlayer != 0 {
		parts = append(parts, "vez: "+PlayerName(state.CurrentPlayer))
	}
	if state.ActiveColor != uno.NoColor {
		parts = append(parts, "cor: "+ColorName(state.ActiveColor))
	}
	if state.DrawCounter > 0 {
		parts = append(parts, fmt.Sprintf("penalidade: %d", state.DrawCounter))
	}
	ids := make([]int, 0, len(state.HandSizes))
	for id := range state.HandSizes {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	hands := make([]string, 0, len(ids))
	for _, rawID := range ids {
		id := uno.PlayerID(rawID)
		hands = append(hands, fmt.Sprintf("J%d=%d", id, state.HandSizes[id]))
	}
	if len(hands) != 0 {
		parts = append(parts, "mãos: "+strings.Join(hands, ", "))
	}
	return strings.Join(parts, "; ")
}

func phaseName(phase uno.Phase) string {
	switch phase {
	case uno.Lobby:
		return "lobby"
	case uno.TakingTurn:
		return "em jogo"
	case uno.ChoosingPlayer:
		return "escolhendo jogador"
	case uno.ChoosingColor:
		return "escolhendo cor"
	case uno.Finished:
		return "encerrada"
	default:
		return fmt.Sprintf("desconhecida (%d)", phase)
	}
}

func eventPlayer(events []uno.Event, kind uno.EventType) uno.PlayerID {
	for _, event := range events {
		if event.Type == kind {
			return event.PlayerID
		}
	}
	return 0
}

func eventCount(events []uno.Event, kind uno.EventType) int {
	for _, event := range events {
		if event.Type == kind {
			return event.Count
		}
	}
	return 0
}

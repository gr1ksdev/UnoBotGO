# Plano: restituir-blefe-e-ordenacao-cartas

## Pedido do usuário
"o blef foi removido, quando um player joga um +4 coringa, nao da pro usuario solicitar o blefe. tbm qurria que as cartas fossem em ordem igual na v1, elas estao espalhadas, vermelhas entre verdes e virce versa. nao precisa criar plano"

## Objetivo
1. **Restituir o desafio de blefe (Call Bluff)**:
   - Quando um jogador joga um `+4 Coringa`, verificar se ele possuía cartas da cor ativa na mão antes do descarte (se possuía, estava blefando).
   - O próximo jogador (vítima), ao abrir suas opções inline, recebe o botão/sticker `option_bluff` ("Desafiar blefe").
   - Se desafiar o blefe:
     - Se o jogador de fato blefou: "Blefe pego! <jogador> recebeu X cartas!" (o blefador compra as cartas acumuladas em DrawCounter).
     - Se não blefou: "<jogador> não blefou! <desafiante> recebeu X cartas!" (o desafiante compra DrawCounter + 2 cartas).
     - Em ambos os casos, a vez avança para o próximo jogador.
2. **Ordenar cartas da mão igual à V1 (sortedCards)**:
   - Agrupar e ordenar as cartas por cor na exibição da mão:
     - Vermelho (Red: 0)
     - Azul (Blue: 1)
     - Verde (Green: 2)
     - Amarelo (Yellow: 3)
     - Especiais/Coringas (Wild / +4: 99)
   - Dentro de cada cor, ordenar por valor/rank crescente (0-9, Skip, Reverse, +2).
   - Aplicar na renderização inline do Telegram para que o carrossel de stickers e o texto descritivo apareçam sempre organizados por cor.

## Arquivos analisados
- `actions.go` (V1: `doCallBluff`)
- `player.go` (V1: `p.Bluffing`)
- `card.go` (V1: `sortedCards`, `colorRank`, `valueRank`, `Stickers["option_bluff"]`)
- `inline.go` (V1: `addCallBluff`, `sortedCards`)
- `internal/uno/action.go`
- `internal/uno/event.go`
- `internal/uno/state.go`
- `internal/uno/game.go`
- `internal/game/views.go`
- `internal/telegram/inline.go`
- `internal/telegram/renderer.go`

## Arquivos que poderão ser modificados
- `internal/uno/action.go`
- `internal/uno/event.go`
- `internal/uno/state.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/game/views.go`
- `internal/telegram/inline.go`
- `internal/telegram/inline_test.go`
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`

## Estratégia de implementação
1. **Engine (`internal/uno`)**:
   - Adicionar `CallBluff` em `ActionType`.
   - Adicionar `BluffCalled` em `EventType`.
   - Adicionar campos `Success bool` e `TargetID PlayerID` na struct `Event`.
   - Adicionar `BluffInfo` (`Actor`, `Target`, `Bluffing`) e campo `PendingBluff *BluffInfo` em `State`.
   - Em `play`, ao jogar `WildDrawFour`, verificar se o jogador possuía cartas da cor ativa na mão e passar a flag para `ColorChoice`.
   - Em `choose`, se for +4, instanciar `PendingBluff` apontando para o alvo.
   - Tratar ação `CallBluff` em `takeAction` executando `g.bluff`.
2. **Visão da aplicação (`internal/game`)**:
   - Expor `CanCallBluff bool` em `PublicGameView`.
3. **Apresentação Telegram (`internal/telegram`)**:
   - Em `inline.go`, implementar ordenação `sortHand` agrupando por cores e ranks na exibição dos stickers e do resumo de cartas.
   - Quando `CanCallBluff` for verdadeiro, adicionar o sticker `Stickers["option_bluff"]` com ação `CallBluff`.
   - Em `renderer.go`, tratar `case uno.CallBluff` exibindo a mensagem fiel à V1.
4. **Testes e Validação**:
   - Criar testes unitários para blefe pego, blefe falho e ordenação.
   - Executar `go test ./...` e `go vet ./...`.

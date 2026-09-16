# Plano: primeira-carta-numerica-e-sticker

## Pedido do usuário
1. O usuário relatou que a partida começou com uma carta especial (`+2`), penalizando o 2º jogador (`snzu`) com 2 cartas e pulando sua vez logo no `/iniciar`, sem que ninguém tivesse jogado ainda.
2. O usuário pediu para que a primeira carta virada seja enviada como sticker no grupo ao iniciar a partida (o bot estava enviando apenas texto).
3. O usuário aprovou explicitamente: "alem disso, eu nao vi a primeira carta virada, poderia colocar ela tbm, apos isso pode iniciar a implementacao de correcao".

## Objetivo
1. Em `internal/uno/rules.go` e `internal/uno/game.go`, adicionar a regra `NumberedStart` ao `BotRules()`, garantindo que a primeira carta virada do baralho no início da partida seja sempre uma carta numérica (0 a 9). Cartas de ação (`+2`, `Skip`, `Reverse`, `Coringa`) serão descartadas de volta ao baralho até encontrar um número (comportamento idêntico ao V1).
2. Adicionar o método `SendSticker` à interface `BotAPI` e ao `SafeAPICaller` em `internal/telegram/client.go` e ao mock em `internal/telegram/mock_test.go`.
3. Em `internal/telegram/commands.go` (`handleIniciar`), enviar o sticker da carta virada no grupo assim que a partida for iniciada, seguido pela mensagem de status com os botões.

## Contexto atual
- `start()` em `internal/uno/game.go` aceitava qualquer carta diferente de `WildDrawFour` como carta inicial. Se saísse um `+2`, o jogador seguinte recebia 2 cartas e era pulado imediatamente na largada.
- `handleIniciar` em `internal/telegram/commands.go` chamava apenas `h.reply(msg)` com texto HTML, sem enviar o sticker visual da carta do topo.
- `BotAPI` não possuía `SendSticker`.

## Arquivos analisados
- `internal/uno/rules.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/telegram/client.go`
- `internal/telegram/mock_test.go`
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`

## Arquivos que serão modificados
- `internal/uno/rules.go`
- `internal/uno/game.go`
- `internal/telegram/client.go`
- `internal/telegram/mock_test.go`
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`

## Estratégia de implementação
1. **Regra de início numérico**:
   - Em `internal/uno/rules.go`, adicionar `NumberedStart bool` à struct `Rules`.
   - Em `BotRules()`, definir `NumberedStart: true`.
   - Em `start()` (`internal/uno/game.go`), quando `NumberedStart` for verdadeiro, pular qualquer carta com `c.Rank >= Skip`.
2. **Envio de sticker pela API**:
   - Em `internal/telegram/client.go`, incluir `SendSticker(ctx context.Context, params *telego.SendStickerParams) (*telego.Message, error)` em `BotAPI` e implementar em `SafeAPICaller`.
   - Em `internal/telegram/mock_test.go`, registrar stickers enviados em `SentStickers`.
3. **Envio do sticker no `/iniciar`**:
   - Em `commands.go` (`handleIniciar`), após o start bem-sucedido, obter o `stickerID := GetCardStickerID(*outcome.View.TopCard)` e chamar `h.bot.SendSticker`.
4. **Testes**:
   - Adicionar teste em `commands_test.go` verificando que `/iniciar` envia o sticker da primeira carta e que a partida começa com uma carta numérica e ambos os jogadores com 7 cartas.

## Riscos
Nenhum. A separação por flag `NumberedStart` garante compatibilidade regressiva com `ClassicRules()`.

## Impactos esperados
- Toda partida criada no Telegram (`BotRules`) iniciará sempre com uma carta de número normal (0-9).
- Todos os jogadores começarão sempre com exatamente 7 cartas.
- Ao dar `/iniciar`, o sticker da carta virada aparecerá no grupo antes da mensagem de estado.

## Compatibilidade
- Linux, macOS, Windows, Docker: Sim.

## Como testar
### Testes automatizados
```bash
go test -count=1 -race ./...
```

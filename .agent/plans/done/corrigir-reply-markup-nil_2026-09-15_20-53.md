# Plano: corrigir-reply-markup-nil

## Pedido do usuário
O usuário relatou o erro:
`telego: sendMessage: api: 400 "Bad Request: object expected as reply markup"`
ao iniciar e testar o bot no Telegram, e perguntou se o banco de dados é necessário para rankings no momento.

## Objetivo
1. Esclarecer que o banco de dados e os rankings fazem parte do escopo da Milestone 4 e que a V2 (M1..M3) roda 100% em memória sem necessidade de PostgreSQL.
2. Corrigir a atribuição de `ReplyMarkup` em `internal/telegram/commands.go` para não injetar ponteiro nulo tipado em campo de interface, evitando que o JSON serialize `"reply_markup": null`.
3. Adicionar teste de serialização JSON para prevenir regressões desse tipo.

## Contexto atual
Em `internal/telegram/commands.go`:
```go
func (h *CommandHandler) reply(ctx context.Context, chatID int64, text string, markup *telego.InlineKeyboardMarkup) {
	_, err := h.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        text,
		ParseMode:   "HTML",
		ReplyMarkup: markup,
	})
...
```
Quando `markup` é `nil`, o tipo concreto `*telego.InlineKeyboardMarkup(nil)` é atribuído à interface `ReplyMarkup`. O pacote `encoding/json` do Go não considera a interface nula e serializa `"reply_markup": null`. A API do Telegram rejeita isso com HTTP 400 "Bad Request: object expected as reply markup".

## Arquivos analisados
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`

## Arquivos que poderão ser modificados
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`

## Estratégia de implementação
1. Em `commands.go`, só atribuir `params.ReplyMarkup = markup` se `markup != nil`.
2. Em `commands_test.go`, adicionar teste que valida a serialização JSON de `SendMessageParams` para mensagens sem markup, garantindo que `"reply_markup"` não esteja presente no payload enviado à API.

## Passos detalhados
1. Modificar `internal/telegram/commands.go` na função `reply`.
2. Adicionar teste em `internal/telegram/commands_test.go`.
3. Executar `go test -count=1 -race ./...`.

## Riscos
Nenhum risco funcional. A alteração apenas impede o envio de `null` para a API do Telegram quando não há botões.

## Impactos esperados
Respostas de comandos como `/ajuda`, avisos de erro e mensagens sem botões voltam a ser entregues com sucesso pelo Telegram.

## Compatibilidade
- Linux, macOS, Windows, Docker: Sim.

## Como testar
### Build e Testes
```bash
go test -count=1 -race ./internal/telegram
```

## Rollback
Desfazer a edição em `internal/telegram/commands.go`.

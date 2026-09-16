# Plano: ajustar-ux-telegram-v1-parity

## Pedido do usuário
O usuário esclareceu e refinou as expectativas em relação à UX do Telegram:
1. O bot DEVE responder a comandos direcionados no grupo como `/novo@usernamebot` (atualmente só respondia a `/novo`).
2. O criador NÃO DEVE ser inscrito automaticamente ao executar `/novo` (deve apenas criar o lobby com 0 inscritos, exigindo `/entrar` para participar).
3. REMOVER o botão de validação inline ("⏳ Aguardando validação" / "✅ Confirmado") anexado abaixo dos stickers jogados. O sticker deve ser enviado limpo ao chat.
4. O menu de visualização das cartas no modo inline NÃO DEVE aparecer em formato de lista vertical; deve restaurar o formato clássico da versão anterior (grade/carrossel horizontal nativo de stickers do Telegram).

## Objetivo
1. Corrigir o filtro de bot username em `internal/telegram/commands.go` utilizando comparação case-insensitive (`strings.EqualFold(targetBot, h.botUsername)`), permitindo que comandos como `/novo@UnoGoBrBot` funcionem perfeitamente.
2. Manter estritamente o comportamento onde `/novo` cria o lobby sem auto-inscrição do criador (o criador permanece como responsável administrativo e só entra se digitar `/entrar`).
3. Remover completamente a anexação de `ReplyMarkup` nos stickers jogados e suas rotinas de edição (`EditMessageReplyMarkup`) em `internal/telegram/inline.go`, enviando os stickers limpos.
4. Remover o resultado do tipo `InlineQueryResultArticle` da resposta de cartas da mão em `internal/telegram/inline.go`. Retornando exclusivamente `InlineQueryResultCachedSticker`, o cliente do Telegram renderiza as cartas na grade/carrossel visual horizontal de stickers idêntico ao V1.

## Contexto atual
- `commands.go`: `targetBot` era convertido para minúsculas, mas comparado diretamente com `h.botUsername` (ex: `"unogobrbot" != "UnoGoBrBot"`), ignorando o comando.
- `inline.go`: Incluía um `InlineQueryResultArticle` de cabeçalho no início das cartas. No protocolo do Telegram, a presença de um único `article` força a UI inteira a virar uma lista vertical.
- `inline.go`: Adicionava botões inline `makeValidatingInlineMarkup` aos stickers jogados e depois disparava `EditMessageReplyMarkup` para `makeConfirmedInlineMarkup`.

## Arquivos analisados
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/inline.go`
- `internal/telegram/inline_test.go`
- `results.go` (referência V1)
- `inline.go` (referência V1)

## Arquivos que poderão ser modificados
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/inline.go`
- `internal/telegram/inline_test.go`

## Estratégia de implementação
1. **Comandos direcionados (`@botusername`)**:
   - Em `commands.go`:
     ```go
     if len(cmdParts) == 2 {
         targetBot := cmdParts[1]
         if h.botUsername != "" && !strings.EqualFold(targetBot, h.botUsername) {
             return
         }
     }
     ```
2. **Manutenção do `/novo` sem auto-inscrição**:
   - Preservar o fluxo onde `/novo` apenas invoca `service.Create`, garantindo 0 jogadores inscritos até que alguém envie `/entrar`.
3. **Remoção de botões dos stickers**:
   - Em `inline.go`, remover `ReplyMarkup: makeValidatingInlineMarkup(...)` das opções de compra, passe e jogada de carta.
   - Em `handleChosenResultTask`, remover as chamadas a `EditMessageReplyMarkup`.
4. **Restauração da grade de stickers no modo inline**:
   - Em `renderGameHand` (`inline.go`), remover a inserção do `InlineQueryResultArticle` (`hdr_...`).
   - Retornar apenas `InlineQueryResultCachedSticker` (comprar, passar, cartas e se aplicável info).
   - Isso reativa a renderização nativa de grade/carrossel de stickers no Telegram.

## Passos detalhados
1. Atualizar `commands.go` com a checagem case-insensitive do username do bot.
2. Atualizar `inline.go` removendo o artigo de cabeçalho da mão e os markups de validação dos stickers.
3. Atualizar testes unitários em `commands_test.go` e `inline_test.go` para validar as novas regras.
4. Executar a suíte de testes com `go test -count=1 -race ./...`.

## Riscos
Nenhum risco de integridade do jogo ou concorrência. O anti-cheat de tokens de 128 bits continua operando na recepção de `ChosenInlineResult`.

## Impactos esperados
- `/novo@usernamebot` é reconhecido e executado corretamente.
- `/novo` cria o lobby sem inscrever o criador automaticamente.
- Stickers são enviados sem botões no chat.
- As cartas aparecem como carrossel/grade visual de stickers (idêntico ao V1).

## Compatibilidade
- Linux, macOS, Windows, Docker: Sim.

## Como testar
### Testes automatizados
```bash
go test -count=1 -race ./internal/telegram
```

## Rollback
Reverter as alterações em `internal/telegram/commands.go` e `internal/telegram/inline.go`.

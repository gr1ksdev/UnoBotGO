# Plano: simplificar-mensagens-e-liberar-iniciar

## Pedido do usuário
O usuário solicitou 4 ajustes visuais e de jogabilidade:
1. **Remover o botão "Atualizar estado"**: remover o botão `🔄 Atualizar estado` do teclado inline das mensagens de rodada, mantendo exclusivamente o botão `🃏 Suas cartas`.
2. **Remover a contagem de cartas dos jogadores na mensagem**: na listagem de "Jogadores em jogo:", remover o indicador de quantidade `(X cartas)` após o nome dos jogadores, exibindo apenas o nome (e o alerta `⚠️ UNO!` quando o jogador tiver apenas 1 carta).
3. **Remover o nome "UnoBotGO"**: remover o cabeçalho `🃏 UnoBotGO` que antecede a mensagem de estado da partida (`RenderPublicState`).
4. **Liberar o comando `/iniciar` para qualquer usuário**: permitir que qualquer jogador possa iniciar a partida com `/iniciar` (e não apenas quem enviou `/novo`), desde que a partida esteja em fase de lobby com pelo menos 2 jogadores inscritos. O cancelamento (`/cancelar`) continua protegido para o responsável.

## Objetivo
1. Em `internal/telegram/commands.go`:
   - Atualizar `makeGameButtons` para conter apenas o botão `"🃏 Suas cartas"`, removendo o botão `"🔄 Atualizar estado"`.
2. Em `internal/telegram/renderer.go`:
   - Em `RenderPublicState`: remover a linha de cabeçalho `🃏 <b>UnoBotGO</b>\n\n`.
   - Em `RenderPublicState`: na lista de jogadores em jogo, não imprimir `(%d cartas)`. Se o jogador tiver 1 carta, anexar apenas `⚠️ <b>UNO!</b>`.
   - Em `RenderLobby`: atualizar o texto de instrução de "O responsável pode usar /iniciar para começar!" para "Use /iniciar para começar!".
3. Em `internal/game/service.go`:
   - Na função `authorize`: para `uno.StartGame`, remover a restrição de que `actor.PlayerID == entry.ownerID`. Qualquer jogador com `actor.ChatID != 0` pode emitir `StartGame` (desde que haja >= 2 jogadores inscritos na partida). O comando `uno.CancelGame` continua restrito a `entry.ownerID`.
4. Atualizar os testes unitários afetados em `internal/game/service_test.go`, `internal/telegram/renderer_test.go` e `internal/telegram/commands_test.go`.

## Contexto atual
- `makeGameButtons` gera dois botões lado a lado: `🃏 Suas cartas` e `🔄 Atualizar estado`.
- `RenderPublicState` iniciava com `🃏 <b>UnoBotGO</b>\n\n` e formatava cada jogador como `<nome> (X cartas)`.
- `internal/game/service.go` bloqueava com `ErrForbidden` chamadas de `StartGame` feitas por qualquer ID diferente de `entry.ownerID`.
- No V1, qualquer membro do chat podia dar `/iniciar` assim que o mínimo de jogadores fosse atingido.

## Arquivos analisados
- `internal/telegram/commands.go`
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`
- `internal/telegram/commands_test.go`
- `internal/game/service.go`
- `internal/game/service_test.go`

## Arquivos que poderão ser modificados
- `internal/telegram/commands.go`
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`
- `internal/telegram/commands_test.go`
- `internal/game/service.go`
- `internal/game/service_test.go`

## Estratégia de implementação
1. **Teclado de botões (`internal/telegram/commands.go`)**:
   - Simplificar `makeGameButtons`:
     ```go
     func makeGameButtons(gameID uno.GameID) *telego.InlineKeyboardMarkup {
         return &telego.InlineKeyboardMarkup{
             InlineKeyboard: [][]telego.InlineKeyboardButton{
                 {
                     {
                         Text:                         "🃏 Suas cartas",
                         SwitchInlineQueryCurrentChat: stringPtr(fmt.Sprintf("g_%s", gameID)),
                     },
                 },
             },
         }
     }
     ```
2. **Mensagem pública da mesa (`internal/telegram/renderer.go`)**:
   - Remover `sb.WriteString("🃏 <b>UnoBotGO</b>\n\n")`.
   - Na iteração de jogadores:
     ```go
     entry := r.userCache.FormatLink(pid)
     if p.CardCount == 1 {
         entry += " ⚠️ <b>UNO!</b>"
     }
     if pid == view.CurrentTurn {
         entry = "👉 <b>" + entry + "</b>"
     }
     playerParts = append(playerParts, entry)
     ```
   - Em `RenderLobby`:
     Substituir `"O responsável pode usar /iniciar para começar!"` por `"Use /iniciar para começar!"`.
3. **Permissão de início (`internal/game/service.go`)**:
   - Em `authorize`:
     ```go
     case uno.StartGame:
         if actor.ChatID == 0 {
             return ErrForbidden
         }
     case uno.CancelGame:
         if actor.ChatID == 0 || actor.PlayerID != entry.ownerID {
             return ErrForbidden
         }
     ```
4. **Testes**:
   - Atualizar `service_test.go` para validar que qualquer participante do chat pode iniciar com `/iniciar`, e que apenas `CancelGame` rejeita não-owners.
   - Atualizar `commands_test.go` testando que outro jogador pode iniciar a partida.
   - Atualizar `renderer_test.go` validando o novo formato sem "UnoBotGO" e sem contagem de cartas.

## Passos detalhados
1. Atualizar `authorize` em `internal/game/service.go` permitindo `StartGame` por qualquer usuário do chat.
2. Atualizar `internal/game/service_test.go` para refletir a nova regra de autorização de `StartGame`.
3. Atualizar `makeGameButtons` em `internal/telegram/commands.go` removendo o botão "🔄 Atualizar estado".
4. Atualizar `RenderPublicState` e `RenderLobby` em `internal/telegram/renderer.go`.
5. Atualizar testes em `internal/telegram/renderer_test.go` e `internal/telegram/commands_test.go`.
6. Executar suíte completa com `go test -count=1 -race ./...`.

## Riscos
- Risco zero de regressão técnica: `CancelGame` continua estritamente protegido; validações de número de jogadores (mínimo 2) e fase de lobby permanecem ativas na engine e no serviço.

## Impactos esperados
- Mensagens mais limpas no grupo, sem poluição de título nem botão redundante de atualização.
- Jogadores não têm o número exato de cartas exposto a cada rodada.
- Partida pode ser iniciada imediatamente por qualquer jogador pronto no grupo sem depender exclusivamente da presença imediata do criador.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD: Sim.

## Como testar

### Testes automatizados
```bash
go test -count=1 -race ./...
```

### Build
```bash
go build ./cmd/bot
```

## Rollback
Desfazer as alterações nos arquivos modificados via git restore.

## Observações
O comando `/cancelar` continua exclusivo do responsável pela partida para evitar abusos no grupo.

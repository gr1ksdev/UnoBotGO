# Plano: selecao-modo-lobby

## Pedido do usuário
Quando o usuário executa o comando `/novo@usernamebot`, a mensagem do lobby exibe atualmente o botão "🃏 Suas cartas". O usuário pediu para substituir esse botão pela seleção de modo enquanto a partida estiver no lobby aguardando o início. Após iniciar a partida com `/iniciar`, não deve mais ser permitido alterar o modo e o botão deve passar a ser "🃏 Suas cartas".

## Objetivo
1. Na fase de lobby (`Phase == uno.Lobby`), trocar o botão inline da mensagem de "🃏 Suas cartas" para botões interativos de seleção de modo (`[ ✅ 🎻 Clássico ]  [ 🏠 Caseiro ]` ou vice-versa).
2. Permitir que o responsável pela partida altere o modo entre **Clássico** e **Caseiro** clicando nos botões diretamente na mensagem do Telegram.
3. Ao alterar o modo, atualizar dinamicamente o texto do lobby (`Regras: Clássico` ou `Regras: Caseiro`) e os botões inline (movendo o indicador `✅`).
4. Bloquear qualquer alteração de modo após a partida ser iniciada com `/iniciar`, respondendo com alerta e substituindo os botões por "🃏 Suas cartas".

## Contexto atual
- `makeGameButtons` em `internal/telegram/commands.go` atualmente retorna o botão `🃏 Suas cartas` para todas as fases ativas (`Lobby`, `TakingTurn`, `ChoosingColor`). No lobby, nenhum jogador possui cartas, tornando o botão "Suas cartas" prematuro e confuso.
- Em `internal/uno`, as regras da partida (`Rules`) são definidas no momento de criação do jogo (`NewGame`), sem uma ação dedicada para atualizar as regras enquanto o jogo está no lobby.
- `internal/telegram/callbacks.go` já possui infraestrutura para processar callbacks de botões inline via `CallbackHandler`, suportando atualização de mensagem e respostas via `AnswerCallbackQuery`.

## Arquivos analisados
- `internal/uno/action.go`
- `internal/uno/event.go`
- `internal/uno/game.go`
- `internal/game/service.go`
- `internal/game/views.go`
- `internal/telegram/commands.go`
- `internal/telegram/callbacks.go`
- `internal/telegram/callbacks_test.go`
- `internal/telegram/renderer_test.go`

## Arquivos que poderão ser modificados
- `internal/uno/action.go`
- `internal/uno/event.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/game/service.go`
- `internal/telegram/commands.go`
- `internal/telegram/callbacks.go`
- `internal/telegram/callbacks_test.go`
- `internal/telegram/renderer_test.go`

## Estratégia de implementação
1. **Engine Uno (`internal/uno`):**
   - Adicionar `SetRules ActionType = 11` e `RulesChanged EventType = "rules_changed"`.
   - Adicionar campo `Rules Rules` em `Action`.
   - No `Game.Apply`, permitir a ação `SetRules` exclusivamente quando `state.Phase == Lobby`.
   - Se `SetRules` for invocado com o jogo já iniciado, rejeitar com `ErrInvalidPhase`.
   - Atualizar `state.Rules`, incrementar `Revision` e emitir evento `RulesChanged`.

2. **Serviço da Aplicação (`internal/game`):**
   - No método `authorize`, permitir `uno.SetRules` apenas se o autor estiver no mesmo chat e for o responsável (`actor.PlayerID == entry.ownerID`).
   - Adicionar helper `s.SetRules(ctx, actor, gameID, rules)` em `Service`, executando a ação sob lock e atualizando a view.

3. **Teclado do Telegram (`internal/telegram/commands.go`):**
   - Atualizar `makeGameButtons`:
     - Se `view.Phase == uno.Lobby`: gerar teclado inline com os dois modos:
       - `[ ✅ 🎻 Clássico ]  [ 🏠 Caseiro ]` (se regras atuais forem Clássico)
       - `[ 🎻 Clássico ]  [ ✅ 🏠 Caseiro ]` (se regras atuais forem Caseiro)
       - `CallbackData`: `mode_classic_<gameID>` e `mode_caseiro_<gameID>`.
     - Se `view.Phase == uno.TakingTurn` ou `uno.ChoosingColor`: manter exclusivamente o botão `🃏 Suas cartas` com revisão dinâmica.
     - Se `view.Closed` ou `view.Phase == uno.Finished`: retornar `nil`.

4. **Handler de Callbacks (`internal/telegram/callbacks.go`):**
   - No `HandleCallback`, tratar prefixos `mode_`:
     - Obter a view pública do jogo.
     - Se a partida não estiver mais em `Lobby`: exibir alerta via `AnswerCallbackQuery(ShowAlert: true)`: *"⚠️ A partida já foi iniciada, não é possível alterar o modo!"*.
     - Se quem clicou não for o criador/responsável (`cq.From.ID != int64(view.OwnerID)`): exibir alerta: *"⚠️ Apenas o responsável pela partida pode alterar o modo."*.
     - Se o modo clicado já for o modo ativo: responder *"O modo já está definido como ..."*.
     - Caso contrário, chamar `s.SetRules(...)`, editar a mensagem no grupo com o novo `RenderLobby` e novo `makeGameButtons`, e responder o callback *"Modo alterado para Caseiro 🏠"* ou *"Modo alterado para Clássico 🎻"*.

5. **Testes:**
   - Adicionar teste unitário em `internal/uno/game_test.go` verificando que `SetRules` funciona no `Lobby` e falha se já iniciado.
   - Atualizar `internal/telegram/renderer_test.go:TestGameButtonsRespectLifecycle` para validar os botões de modo no `Lobby` e `Suas cartas` nas fases de jogo.
   - Adicionar testes em `internal/telegram/callbacks_test.go` validando o fluxo de troca de modo, permissão de criador e bloqueio após início da partida.

## Riscos
- **Clique após início:** Mensagens antigas do lobby no histórico do chat do Telegram podem conter botões com callback de modo.
  - *Mitigação:* Validação estrita no servidor verificando `view.Phase == uno.Lobby`. Qualquer clique posterior recebe feedback imediato informando que o jogo já começou e o modo está travado.
- **Concorrência:** Múltiplos cliques simultâneos de jogadores diferentes.
  - *Mitigação:* `SetRules` passa pelo lock da partida e valida `actor.PlayerID == entry.ownerID`, com atualização atômica de revisão.

## Impactos esperados
- Logo após `/novo`, a mensagem exibe a seleção direta de modo de jogo (`Clássico` ou `Caseiro`) de forma visual e intuitiva.
- Eliminação do botão prematuro "Suas cartas" enquanto a partida não tem cartas distribuídas.
- Travamento garantido do modo a partir do `/iniciar`.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD.
- Nenhuma dependência externa adicional.

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -v ./...
```

### Execução
```bash
go run cmd/bot/main.go
```

## Rollback
Desfazer alterações com `git checkout` ou `git restore`.

## Observações
O layout e textos existentes de `RenderLobby` e `RenderPublicState` serão rigorosamente preservados.

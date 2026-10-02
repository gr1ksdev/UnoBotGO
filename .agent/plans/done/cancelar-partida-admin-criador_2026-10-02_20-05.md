# Plano: cancelar-partida-admin-criador

## Pedido do usuário
Permitir que qualquer administrador do grupo e quem criou a sala possam cancelar/matar a partida (via `/cancelar` e seu alias `/kill`).

## Objetivo
Expandir a regra de autorização dos comandos `/cancelar` e `/kill` no Telegram e no motor de jogo (`internal/game`), permitindo o encerramento da partida por:
1. O criador da sala (`CreatorID`);
2. O responsável atual da partida (`OwnerID`);
3. Qualquer administrador do grupo no Telegram (`ChatAdmin == true`), verificado via Telegram Bot API (`lookupMembershipAPI`).

## Contexto atual
- Atualmente, a autorização de `uno.CancelGame` em `internal/game/service.go` (`authorize`) exige estritamente `actor.PlayerID == entry.ownerID`.
- No Telegram, `handleCancelar` apenas despacha a ação para o serviço sem consultar a membresia de administrador do Telegram. Caso o remetente não seja o `ownerID`, a ação é rejeitada com `game.ErrForbidden` e o bot responde: *"⚠️ Apenas o responsável pela partida pode cancelá-la."*.
- Se o criador da sala abandonar a partida via `/sair`, a liderança (`ownerID`) é transferida para outro jogador, impedindo que o criador original encerre a sala.
- Administradores do grupo atualmente dependem do comando de recuperação forçada `/reset` para limpar salas presas, não podendo usar o fluxo normal e gracioso de `/cancelar` ou `/kill`.

## Arquivos analisados
- `internal/game/service.go`: validação de `authorize` para `uno.CancelGame` e estrutura `Actor`.
- `internal/game/manager.go`: gerenciamento de `creatorID` e `ownerID` no `managedGame`.
- `internal/game/views.go`: projeções públicas `PublicGameView` e `GameSummary`.
- `internal/telegram/commands.go`: rotas de comando e método `handleCancelar`.
- `internal/telegram/membership.go`: consulta de perfil e cargo no chat via `lookupMembershipAPI`.
- `internal/telegram/renderer.go`: texto de ajuda para `/cancelar` em `RenderHelp`.
- `internal/telegram/commands_test.go`: testes dos comandos de cancelamento.
- `internal/game/service_test.go`: testes unitários de ciclo de vida e permissões do serviço de jogo.

## Arquivos que poderão ser modificados
- `internal/game/views.go`: inclusão do campo `CreatorID` em `GameSummary` para rápida verificação sem chamadas desnecessárias à API externa.
- `internal/game/service.go`: flexibilização de `authorize` para `uno.CancelGame`, aceitando `actor.ChatAdmin || actor.PlayerID == entry.ownerID || actor.PlayerID == entry.creatorID`.
- `internal/game/service_test.go`: testes unitários garantindo que admin e criador podem cancelar, enquanto terceiros sem permissão são negados.
- `internal/telegram/commands.go`: atualização de `handleCancelar` para consultar admin via `lookupMembershipAPI` quando o ator não for o criador nem o responsável atual, além de mensagens de sucesso contextualizadas.
- `internal/telegram/renderer.go`: atualização sutil do texto de ajuda em `RenderHelp`.
- `internal/telegram/commands_test.go`: testes de integração validando cancelamento por criador, admin e rejeição para usuário comum.

## Estratégia de implementação
1. **Modelagem no Game Engine (`internal/game`)**:
   - Em `internal/game/views.go`, adicionar `CreatorID uno.PlayerID` à struct `GameSummary` e preenchê-la a partir de `v.CreatorID` no método `(v PublicGameView) summary()`.
   - Em `internal/game/service.go`, na função `authorize`, atualizar o caso `uno.CancelGame`:
     ```go
     case uno.CancelGame:
         if actor.ChatID == 0 || (!actor.ChatAdmin && actor.PlayerID != entry.ownerID && actor.PlayerID != entry.creatorID) {
             return ErrForbidden
         }
     ```
   - Manter `uno.SetRules` restrito a `entry.ownerID` (apenas o dono ativo da sala no lobby configura regras).

2. **Camada Telegram (`internal/telegram`)**:
   - Em `handleCancelar`, ao obter o `summary := h.service.FindChatGame`:
     - Se `actorID == summary.OwnerID || actorID == summary.CreatorID`: ator autorizado diretamente (caminho rápido sem chamada de rede).
     - Caso contrário: invocar `lookupMembershipAPI(ctx, h.bot, int64(chatID), int64(actorID))`.
       - Se erro na chamada da API: avisar que não foi possível checar a permissão de administrador.
       - Se não for admin (`!membership.Admin`): responder *"⚠️ Apenas o responsável pela partida ou um administrador do grupo pode cancelá-la."*.
       - Se for admin (`membership.Admin`): definir `actor.ChatAdmin = true`.
     - Invocar `h.service.Apply(ctx, actor, summary.GameID, action)`.
     - Ao concluir com sucesso:
       - Se for admin externo (não criador/dono): responder *"🛑 <b>Partida cancelada por um administrador.</b>"*.
       - Se for criador/dono: responder *"🛑 <b>Partida cancelada pelo responsável.</b>"*.
   - Em `RenderHelp`, atualizar a descrição do comando `/cancelar`: *"Cancela a partida (criador ou administrador). O comando /kill é um alias."*.

3. **Testes**:
   - `internal/game/service_test.go`:
     - Validar que `Actor{PlayerID: nonOwnerNonCreator, ChatAdmin: true}` consegue cancelar com sucesso.
     - Validar que o criador original (`creatorID`), mesmo após transferir o `ownerID`, consegue cancelar com sucesso.
     - Validar que jogador não-criador, não-dono e não-admin recebe `ErrForbidden`.
   - `internal/telegram/commands_test.go`:
     - Testar cancelamento por administrador do grupo via `mockAPI.ChatMembers`.
     - Testar recusa para jogador comum do grupo.
     - Testar cancelamento pelo criador original da sala.

## Passos detalhados
1. Adicionar `CreatorID` ao `GameSummary` em `internal/game/views.go`.
2. Atualizar a checagem de `uno.CancelGame` em `authorize` dentro de `internal/game/service.go`.
3. Ajustar testes existentes e adicionar novos casos em `internal/game/service_test.go`.
4. Atualizar `handleCancelar` em `internal/telegram/commands.go` com verificação de `lookupMembershipAPI` e mensagens informativas.
5. Atualizar descrição de ajuda em `internal/telegram/renderer.go`.
6. Adicionar testes unitários de cancelamento por admin e criador em `internal/telegram/commands_test.go`.
7. Rodar `make check` e verificar que todas as suítes passam sem regressões.

## Riscos
- **Performance de rede**: Chamadas síncronas ao Telegram para checar permissão de administrador poderiam atrasar o comando.
  - *Mitigação*: Criadores e donos da sala são verificados localmente na memória (caminho rápido sem I/O). A consulta à API do Telegram só ocorre se o remetente não for o criador nem o dono atual, utilizando timeout estrito de 5 segundos.
- **Falha temporária da API do Telegram**: Se o Telegram estiver instável, a checagem de admin pode falhar.
  - *Mitigação*: Retorno de mensagem clara orientando que o criador/dono ainda pode cancelar normalmente.

## Impactos esperados
- Administradores do grupo passam a ter autoridade para cancelar jogos normais sem precisar recorrer ao `/reset`.
- O criador da sala não perde o poder de encerrar sua própria criação caso tenha saído temporariamente do jogo.
- Nenhuma alteração no gameplay, regras do Uno ou pontuações de ranking.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -v ./internal/game/...
go test -v ./internal/telegram/...
TEST_DATABASE_URL="postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable" make check
```

### Execução
Executar o bot e testar em um grupo:
1. Usuário A cria a partida via `/novo@bot`.
2. Usuário B (não admin) tenta `/cancelar@bot` -> deve ser rejeitado.
3. Usuário C (admin do grupo) tenta `/cancelar@bot` -> partida cancelada com sucesso por um administrador.
4. Repetir criando partida e testando cancelamento pelo criador da sala.

## Rollback
Desfazer as alterações nos arquivos `internal/game/views.go`, `internal/game/service.go`, `internal/telegram/commands.go`, `internal/telegram/renderer.go` e seus respectivos arquivos de teste via `git restore`.

## Observações
O comando `/kill` já é um alias direto de `/cancelar` no parser de comandos do Telegram (`case "cancelar", "kill":`), portanto passará a usufruir automaticamente das novas permissões sem necessidade de alterações extras.

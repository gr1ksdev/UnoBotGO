# Plano: recriar-lobby-perdido-com-novo

## Pedido do usuário
O usuário relatou que atualmente, quando há um jogo perdido/abandonado que não foi iniciado (lobby) e um usuário tenta criar um novo jogo com `/novo`, o bot impede a criação e solicita que use `/estado`. Isso é inconveniente, especialmente se algum jogador entrou e abandonou ou se o criador original não está mais disponível para cancelar. O usuário solicitou que, para partidas que ainda não iniciaram, ao enviar `/novo` novamente, o bot crie diretamente um novo lobby limpo em vez de enviar a mensagem solicitando `/estado`.

**Requisito explícito adicional:**
Se a partida já estiver rolando (já iniciada), essa substituição **NÃO PODE ACONTECER DE FORMA ALGUMA**. O bot deve informar expressamente que já existe uma partida iniciada/em andamento e preservar a partida intacta.

## Objetivo
Permitir que o comando `/novo` descarte com segurança um lobby anterior não iniciado (`Phase == uno.Lobby`) no mesmo grupo e crie um novo lobby limpo, invalidando tokens anteriores. Partidas que já iniciaram (`Phase != uno.Lobby`) continuam estritamente protegidas contra substituição, avisando ao jogador que a partida já foi iniciada e está em andamento.

## Contexto atual
- No `game.Manager` (`internal/game/manager.go`), `create` verifica se já existe uma sessão em `m.byChat[actor.ChatID]`. Se existir, retorna imediatamente `ErrChatOccupied`.
- Em `internal/telegram/commands.go` (`handleNovoObserved`), ao receber `ErrChatOccupied`, o bot responde:
  `⚠️ Já existe uma partida aberta ou em andamento neste grupo.\nUse /estado para ver ou /cancelar para encerrar.`
- Se o jogador que enviar `/novo` não for administrador do grupo nem criador/responsável da partida anterior, ele não tem autorização para usar `/cancelar` nem `/reset`, deixando o grupo travado no lobby antigo.

## Arquivos analisados
- `internal/game/views.go`
- `internal/game/service.go`
- `internal/game/manager.go`
- `internal/game/service_test.go`
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`

## Arquivos que poderão ser modificados
- `internal/game/service.go`
- `internal/game/manager.go`
- `internal/game/service_test.go`
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`

## Estratégia de implementação
1. **Camada de Domínio/Aplicação (`internal/game`):**
   - Adicionar o campo `ReplaceLobby bool` em `game.CreateRequest`.
   - Adicionar o campo `ReplacedGameID uno.GameID` em `game.Outcome`.
   - No gerenciador (`manager.create`), quando `req.RoomType == ranking.RoomGroup` e `m.byChat[actor.ChatID]` já existir:
     - Se `req.ReplaceLobby` for `false`, mantém o retorno de `ErrChatOccupied`.
     - Se `req.ReplaceLobby` for `true`:
       - Respeitar a ordem de lock estrita (`entry.mu` -> `m.indexMu`).
       - Obter a referência da partida atual e adquirir `entry.mu`.
       - Verificar o estado da partida com `entry.engine.Snapshot().Phase`:
         - Se a partida **já tiver iniciado** (`Phase != uno.Lobby`): liberar o lock de `entry.mu` imediatamente e retornar `ErrChatOccupied`. A partida em andamento **não é tocada**.
         - Se e somente se a partida **ainda for um lobby não iniciado** (`Phase == uno.Lobby`): adquirir `m.indexMu`, confirmar o binding do chat, marcar o lobby antigo como tombstone (`entry.reset = true`, `entry.engine = nil`), expurgar o vínculo dos jogadores em `m.byPlayer` e registrar a nova partida em `m.byChat` e `m.byID`.
       - Liberar os locks e retornar o novo `Outcome` com `ReplacedGameID = existingID`.
2. **Camada do Adaptador Telegram (`internal/telegram`):**
   - Em `handleNovoObserved`, configurar `ReplaceLobby: true` no `game.CreateRequest`.
   - Se `outcome.ReplacedGameID != ""`, chamar `h.tokens.InvalidateGame(outcome.ReplacedGameID)` para invalidar botões e tokens inline da partida descartada.
   - Caso `service.Create` retorne `errors.Is(err, game.ErrChatOccupied)` (o que agora ocorre estritamente quando a partida já foi iniciada), responder com a mensagem clara:
     `⚠️ Já existe uma partida em andamento neste grupo.\nUse /estado para ver ou /cancelar para encerrar.`
3. **Testes automatizados:**
   - Teste no `internal/game`:
     - Criar partida A (lobby) -> chamar `Create` com `ReplaceLobby: true` -> partida A é substituída pela nova partida B com lobby limpo.
     - Criar partida A, jogadores entram e partida é iniciada (`uno.StartGame`) -> chamar `Create` com `ReplaceLobby: true` -> **deve falhar obrigatoriamente** com `ErrChatOccupied` e partida A segue intacta.
   - Teste no `internal/telegram`:
     - Simular `/novo`, entrada de jogador, novo `/novo` por outro usuário -> novo lobby limpo é aberto e renderizado.
     - Simular `/novo`, entrar, dar `/iniciar`, e tentar `/novo` -> **não substitui** e envia mensagem avisando que a partida já está em andamento.

## Passos detalhados
1. Atualizar structs em `internal/game/service.go`: adicionar `ReplaceLobby` em `CreateRequest` e `ReplacedGameID` em `Outcome`.
2. Implementar substituição segura e validação estrita de fase em `internal/game/manager.go` (bloqueando categoricamente se `Phase != uno.Lobby`).
3. Atualizar `handleNovoObserved` em `internal/telegram/commands.go` para habilitar `ReplaceLobby: true`, invalidar tokens do jogo substituído e exibir aviso de partida em andamento quando `ErrChatOccupied`.
4. Escrever testes unitários em `internal/game/service_test.go` cobrindo substituição de lobby e recusa estrita de partida em andamento.
5. Escrever testes de comando em `internal/telegram/commands_test.go` verificando fluxo do `/novo` limpo e bloqueio quando partida em andamento.
6. Executar todos os testes (`go test ./...` e `go test -race ./...`).

## Riscos
- **Inversão de locks:** Mitigado seguindo a hierarquia `entry.mu -> indexMu`.
- **Substituição indevida:** Mitigado com verificação estrita de `Snapshot().Phase == uno.Lobby` sob o lock da partida. Partida iniciada nunca é substituída.

## Impactos esperados
- Lobbies abandonados não bloqueiam mais novos jogos.
- Partidas já iniciadas estão 100% blindadas contra qualquer tentativa de recriação acidental.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD.

## Como testar

### Build
```bash
go build ./cmd/bot
```

### Testes
```bash
go test -v ./internal/game/... ./internal/telegram/...
go test -race ./internal/...
```

### Execução
```bash
go run ./cmd/bot
```

## Rollback
Caso necessário reverter:
```bash
git checkout -- internal/game internal/telegram
```

## Observações
O comportamento é backward-compatible: chamadas a `Create` sem a flag `ReplaceLobby` mantêm exatamente o comportamento anterior.

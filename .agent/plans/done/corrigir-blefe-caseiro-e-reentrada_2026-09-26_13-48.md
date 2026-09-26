# Plano: corrigir blefe caseiro e reentrada

## Pedido do usuário
Corrigir dois edge cases encontrados durante a homologação real no Telegram do UnoBotGO V2 na branch `dev` (HEAD `226ac0193b88b0afc1e5b8e11162417c65b4145c`):
1. **Blefe em +4 jogado sobre +2 no Caseiro:** quando um +4 é jogado legalmente em resposta a um +2 pendente no modo Caseiro (`StackWildDrawFourOnTwo`), a jogada não está sob as restrições normais de cor do Wild Draw Four, sendo um counter/stack legal. Portanto, ela NÃO deve ser passível de desafio de blefe (não oferecer botão/sticker de blefe e rejeitar com segurança chamadas stale/forçadas com `ErrInvalidAction`). O +4 normal fora de stacking continua desafiável como antes, e o modo Classic permanece inalterado.
2. **Reentrada de jogador na mesma partida:**
   - Jogador que usou `/sair` sem ter colocado pode voltar via `/entrar` caso a sala esteja aberta, recebendo tratamento de late join idêntico ao de um novo participante (7 cartas novas, inserido na cauda da ordem lógica, sem restaurar mão antiga nem alterar jogador da vez, penalidade ou placements).
   - Se a sala estiver trancada (`RoomLocked`), o jogador que saiu recebe a mesma mensagem que qualquer outro jogador: `"🔒 Esta partida está trancada e não aceita novos jogadores."`.
   - Jogador que já terminou conquistando colocação (`WentOut` / presente em `Placements`) fica definitivamente impedido de reentrar na partida, mesmo com sala aberta, retornando erro estruturado (`ErrAlreadyFinished`) e mensagem amigável: `"🏁 Você já terminou esta partida e não pode entrar novamente."`.
   - Precedência de checagens no Join: Já colocado (`ErrAlreadyFinished`) -> Já participando ativo (`ErrAlreadyJoined`) -> Sala trancada (`ErrRoomLocked`) -> Admissão (novo join ou reentrada).
   - Invariante rigorosa: um mesmo `PlayerID` nunca pode aparecer duas vezes nos `Placements`.

## Objetivo
Garantir coerência das regras de blefe no modo Caseiro e do ciclo de vida de participantes nas partidas do UnoBotGO V2, assegurando integridade na engine (`internal/uno`), camada de aplicação (`internal/game`) e interface Telegram (`internal/telegram`), sem regressões nas regras clássicas ou no fluxo de jogo.

## Contexto atual
- **Branch:** `dev` em `226ac0193b88b0afc1e5b8e11162417c65b4145c`. Árvore de trabalho limpa.
- **Blefe atual:** em `internal/uno/game.go`, ao jogar `WildDrawFour`, a engine calcula `bluffing` verificando apenas se o jogador possui carta da cor anterior `s.ActiveColor`. Em `choose()`, `s.PendingBluff = &BluffInfo{...}` é instanciado incondicionalmente sempre que `pending.DrawCount != 0` e houver stacking. A engine perde a informação de que aquele +4 foi aceito como resposta ao +2.
- **Reentrada atual:** em `internal/uno/game.go`, `join()` rejeita qualquer jogador com `s.player(id) != nil` com `ErrAlreadyJoined`. Como `leave()` mantém o jogador no slice `s.Players` com status `Left`, o jogador nunca consegue reentrar. Além disso, jogadores com colocação recebiam o mesmo `ErrAlreadyJoined`, sem distinção entre "já participando", "já finalizado" ou "sala trancada". Em `internal/game/service.go`, a checagem de `entry.locked` ocorria antes de validar se o jogador já havia terminado ou já estava na partida.

## Arquivos analisados
- `internal/uno/game.go`
- `internal/uno/state.go`
- `internal/uno/errors.go`
- `internal/uno/rules.go`
- `internal/uno/action.go`
- `internal/uno/lifecycle_test.go`
- `internal/uno/game_test.go`
- `internal/game/service.go`
- `internal/game/views.go`
- `internal/game/service_test.go`
- `internal/game/room_lock_test.go`
- `internal/telegram/commands.go`
- `internal/telegram/inline.go`
- `internal/telegram/renderer.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/inline_test.go`
- `internal/telegram/renderer_test.go`
- `docs/project-status.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`

## Arquivos que poderão ser modificados
- `internal/uno/errors.go` (definição de `ErrAlreadyFinished`)
- `internal/uno/state.go` (campos de contexto do +4 e métodos utilitários `HasPlacement`, `Player`, invariant em `Validate`)
- `internal/uno/game.go` (lógica de blefe em stacking sobre +2, reentrada em `join`)
- `internal/uno/game_test.go` (testes de blefe normal vs caseiro stacked, rejeição de blefe indevido, integridade do classic)
- `internal/uno/lifecycle_test.go` (atualização e novos testes de reentrada de jogador após sair vs bloqueio de jogador colocado)
- `internal/game/views.go` (garantia de `CanCallBluff` considerando o contexto challengeable)
- `internal/game/service.go` (ordem de precedência no Join: colocado -> ativo -> trancado)
- `internal/game/service_test.go` (testes de precedência e autorização de reentrada e blefe)
- `internal/game/room_lock_test.go` (testes de interação entre reentrada e room lock)
- `internal/telegram/commands.go` (mapeamento de `ErrAlreadyFinished` para mensagem amigável no `/entrar`)
- `internal/telegram/commands_test.go` (testes de mensagens de reentrada e término)
- `docs/project-status.md`, `.agent/context.md`, `.agent/memory/memory.md`, `.agent/decisions.md`

## Estratégia de implementação

### 1. Blefe em +4 sobre +2 no Caseiro
- Em `internal/uno/state.go`:
  - Adicionar o campo booleano `DrawFourChallengeable` na struct `ColorChoice`.
  - Adicionar o campo booleano `DrawFourChallengeable` na struct `State` (para preservar o contexto caso snapshots sejam consultados).
- Em `internal/uno/game.go` (`play`):
  - Identificar se a jogada de `WildDrawFour` foi um counter sobre +2 pendente:
    `top, _ := s.card(s.DiscardPile[len(s.DiscardPile)-1])`
    `stackedOnDrawTwo := s.DrawCounter > 0 && top.Rank == DrawTwo && s.Rules.StackWildDrawFourOnTwo`
  - Se `stackedOnDrawTwo` for verdadeiro:
    `challengeable := false`
    `bluffing := false`
  - Se for falso (jogada normal de +4):
    `challengeable := true`
    `bluffing := <avalia se possui carta da cor anterior s.ActiveColor>`
  - Passar `DrawFourChallengeable: challengeable` para `s.Pending` (`ColorChoice`).
  - Em `choose()`:
    Instanciar `s.PendingBluff = &BluffInfo{...}` e `s.DrawFourChallengeable = true` APENAS se `pending.DrawFourChallengeable` for verdadeiro.
    Se for falso, `s.PendingBluff = nil` e `s.DrawFourChallengeable = false`.
  - Em `takeAction` (`case CallBluff`):
    Validar `if s.PendingBluff == nil || !s.DrawFourChallengeable || s.PendingBluff.Target != a.PlayerID || s.DrawCounter == 0 { return ErrInvalidAction }`.
    Limpar `s.DrawFourChallengeable = false` ao executar o blefe ou ao mudar de ação/jogada.
- Em `internal/game/views.go`:
  - `CanCallBluff` só é true se `state.PendingBluff != nil && state.DrawFourChallengeable && ...`.
  - Assim, o sticker/botão inline de blefe não será renderizado quando o +4 for resposta ao +2.
  - Qualquer callback/ação com token obsoleto será rejeitado pela engine com `ErrInvalidAction`, sem mutação de estado.

### 2. Reentrada na mesma partida & ciclo de vida
- Em `internal/uno/errors.go`:
  - Adicionar `ErrAlreadyFinished = errors.New("player already finished")`.
- Em `internal/uno/state.go`:
  - Adicionar método `HasPlacement(id PlayerID) bool`: retorna true se `id` estiver em `s.Placements`.
  - Adicionar método `Player(id PlayerID) *Player`: expõe lookup de jogador no snapshot.
  - Em `s.Validate()`, adicionar verificação explícita de invariante: nenhum `PlayerID` pode aparecer mais de uma vez em `s.Placements`.
- Em `internal/uno/game.go` (`join`):
  - 1. Verificar `if s.HasPlacement(id) { return ErrAlreadyFinished }`.
  - 2. Verificar `p := s.player(id); if p != nil && p.Status == Playing { return ErrAlreadyJoined }`.
  - 3. Se `p == nil && len(s.Players) >= 10 { return ErrPlayerLimit }`.
  - 4. Se `s.Phase != Lobby && !s.Rules.AllowLateJoin { return ErrLobbyClosed }`.
  - 5. Compra de cartas: se `s.Phase != Lobby`, comprar 7 novas cartas da pilha (`g.draw(s, 7)`).
  - 6. Atualização de dados do participante:
       - Se `p != nil` (jogador que havia usado `/sair`, com `Status == Left`):
         Reutilizar o registro existente em `s.Players` para evitar duplicatas de `PlayerID` no slice (satisfazendo `s.Validate()`):
         `p.Status = Playing`
         `p.Hand = newCards`
       - Se `p == nil`:
         Adicionar novo participante: `s.Players = append(s.Players, Player{ID: id, Status: Playing, Hand: newCards})`.
  - 7. Ordem de turnos:
       - No Lobby: `s.Order = append(s.Order, id)`.
       - Em partida ativa: inserir na cauda lógica da ordem (após todos os outros no sentido do jogo), preservando o algoritmo existente de late join.
       - Emitir eventos `PlayerJoined` e `CardsDrawn`.
- Em `internal/game/service.go` (`Apply`):
  - Para `action.Type == uno.JoinGame`:
    Garantir a precedência exata de erros:
    1. `if before.HasPlacement(actor.PlayerID) { return Outcome{}, uno.ErrAlreadyFinished }`
    2. `if p := before.Player(actor.PlayerID); p != nil && p.Status == uno.Playing { return Outcome{}, uno.ErrAlreadyJoined }`
    3. `if entry.locked { return Outcome{}, ErrRoomLocked }`
    4. Seguir para execução de `entry.engine.Apply(action)`.
- Em `internal/telegram/commands.go` (`handleEntrar`):
  - Tratar `errors.Is(err, uno.ErrAlreadyFinished)` retornando:
    `"🏁 Você já terminou esta partida e não pode entrar novamente."`.
  - Manter tratamento de `ErrAlreadyJoined`, `ErrRoomLocked`, etc.

## Passos detalhados

1. **Aprovação do plano:** Obter aprovação explícita do usuário antes de qualquer modificação no código.
2. **Mover plano para approved:** Transferir este plano para `.agent/plans/approved/`.
3. **Implementar alterações na Engine (`internal/uno`):**
   - Adicionar `ErrAlreadyFinished` em `internal/uno/errors.go`.
   - Adicionar `DrawFourChallengeable` em `ColorChoice` e `State` em `internal/uno/state.go`.
   - Adicionar `HasPlacement(id)` e `Player(id)` em `internal/uno/state.go`.
   - Adicionar validação de unicidade de placements em `s.Validate()`.
   - Implementar distinção de stacking +4 sobre +2 em `play()` e `choose()` em `internal/uno/game.go`.
   - Implementar reentrada sem placement e bloqueio de jogadores já finalizados em `join()` em `internal/uno/game.go`.
4. **Implementar alterações na camada de Aplicação (`internal/game`):**
   - Atualizar `CanCallBluff` em `internal/game/views.go` para verificar `DrawFourChallengeable`.
   - Ajustar precedência de checagens de `JoinGame` em `internal/game/service.go`.
5. **Implementar alterações na camada de Apresentação (`internal/telegram`):**
   - Em `internal/telegram/commands.go` (`handleEntrar`), tratar `ErrAlreadyFinished` com a mensagem especificada.
6. **Implementar testes automatizados:**
   - Testes de blefe (Test 1 a 5: normal com blefe, normal sem blefe, caseiro +2->+4 não desafiável, rejeição forçada de challenge, integridade do modo clássico).
   - Testes de reentrada (saída com /sair e retorno como late join, bloqueio de sala trancada para quem saiu, desbloqueio permitindo reentrada, bloqueio de jogador com colocação, preservação de turno/ordem/invariantes).
   - Testes de precedência de erro em Join (colocado vs ativo vs trancado).
   - Testes de comandos Telegram para mensagens de resposta.
7. **Verificação rigorosa de qualidade:**
   - Rodar `go test ./...`
   - Rodar `go test -tags debugcards ./...`
   - Rodar `go test -race ./...` (analisar restrição local de VMA/ARM64 se aplicável)
   - Rodar `go vet ./...` e `go vet -tags debugcards ./...`
   - Rodar `go build ./...` e `go build -tags debugcards ./...`
   - Rodar `git diff --check`
8. **Documentação e finalização:**
   - Atualizar `docs/project-status.md`, `.agent/context.md`, `.agent/memory/memory.md`, `.agent/decisions.md`.
   - Mover plano para `.agent/plans/done/`.
   - Realizar commit com a mensagem `fix(game): handle stacked draw four bluff and player reentry`.
   - Realizar push na branch `dev` (`origin/dev`).

## Riscos
- **Risco de mutação indevida de histórico de participantes:** Se um jogador que saiu reentrar, criar um segundo registro com o mesmo `PlayerID` em `s.Players` corromperia o invariante de integridade (`s.Validate()`).
  - *Mitigação:* Reutilizar o slot existente no slice `s.Players` com `Status = Playing` e nova mão atribuída.
- **Risco de efeito colateral em +4 normal ou no modo Classic:** Modificar a lógica de +4 pode acidentalmente desabilitar o blefe onde ele é legítimo.
  - *Mitigação:* Isolar a exceção estritamente para `s.DrawCounter > 0 && top.Rank == DrawTwo && s.Rules.StackWildDrawFourOnTwo`. Todos os outros caminhos de +4 mantêm a lógica idêntica e testes cobrirão ambos os fluxos.
- **Risco de stale callbacks:** Um cliente pode disparar o callback de blefe de uma jogada anterior.
  - *Mitigação:* `takeAction` rejeita com `ErrInvalidAction` na engine, não confiando apenas na omissão do botão na interface.

## Impactos esperados
- Jogadores em partidas do modo Caseiro não serão acusados injustamente de blefe ao contra-atacarem um +2 com +4.
- Jogadores que saíram de uma partida em andamento sem terem terminado poderão retornar como novos participantes tardios se a sala estiver aberta.
- Jogadores que já venceram/conquistaram colocação não poderão reentrar e receberão mensagem clara e direta.
- O modo Clássico, a ordem lógica de turnos e o bloqueio de sala permanecem intactos.

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
go build -tags debugcards ./...
```

### Testes
```bash
go test ./...
go test -race ./...
go vet ./...
go test -tags debugcards ./...
go vet -tags debugcards ./...
git diff --check
```

### Execução
Homologação manual no Telegram será conduzida pelo usuário após o push. O bot não será iniciado localmente nesta tarefa.

## Rollback
Desfazer as alterações ou reverter o commit correspondente na branch `dev` (`git reset --hard HEAD~1` ou `git revert`). Nenhuma alteração afetará a branch `main`.

## Observações
O repositório atual na branch `dev` é a única fonte de verdade. Nenhuma alteração será feita na branch `main` e nenhum merge ou promoção será realizado.

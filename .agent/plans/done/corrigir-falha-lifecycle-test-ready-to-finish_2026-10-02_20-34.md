# Plano: corrigir-falha-lifecycle-test-ready-to-finish

## Pedido do usuário
Investigar e corrigir a falha no CI do GitHub Actions no pacote `github.com/malbs/UnoGoBot/internal/telegram` ocorrida na branch `dev`. A instrução exige reproduzir exatamente a falha, localizar a assertion/teste real, diferenciar bug real de teste desatualizado, não inferir causas pelos logs normais de teste (ex.: "DB offline"), auditar `/cancelar` e `/kill`, e corrigir estritamente a causa raiz sem alterar regras de gameplay.

## Objetivo
Corrigir a condição de parada no helper de simulação `readyToFinish` em `internal/telegram/lifecycle_test.go` para que reconheça a carta `SwapHands` como carta terminal de jogada direta quando jogada com 1 carta restante na mão, evitando que o teste execute `svc.Apply` na carta vencedora e tente subsequentemente consultar `svc.PlayerView` com `Actor{PlayerID: 0}`, o que causa o erro `ErrInvalidArgument` ("invalid application argument").

## Contexto atual
- No commit recente `31e07d3` (`feat(telegram): permite cancelar e matar partidas por criador ou administradores`), as regras de `/cancelar` e `/kill` foram atualizadas e seus testes passaram integralmente (`TestCancelGameByAdminAndCreator` e `TestCancelGamePermissions`).
- A falha no CI ocorreu no passo `go test -tags debugcards ./...` (Job 111066515840, Run ID 37076185986).
- Reproduzido localmente com:
  `go test -count=100 -run 'TestFinalInlineActionAcrossTransports/webhook/caseiro' ./internal/telegram`
  Gerando:
  ```text
  --- FAIL: TestFinalInlineActionAcrossTransports (0.00s)
      --- FAIL: TestFinalInlineActionAcrossTransports/webhook/caseiro (0.00s)
          lifecycle_test.go:145: invalid application argument
  ```
- **Causa Raiz**:
  - Em `internal/telegram/lifecycle_test.go`, a função helper `readyToFinish` simula rodadas do jogo até que falte 1 ação para encerrar a partida, a fim de retornar `(view, action)` para que o teste teste essa ação final via inline token (`ChosenInlineResult`).
  - Na linha 83 de `lifecycle_test.go`:
    ```go
    if len(pv.Hand) == 1 && (cv.Card.Rank < uno.Wild || !resolveWild) {
        return view, action
    }
    ```
  - Em regras `classic`, as cartas vão de 0 a 14 (`Wild` = 13, `WildDrawFour` = 14). Como `NoWildFinish` é verdadeiro, cartas Wild não são jogáveis com 1 carta na mão (`len(pv.Hand) == 1`), restando apenas cartas com `Rank < uno.Wild`. Logo, `readyToFinish` sempre retornava no `classic`.
  - Em regras `caseiro` (`uno.CaseiroRules()`), existe a carta `SwapHands` (`Rank = 15`). O motor do UNO (`internal/uno/game.go:450`) permite jogar `SwapHands` como última carta mesmo com `NoWildFinish == true` (`card.Rank >= Wild && card.Rank != SwapHands`).
  - No entanto, `cv.Card.Rank < uno.Wild` avaliava como `false` para `SwapHands` (15 < 13 é falso), e com `resolveWild == true`, `readyToFinish` não retornava a ação.
  - Em vez disso, `readyToFinish` chamava `svc.Apply(PlayCard, SwapHands)`.
  - No motor de jogo (`internal/uno/game.go:518`), quando `SwapHands` é jogada e `len(p.Hand) == 0`, a troca de mãos e escolha de cor não acontecem: a partida é imediatamente encerrada via `completePlay` (`s.Phase = Finished`, `s.CurrentPlayerID = 0`).
  - Na iteração seguinte do loop em `readyToFinish`, como a partida já havia sido finalizada na iteração anterior, `view.CurrentTurn` era 0. A chamada `svc.PlayerView(..., game.Actor{PlayerID: 0}, ...)` retornava `ErrInvalidArgument` ("invalid application argument"), falhando o teste na linha 98 (reportada como linha 145 pelo `t.Helper()`).

## Arquivos analisados
- `internal/telegram/lifecycle_test.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/commands.go`
- `internal/game/service.go`
- `internal/game/service_test.go`
- `internal/game/manager.go`
- `internal/game/errors.go`
- `internal/uno/game.go`
- `internal/uno/rules.go`
- `internal/uno/card.go`
- `internal/uno/swap_test.go`
- `.github/workflows/dev-ci.yml`

## Arquivos que poderão ser modificados
- `internal/telegram/lifecycle_test.go`

## Estratégia de implementação
1. Ajustar a verificação em `readyToFinish` na linha 83 de `internal/telegram/lifecycle_test.go`:
   Substituir a checagem:
   ```go
   if len(pv.Hand) == 1 && (cv.Card.Rank < uno.Wild || !resolveWild) {
       return view, action
   }
   ```
   por:
   ```go
   if len(pv.Hand) == 1 && (cv.Card.Rank < uno.Wild || cv.Card.Rank == uno.SwapHands || !resolveWild) {
       return view, action
   }
   ```
   (ou alternativamente `cv.Card.Rank != uno.Wild && cv.Card.Rank != uno.WildDrawFour || !resolveWild`).
   Dessa forma, cartas de ação direta que encerram o jogo (`Rank < uno.Wild` ou `SwapHands`) retornam imediatamente quando resta 1 carta na mão para serem usadas como ação final pelo teste.
2. Adicionar uma salvaguarda defensiva após `svc.Apply` em `readyToFinish`:
   ```go
   if view.Closed || view.Phase == uno.Finished {
       t.Fatalf("game closed unexpectedly during simulation at step %d", step)
   }
   ```
   para que, caso alguma jogada imprevista encerre o jogo dentro da simulação, o teste falhe com mensagem descritiva imediata em vez de causar erro indireto de argumento inválido no início da próxima iteração.

## Passos detalhados
1. Obter aprovação explícita do usuário.
2. Mover o plano de `.agent/plans/pending/` para `.agent/plans/approved/`.
3. Editar `internal/telegram/lifecycle_test.go` para atualizar a condição em `readyToFinish` e adicionar o guard de encerramento inesperado.
4. Executar os testes repetidos de estresse:
   - `go test -count=100 -run 'TestFinalInlineActionAcrossTransports/webhook/caseiro' ./internal/telegram`
   - `go test -count=100 -run 'TestFinalInlineActionAcrossTransports/polling/caseiro' ./internal/telegram`
   - `go test -tags debugcards -count=100 -run '^TestFinalInlineActionAcrossTransports$' ./internal/telegram`
5. Executar a suíte completa de validação requerida:
   - `go test -count=1 -v ./internal/telegram`
   - `go test -race -count=1 -v ./internal/telegram`
   - `go test ./...`
   - `go test -race ./...`
   - `go test -tags debugcards ./...`
   - `go vet ./...`
   - `make check`
   - `git diff --check`
6. Atualizar memória persistente em `.agent/memory/memory.md` e registrar em `.agent/decisions.md`.
7. Mover o plano para `.agent/plans/done/`.
8. Apresentar o relatório final detalhado com todos os 16 itens requisitados.

## Riscos
- Risco zero de regressão em produção, pois a alteração é restrita exclusivamente ao helper de teste `readyToFinish` em `lifecycle_test.go`. O código de produção (`internal/telegram`, `internal/game`, `internal/uno`) permanece 100% inalterado.

## Impactos esperados
- Eliminação do flake intermitente em `TestFinalInlineActionAcrossTransports` com a regra `caseiro` no CI.
- Todos os passos do workflow `dev-ci` passarão com sucesso no GitHub Actions.

## Compatibilidade
- Linux: Sim
- macOS: Sim
- Windows: Sim
- Docker: Sim
- CI/CD: Sim

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -count=100 -run 'TestFinalInlineActionAcrossTransports/webhook/caseiro' ./internal/telegram
go test -race -count=1 -v ./internal/telegram
go test ./...
go test -race ./...
go test -tags debugcards ./...
go vet ./...
make check
git diff --check
```

### Execução
N/A (correção em suíte de testes de automação).

## Rollback
Restaurar `internal/telegram/lifecycle_test.go` com `git checkout internal/telegram/lifecycle_test.go`.

## Observações
- A hipótese inicial de testes desatualizados de autorização de `/cancelar` e `/kill` foi investigada a fundo: os testes de `/cancelar` e `/kill` adicionados em `31e07d3` já cobrem explicitamente criador, dono, administrador e membro comum, e todos passam sem erros.
- A causa raiz real da falha do CI é um edge case no gerador de estado do teste `readyToFinish` com a regra `caseiro` ao lidar com a carta `SwapHands` como última carta.

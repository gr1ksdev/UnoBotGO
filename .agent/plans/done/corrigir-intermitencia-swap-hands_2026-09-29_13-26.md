# Plano: corrigir-intermitencia-swap-hands

## Pedido do usuário
Investigar e corrigir a falha intermitente observada no CI do GitHub Actions em `TestKeepHandInlineFlowStaleColorAndMultigroup` (`optional_swap_test.go:117: not enough drawable cards`), tornando o teste determinístico sem alterar regras ou UX de Swap Hands e sem mexer na main.

## Objetivo
Tornar o helper de teste `readyToSwap` determinístico e resiliente a variações do embaralhamento aleatório (`rand.Shuffle`), garantindo que qualquer partida gerada pelo helper preserve cartas suficientes na pilha de compras (`DrawPile >= 14`) para suportar a entrada tardia (`uno.JoinGame`) de novos jogadores nos testes subsequentes, eliminando a intermitência de `ErrDeckEmpty`.

## Contexto atual
- O helper `readyToSwap` (em `internal/telegram/swap_test.go`) simula compras e passes sucessivos até encontrar a carta `SwapHands` na mão do jogador ativo.
- Como nenhuma carta é descartada na mesa, todas as cartas compradas ficam retidas nas mãos dos jogadores.
- Em ~10% das execuções, o shuffle aleatório posiciona a única carta `SwapHands` nas últimas 6 posições da pilha de compras (posições 88 a 93 de 94 cartas).
- Quando isso ocorre, restam menos de 7 cartas no monte de compras.
- Logo em seguida, os testes `TestKeepHandInlineFlowStaleColorAndMultigroup` e `TestSwapInlineTargetDepartureInvalidatesColor` (em `optional_swap_test.go`) executam `uno.JoinGame` para um terceiro jogador, que pelas regras do jogo precisa comprar 7 cartas.
- Como não há 7 cartas disponíveis, a engine retorna `ErrDeckEmpty` ("not enough drawable cards") e o teste falha intermitentemente.

## Arquivos analisados
- `internal/telegram/swap_test.go`
- `internal/telegram/optional_swap_test.go`
- `internal/uno/game.go`
- `internal/uno/deck.go`
- `internal/uno/errors.go`
- `internal/game/service.go`

## Arquivos que poderão ser modificados
- `internal/telegram/swap_test.go`
- `.agent/memory/memory.md`

## Estratégia de implementação
1. No helper de teste `readyToSwap` em `internal/telegram/swap_test.go`:
   - Adicionar controle de tentativas com ChatIDs isolados (`ChatID(-901 - int64(attempt)*10)`).
   - Rastrear a contagem de compras realizadas (`cardsDrawn`).
   - Se a carta `SwapHands` for encontrada com `cardsDrawn <= 80` (garantindo pelo menos 14 cartas no `DrawPile`), retornar imediatamente a partida válida.
   - Caso `cardsDrawn > 80`, descartar a tentativa e iniciar nova partida com baralho reembaralhado. A chance de sucesso na primeira tentativa é ~91%, na segunda ~99%, e em 5 tentativas é estatisticamente quase 100%.
2. Executar baterias repetidas (`-count=50` e `-count=100`) para validar que nenhuma falha ocorra mais em `TestKeepHandInlineFlowStaleColorAndMultigroup` e `TestSwapInlineTargetDepartureInvalidatesColor`.
3. Executar a suíte completa de testes (`go test -count=1 ./...`, `go test -count=1 -race ./...`, `go vet ./...`, `go build ./...`).
4. Atualizar a memória do agente (`.agent/memory/memory.md`).

## Passos detalhados
1. Editar `internal/telegram/swap_test.go`: refatorar `readyToSwap` para incluir o teto de 80 compras e tentativas com ChatIDs isolados.
2. Validar reprodução com `go test -count=50 -run '^TestKeepHandInlineFlowStaleColorAndMultigroup$' ./internal/telegram` e `go test -count=50 -run '^TestSwapInlineTargetDepartureInvalidatesColor$' ./internal/telegram`.
3. Validar suíte completa com e sem race detector.
4. Mover o plano para `.agent/plans/approved/` após aprovação, e posteriormente para `.agent/plans/done/`.
5. Registrar na memória persistente.

## Riscos
- **Risco**: Interferir com outros testes de telegram.
  - **Mitigação**: Os outros testes que usam `readyToSwap` passam a receber uma partida com baralho garantidamente saudável, o que apenas aumenta sua estabilidade. `ChatID` continua na faixa negativa de teste.
- **Risco**: Alterar código de produção.
  - **Mitigação**: Nenhuma linha fora de `internal/telegram/swap_test.go` será alterada. A engine, serviço e handlers permanecem 100% intocados.

## Impactos esperados
- Eliminação completa da intermitência `"not enough drawable cards"` no CI do GitHub Actions.
- Suite de testes determinística e estável.
- Zero impacto no código de produção ou na UX do jogo.

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
go test -count=50 -run '^TestKeepHandInlineFlowStaleColorAndMultigroup$' ./internal/telegram
go test -count=50 -run '^TestSwapInlineTargetDepartureInvalidatesColor$' ./internal/telegram
go test -count=1 -race ./...
```

### Execução
```bash
git diff --check
```

## Rollback
Restaurar `internal/telegram/swap_test.go` para o estado anterior via `git checkout internal/telegram/swap_test.go`.

## Observações
Nenhum arquivo de produção será modificado. A branch `main` não será tocada.

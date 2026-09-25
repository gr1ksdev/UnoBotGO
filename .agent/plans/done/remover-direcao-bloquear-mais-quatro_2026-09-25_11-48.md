# Plano: remover-direcao-bloquear-mais-quatro

## Pedido do usuário
Remover a linha redundante `Direção: ➡️ Sentido horário` do estado da partida, pois a direção já aparece entre os jogadores, e impedir que no modo Caseiro um jogador responda a um `+4` com outro `+4`.

## Objetivo
Simplificar a mensagem pública do Telegram e ajustar somente a regra de empilhamento `+4 → +4` do modo Caseiro, preservando as respostas cruzadas Caseiras já aprovadas (`+2 → +4` e `+4 → +2`) e o comportamento atual do modo Clássico.

## Contexto atual
- `Renderer.RenderPublicState` imprime a direção duas vezes: em uma linha textual própria e, depois, como separador `➡️` ou `⬅️` na lista ordenada de jogadores.
- A remoção pode ficar limitada à linha textual; a ordem, as setas entre jogadores e a inversão continuam visíveis.
- `CaseiroRules()` herda `StackWildDrawFour: true` de `BotRules()`. Por isso, durante uma penalidade cujo topo é `WildDrawFour`, `playable` aceita outro `WildDrawFour`.
- A engine já possui a separação necessária por flags. Definir `StackWildDrawFour = false` somente em `CaseiroRules()` bloqueia `+4 → +4` sem alterar `BotRules()`.
- `StackWildDrawFourOnTwo` e `StackDrawTwoOnWildFour` são flags independentes. Elas continuarão permitindo no Caseiro `+2 → +4 = 6` e um `+2` da cor escolhida como resposta a `+4`.
- O workspace contém as alterações aprovadas e ainda não commitadas do sticker cinza de Trocar cartas. Esta entrega não deve sobrescrevê-las nem misturar sua lógica com a correção atual.

## Arquivos analisados
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`
- `internal/uno/rules.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/uno/lifecycle_test.go`
- `docs/v2-rules.md`
- `docs/v2-telegram.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- `.agent/plans/done/restaurar-regras-v1_2026-09-23_19-55.md`
- `.agent/plans/done/corrigir-empilhamento-cruzado_2026-09-24_22-41.md`

## Arquivos que poderão ser modificados
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`
- `internal/uno/rules.go`
- `internal/uno/game_test.go`
- `docs/v2-rules.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- este plano, movido entre `pending`, `approved` e `done`

## Estratégia de implementação
Remover de `RenderPublicState` apenas o bloco que monta e imprime `Direção: ...`. A lista **Jogadores em jogo** continuará usando `➡️` para direção positiva e `⬅️` para direção invertida, mantendo a informação visível sem duplicação.

Em `CaseiroRules`, sobrescrever `StackWildDrawFour` para `false` após herdar as regras comuns de `BotRules`. A validação existente em `playable` passará a recusar naturalmente um `WildDrawFour` quando a carta no topo e a penalidade pendente vierem de outro `WildDrawFour`. Nenhuma condição nova será adicionada à engine.

Adicionar regressão específica do Caseiro que execute o primeiro `+4`, escolha uma cor e confirme: outro `+4` é recusado; um `+2` da cor escolhida continua disponível; a penalidade permanece quatro até uma resposta válida ou compra. O teste existente de `BotRules` continuará garantindo que o modo Clássico não mudou.

## Passos detalhados

1. Após aprovação, mover este plano para `approved`.
2. Remover a linha textual de direção de `RenderPublicState`, preservando as setas na sequência dos jogadores.
3. Atualizar os testes do renderer para exigir ausência de `Direção:`/`Sentido horário` e presença das setas conforme os dois sentidos.
4. Definir `CaseiroRules().StackWildDrawFour = false`.
5. Adicionar teste da sequência Caseira `+4 → +4`, exigindo `ErrCardNotPlayable`, e preservar cobertura de `+4 → +2` e `+2 → +4`.
6. Confirmar pelo teste existente que `BotRules()` continua aceitando `+4 → +4`.
7. Atualizar documentação, contexto, memória e decisão técnica.
8. Executar gofmt, testes focados repetidos, suíte completa, vet, build e `git diff --check`.
9. Mover o plano concluído para `done`.

## Riscos
- **Bloquear também `+2 → +4`:** evitar alteração em `StackWildDrawFourOnTwo` e cobrir a cadeia existente nos testes.
- **Bloquear também `+4 → +2`:** manter `StackDrawTwoOnWildFour` e testar uma carta `+2` com a cor escolhida.
- **Alterar o modo Clássico sem intenção:** modificar somente o retorno de `CaseiroRules`; o teste de `BotRules` continuará exigindo `+4 → +4 = 8`.
- **Ocultar a direção completamente:** preservar os separadores direcionais da lista de jogadores e testar ambos os sentidos.
- **Conflitar com o sticker cinza não commitado:** limitar os patches aos arquivos indicados e revisar o diff agregado sem descartar mudanças anteriores.

## Impactos esperados
- O estado da partida deixará de mostrar a linha `Direção: ...`.
- A lista de jogadores continuará indicando o sentido por `➡️` ou `⬅️`.
- No Caseiro, outro `+4` ficará cinza/indisponível diante de uma penalidade `+4`.
- No Caseiro, `+2 → +4` e `+4 → +2` continuarão válidos.
- O modo Clássico continuará com o comportamento atual de `+4 → +4`.

## Compatibilidade
- Linux: nenhuma dependência nova.
- macOS: nenhuma dependência nova.
- Windows: nenhuma dependência nova.
- Docker: nenhuma mudança de imagem ou configuração.
- CI/CD: os comandos atuais continuam aplicáveis; nenhum workflow será alterado.

## Como testar

### Build
```bash
go build ./...
go vet ./...
```

### Testes
```bash
go test ./internal/uno -run 'TestCaseiro.*WildDrawFour|TestCaseiroPenaltyResponses|TestBotRulesStackWildDrawFour' -count=20
go test ./internal/telegram -run 'TestRenderer' -count=20
go test ./...
git diff --check
```

### Execução
```bash
go run ./cmd/bot
```

Homologação manual: abrir o estado de uma partida normal e invertida para conferir somente as setas entre jogadores; no Caseiro, jogar um `+4`, escolher a cor e confirmar que outro `+4` aparece indisponível enquanto um `+2` da cor escolhida ainda pode responder.

## Rollback
Restaurar a linha textual no renderer e remover somente a sobrescrita `StackWildDrawFour = false` do Caseiro, acompanhadas dos testes e registros desta entrega. Não usar reset destrutivo nem descartar as alterações anteriores do sticker.

## Observações
- O pedido foi interpretado como uma mudança exclusiva do modo Caseiro, pois esse foi o modo citado. O modo Clássico permanece inalterado.
- A composição e a frequência das cartas não serão modificadas.
- Plano aprovado pelo usuário com `sim` em 2026-09-25.

## Resultado da implementação

- Removida de `RenderPublicState` a linha textual `Direção: ...`; os separadores `➡️` e `⬅️` continuam exibindo o sentido na lista de jogadores.
- `CaseiroRules()` agora define `StackWildDrawFour = false`, bloqueando somente `+4 → +4` nesse modo.
- As flags cruzadas foram preservadas: `+2 → +4` continua acumulando seis e um `+2` da cor escolhida continua respondendo ao `+4`.
- `BotRules()` permaneceu inalterado e seu teste continua exigindo `+4 → +4 = 8`.
- A regressão Caseira confirma que a tentativa recusada não altera revisão, turno ou contador antes da resposta válida.
- Documentação, contexto, memória e decisão técnica foram atualizados sem descartar o trabalho pendente do sticker cinza.

### Validação executada

```text
go test ./internal/uno -run 'TestCaseiroRejectsWildDrawFourOnWildDrawFour|TestCaseiroPenaltyResponses|TestBotRulesStackWildDrawFour' -count=20  PASS
go test ./internal/telegram -run 'TestRenderer' -count=20                                                                                 PASS
go test ./...                                                                                                                             PASS
go vet ./...                                                                                                                              PASS
go build ./...                                                                                                                            PASS
gofmt + git diff --check                                                                                                                  PASS
```

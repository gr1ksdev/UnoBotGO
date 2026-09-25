# Plano: corrigir-empilhamento-cruzado

## Pedido do usuário
Confirmar e corrigir o comportamento do modo Caseiro em que um `+4` jogado como resposta a um `+2` resulta em penalidade `4`, quando deveria acumular `2 + 4 = 6`. Verificar também se o modo Caseiro possui menos cartas especiais do que o modo Clássico.

## Objetivo
Preservar toda penalidade já acumulada ao resolver a escolha de cor de um `+4`, garantindo os cruzamentos do modo Caseiro, e documentar/testar que Clássico e Caseiro usam exatamente o mesmo baralho Classic de 108 cartas.

## Contexto atual
- A falha `+2 → +4` foi confirmada em `internal/uno/game.go`.
- Ao jogar o `+4`, `DrawCounter` continua em `2` durante `ChoosingColor`, mas `choose` substitui o valor por `pending.DrawCount` (`4`) quando a carta anterior é `+2`.
- O teste `TestCaseiroPenaltyResponses` atualmente formaliza o resultado incorreto, esperando `DrawCounter == 4` depois da escolha de cor.
- O modo Caseiro permite corretamente o descarte cruzado por `StackWildDrawFourOnTwo` e `StackDrawTwoOnWildFour`; o defeito está somente na acumulação após escolher a cor.
- A suspeita sobre quantidade de cartas especiais não foi confirmada. `NewGame` sempre usa `ClassicDeck()`, independentemente de `BotRules()` ou `CaseiroRules()`.
- Ambos os modos usam 108 cartas: oito `+2`, quatro Coringas, quatro Coringas `+4`, oito Skips e oito Reverses. O modo altera somente as regras de empilhamento, não o inventário nem a distribuição.
- Diferenças percebidas em uma partida são consequência do embaralhamento aleatório e da amostra inicial. Com dois jogadores, apenas 14 das 108 cartas são distribuídas no início.
- `docs/v2-rules.md` preserva trechos históricos desatualizados sobre as flags atuais e deve receber uma seção clara sobre os modos vigentes.

## Arquivos analisados
- `internal/uno/card.go`
- `internal/uno/deck.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/uno/rules.go`
- `internal/uno/state.go`
- `internal/game/service.go`
- `internal/telegram/callbacks.go`
- `internal/telegram/commands.go`
- `internal/simulation/config.go`
- `internal/simulation/runner.go`
- `docs/v2-rules.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`

## Arquivos que poderão ser modificados
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `docs/v2-rules.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- este plano, movido entre `pending`, `approved` e `done`

## Estratégia de implementação
Na resolução do `+4`, somar `pending.DrawCount` ao `DrawCounter` existente sempre que a regra de empilhamento estiver ativa. Uma jogada inicial continua produzindo `0 + 4 = 4`; `+4 → +4` continua produzindo `8`; e `+2 → +4` passa a produzir `6`. A validação de quais cartas podem responder permanece em `playable`, portanto a mudança não libera combinações novas no modo Clássico.

O teste Caseiro será ampliado para executar a cadeia completa `+2 → +4 → +2`, esperando contadores `2`, `6` e `8`, seguida da compra das oito cartas. Testes existentes continuarão cobrindo `+2 → +2`, `+4 → +4`, blefe, escolha de cor e atomicidade.

Não será aumentada nem reduzida a quantidade de cartas especiais. Um teste de contrato verificará que jogos criados com ambos os modos recebem o mesmo inventário Classic, e a documentação registrará a composição para evitar que variações aleatórias sejam confundidas com uma regra de modo.

## Passos detalhados

1. Mover o plano para `approved` após autorização.
2. Simplificar a acumulação do `+4` em `choose` para preservar o contador existente.
3. Atualizar `TestCaseiroPenaltyResponses` para exigir `6` após `+2 → +4`, `8` após a resposta com `+2` e compra integral da penalidade.
4. Adicionar teste de contrato garantindo inventário idêntico nos modos Clássico e Caseiro.
5. Atualizar `docs/v2-rules.md` com as diferenças atuais dos modos e a composição comum do baralho.
6. Atualizar memória e decisão técnica.
7. Executar gofmt, testes focados, suíte completa, vet, build e `git diff --check`.
8. Mover o plano concluído para `done`.

## Riscos
- **Somar duas vezes um `+4`:** mitigado por testes de `+4` inicial e `+4 → +4`; `Pending.DrawCount` é aplicado uma única vez na escolha de cor.
- **Liberar cruzamento no modo Clássico:** não haverá alteração em `playable` nem nas flags; o Clássico continuará recusando `+4` sobre `+2`.
- **Interação com blefe:** o desafio usa `DrawCounter`; após a correção ele considerará a penalidade total acumulada, que é o comportamento coerente com a pilha.
- **Mudar o baralho sem intenção:** nenhuma alteração será feita em `ClassicDeck`; apenas contrato e documentação serão adicionados.
- **Efeito terminal:** regras existentes para jogador que fica sem cartas, escolha de cor e encerramento serão preservadas e verificadas pela suíte.

## Impactos esperados
- No Caseiro: `+2 → +4` resulta em `6` cartas pendentes.
- No Caseiro: `+2 → +4 → +2` resulta em `8` cartas pendentes.
- No Clássico: as combinações permitidas permanecem iguais.
- Ambos os modos continuam usando exatamente o mesmo baralho de 108 cartas.
- Relatórios do simulador passam a mostrar o total correto porque leem o `DrawCounter` da engine.

## Compatibilidade
- Linux: sem dependências novas.
- macOS: sem dependências novas.
- Windows: sem dependências novas.
- Docker: nenhuma alteração de imagem ou configuração.
- CI/CD: coberto pelos comandos existentes; nenhuma alteração de workflow.

## Como testar

### Build
```bash
go build ./...
go vet ./...
```

### Testes
```bash
go test ./internal/uno -run 'TestCaseiroPenaltyResponses|TestModesUseSameClassicDeck' -count=1 -v
go test ./...
git diff --check
```

### Execução
```bash
go run ./cmd/simulator --players 3 --mode caseiro --seed 20260924
```

Para homologação manual no Telegram, montar a sequência `+2 → +4`, escolher a cor e confirmar que o botão de compra do próximo jogador mostra seis cartas.

## Rollback
Reverter somente a soma do contador, os testes e a documentação desta entrega com um novo commit. Não usar `git reset --hard`, force push ou limpeza ampla. Como não há migração ou persistência, o rollback não exige conversão de dados.

## Observações
- Esta análise confirmou o erro de soma e descartou diferença estrutural no número de cartas especiais entre os modos.
- Caso o objetivo futuro seja um baralho Caseiro com mais cartas especiais, isso será uma nova regra de produto e deverá definir quantidades, limite total e impacto com 2–10 jogadores; não faz parte desta correção.
- Aprovado pelo usuário em 2026-09-24 com escopo reduzido: manter modos e baralho como estão e implementar somente a correção da penalidade.
- Por essa restrição, não será adicionado teste novo de composição por modo nem alterada a documentação dos modos; os testes existentes de `ClassicDeck` permanecem como garantia do inventário.

## Resultado da implementação

- Corrigida a resolução do `+4` para somar quatro ao `DrawCounter` existente, sem depender do rank anterior no descarte.
- `+2 → +4` agora resulta em 6 cartas pendentes.
- O teste executa também `+2 → +4 → +2`, confirma 8 cartas pendentes e verifica a compra integral.
- Modos, composição do baralho, seleção Telegram e simulador não foram alterados.

### Validação executada

```text
go test ./internal/uno -run 'TestCaseiroPenaltyResponses|TestBotRulesStackWildDrawFour|TestStackDrawTwoRules' -count=20 PASS
go test ./...                                                                            PASS
go vet ./...                                                                             PASS
go build ./...                                                                           PASS
gofmt + git diff --check                                                                 PASS
```

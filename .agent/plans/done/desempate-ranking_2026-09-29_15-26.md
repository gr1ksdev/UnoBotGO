# Plano: desempate sequencial do ranking

Status: implementação concluída; usuário autorizou posteriormente commit e push para dev. Homologação não confirmada explicitamente.

## Autorização posterior
Pedido explícito: “faça o commit e o push pra dev”. Autoriza registrar a correção e enviar dev, incluindo o commit local anterior 64bb97d. Restrições de commit/push abaixo documentam a etapa anterior à autorização. Main, deploy e publicação de container continuam proibidos.

## Pedido do usuário
Substituir empate competitivo por posições únicas; implementar somente no working tree da dev. Pedido e “continue” autorizam implementação. Commit/push/amend/rebase expressamente proibidos.

## Objetivo
Ordenar por score DESC, última colocação elegível ASC, conclusão DESC, UserID ASC.

## Contexto atual
HEAD 64bb97d, dev limpa inicialmente. Renderer compartilha posições por score. completed_games possui finished_at/scoring_status/chat_id; completed_game_players possui position/final_status/went_out e PK(game_id,user_id). Stats mantêm score cumulativo e last_finished_at. Índice de ranking por chat/score/user; não existe índice de histórico por chat/data. Dados suficientes sem migration.

## Arquivos analisados
- internal/storage/postgres/ranking.go, results.go, migrations/0002_results.up.sql
- internal/ranking/service.go, eligibility.go
- internal/telegram/ranking.go, ranking_test.go
- internal/storage/postgres/ranking_integration_test.go
- .agent/context.md e documentação pública de ranking

## Arquivos que poderão ser modificados
- Consulta ranking PostgreSQL, contrato comentado, renderer e testes relacionados
- README.md, docs/m7-persistence.md, docs/v2-telegram.md
- .agent/context.md, memory/memory.md, decisions.md e plano

## Estratégia de implementação
CTE DISTINCT ON por jogador seleciona resultado elegível mais recente do ChatID. JOIN único com stats preserva fonte de score; ordenação completa antes do LIMIT. Critério game_id estabiliza seleção de históricos com mesmo timestamp. Sem histórico correspondente: NULLS LAST, sem inventar colocação. Renderer usa índice+1. Não alterar gravação ou schema.

## Passos detalhados
1. Registrar aprovação e mover plano para approved.
2. Implementar seleção da última participação e ordenação SQL, numeração sequencial.
3. Atualizar expectativas antigas e testar desempates, histórico individual, abandono, idempotência e falha de commit em PostgreSQL local real.
4. Auditar caminho do nome sem modificar dados; atualizar documentação/memória.
5. Executar gates e registrar limitações; concluir plano mantendo mudanças unstaged no working tree.

## Riscos
- Não considerar abandono/N<2/pending como último resultado elegível.
- Histórico volumoso exige varredura do histórico elegível: avaliar plano SQL sem criar índice por conveniência.
- Preservar ordenação também no SELECT externo e aplicar LIMIT só após desempate.

## Impactos esperados
- Posições únicas; comando e automático continuam compartilhando consulta/renderer.
- Sem alteração de fórmula, stats, transação, UTF-16 ou layout.

## Compatibilidade
Linux/macOS/Windows: Go existente. Docker/CI: sem mudanças de distribuição. PostgreSQL com DISTINCT ON já suportado.

## Como testar
### Build
```bash
go build ./...
```
### Testes
```bash
go test -count=1 ./...
go test -tags debugcards ./...
go vet ./...
git diff --check
go test -count=1 -tags integration ./internal/storage/postgres/...
CGO_ENABLED=1 go test -race ./...
```
### Execução
Homologação manual do usuário: scores empatados, últimas colocações distintas e /ranking após encerramento.

## Rollback
Reverter somente os hunks desta tarefa; preservar commit 64bb97d e alterações do usuário. Sem rollback de banco.

## Observações
Não consultar produção. Não confirmar valor real de display_name sem acesso à base de homologação. main inicial 6eea6c1399d287b58116d5be16a1807238653397. Nenhum commit/push/publicação/deploy/migration de produção.

## Resultado

- Duplicidade vinha do renderer reutilizando posição em scores iguais, conforme decisão anterior. Agora enumera índice+1; query aplica todos os desempates antes do LIMIT e repete ORDER BY externo.
- Consulta única, CTE DISTINCT ON por jogador, histórico scored/elegível, JOIN com stats. Dados de score inalterados. Última colocação vem de position e conclusão de finished_at. Sem schema novo. Ausência defensiva de histórico usa NULLS LAST; GameID DESC desempata seleção de jogos simultâneos de um mesmo jogador.
- Índices auditados: PK completed_games(game_id), PK completed_game_players(game_id,user_id), UNIQUE(game_id,position), índice stats(chat_id,score_units DESC,user_id). Novo sort depende de colunas derivadas; seleção lê histórico do grupo, sem queries por jogador. Não executado benchmark de volume de produção; docs registram possibilidade futura de índice após medição. Transferência para aplicação continua limitada a 512.
- Novos testes PostgreSQL usam partidas efetivamente gravadas, sem alterar score_units para fabricar empates. Legacy permite iguais pontos com placements diferentes. Updated chega a 3000/3000/0 e depois 4000/4000/0 em partidas pessoais, invertendo corretamente a ordem. Regressões de abandono, commit rejeitado, pending/N<2 e duplicata mantêm referência anterior. Testes do renderer agora exigem medalhas únicas e sequência até 6; teste existente garante saída idêntica do comando/automático.
- Nome “.”: auditoria confirma caminho observado/persistido direto com escape HTML; teste verifica preservação. Não existe evidência de corrupção introduzida pela feature; valor real do UserID em homologação não foi consultado.

## Validação executada

| Comando | Resultado |
|---|---|
| go test -count=1 ./... | Passou |
| go test -tags debugcards ./... | Primeira execução falhou no teste preexistente abaixo; segunda passou |
| go vet ./... | Passou |
| go build ./... | Passou |
| git diff --check | Passou |
| go test -count=1 -tags integration ./internal/storage/postgres/... | Passou, PostgreSQL 18.6 isolado local, 10.445s |
| go vet/build -tags debugcards ./... | Passaram |
| CGO_ENABLED=1 go test -race ./... | Falhou antes de testes: ThreadSanitizer unsupported VMA range; Found 39 - Supported 48 |

Intermitência: TestSwapInlineTargetDepartureInvalidatesColor, optional_swap_test.go:224, “not enough drawable cards”. Reproduzida 4/20 em /tmp/unobot-ranking-baseline.DfmvqV (base 4f25eeb), com go test -tags debugcards -count=20 -run '^TestSwapInlineTargetDepartureInvalidatesColor$' ./internal/telegram. Verificado git diff entre 4f25eeb e 64bb97d sem mudanças em optional_swap_test.go/swap_test.go. Sem correção fora do escopo ou mascaramento.

PostgreSQL usado somente em /tmp/unobot-ranking-pg.wwCuJ7, banco local unobot_ranking_test/porta55439; schemas dos testes removidos pelo cleanup existente. HEAD 64bb97d preservado. Todos os arquivos desta tarefa ficam unstaged; nenhum git add/commit/push/amend/rebase/cherry-pick.

# Plano: ranking acumulado e encerramento

Status: implementação concluída na dev; homologação manual pelo usuário pendente.

## Pedido do usuário
Expor resultado com pontos confirmados, ranking histórico separado e /ranking, somente na dev, sem publicação. Aprovação explícita no pedido: “Pode implementar”, reiterada por “continue”.

## Objetivo
Consultar player_group_stats via serviço/repositório e compartilhar a apresentação acumulada.

## Contexto atual
Ranking calcula inteiros em centésimos; Store.RecordCompletedGame faz transação idempotente. Telegram aguarda commit e anuncia pontos por callback, após resumo redundante. Não existe leitura pública do ranking V2. Índice existente (chat_id,score_units DESC,user_id).

## Arquivos analisados
- internal/ranking/ranking.go e eligibility.go
- internal/game/results.go
- internal/storage/postgres/results.go e migrations/0002_results.up.sql
- internal/telegram/results.go, commands.go, inline.go, bot.go, renderer.go e testes
- cmd/bot/main.go, docs/build.md, .github/workflows/dev-ci.yml
- .agent/context.md e .agent/memory/memory.md

## Arquivos que poderão ser modificados
- Novos arquivos de consulta/serviço/testes em internal/ranking e internal/storage/postgres
- internal/telegram/results.go, commands.go, inline.go, bot.go, renderer.go e testes
- cmd/bot/main.go
- README.md, docs/m7-persistence.md, docs/v2-telegram.md
- .agent/plans, .agent/memory/memory.md, .agent/context.md, .agent/decisions.md

## Estratégia de implementação
Interface separada de leitura e serviço em ranking; SQL limitado e ordenado retorna sistema, total e registros num snapshot único. Renderer HTML compartilhado, classificação competitiva por score exato, nomes persistidos, formatação inteira por sistema. Callback pós-commit envia resultado e consulta/envia ranking separado; caminhos sem commit preservam encerramento sem pontos e retry.

## Passos detalhados
1. Registrar aprovação e mover plano para approved.
2. Adicionar DTO, interface e serviço de consulta, implementação PostgreSQL com escopo ChatID e proteção de sistema.
3. Compartilhar formatter e renderer, medalhas top 3, empates 1/1/3, abandono explícito e limite seguro com total omitido.
4. Integrar /ranking, ajuda/menu e dois envios no encerramento pontuado, preservando idempotência/retry.
5. Cobrir serviço, Telegram e integração PostgreSQL real incluindo acumulação, histórico, isolamento e falhas.
6. Executar gates, atualizar docs/memória/decisão, revisar diff e concluir plano; commit local se verificações permitirem.

## Riscos
- Encerramento pode acontecer por inline, comando ou timeout: cobrir os três caminhos.
- Consulta/envio pode falhar após commit: informar indisponibilidade sem desfazer/repetir pontuação.
- Unicode, HTML e muitos jogadores podem exceder limite: contar comprimento seguro e preservar linhas completas.
- Race pode falhar no runtime VMA local; registrar resultado real.

## Impactos esperados
- Duas mensagens independentes no encerramento pontuado e leitura barata do histórico por qualquer membro.
- Nenhuma alteração na persistência de resultados, regras ou schema.

## Compatibilidade
- Linux, macOS, Windows: Go existente.
- Docker e CI/CD: nenhum deploy/publicação; manter builds existentes.

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
go test -race ./...
```
PostgreSQL local isolado, sem usar credenciais/servidor de produção. Também gates adicionais pertinentes da dev, incluindo race integration se executável.
### Execução
Homologação manual pelo usuário no Telegram antes de publicar: /ranking vazio, legado/atualizado, duas partidas, empate, abandono e outro grupo.

## Rollback
Reverter apenas commit local desta feature; não há migration ou conversão de dados.

## Observações
Nenhum push, main, container publicado, deploy ou migration de produção. main inicial: 6eea6c1399d287b58116d5be16a1807238653397.

## Entrega e validação

- Leitura nova: ranking.GroupRanking, ReadRepository, Service.ListGroupRanking e postgres.Store.ListGroupRanking. Interface de escrita e arquivos de persistência/fórmula/eligibilidade/migrations sem alteração.
- PostgreSQL usa índice existente, ChatID, score_units DESC e user_id estável; snapshot único inclui config, total e até 512 entradas. Empates competitivos são atribuídos pelo renderer sem usar UserID como desempate.
- Resultado com 🏁, colocação da engine e ganhos reais; histórico em mensagem 🏆 separada, compartilhada com /ranking. Formatação inteiro/centésimos sem floats. Abandono sem medalha/posição/pontos competitivos.
- Limite 4000 unidades UTF-16 após entidades, prefixo de linhas completas, sufixo com total omitido; sem callbacks, truncamento de nomes ou alinhamento ASCII.
- Testes adicionados: serviço/formatter, empate 1/1/3 e 1/2/2/4, Legacy/Updated, Unicode/HTML, top 3 e 4–10, grandes rankings/nome extremo, comando vazio/privado/isolamento/sem admin/erro, pós-commit bloqueado e valores históricos, falha/duplicata sem mensagem, encerramento inline com exatamente duas mensagens e HTML sem teclado.
- Integração PostgreSQL real 18.6: Game A e B com três elegíveis preservam fórmula homologada (1000/500/0 e 500/1000/0); retorna 1500/1500/0, além de dois jogadores históricos. Outro grupo Legacy isolado; abandono excluído; duplicata não acumula; limita 1000 rows a 512 com total exato; sistema incompatível além do prefixo recusado; trigger diferido força falha no COMMIT e consulta permanece vazia até retry bem-sucedido.
- Instância de teste criada somente em /tmp/unobot-ranking-pg.wwCuJ7, loopback porta 55439, banco unobot_ranking_test; nenhuma configuração/credencial de produção usada.

| Gate | Resultado |
|---|---|
| go test -count=1 ./... | Passou na execução final; intermitência preexistente detalhada abaixo |
| go test -tags debugcards ./... | Passou na execução final; mesma intermitência inicial |
| go vet ./... | Passou |
| go build ./... | Passou |
| go vet/build -tags debugcards ./... | Passaram |
| git diff --check | Passou |
| go test -count=1 -tags integration ./internal/storage/postgres/... | Passou em PostgreSQL local real; schemas de teste isolados |
| CGO_ENABLED=1 go test -race ./... | Falhou antes dos testes: ThreadSanitizer VMA 39, suporta 48 |
| CGO_ENABLED=1 go test -race -tags integration ./internal/storage/postgres/... | Mesmo impedimento VMA antes de executar os testes |

Intermitência encontrada: TestKeepHandInlineFlowStaleColorAndMultigroup, optional_swap_test.go:117, “not enough drawable cards”. Reproduzida na base 4f25eeb, extraída por git archive em /tmp/unobot-ranking-baseline.DfmvqV: `go test -count=20 -run '^TestKeepHandInlineFlowStaleColorAndMultigroup$' ./internal/telegram` falhou em 4 das 20 execuções. Sem mudanças no teste ou gameplay de Trocar Mãos. Gates completos mais recentes passaram. Não mascarar como suite sempre estável, nem atribuir ao V1.

Docker não está disponível no ambiente; job de imagem do CI não reproduzido localmente. Nenhuma imagem publicada. README/docs técnicas e memória/decisão/contexto atualizados. Homologação Telegram permanece separada dos testes automatizados.

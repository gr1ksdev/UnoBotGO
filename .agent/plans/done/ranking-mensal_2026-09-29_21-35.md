# Plano: ranking-mensal

## Pedido do usuário
Implementar a PRIMEIRA etapa do novo sistema de ranking no UnoBotGO V2 (na branch `dev`): transformar o ranking acumulado atual do grupo em um RANKING MENSAL do mês corrente, preservando o histórico existente, a compatibilidade de regras homologadas e o fuso horário canônico de negócios `America/Sao_Paulo`. Não implementar ranking global, WebApp, ranking privado, comandos para outros meses ou APIs nesta etapa.

## Objetivo
1. Transformar o ranking ativo exibido pelo comando `/ranking` e pelas mensagens automáticas pós-partida em um ranking do mês calendário vigente.
2. Definir o fuso horário canônico de transição de mês estritamente como `America/Sao_Paulo` (virada às 00:00:00 do dia 01).
3. Uma partida pontuada pertence inteiramente ao mês do seu `finished_at` canônico.
4. Preservar o histórico anterior intacto no banco de dados, sem resets físicos ou cron jobs.
5. Criar a tabela `player_group_monthly_stats` via migration `0006_monthly_ranking.up.sql` com backfill 100% exato e idempotente das partidas já homologadas.
6. Atualizar a transação síncrona `RecordCompletedGame` para persistir atomicamente `player_group_monthly_stats` (ao lado de `player_group_stats`).
7. Atualizar a consulta `ListGroupRanking` para ler `player_group_monthly_stats` do mês corrente com desempate restrito às colocações daquele mês.
8. Atualizar o formato visual em `RenderGroupRanking`:
   - Título: `🏆 Ranking do grupo · <NomeDoMês>` (ex.: `🏆 Ranking do grupo · Setembro`, `🏆 Ranking do grupo · Outubro`).
   - Mensagem vazia: `Ainda não há partidas pontuadas neste mês.`.
   - Manter rótulos sequenciais estritos `1º, 2º, 3º...`, regras de desempate, pontuações 0,00 para elegíveis e exclusão de desistentes.

## Contexto atual
- O banco possui as tabelas `group_configs`, `completed_games`, `completed_game_players` e `player_group_stats`.
- A auditoria no banco local/homologação confirmou 6 partidas concluídas e 16 participações de jogadores em setembro/2026.
- A consulta `ListGroupRanking` em `internal/storage/postgres/ranking.go` lê de `player_group_stats` e calcula desempate pela CTE `latest`.
- O fechamento da partida em `internal/storage/postgres/results.go` grava síncrono e atomicamente em `player_group_stats`.
- `internal/telegram/ranking.go` renderiza o título fixo `🏆 Ranking do grupo` e mensagem de vazio `Ainda não há partidas pontuadas neste grupo.`.
- O fuso horário de referência para a virada mensal precisa ser garantido independentemente do SO hospedeiro (`America/Sao_Paulo` com `_ "time/tzdata"`).

## Arquivos analisados
- `AGENTS.md`
- `internal/ranking/service.go`
- `internal/ranking/ranking.go`
- `internal/storage/postgres/results.go`
- `internal/storage/postgres/ranking.go`
- `internal/storage/postgres/migrations/0002_results.up.sql`
- `internal/telegram/ranking.go`
- `internal/telegram/results.go`
- `internal/storage/postgres/ranking_tiebreak_integration_test.go`
- `internal/telegram/ranking_test.go`

## Arquivos que poderão ser modificados
- `internal/storage/postgres/migrations/0006_monthly_ranking.up.sql` (novo)
- `internal/ranking/service.go`
- `internal/ranking/time.go` (novo ou dentro de service/ranking: timezone `America/Sao_Paulo`, cálculo de `MonthStart` e `MonthName`)
- `internal/storage/postgres/results.go`
- `internal/storage/postgres/ranking.go`
- `internal/telegram/ranking.go`
- `internal/telegram/ranking_test.go`
- `internal/storage/postgres/ranking_monthly_integration_test.go` (novo)
- `internal/storage/postgres/ranking_tiebreak_integration_test.go`

## Estratégia de implementação
1. **Timezone e Funções Canônicas de Tempo**:
   - Em `internal/ranking`, importar `_ "time/tzdata"` para garantir disponibilidade de `America/Sao_Paulo` em qualquer build estático/distroless.
   - Definir `RankingLocation = time.LoadLocation("America/Sao_Paulo")`.
   - Adicionar helper `MonthStart(t time.Time) time.Time` truncando para o primeiro instante do mês em São Paulo.
   - Adicionar helper `MonthName(t time.Time) string` retornando o nome em português (Janeiro, Fevereiro, ..., Setembro, Outubro, ..., Dezembro).
2. **Migration 0006 (DDL + Backfill)**:
   - Criar tabela `player_group_monthly_stats`:
     - `chat_id bigint NOT NULL REFERENCES group_configs(chat_id)`
     - `user_id bigint NOT NULL CHECK(user_id>0)`
     - `month_start date NOT NULL`
     - `ranking_system text NOT NULL CHECK(ranking_system IN ('legacy','updated'))`
     - `score_units bigint NOT NULL CHECK(score_units>=0)`
     - `completed_games bigint NOT NULL CHECK(completed_games>0)`
     - `wins bigint NOT NULL CHECK(wins>=0)`
     - `display_name text NOT NULL`
     - `last_finished_at timestamptz NOT NULL`
     - `updated_at timestamptz NOT NULL DEFAULT now()`
     - `PRIMARY KEY (chat_id, user_id, month_start)`
   - Criar índice para performance do ranking:
     - `CREATE INDEX player_group_monthly_stats_ranking ON player_group_monthly_stats(chat_id, month_start, score_units DESC, user_id);`
   - Backfill idempotente:
     - Inserir agregações de `completed_games` + `completed_game_players` (apenas `scoring_status = 'scored'` e elegíveis) agrupados por `chat_id, user_id, month_start, ranking_system`.
     - `ON CONFLICT (chat_id, user_id, month_start) DO NOTHING`.
3. **Persistência de Resultados (`RecordCompletedGame`)**:
   - Na mesma transação de banco de dados onde já é inserido o jogo e atualizado `player_group_stats`, calcular `monthStart := ranking.MonthStart(r.FinishedAt)`.
   - Executar UPSERT em `player_group_monthly_stats` com a mesma semântica de lock e verificação de compatibilidade de sistema.
   - Idempotência preservada: retentativas com o mesmo `GameID` já caem no `AlreadyPersisted` e não incrementam pontuação.
4. **Consulta do Ranking Mensal (`ListGroupRanking`)**:
   - Atualizar a interface `ranking.ReadRepository`:
     `ListGroupRanking(ctx context.Context, chatID int64, at time.Time) (GroupRanking, error)`
   - Atualizar `ranking.GroupRanking` com campos `MonthName` e `MonthStart`.
   - `ranking.Service.ListGroupRanking(ctx context.Context, chatID int64)` usa `s.now()` (onde `Now func() time.Time` permite mock determinístico em testes, default `time.Now`).
   - A query SQL de `ListGroupRanking`:
     - Filtra `player_group_monthly_stats` por `chat_id = $1 AND month_start = $2`.
     - Filtra o desempate CTE `latest` por partidas concluídas no mesmo mês:
       `(date_trunc('month', (g.finished_at AT TIME ZONE 'America/Sao_Paulo')))::date = $2`.
5. **Renderização Visual (`RenderGroupRanking`)**:
   - Título: `🏆 Ranking do grupo · <NomeDoMês>\n\n`.
   - Caso `Total == 0`: mensagem `Ainda não há partidas pontuadas neste mês.`.
   - Rótulos `1º`, `2º`, `3º`... sequenciais e preservação de participantes com 0 pontos.

## Passos detalhados
1. Criar helper de tempo e timezone em `internal/ranking/time.go` com mapeamento dos meses em pt-BR e testes unitários.
2. Criar a migration `internal/storage/postgres/migrations/0006_monthly_ranking.up.sql`.
3. Ajustar modelos e interfaces em `internal/ranking/service.go`.
4. Atualizar `RecordCompletedGame` em `internal/storage/postgres/results.go` para gravar `player_group_monthly_stats`.
5. Atualizar `ListGroupRanking` em `internal/storage/postgres/ranking.go` para filtrar por mês e aplicar desempate contextual do mês.
6. Atualizar `RenderGroupRanking` e o handler em `internal/telegram/ranking.go`.
7. Atualizar testes unitários em `internal/telegram/ranking_test.go`.
8. Criar suite de testes de integração em `internal/storage/postgres/ranking_monthly_integration_test.go`:
   - Teste de virada de mês (partida às 23:59:59 do dia 30 vs 00:00:00 do dia 01 em São Paulo).
   - Teste de isolamento de pontos entre meses.
   - Teste de desempate restrito ao mês corrente.
   - Teste de idempotência e elegibilidade.
9. Executar suites completas:
   - `go test -count=1 ./...`
   - `go test -count=1 -race ./...`
   - `go vet ./...`
   - `go build ./...`
   - `git diff --check`
   - Testes de integração do PostgreSQL.
10. Validar banco real/container e certificar que a árvore de trabalho continua limpa, sem commits e sem push.

## Riscos
- **Risco de timezone incorreto**: Se o fuso não for explicitamente `America/Sao_Paulo` (ex.: se usar UTC ou localtime da máquina), partidas próximas da meia-noite (21h-23h59 de Brasília) cairiam no dia 1 do mês seguinte em UTC.
  - *Mitigação*: Uso de `America/Sao_Paulo` carregado via `_ "time/tzdata"` tanto no Go quanto na expressão SQL `(finished_at AT TIME ZONE 'America/Sao_Paulo')`.
- **Risco de regressão na persistência**: Quebra da transação atômica em `RecordCompletedGame`.
  - *Mitigação*: Mesma transação `tx` já existente, com locks de linha e UPSERT na nova tabela.
- **Risco de quebra de idempotência**: Reenvio de payload de jogo duplicar pontos mensais.
  - *Mitigação*: A idempotência por `GameID` ocorre antes de qualquer escrita de stats e retorna `AlreadyPersisted: true`.

## Impactos esperados
- `/ranking` exibirá o mês atual no cabeçalho.
- Ao mudar o mês calendário em São Paulo, o ranking automaticamente zera sua exibição pública sem necessidade de job cron ou deleção física.
- Histórico de todos os meses permanece guardado no PostgreSQL na tabela `player_group_monthly_stats` e `player_group_stats`.

## Compatibilidade
- Linux: Sim
- macOS: Sim
- Windows: Sim
- Docker: Sim (com tzdata embutido no Go)
- CI/CD: Sim

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -count=1 ./...
go test -count=1 -race ./...
```

### Testes de Integração PostgreSQL
```bash
DATABASE_URL="postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable" go test -count=1 -v ./internal/storage/postgres/ -run "TestMonthly"
```

## Rollback
Caso necessário, reverter as alterações na branch `dev` com `git checkout` dos arquivos modificados e executar a remoção da tabela `player_group_monthly_stats` no banco.

## Observações
- Não será feito commit nem push nesta etapa.
- `main` permanece intacta.
- Não serão criados comandos para listar meses anteriores nesta etapa (escopo restrito ao mês corrente).

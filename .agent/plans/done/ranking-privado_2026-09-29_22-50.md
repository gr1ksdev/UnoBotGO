# Plano: ranking-privado

## Pedido do usuário
Implementar a segunda etapa do novo sistema de ranking: fazer o comando `/ranking` funcionar também no privado do bot, exibindo ao usuário remetente a sua pontuação mensal nos grupos em que possui participação elegível no mês atual (America/Sao_Paulo), separando claramente as seções de pontuação dos sistemas "Atualizado" e "Legado" com seus respectivos totais, sem introduzir ranking global, WebApp ou API pública nesta etapa.

## Objetivo
1. Permitir que `/ranking` no privado do bot mostre a pontuação mensal do próprio usuário (Telegram UserID remetente) separada por grupo no mês atual.
2. Manter estritamente inalterado o `/ranking` executado em grupos (ranking competitivo do grupo com medalhas e desempate) e o fluxo automático de notificação pós-partida.
3. Separar pontuações dos sistemas "Atualizado" e "Legado", sem jamais somá-los ou misturar casas decimais (Atualizado com duas casas decimais pt-BR e Legado com pontuação inteira "pt/pts").
4. Armazenar de forma durável o último título observado de cada grupo através de uma nova migração (`0007_group_title.up.sql`) na tabela `group_configs`, atualizando-o a partir das mensagens/interações observadas, com fallback determinístico (`Grupo <ChatID>`) quando o título não for conhecido.
5. Garantir ordenação (`score_units DESC`, `last_finished_at DESC`, `nome do grupo ASC`, `chat_id ASC`) e formatação sob limites UTF-16 do Telegram com cálculo exato de total mesmo sob linhas omitidas.
6. Adicionar cobertura completa de testes unitários e de integração PostgreSQL.

## Contexto atual
- A etapa 1 (ranking mensal de grupo) já está implementada e commitada na branch `dev`.
- A persistência mensal em `player_group_monthly_stats` registra atomicamente os pontos elegíveis de cada partida por mês canônico (`month_start` em `America/Sao_Paulo`).
- A tabela `group_configs` armazena configurações duráveis por grupo (`chat_id`), mas não possui coluna para o título do chat. O título em tempo de execução só trafegava na memória de partidas ativas (`game.CreateRequest.ChatName`).
- O comando `/ranking` no chat privado hoje rejeita a consulta com `"🏆 Consulte o ranking em um grupo."`.

## Arquivos analisados
- `internal/telegram/commands.go`: Ponto de bifurcação de comandos entre chat privado e grupo/supergrupo.
- `internal/telegram/ranking.go`: Renderização de ranking de grupo e handler de ranking.
- `internal/telegram/ranking_test.go`: Testes de apresentação e limites UTF-16.
- `internal/ranking/service.go`: Interfaces `ReadRepository` e serviço de leitura de ranking.
- `internal/ranking/service_test.go`: Testes de formatação e serviço.
- `internal/ranking/time.go`: Centralização de timezone `America/Sao_Paulo`, cálculo de `MonthStart`, `MonthName` e `MonthDateString`.
- `internal/storage/postgres/ranking.go`: Query SQL para ranking de grupo.
- `internal/storage/postgres/groups.go`: Acesso e persistência em `group_configs`.
- `internal/storage/postgres/migrations/0001_groups.up.sql` a `0006_monthly_ranking.up.sql`: Histórico de migrations aplicadas.
- `internal/groups/groups.go`: Modelos e interfaces de repositório de grupos.

## Arquivos que poderão ser modificados
- `internal/storage/postgres/migrations/0007_group_title.up.sql` (novo): Adiciona coluna `title` em `group_configs` e índice em `player_group_monthly_stats(user_id, month_start)`.
- `internal/groups/groups.go`: Adiciona método `ObserveGroupTitle` na interface `Repository` e no `Service`.
- `internal/groups/groups_test.go`: Atualiza mock em memória de `Repository`.
- `internal/storage/postgres/groups.go`: Implementa `ObserveGroupTitle` com update condicional idempotente.
- `internal/ranking/service.go`: Adiciona DTOs para ranking de usuário (`UserMonthlyRankings`, `UserMonthlyRankingSection`, `UserGroupRankingEntry`) e método `ListUserMonthlyRankings` em `ReadRepository` e `Service`.
- `internal/storage/postgres/ranking.go`: Implementa `ListUserMonthlyRankings` em uma única query SQL sem N+1.
- `internal/telegram/ranking.go`: Implementa `RenderUserMonthlyRankings` com suporte a limites UTF-16, texto amigável de vazio, pluralização correta e `handlePrivateRanking`.
- `internal/telegram/commands.go`: Bifurca `/ranking` (grupo -> `handleRanking`; privado -> `handlePrivateRanking` usando `msg.From.ID`) e observa título do grupo em interações de grupos.
- `internal/telegram/renderer.go`: Atualiza texto de ajuda para `/ranking`.
- `internal/telegram/mock_test.go` e `internal/telegram/config_test.go`: Atualiza mocks de `groups.Repository`.
- `internal/telegram/ranking_test.go`: Atualiza testes para validar novo comportamento no privado, limites UTF-16, separação Legado/Atualizado e fallbacks.
- `internal/storage/postgres/ranking_monthly_integration_test.go`: Adiciona testes de integração PostgreSQL para `ListUserMonthlyRankings` (múltiplos grupos, isolamento de usuário, meses diferentes, fallback de título e renomeação de grupo).
- `.agent/memory/memory.md`, `.agent/context.md`, `.agent/decisions.md`: Documentação interna persistente.

## Estratégia de implementação

1. **Migração 0007 (`0007_group_title.up.sql`)**:
   - `ALTER TABLE group_configs ADD COLUMN IF NOT EXISTS title text NOT NULL DEFAULT '';`
   - `CREATE INDEX IF NOT EXISTS player_group_monthly_stats_user_month ON player_group_monthly_stats(user_id, month_start);`
   - Não altera migrações anteriores nem dados pré-existentes.

2. **Captura e persistência do título do grupo**:
   - No `Store` do PostgreSQL, adicionar `ObserveGroupTitle(ctx context.Context, chatID int64, title string) error`:
     ```sql
     INSERT INTO group_configs(chat_id, title) VALUES($1, $2)
     ON CONFLICT(chat_id) DO UPDATE SET title=EXCLUDED.title, updated_at=now()
     WHERE group_configs.title <> EXCLUDED.title
     ```
     Executa apenas quando `title != ""` e altera o banco somente se o título observado for diferente do atual.
   - No `CommandHandler` do Telegram:
     - Sempre que receber mensagem de comando em um grupo/supergrupo com título não vazio (`isGroup && msg.Chat.Title != ""`), aciona `ObserveGroupTitle`.
     - No evento `HandleMyChatMember`, se for grupo com título não vazio, aciona `ObserveGroupTitle`.

3. **Modelagem de dados no pacote `ranking`**:
   - `UserGroupRankingEntry`: `ChatID int64`, `GroupName string`, `RankingSystem groups.RankingSystem`, `ScoreUnits Units`, `LastFinishedAt time.Time`.
   - `UserMonthlyRankingSection`: `System groups.RankingSystem`, `Entries []UserGroupRankingEntry`, `TotalScore Units`.
   - `UserMonthlyRankings`: `UserID int64`, `MonthName string`, `MonthStart time.Time`, `Updated *UserMonthlyRankingSection`, `Legacy *UserMonthlyRankingSection`.
   - Métodos `ListUserMonthlyRankings(ctx context.Context, userID int64) (UserMonthlyRankings, error)` e `ListUserMonthlyRankingsAt(ctx context.Context, userID int64, at time.Time) (UserMonthlyRankings, error)`.

4. **Query SQL única (sem N+1)**:
   - Em `internal/storage/postgres/ranking.go`:
     ```sql
     SELECT
       s.chat_id,
       COALESCE(NULLIF(c.title, ''), '') AS group_title,
       s.ranking_system,
       s.score_units,
       s.last_finished_at
     FROM player_group_monthly_stats s
     LEFT JOIN group_configs c ON c.chat_id=s.chat_id
     WHERE s.user_id=$1 AND s.month_start=$2::date
     ORDER BY
       CASE WHEN s.ranking_system='updated' THEN 1 ELSE 2 END ASC,
       s.score_units DESC,
       s.last_finished_at DESC,
       COALESCE(NULLIF(c.title, ''), 'Grupo ' || s.chat_id::text) ASC,
       s.chat_id ASC
     ```
   - Agrupa em memória nas seções `Updated` e `Legacy`.
   - Fallback de nome de grupo se `group_title` for vazio: `Grupo <ChatID>`.

5. **Renderização no Telegram (`RenderUserMonthlyRankings`)**:
   - Título: `🏆 Seus rankings · <Mês>`
   - Se nenhuma participação elegível no mês:
     ```
     🏆 Seus rankings · <Mês>

     Você ainda não possui partidas pontuadas neste mês.
     ```
   - Se houver `Updated`:
     ```
     Atualizado

     <Grupo> · <Score>
     ...
     Total · <TotalScore>
     ```
   - Se houver `Legacy`:
     ```
     Legado

     <Grupo> · <Score>
     ...
     Total · <TotalScore>
     ```
   - Se houver ambos, exibe ambas as seções separadas por linha em branco, Atualizado primeiro.
   - Aplica contagem UTF-16 (`rankingMessageLimit = 4000`), truncando grupos se necessário com `… e mais X grupos.` sem cortar linhas e mantendo o `Total` acumulado com o valor real de todos os grupos daquele sistema.
   - `html.EscapeString` no nome dos grupos.
   - Sem medalhas para grupos. Sem botões de WebApp ou ranking global.

6. **Bifurcação no Telegram `commands.go`**:
   - Se `!isGroup` e comando for `"ranking"`: chama `h.handlePrivateRanking(ctx, msg.Chat.ID, int64(actorID))`.
   - Usa estritamente `msg.From.ID` (nunca aceita parâmetro de texto).
   - Se for grupo, continua chamando `h.handleRanking(ctx, msg.Chat.ID)`.
   - `notifyPoints` pós-partida continua enviando o ranking competitivo do grupo.

## Passos detalhados
1. Criar `internal/storage/postgres/migrations/0007_group_title.up.sql`.
2. Atualizar interfaces em `internal/groups/groups.go` e implementar `ObserveGroupTitle` em `internal/storage/postgres/groups.go`.
3. Atualizar mocks de repositório de grupo existentes nos testes.
4. Adicionar tipos DTO e métodos em `internal/ranking/service.go`.
5. Implementar `ListUserMonthlyRankings` em `internal/storage/postgres/ranking.go`.
6. Implementar `RenderUserMonthlyRankings` e `handlePrivateRanking` em `internal/telegram/ranking.go`.
7. Conectar a bifurcação de `/ranking` em `internal/telegram/commands.go` e a observação de título de chat.
8. Criar suíte completa de testes unitários para o ranking privado em `internal/telegram/ranking_test.go` e `internal/ranking/service_test.go`.
9. Criar testes de integração PostgreSQL para o ranking privado em `internal/storage/postgres/ranking_monthly_integration_test.go`.
10. Executar validação de compilação, `go vet`, testes com race detector (`-race`), testes de integração e linters.
11. Atualizar artefatos de memória e documentação interna (`.agent/memory/memory.md`, `.agent/context.md`, `.agent/decisions.md`).

## Riscos
- *Risco de truncamento de mensagem longa*: Tratado com checagem de limite UTF-16 preventiva preservando linha de total e exibindo `… e mais N grupos.`.
- *Risco de injeção HTML por títulos de grupos arbitrários*: Mitigado por `html.EscapeString` ao formatar o título do grupo.
- *Risco de overhead no banco em mensagens de grupo*: A observação de título usa cláusula `WHERE group_configs.title <> EXCLUDED.title`, resultando em no-op quando o título não mudou.
- *Risco de regressão no ranking de grupo*: Isolamento absoluto — `handleRanking` e `RenderGroupRanking` permanecem inalterados.

## Impactos esperados
- Usuários poderão acompanhar seus pontos mensais acumulados por grupo no privado do bot.
- Sistema Legacy e Updated ficam visualmente e numericamente isolados.
- Nenhuma alteração nas regras de pontuação de partidas existentes.
- Desempenho preservado com uma única query indexada para o ranking privado.

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
go test ./...
go test -race ./...
go vet ./...
TEST_DATABASE_URL="postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable" go test -tags=integration -race ./internal/storage/postgres/...
```

### Execução
Executar bot localmente e enviar `/ranking` tanto em grupo quanto no privado com usuário de teste.

## Rollback
Remover arquivos novos (0007 e novos testes) e reverter alterações via `git checkout -- .`.

## Observações
- Não fazer commit ou push sem autorização explícita do usuário.
- Não mexer na branch main.
- Nenhuma migration será aplicada em produção nesta etapa.

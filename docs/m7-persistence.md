# M7 — Persistência, configuração por grupo e ranking automático

Revisão: 2026-09-27, branch `dev`.
Status: **CONCLUÍDA E HOMOLOGADA NO TELEGRAM REAL** (import antigo adiado / DEFERRED).

## Commits desta implementação

| Commit | Escopo |
|---|---|
| `8b46c2d` | M7.1: pgx, migrations, configuração, startup e serviço PostgreSQL de CI |
| `d4e34ee` | M7.2 base: defaults persistentes, API/autorização e snapshot por partida |
| `806e022` | M7.3 base: cálculo, resultado público final, transação/idempotência e fechamento síncrono |
| `35e72de` | M7.4: usuários conhecidos e nomes observados |
| `5364016` | M7.5 base: parser, reconciliação e staging de imports (fundação retida; import adiado) |
| `6e60203` | Limitar cleanup de transações em falha/cancelamento |
| `9dc0cd8` | Elegibilidade definitiva: concluintes com placement válido, abandono fora de N e zero pontos |
| `25f7f90` | Persistência postgres: migration 0005, concessão e stats somente para elegíveis em N>=2 |
| `e1ebb41` | Integração telegram: anúncio de pontuação pós-commit após a mensagem final compacta |
| `33f4c6b` | feat(groups): expand groups repository and service with ranking switch and installer tracking |
| `671d4fb` | feat(telegram): implement group config UX, /config command, callbacks, and my_chat_member installer tracking |
| `fd5f7df` | test(telegram): add comprehensive test suite for group config UX scenarios |
| `bb352b3` | docs(m7): update documentation, plans and persistent memory for group config UX |

Sem push, promoção ou alteração na main.



## Arquitetura e arquivos principais

- `internal/uno` permanece sem dependência de SQL, Telegram ou ambiente.
- `internal/game` mantém engine, cartas e sessões em memória. GameID nasce em
  `manager.create`, por 16 bytes aleatórios codificados em hexadecimal.
- `internal/groups`: configuração, snapshot, repository e autorização independentes de SQL.
- `internal/ranking`: DTO público final e cálculo determinístico.
- `internal/storage/postgres`: pgxpool, SQL e transações.
- `internal/rankingimport`: parser/reconciliação sem storage ou aplicação de pontos.
- `internal/telegram/results.go`: integração síncrona somente no fechamento.
- `cmd/bot` / `internal/app`: conecta PostgreSQL e executa migrations/verificação antes de HTTP, Telegram e workers.
- `.github/workflows/dev-ci.yml`: PostgreSQL isolado para testes reais.

GameFinished produz resultado independente de mãos/pilhas. A engine é descartada
como antes. O DTO final permanece em mapa de memória separado do histórico público
até confirmação do commit, inclusive se histórico for evictado ou chat resetado.
O adapter aguarda a transação fora dos locks do game. Falha mantém o DTO para retry
síncrono pela API de aplicação `RetryPendingResults`; não foi criado comando para isso.
Não há scheduler/worker de retry. Crash antes de commit perde RAM; não há journal
ou recuperação de partidas ativas.

Não existem fila/worker de resultados, outbox ou `result_notifications`.
A mensagem compacta final é preservada. Entrega Telegram exatamente uma vez não
é garantida; dados já commitados não são desfeitos se a mensagem falhar.

## Schema e migrations

Migrations embutidas versionadas em `internal/storage/postgres/migrations`:

| Migration | Tabelas e propósito |
|---|---|
| `0001_groups.up.sql` | `group_configs`: ChatID único, Classic/Legacy defaults, instalador nullable, revision/timestamps |
| `0002_results.up.sql` | `completed_games`: GameID PK, snapshots/auditoria/hash/policy/status; `completed_game_players`: UserID/nomes/colocação/status/score/counters; `player_group_stats`: uma linha por chat/user, unidades acumuladas, partidas e vitórias |
| `0003_known_users.up.sql` | `known_group_users`: identidade chat/user, nomes mutáveis, username nullable, last_seen monotônico |
| `0004_imports.up.sql` | `ranking_imports`: fonte/hash único por chat e auditoria; `ranking_import_entries`: IDs próprios, linhas, nomes e status, sem chave pelo nome |
| `0005_insufficient_eligible_players.up.sql` | `completed_games`: adiciona o status `insufficient_eligible_players` para partidas concluídas onde N < 2 elegíveis |
| `0006_monthly_ranking.up.sql` | `player_group_monthly_stats`: ranking mensal e backfill do ledger de partidas |
| `0007_group_title.up.sql` | Título do grupo e índice de consulta mensal por usuário |

`schema_migrations` registra versão, checksum SHA-256 e `applied_at`. O runtime aplica automaticamente somente os SQLs versionados embutidos, sem inferência de schema. Todas as pendências e registros do ledger compartilham uma única transação, serializada por `pg_advisory_xact_lock(71870101)`. O ledger completo é validado antes de qualquer SQL pendente; versões desconhecidas, ordem inconsistente e checksums alterados são rejeitados. Não há reparo automático de ledger ou down migration.

Resultado e ranking, quando houver policy explícita, usam a mesma transação.
GameID+hash confirmam retries idênticos; conteúdo divergente é recusado. Lock da
configuração serializa conclusões do grupo. Stats com outro sistema recusam a
transação integralmente. Não há reset ou conversão silenciosa.

## Ranking e elegibilidade definitiva

100 unidades inteiras = 1 ponto. Legacy concede 100 unidades para posições antes
do último e 0 ao último. Updated calcula `1000*(N-position)/(N-1)`, arredondado a
unidades inteiras pelo método half-up. Exibição preparada com vírgula e duas casas
decimais para Updated e inteiro para Legacy, com pluralização `1 pt`/`0 pts`/`2 pts`.
O formatter compartilhado `ranking.FormatScore` usa somente inteiros armazenados.

A política de elegibilidade definitiva foi aprovada e integrada (`completed-placements-v1`):
- Participam do cálculo (`N`) e recebem pontuação apenas os jogadores que concluíram
  efetivamente a partida e possuem colocação válida no resultado.
- Abandono definitivo: jogador que sai e não retorna recebe 0 pontos, fica fora de `N`,
  sem colocação artificial, permanecendo no registro persistido para fins de auditoria
  e exibido no resultado como `Nome · fora do ranking`, sem medalha ou score competitivo.
- Late join e saída com reentrada válida: se o jogador concluiu normalmente a partida,
  participa de `N` e pontua pela sua colocação final sem penalidade.
- Partidas com `N < 2` elegíveis (ex: encerramento por departure com 1 jogador restante)
  são persistidas para auditoria com status `insufficient_eligible_players`, com scores
  zerados e sem alteração de `player_group_stats`. Partidas canceladas não são persistidas.
- A finalização síncrona aguarda o COMMIT do PostgreSQL e, quando confirmado com concessão
  de pontos (`commit.Scored == true`), envia o resultado com pontos daquela partida e,
  em outra mensagem, consulta e apresenta o ranking histórico atualizado. O resultado
  substitui o resumo redundante no fechamento pontuado. Em retry já persistido
  (`AlreadyPersisted`), as duas notificações não são reenviadas. Falhas de DB retêm o resultado na
  memória do serviço para retry síncrono via `RetryPendingResults`.

### Leitura mensal e /ranking (dev, 2026-09-29)

`Telegram → ranking.Service.ListGroupRanking → ranking.ReadRepository →
postgres.Store.ListGroupRanking` consulta a tabela `player_group_monthly_stats` pelo
ChatID e pelo início do mês corrente (`month_start date`), determinado estritamente
no fuso horário canônico de negócios `America/Sao_Paulo` (carregado via `_ "time/tzdata"`).
A interface de escrita `ranking.Repository` e a transação síncrona de resultados persistem
atomicamente tanto em `player_group_stats` (histórico acumulado geral) quanto em
`player_group_monthly_stats` (bucket mensal particionado). Não há deleção física, cron
job ou reset manual: a virada é lógica e baseada em bucket temporal no dia 1 às 00:00:00 SP.

Uma única instrução SQL retorna configuração, total e até 512 entradas do mês, ordenadas
por `score_units DESC, last_placement ASC, last_completed_game_at DESC, user_id ASC`.
Uma CTE com `DISTINCT ON (user_id)` faz JOIN de `completed_games` e
`completed_game_players`, filtrando ChatID, status `scored`, o mês corrente
`date_trunc('month', (g.finished_at AT TIME ZONE 'America/Sao_Paulo'))::date = $2`
e participação elegível (posição válida e status/went_out coerentes com `ranking.Player.Eligible`).
Ordena por `finished_at DESC, game_id DESC` para selecionar a última partida daquele jogador
*dentro daquele mês*; GameID apenas estabiliza a seleção se duas partidas tiverem o mesmo timestamp.
`last_placement` vem de `position`; `last_completed_game_at` vem de `finished_at`.
Abandono, N<2 e resultados sem política pontuada não substituem essa referência, e partidas
de meses anteriores não influenciam o critério de desempate do mês vigente.

O JOIN do histórico do mês selecionado com `player_group_monthly_stats` ocorre antes do LIMIT.
A migration `0006_monthly_ranking.up.sql` cria `player_group_monthly_stats` com chave primária
`(chat_id, user_id, month_start)` e índice de ranking `(chat_id, month_start, score_units DESC, user_id)`,
executando também um backfill 100% exato e idempotente das partidas concluídas anteriores.
Somente até 512 registros são transferidos à aplicação.
Referência ausente usa NULLS LAST, sem inventar colocação ou timestamp.
O snapshot único evita divergência entre total, sistema e linhas. Incompatibilidade
de sistema em qualquer registro (mesmo fora do prefixo) recusa a leitura.
Stats de jogadores com score zero no mês continuam válidas; grupo ausente/mês vazio não é erro
e exibe mensagem amigável: `Ainda não há partidas pontuadas neste mês.`.

`RenderGroupRanking` é usado tanto no pós-commit quanto no comando público `/ranking`.
O cabeçalho traz o nome do mês: `🏆 Ranking do grupo · <NomeDoMês>` (ex.: `Setembro`, `Outubro`).
Nomes vêm de `display_name`, escapados em HTML, sem chamadas `GetChatMember`.
Posições são únicas (`1, 2, 3, 4...`): o renderer usa índice + 1 na ordem recebida.
UserID só resolve igualdade absoluta de score, última colocação e timestamp dentro do mês,
sem representar mérito adicional. Medalhas somente para posições 1–3; posições seguintes usam `4.`, `5.` etc.

O renderer admite até 4000 unidades UTF-16 após interpretar entidades HTML (margem
abaixo de 4096), reservando espaço para `… e mais N jogadores.`. Preserva linhas e
nomes completos; até um nome excepcionalmente grande é omitido com o restante,
sem quebrar Unicode ou entidades. As 512 entradas excedem a capacidade de linhas
mínimas dessa mensagem. Não há callbacks de paginação.

Falha de consulta pós-commit não desfaz pontos: uma resposta curta orienta usar
`/ranking` novamente. Falha no envio Telegram continua sem outbox/reenvio automático;
não se repete a gravação. Persistência, consulta e envio têm prazos limitados.
Sem concessão (N<2), mantém-se apenas o encerramento sem anúncio de ganhos.

Continuam NEEDS PRODUCT DECISION:

- Conversão/migração definitiva de histórico/scores acumulados caso produto venha a permitir troca de ranking em grupos já ativos (atualmente recusada com `ErrNeedsProductDecision`).
- Aproximação/confirmação de import Updated; nenhum fator ×5 aprovado/implementado.
- Reavaliação de resultados legados em status `needs_product_decision`.

## Configuração e permissões

GetOrCreate cria Classic+Legacy sem setup. `/novo` consulta config uma vez;
`/novo classico` e `/novo caseiro` sobrescrevem apenas a partida sem alterar `GroupConfig`.
Ranking e revisão ficam congelados na criação da partida; regras efetivas do lobby determinam modo no início,
preservando o seletor de modo existente.

`groups.Service` valida a cada operação de modo e ranking: admin atual OU instalador ainda
membro. Falha na consulta de associação recusa ação (fail closed).

A UX Telegram de configuração de grupos foi integrada com sucesso:
- Comando `/config` disponível em grupos para administradores e instalador ativo.
- Interface em botões inline para alternar Modo (`Clássico` / `Caseiro`) e Ranking (`Legado` / `Atualizado`).
- Tentativa de alternância de ranking em grupos com histórico incompatível recusa a operação com alerta amigável ao usuário via `ErrNeedsProductDecision`, preservando a configuração anterior.
- Monitoramento de `my_chat_member` captura transições reais de instalação/reentrada (`left`/`kicked` -> `member`/`administrator`), grava `installed_by_user_id` e envia mensagem curta de boas-vindas com botão `[ ⚙️ Configurar ]`. Updates irrelevantes (promoções/demissões) são ignorados.

## Usuários e importação de ranking antigo

`/novo` observa o criador; comandos/chosen inline observam participantes em RAM,
com flush transacional no fechamento. Não há consultas de membros em massa,
escrita DB por carta, nem cache global como identidade. Última observação prevalece
por timestamp. Nomes/username podem mudar; UserID é definitivo.

> **Importação de ranking antigo: DEFERRED**
>
> *Old ranking import is deferred to a future milestone.*
>
> O parser, staging idempotente e reconciliação pura em `internal/rankingimport` e as tabelas `ranking_imports` / `ranking_import_entries` (migration `0004_imports.up.sql`) permanecem preservados como fundação técnica no repositório.
> No entanto:
> - Nenhum comando de importação foi criado;
> - Nenhum score importado é aplicado;
> - Nenhuma regra de conversão Legacy → Updated ou multiplicador foi assumida;
> - A UX de importação completa não faz parte do fechamento desta M7.


## Ambiente e operação

Novas variáveis:

- `DATABASE_URL`: obrigatória no runtime V2; conexão compartilhada entre migrator e aplicação.
- `TEST_DATABASE_URL`: somente testes de integração, em uma base exclusiva de testes.

```sh
# Configure o .env conforme .env.example
go run ./cmd/bot
# Em produção: ./bin/unobotgo
```

Startup tem prazo de 10s para conexão e 2m (política interna) para migrations/verificação antes de qualquer serviço funcional. DB offline, timeout, migration inválida ou schema incompatível encerram com erro e código não zero, sem HTTP/readiness/bot/workers parciais. Erros identificam estágio/migration e preservam a causa, sem registrar URL/senha ou detalhes de linhas.
Fechamento tem prazo de operação 10s e cleanup com prazo próprio 3s. Nenhuma
jogada/compra/turno/stack/challenge/join/leave precisa consultar PostgreSQL durante
a partida; apenas a transição terminal chama persistência. Polling segue recomendado.

Docker V2 multiarch inicia apenas `/unobotgo`; migrations estão embutidas e são aplicadas pelo mesmo startup, sem wrapper, serviço ou binário adicional. Não foram adicionados Redis/Kubernetes.
Migrations usadas pela versão nova exigem runtime compatível; rollback operacional
não deve apagar dados nem executar down migrations sem autorização/backup.

## Validação executada

Passaram localmente:

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./...
go test -tags debugcards ./...
go vet -tags debugcards ./...
go build -tags debugcards ./...
TEST_DATABASE_URL='<postgres-de-testes>' go test -race -tags integration ./internal/storage/postgres/...
git diff --check
```

PostgreSQL 17 em container efêmero local, sem volume de produção; cada teste usa
schema próprio e remove somente esse schema. Integração cobre migrations/reapply/
concorrência/checksum/rollback, defaults/config, acúmulo Legacy/Updated,
GameID/retry concorrente/hash divergente, rollback após game/player/stats parciais,
sistemas incompatíveis, pending sem score, usuários e import idempotente.
Unit tests cobrem autorização, snapshot/overrides, fórmula N2/N3/N8/N20 e half-up,
Unicode/duplicatas/ambiguous, retenção final fora do FIFO/reset, fechamento síncrono
bloqueado em commit e gameplay disponível enquanto persistência aguarda.

Não houve limitação local de ThreadSanitizer/VMA nesta execução; race passou.
Workflow CI foi atualizado com container de serviço PostgreSQL e suíte completa passando.

## Homologação manual no Telegram real

Os seguintes fluxos e cenários foram homologados manualmente no Telegram real:

### 1. Sistema de Ranking Legacy
- **N=2**: Conclusão da partida com persistência e concessão correta de pontos aos concluintes.
- **N=3**: Conclusão da partida com persistência e anúncio de pontuação pós-commit.

### 2. Sistema de Ranking Updated
- **N=2**:
  - 1º lugar = `+10,00`
  - 2º lugar = `+0,00`
- **N=3**:
  - 1º lugar = `+10,00`
  - 2º lugar = `+5,00`
  - 3º lugar = `+0,00`

### 3. Abandono definitivo
- Jogador que saiu e não voltou (`/sair` sem reentrada) ficou categorizado como `(fora do ranking)`;
- Não entrou na contagem de participantes elegíveis (`N`);
- Recebeu `+0,00` pontos;
- Os demais jogadores elegíveis foram pontuados normalmente pelo total de concluintes.

### 4. Saída + Reentrada
- Jogador utilizou `/sair`, reentrou na partida enquanto a sala estava aberta via `/entrar`, concluiu a partida normalmente e recebeu pontuação de acordo com sua colocação final conquistada.

### 5. Configuração do Grupo (`/config`)
- Comando `/config` operacional em grupos exibindo interface interativa com botões inline;
- Seleção de modo de jogo padrão (`Clássico` / `Caseiro`) funcionando com persistência;
- Seleção de sistema de ranking (`Legado` / `Atualizado`) funcionando com persistência;
- Defaults `Classic` + `Legacy` operacionais sem necessidade de qualquer configuração prévia;
- Tentativa de alternar o sistema de ranking em grupo com histórico de pontuações já acumulado no sistema anterior foi devidamente recusada com alerta pop-up (`ErrNeedsProductDecision`) e preservação da configuração original.

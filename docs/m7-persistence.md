# M7 — persistência, configuração e ranking (fundação)

Revisão: 2026-09-27, somente `dev`. Base auditada antes do BUILD:
`e72cd66afd667681a7d8fb805643927c06760f53`.
A M7 completa permanece pendente de decisões de produto; não é uma entrega de
ranking automático homologado.

## Commits desta implementação

| Commit | Escopo |
|---|---|
| `8b46c2d` | M7.1: pgx, migrations, configuração, startup e serviço PostgreSQL de CI |
| `d4e34ee` | M7.2 base: defaults persistentes, API/autorização e snapshot por partida |
| `806e022` | M7.3 base: cálculo, resultado público final, transação/idempotência e fechamento síncrono |
| `35e72de` | M7.4: usuários conhecidos e nomes observados |
| `5364016` | M7.5 base: parser, reconciliação e staging de imports |
| `6e60203` | Limitar cleanup de transações em falha/cancelamento |
| `9dc0cd8` | Elegibilidade definitiva: concluintes com placement válido, abandono fora de N e zero pontos |
| `25f7f90` | Persistência postgres: migration 0005, concessão e stats somente para elegíveis em N>=2 |
| `e1ebb41` | Integração telegram: anúncio de pontuação pós-commit após a mensagem final compacta |

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
- `cmd/migrate`: migrations explícitas sem TOKEN.
- `cmd/bot`: verifica PostgreSQL/schema antes do transporte Telegram.
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

`schema_migrations` registra versão/checksum. Migrations são aplicadas em transação,
serializadas com advisory lock; alterações/versões desconhecidas são rejeitadas.
Não há down migration destrutiva. O runtime não executa DDL automaticamente.

Resultado e ranking, quando houver policy explícita, usam a mesma transação.
GameID+hash confirmam retries idênticos; conteúdo divergente é recusado. Lock da
configuração serializa conclusões do grupo. Stats com outro sistema recusam a
transação integralmente. Não há reset ou conversão silenciosa.

## Ranking e elegibilidade definitiva

100 unidades inteiras = 1 ponto. Legacy concede 100 unidades para posições antes
do último e 0 ao último. Updated calcula `1000*(N-position)/(N-1)`, arredondado a
unidades inteiras pelo método half-up. Exibição preparada com vírgula e duas casas
decimais para Updated (`fmt.Sprintf("%d,%02d", p.Score/100, p.Score%100)`) e inteiro
para Legacy.

A política de elegibilidade definitiva foi aprovada e integrada (`completed-placements-v1`):
- Participam do cálculo (`N`) e recebem pontuação apenas os jogadores que concluíram
  efetivamente a partida e possuem colocação válida no resultado.
- Abandono definitivo: jogador que sai e não retorna recebe 0 pontos, fica fora de `N`,
  sem colocação artificial, permanecendo no registro persistido para fins de auditoria
  e exibido no Telegram como `(fora do ranking)`.
- Late join e saída com reentrada válida: se o jogador concluiu normalmente a partida,
  participa de `N` e pontua pela sua colocação final sem penalidade.
- Partidas com `N < 2` elegíveis (ex: encerramento por departure com 1 jogador restante)
  são persistidas para auditoria com status `insufficient_eligible_players`, com scores
  zerados e sem alteração de `player_group_stats`. Partidas canceladas não são persistidas.
- A finalização síncrona aguarda o COMMIT do PostgreSQL e, quando confirmado com concessão
  de pontos (`commit.Scored == true`), dispara uma mensagem adicional de pontuação logo
  após a mensagem final compacta do jogo. Em caso de retry já persistido (`AlreadyPersisted`),
  a mensagem de pontos não é reenviada. Falhas temporárias de DB retêm o resultado na
  memória do serviço para retry síncrono via `RetryPendingResults`.

Continuam NEEDS PRODUCT DECISION:

- Troca Legacy/Updated em grupos com histórico ou score acumulado.
- UX do setup inicial, superfície definitiva de configuração e comandos dedicados.
- Aproximação/confirmação de import Updated; nenhum fator ×5 aprovado/implementado.
- Reavaliação de resultados legados em status `needs_product_decision`.

## Configuração e permissões

GetOrCreate cria Classic+Legacy sem setup. `/novo` consulta config uma vez;
`/novo classico` e `/novo caseiro` sobrescrevem apenas a partida. Ranking/revision
ficam congelados na criação; regras efetivas do lobby determinam modo no início,
preservando o seletor de modo existente.

`groups.Service` valida a cada operação de modo: admin atual OU instalador ainda
membro. Falha na consulta de associação recusa ação. A integração Telegram de
setup/callbacks e instalador ainda não foi ligada: `my_chat_member` não é recebido
pelo pipeline atual e não se inventa instalador a partir de mensagens comuns.
A Bot API oferece `ChatMemberUpdated.From`; a futura integração deve registrar
apenas uma transição confiável de instalação e validar membros atuais via
GetChatMember, cuja garantia para outros usuários requer bot administrador.
Nenhum comando de configuração novo foi criado.

## Usuários e import

`/novo` observa o criador; comandos/chosen inline observam participantes em RAM,
com flush transacional no fechamento. Não há consultas de membros em massa,
escrita DB por carta, nem cache global como identidade. Última observação prevalece
por timestamp. Nomes/username podem mudar; UserID é definitivo.

Parser preserva Unicode, emojis, combining marks e invisíveis. Usa o último sufixo
` - inteiro`; inválidos permanecem com erro auditável. Nomes iguais geram entradas
separadas. Match literal por display name ou @username explícito; múltiplos
candidatos/entradas reivindicando UserID ficam ambiguous sem escolha arbitrária.
Staging é idempotente pelo hash da fonte/chat e não altera stats. Reconciliação
pura está preparada; UI de resolução/manual link e aplicação oficial são futuras.
Não há conversão para Updated nem aplicação automática de import Legacy.

## Ambiente e operação

Novas variáveis:

- `DATABASE_URL`: obrigatória em cmd/bot/cmd/migrate.
- `TEST_DATABASE_URL`: somente testes de integração, em uma base exclusiva de testes.

```sh
export DATABASE_URL='postgres://unobot:senha@localhost:5432/unobot?sslmode=disable'
go run ./cmd/migrate
# TOKEN configurado no ambiente ou .env
go run ./cmd/bot
```

Bot tem prazo de 10s para conexão/verificação antes de Telegram. DB offline/schema
incompatível encerra com erro claro e código não zero, sem registrar URL/senha.
Fechamento tem prazo de operação 10s e cleanup com prazo próprio 3s. Nenhuma
jogada/compra/turno/stack/challenge/join/leave precisa consultar PostgreSQL durante
a partida; apenas a transição terminal chama persistência. Polling segue recomendado.

Docker V2 multiarch continua usando banco externo; preparar migrations pelo comando
acima antes de iniciar a imagem. Não foram adicionados Redis/Kubernetes/Makefile.
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
Workflow CI foi atualizado, mas nenhuma execução remota nova foi disparada sem push.
Homologação no Telegram e build Docker no CI permanecem pendentes.

## Homologação manual no Telegram

1. Em ambiente separado, aplicar migrations e iniciar em polling. Confirmar que
   DB offline ou schema antigo impedem startup com erro, sem credentials nos logs.
2. Grupo sem setup: `/novo` Classic/Legacy; conferir os dois overrides e o seletor
   existente de modo no lobby, sem alteração permanente do default.
3. Jogar Clássico e Caseiro com stacking, +4 challenge e troca de mãos. Conferir
   late join/reentrada/room lock/timeout/colocações e mensagens já homologadas.
4. Durante uma partida, interromper DB de testes: cartas, compra e entrada devem
   continuar; encerramento registra erro e mantém DTO. Reestabelecer DB e testar
   retry síncrono pela API de aplicação em harness, sem comando novo no bot.
5. Após término, conferir uma completed_game e seus participantes, sem mãos/deck.
   Na entrega atual: status pending, score NULL, nenhum ponto concedido.
6. Cancelar ou reiniciar durante partida: nenhum resultado/ponto parcial persistido.
7. Conferir conhecidos com Unicode/renomeação, mantendo UserIDs distintos.
8. Setup real, ranking público e aplicação de import só poderão ser homologados
   após as decisões pendentes; esta entrega não fornece essas interfaces.

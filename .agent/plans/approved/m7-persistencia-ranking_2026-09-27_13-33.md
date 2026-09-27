# Plano: M7 — Persistence, Group Configuration & Ranking

## Pedido do usuário
Introduzir PostgreSQL, configuração por grupo, ranking Legado/Atualizado e base auditável de importação na V2. Partidas ativas permanecem exclusivamente em memória. Trabalhar na dev em commits pequenos após audit/plan, preservar gameplay e não inventar regras competitivas pendentes.

## Objetivo
Persistir somente resultados de partidas oficialmente concluídas, com score/statistics atômicos e idempotentes por GameID; PostgreSQL fora do processamento das ações. Construir fundação independentemente de decisões de produto ainda bloqueadas.

## Contexto atual — HEAD auditado
- Checkout/pull executados; dev limpa e atualizada em e72cd66afd667681a7d8fb805643927c06760f53 (`fix(game): handle stacked draw four bluff and player reentry`).
- go.mod: Go 1.26.3, telego 1.10.0 e godotenv. lib/pq aparece como indireta por código legado; V2 não possui armazenamento externo. Não reutilizar V1.
- GameID: manager.create chama randomID (16 bytes crypto/rand, 32 dígitos hex) antes de criar/publicar lobby.
- internal/uno é transacional por cópia/revision; internal/game serializa por partida e mantém índices/histórico em memória. manager.publish guarda PublicGameView final e descarta engine ao terminar.
- Placement contém PlayerID, Position e WentOut. Último recebe WentOut=false; GameFinished contém FinishReason. completed, departure e cancelled são distintos. Saída pode encerrar por departure, que não comprova conclusão competitiva normal.
- Players retém Left sem placement; reentrada reutiliza PlayerID/registro, compra nova mão e preserva cauda lógica. WentOut/colocados não reentram. Late join permanece permitido conforme regras/sala. Resultado final não informa instante da entrada nem histórico completo de saídas/reentradas.
- /novo escolhe classic no parser e caseiro por argumento; handleNovo cria Rules diretamente. Seletor de modo no lobby muda Rules. Modos devem ser obtidos do estado real, não do texto histórico.
- Nomes estão no UserCache de apresentação, não no Player/resultado da engine. Precisamos capturar nomes por sessão para que evicção/mudança posterior não reescreva auditoria.
- Telegram allowedUpdates contém apenas message, inline_query, chosen_inline_result, callback_query. processUpdate não trata MyChatMember. GetChatMember já existe para /reset; a telego expõe MemberStatus/MemberIsMember e ChatMemberUpdated.From.
- Polling recomendado; webhook experimental. main tem promoção seletiva, sem merge. Docker V2 estático multiarch; workflow dev-ci valida Go/race/Docker.
- go test ./... passou. Race de uno/game passou neste ambiente atual (não presumir limitação histórica de VMA). Docker daemon disponível, PostgreSQL local não identificado.

## Arquivos analisados
- AGENTS.md; README.md; docs/project-status.md; docs/branching.md
- .agent/context.md; go.mod; .env.example; Dockerfile.v2; .github/workflows/dev-ci.yml
- cmd/bot/main.go; internal/config/config.go
- internal/uno/game.go, state.go, player.go, event.go, rules.go, action.go
- internal/game/manager.go, service.go, views.go
- internal/telegram/bot.go, transport.go, commands.go, callbacks.go, inline.go, client.go
- Tipos ChatMemberUpdated/Message da telego e documentação oficial Telegram/pgx

## Arquivos que poderão ser modificados
- Novos pacotes internal/groups, internal/ranking, internal/storage/postgres e internal/rankingimport
- Migrations SQL versionadas incorporadas ao adapter PostgreSQL; cmd/migrate
- internal/game (metadata de sessão/resultado final, sem SQL nem pgx)
- internal/telegram (injeção de serviços, setup mínimo, observação/finalização)
- cmd/bot; internal/config; go.mod/go.sum; .env.example; Dockerfile.v2 somente se necessário
- Workflow dev-ci para testes com PostgreSQL; documentação técnica/status e .agent

## Estratégia de implementação

### Fronteiras
- internal/uno permanece independente de DB/configuração/grupo/Telegram; não alterar regras homologadas.
- internal/game recebe metadata imutável de criação (ranking_system, config_revision) e observação confiável de nomes. Modo final reflete a escolha efetiva do lobby, congelada ao iniciar; defaults/override só escolhem o modo inicial. Produce DTO final sem mãos/deck na transição terminal antes de descartar engine; nenhuma chamada a repository no lock/caminho de ações.
- internal/groups define GroupConfig, enums classic/caseiro e legacy/updated, interfaces ConfigRepository e MembershipVerifier, serviço de configuração. Autorização no serviço, com consulta atual ao verifier Telegram, não apenas callback/interface.
- internal/ranking define ScoreUnits int64, cálculo puro, FinalResult, FinalizationRepository e serviço de finalização. PersistCompletedResult é uma única operação transacional; não distribuir escrituras de stats/resultados por repositories independentes.
- internal/storage/postgres implementa interfaces, SQL, pool/migrations. Injetado em cmd/bot; não importado por uno/game. Escolha proposta: pgx/v5 + pgxpool, versão estável fixada compatível com Go 1.26.3 ao implementar. SQL explícito parametrizado, sem ORM/global DB.
- Worker separado para persistência/finalização e observação de usuários. Ações não fazem leitura/escrita de DB nem aguardam worker. /novo e callbacks de configuração podem consultar DB fora dos locks da partida; falhas retornam mensagem clara, sem fabricar defaults quando o DB está indisponível.

### Schema mínimo proposto
Todos IDs Telegram BIGINT; horários TIMESTAMPTZ UTC; texto UTF-8. score_units BIGINT, escala 100 compartilhada com auditoria. PK/FK/checks e índices mínimos.

1. group_configs: chat_id PK, default_game_mode CHECK, ranking_system CHECK, installed_by_user_id nullable, installed_at nullable, installation_update_id nullable, config_revision BIGINT, created_at, updated_at. INSERT ON CONFLICT preserva configuração existente, defaults classic/legacy. Evento de instalação tardio não sobrescreve evento mais recente.
2. known_group_users: (chat_id,user_id) PK, display_name, username nullable, last_seen_at. Atualização monotônica por horário observado; não sobrescrever com eventos antigos. Não é fonte de identidade por nome.
3. completed_games: game_id TEXT PK (compatível com IDs atuais, sem converter para UUID), chat_id FK, game_mode, ranking_system do snapshot, config_revision, started_at, finished_at, final_revision, finish_reason, payload_hash, participant_count, scoring_status (needs_product_decision/scored), policy_version nullable, scored_at nullable. Somente encerramento normal oficialmente concluído é candidato. cancelled/departure não recebem pontos. Política sobre persistir departure está bloqueada abaixo; não chamá-lo de vitória normal.
4. completed_game_players: (game_id,user_id) PK, observed_name, observed_username nullable, final_status, position nullable, went_out, score_units nullable, joined_after_start, leave_count/reentry_count. UNIQUE(game_id,position), posição positiva quando existe. Resultados incluem evidência, não inventam placement para Left. NULL em score significa ainda não avaliado, não zero concedido.
5. player_group_stats: (chat_id,user_id) PK, ranking_system CHECK, score_units, completed_games_count, wins_count, last_finished_at, updated_at. Um acumulado por usuário/grupo; nenhuma PK por sistema criando rankings paralelos. Constraints impedem soma de sistemas incompatíveis. Estatísticas de elegibilidade condicionadas à decisão de produto.
6. ranking_imports: import_id PK, chat_id FK, created_by_user_id, created_at, source_hash, source_system=legacy, raw_text, status staged/applied, applied_at nullable. UNIQUE(chat_id,source_hash). Sem aplicar import nesta milestone enquanto políticas relevantes pendentes.
7. ranking_import_entries: entry_id PK, import_id FK, line_number, imported_name, source_score_units, linked_user_id nullable, status unresolved/linked/ambiguous/invalid, error nullable. UNIQUE(import_id,line_number), jamais UNIQUE(name). Preservar entradas e nomes originais, inclusive duplicados e linhas inválidas.
8. result_notifications: game_id PK/FK, payload de pontuação/ranking composto na transação, delivery_status pending/sent, attempts, next_attempt_at, sent_at nullable. Outbox somente para resultado/ranking pós-commit, não para jogadas.
9. schema_migrations: version PK, checksum, applied_at. Migrations .up.sql versionadas, aplicadas transacionalmente sob advisory lock; validar checksums. Startup valida versão esperada. Aplicação por cmd/migrate explícito, sem DDL destrutivo automático.

### Transação e idempotência
- Validar DTO/hash e política antes de conceder score. Inserir completed_games por GameID, resultados, atualizar stats e criar notificação em uma transação. Commit é a confirmação definitiva.
- Mesmo GameID/hash retorna resultado já persistido sem reaplicar pontos; mesmo ID com payload divergente retorna conflito observável. Testar duas conexões concorrentes, rollback e commit de resultado desconhecido seguido de retry.
- Linha group_configs bloqueada antes das stats; ordenação de user_id reduz deadlocks. Revalidar compatibilidade de sistema sob lock. Nenhuma soma de legacy/updated incompatível.
- Uma partida concluída cuja elegibilidade depende de produto pode ser arquivada como needs_product_decision com scores NULL, sem alterar stats/notificação de pontuação. Aprovação futura da política permite transição idempotente única para scored. Não aplicar uma política presumida.
- DTO final captura nomes e evidência em memória antes de runtime ser descartado, independentemente de HISTORY_LIMIT. Saídas/entradas não persistem durante jogo; metadata é capturada só em memória.
- Mensagem final compacta existente permanece imediata. DTO final é entregue a worker independente (mapa de pendências em memória com sinalização coalescida, sem descartar resultado por saturação de canal). Retry limitado por tentativa, backoff e log de backlog. Persistência não bloqueia tarefas de jogada do mesmo grupo/nova partida.
- Depois de commit, outbox permite entrega/retry após restart. Bot API não oferece idempotência geral de SendMessage: crash entre envio e marcação pode duplicar notificação; score não duplica. Não prometer exatamente uma mensagem.
- Crash antes de persistir uma conclusão pendente perde esse DTO e não concede pontos. Garantia de recuperar conclusões ainda não commitadas exigiria journal durável adicional; não incluí-lo silenciosamente. Esse limite requer aceite operacional.
- /reset apaga estado da partida/grupo em memória conforme hoje; não apaga config, estatísticas ou resultados duráveis. Pendência final já concluída é independente do runtime e não deve ser descartada pelo reset.

### Configuração/snapshot e instalação
- GetOrCreateGroupConfig usado ao criar partida e ao observar instalação; setup opcional.
- /novo sem argumento usa default persistente. /novo classico e /novo caseiro fazem override sem alterar grupo. Grupo sem instalação observada recebe installed_by=NULL, não inventar dono/instalador.
- RankingSystem congelado na criação. Modo inicial vem da config/override; selector do lobby já homologado continua funcionando, e modo jogado é fixado em StartGame. Mudança posterior de configuração não reescreve sessão existente.
- Receber my_chat_member em ambos transportes; detectar transição left/kicked -> membro/admin, usar From quando confiável/identificável, respeitar ordem de eventos e atualizar instalador apenas na reinstalação observada. Mudança member -> admin não equivale a nova instalação. /startgroup sozinho não comprova quem instalou.
- Serviço de config verifica isCurrentAdmin OU installedBy==actor E isCurrentMember. Restricted só conta como membro quando is_member true. Erro/ausência de resposta Telegram recusa alteração; não confiar em nome/cache. GetChatMember de outros usuários só é garantido quando bot é administrador: documentar limite.
- UX proposta, pendente de aprovação: uma mensagem de setup com duas linhas de botões (Classic/Caseiro e Legacy/Updated), marcando valores atuais; callbacks opacos vinculados ao chat e revisão, autorização revalidada. Sem /config ou novos comandos. Não decidir tamanho/recuperação do menu depois da instalação sem aceite.
- Alteração de modo padrão pode ocorrer sem alterar partida ativa. Troca de ranking com scores/partidas antigas pendentes é NEEDS PRODUCT DECISION; retornar erro específico e não resetar/converter/misturar. Corrida com resultados de snapshot antigo precisa da mesma política, mesmo com score ainda zero.

### Representação Updated
- ScoreUnits representa centésimos de ponto; primeiro=1000, último=0. Legacy +1=100 units; apresentação Legacy continua inteiro.
- Updated calculado por divisão inteira com arredondamento half-up: q=(1000*(N-position))/(N-1), r=resto; incrementar q se 2*r>=N-1. Não usar float; validar N>=2 e 1<=position<=N, proteger overflow.
- N=8: 1000,857,714,571,429,286,143,0. N=3:1000,500,0. Armazenar política/version e units de cada concessão. Exibição proposta Updated em uma casa decimal pt-BR, half-up a partir de units; definir exemplo no relatório para diferenciar arredondamento de armazenamento/exibição.
- Cálculo N=20 exigido pelos testes não altera o limite de 10 jogadores da engine.

### Conhecidos e importação
- Observar identidades de mensagens/callbacks com chat confirmado e resultados inline roteados por token, nunca inferir grupo de query inline sem contexto confiável. Buffer separado/coalescido, gravação fora do caminho crítico, snapshot de nomes da partida independente de DB.
- Parser usa último sufixo ` - <inteiro não negativo>` ancorado ao fim. Cabeçalho reconhecido é separado; demais linhas inválidas preservadas com diagnóstico. Unicode/emoji/combining/invisíveis e nome original preservados; números com overflow recusados. Não aceitar conversões silenciosas decimal/negativo.
- Cada linha tem ID próprio. Reconciliation apenas propõe candidato único por username/nome observados; duplicação de nome nas entradas ou candidatos múltiplos => ambiguous, nunca auto-link. Nome sem prova suficiente permanece unresolved; resolução manual futura por UserID.
- Importação staged/reconciliation não concede pontos. Aplicação Legacy idempotente fica preparada por interface/ledger; integração final e import Updated dependem de políticas/UX. Não hardcode ×5.

## Decisões bloqueadas — NEEDS PRODUCT DECISION
1. Quem conta em N e recebe elegibilidade quando há Left sem placement? Mantê-lo fora muda a fórmula dos demais. Não inferir abandono como último.
2. Late join e reentrada: elegíveis? Como contar participantes que entram depois? Final placement identifica quem terminou, mas não define política competitiva.
3. Encerramento departure: é partida pontuável ou apenas saída? O HEAD distingue de completed; não pontuar por suposição.
4. Requisitos mínimos de ranking (jogadores/tempo/etc.). Fórmula N>=2 não decide elegibilidade de partidas pequenas.
5. Troca de ranking com score existente e resultados antigos pendentes: conversão/reset/proibição/snapshot concorrente ainda sem regra.
6. UX de setup, mensagem adicional Updated/top ranking (quantidade de posições) e reabertura posterior sem novos comandos; implementação final da superfície espera aceite.
7. Aplicação/import Updated: aproximação e confirmação manual ainda indefinidas. Estrutura/parser podem avançar; aplicação não.
8. Aceite de perder conclusões que ainda não chegaram ao DB após crash (sem journal adicional) e de possível repetição de mensagem pós-commit.

Esses bloqueios não impedem schema, repositories, cálculo puro, snapshot/result capture, defaults/config API, transação testável ou import staging. Integração de score real fica bloqueada nos casos afetados; não declarar M7 integralmente concluída enquanto faltar regra indispensável.

## Passos detalhados e commits
0. Aprovar fase 1/este plano; mover para approved. Não implementar antes da aprovação.
1. M7.1: pgx/pool, interfaces de domínio, migrations, cmd/migrate, env e startup fail-fast; PostgreSQL service de CI e testes de migrations/transações. Commit `feat(storage): add postgres persistence foundation`.
2. M7.2: GetOrCreate defaults, config revision/autorização, metadata da sessão/snapshot do ranking e modo efetivo; overrides. Testes puros/mocks sem DB para game/engine. Commit `feat(groups): add persistent group configuration`.
3. M7.3a: calculator determinístico, resultado final sem mãos, metadata de participação/nomes, store transacional/idempotência, pending-product/status e outbox. Commit `feat(ranking): add legacy and updated ranking foundations`.
4. M7.4: conhecidos por interação natural fora das filas de gameplay; atualização monotônica/names por sessão. Commit `feat(groups): track observed group users`.
5. M7.5: schema/staging/parser e reconciliation conservadora preservando duplicatas, sem pontuação aproximada/aplicação arbitrária. Commit `feat(import): prepare legacy ranking reconciliation`.
6. M7.3b/Telegram: após decisões, ligar finalização e score policy, receber instalação/setup mínimo e mensagem adicional pós-commit. Manter compact final. Commit `feat(telegram): integrate group setup and ranking results`.
7. A cada etapa executar validações, reportar resultado/SHA e só então avançar. Sem push solicitado. Atualizar status/relatório técnico de M7, memória e decisões; plano só vai done quando escopo aprovado estiver concluído ou claramente reduzido pelo usuário.

## Riscos
- Escrita acoplada à jogada ou locks: proibir repository no Apply/publish; teste com DB fake travado verificando continuidade do gameplay.
- Pendência final evictada/resetada: extrair DTO no fechamento e manter fila independente antes de descartar runtime.
- Score duplicado: transação e PK GameID, conflitos de hash e concorrência real em PostgreSQL.
- Mudança de ranking e snapshot antigo: bloquear aplicação incompatível até política explícita.
- Perda de conexão/commit indeterminado: retry consulta GameID, nunca aplicar compensação de score sem confirmação.
- Import Unicode/nomes duplicados: preservar origem, IDs por entrada e ambiguidade explícita.
- Homologação Telegram não substituída por mocks.

## Impactos esperados
Dados de grupo, resultados finais e ranking duráveis; cartas/turnos/pilhas exclusivamente em memória. Reinício não concede pontos a partida interrompida. Sem Redis/RPG/recuperação ativa ou alterações de regras.

## Compatibilidade
- Linux/macOS/Windows: lógica Go; PostgreSQL externo.
- Docker: manter multiarch e build estático; DB separado. Sem novo Makefile/Kubernetes/Redis. Não reutilizar compose V1 como runtime V2.
- CI/CD: serviço PostgreSQL isolado para testes de integração; promoção seletiva main continua intacta.

## Como testar

### Build
```bash
go build ./...
go build -tags debugcards ./...
go vet ./...
```

### Testes
```bash
go test ./...
go test -race ./...
go test -tags debugcards ./...
TEST_DATABASE_URL=<postgres-de-testes> go test -tags integration ./internal/storage/postgres/...
git diff --check
```
- Unit tests uno/game sem DB/env/rede, suíte completa atual e invariantes de gameplay.
- Migrations DB vazio/reaplicação/checksum; rollback forçado após inserts/stats; duas finalizações concorrentes; retry pós-commit; hash divergente; não conclusão/abandono gate; nenhum ponto em PlayerWon intermediário.
- Config defaults/setup opcional/overrides/snapshot/lobby; permissões admin/instalador membro/instalador Left/usuário comum/role lookup falho; reinstallation e eventos antigos.
- Legacy N-1/+1/último0 e acúmulo; Updated N2,N3,N8,N20, extremos e monotonicidade, half-up, apresentação; nenhuma representação float persistida.
- Conhecidos/import Unicode/emoji/duplicados/último sufixo/invalid/overflow/ambiguous e repetição de import.
- DB offline durante gameplay não impede Play/Draw/Pass/Join/Leave/Challenge/timeout. Falha na persistência não torna resultado parcialmente pontuado.
- CI Linux suportado para race se surgir VMA/TSan local; registrar evidência específica, não atribuir limitação por histórico.

### Execução
DATABASE_URL obrigatória no runtime final da M7. Startup: parse URL sem logar credenciais, connect/ping com prazo, validar migrations; falha explícita e saída não zero antes de iniciar polling/webhook. Aplicar migrations antes com cmd/migrate. Shutdown drena finalizações por prazo e fecha pool; não promete durabilidade de pendências não commitadas.

## Rollback
Commits pequenos reversíveis; não executar migrations destrutivas/reset/rm sem autorização. Backup DB antes de alteração operacional. Versão antiga não compreenderá configuração/ranking novos; interromper runtime de forma coordenada, manter dados duráveis e migrations existentes. Não mexer main.

## Observações
Fase 1 concluída em auditoria/plano; somente arquivo de plano criado. Aguardar aprovação e registrar decisões de produto antes de integrar partes bloqueadas. Fontes: https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool ; https://core.telegram.org/bots/api#chatmemberupdated ; https://core.telegram.org/bots/api#getchatmember .


## Aprovação e alterações vinculantes — 2026-09-27

Usuário aprovou o BUILD com estas alterações, que substituem qualquer menção anterior incompatível:
- Sem worker ou fila assíncrona de partidas concluídas. Finalização constrói resultado imutável, aguarda transação atômica e COMMIT e só então publica pontuação. Resultado final permanece em memória para retry enquanto necessário. GameID garante idempotência.
- Sem result_notifications/outbox durável. Falha de envio Telegram após commit não invalida os dados.
- Nenhum snapshot privado de engine/mãos no banco.
- Decisões de produto permanecem bloqueadas; nenhum reset/conversão ou elegibilidade arbitrária.
- Memória não garante sobrevivência a crash antes de commit; nenhuma recuperação ativa ou journal adicional foi aprovada.

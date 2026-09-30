# Passagem de implementação — Mini App de ranking global

Data: 2026-09-30. Implementação **PARCIAL**, pausada a pedido do usuário para continuidade em outra IA. Não está pronta nem homologada.

## 1. Instrução de continuidade

Leia primeiro `AGENTS.md`, este documento e `.agent/plans/approved/miniapp-ranking-global_2026-09-30_11-20.md`. O usuário aprovou expressamente a implementação desse plano; não é necessário repropor o mesmo escopo. Revalide o working tree antes de continuar. Se precisar ampliar o escopo, peça aprovação.

Trabalhe **somente na dev**. Preserve todos os arquivos modificados e não rastreados. Não use reset, restore ou stash automático. **NÃO faça commit, push, amend, rebase, merge/cherry-pick para main, deploy, publicação de container ou migration em produção.** Tudo deve permanecer no working tree para homologação manual.

Última instrução do usuário: gerar este documento para continuar em outra IA. Não interpretar a passagem como conclusão da feature.

## 2. Fontes de verdade e decisões aprovadas

- Plano completo: `.agent/plans/approved/miniapp-ranking-global_2026-09-30_11-20.md` (movido de pending após aprovação).
- Mockup oficial: `mockup_de_rankings_uno_em_iphones-2.png`, arquivo do usuário, não rastreado; preservar. A imagem é requisito de produto, não inspiração. **As cores da imagem prevalecem sobre qualquer descrição textual como “header global azul”.** Não substituir por dashboard, tabela ou Material UI genérico.
- Backend Go, pgx e SQL versionado existentes; frontend React/TypeScript/Vite/Tailwind/TanStack Query/React Router no mesmo repositório.
- Build frontend → web/dist → go:embed → um binário Go de runtime. Node LTS/npm só no build/dev.
- `score_units` JSON como string decimal inteira; BigInt no frontend. Nada de Number para int64 arbitrário ou floats para pontos.
- Referências opacas com MINIAPP_SECRET aprovadas; interface só exibe `ID ••••1234`.
- Runtime: config → PostgreSQL → advisory lock → verify/apply migrations → release lock → HTTP/API/Mini App → bot/polling/webhook/workers. Falha de migration impede qualquer serviço funcional.
- HTTPS/direct link/BotFather serão configurados posteriormente pelo usuário para homologação.
- Mês atual exclusivamente calculado pelo backend em America/Sao_Paulo; sem seletor/histórico.
- Universos Updated/Legacy separados; default Updated + Grupos. Players não clicáveis; grupos abrem detalhe.
- Fórmulas, elegibilidade, abandono, idempotência e gravação transacional existentes NÃO devem mudar.

## 3. Base Git e estado ao pausar

Branch dev, base observada `c86556b` (origin/dev também). Antes da implementação só havia plano e mockup não rastreados. Ranking mensal, privado e /config recentes já estavam commitados nessa base.

Último status verificado na passagem:

```text
## dev...origin/dev
 M .gitignore
 M cmd/bot/main.go
 M internal/storage/postgres/migrations.go
 M internal/storage/postgres/ranking.go
 M internal/telegram/bot.go
 M internal/telegram/client.go
 M internal/telegram/commands.go
 M internal/telegram/ranking.go
?? .agent/plans/approved/miniapp-ranking-global_2026-09-30_11-20.md
?? internal/app/
?? internal/config/web.go
?? internal/httpapi/
?? internal/media/
?? internal/ranking/global.go
?? internal/ranking/global_test.go
?? internal/storage/postgres/global_rankings.go
?? internal/storage/postgres/global_rankings_integration_test.go
?? internal/telegram/avatar.go
?? internal/telegram/shared_http.go
?? mockup_de_rankings_uno_em_iphones-2.png
?? web/
```

Este documento será mais um arquivo novo. `git diff --stat` não inclui arquivos não rastreados: não confundir o diff pequeno dos arquivos existentes com o tamanho real da implementação.

Nenhum commit/push foi executado. main não foi alterada. Nenhuma migration de produção, deploy ou publicação foi executada.

## 4. Arquitetura existente que deve ser preservada

- V2 em `cmd/bot`; V1 permanece na raiz. Não reorganizar o repositório só para seguir uma árvore idealizada.
- `player_group_monthly_stats` já é a fonte mensal: chat_id/user_id/month_start, ranking_system, score_units, completed_games, wins, display_name, last_finished_at.
- `RecordCompletedGame` já grava acumulado/mensal atomicamente, exclui abandonos e é idempotente por GameID/hash. Nenhuma alteração nessa escrita foi feita.
- `group_configs.title` persiste título. Evitar fallback público antigo que expõe ChatID completo.
- `ranking.Service` atende Telegram; helpers de mês em `internal/ranking/time.go` usam America/Sao_Paulo e tzdata embutida.
- Desempate interno: score DESC, última posição elegível ASC, conclusão DESC, user_id ASC, posições únicas.
- Migrations mais recentes: 0006_monthly_ranking e 0007_group_title. Não foi criada 0008. Não editar SQL aplicado.
- `Store.Migrate` **já tinha** advisory lock transacional `71870101`, ledger/checksums, commit/rollback. Foi reutilizado, não substituído.
- `telegram.New` inicia dispatcher; ordem de construção importa para startup.
- Antes havia HTTP próprio apenas no webhook; polling não tinha API. O novo caminho compartilha HTTP, mas caminho antigo standalone permanece para compatibilidade/testes.
- Dockerfile raiz/Makefile/compose ainda têm caminhos V1; Dockerfile.v2 é o utilizado pelos workflows e já era distroless nonroot.

## 5. Código já escrito — requer revisão e testes completos

### Ranking/SQL

- `internal/ranking/global.go`: GlobalRepository, GlobalService, request/page/row/key de paginação, relógio injetável, validação de sistema/mês/limite, máscara de ID sem overflow para MinInt64.
- `internal/storage/postgres/global_rankings.go`: agregações grupos/players por mês/sistema; nome mais recente no universo; detalhe elegível; keyset; row_number para posições; total/cabeçalho e página em uma instrução. SUM(bigint)::bigint falha em overflow em vez de arredondar.
- CTE `eligibleMonthlyLatest` compartilhada com `ListGroupRanking`; `ranking.go` passou a usar intervalo temporal do mês e novos parâmetros, preservando wrapper Telegram/limite512.
- `global_rankings_integration_test.go` foi criado, mas **ainda não foi executado**. Cobre agregado, sistemas/meses, paginação, empate, nome, abandono, duplicação GameID, detalhe vs Telegram e alteração não commitada.
- Não há EXPLAIN executado nem índice novo. Queries reais ainda precisam ser provadas no PostgreSQL.

### HTTP, autenticação e identificadores

- `internal/httpapi/auth.go`: HMAC initData original, hash constante, chaves duplicadas recusadas, auth_date/idade/futuro, user validado após assinatura, tamanho limitado.
- `references.go`: AES-GCM para refs/cursors com expiração e escopo; HMAC separado para chave estável. Secret32bytes.
- `server.go`: API GET grupos/players/detalhe/media, DTO sem raw IDs, score string, metadados do mês, cursor opaco, rate limit em memória por usuário e limite de concorrência; timeout de consulta; erros genéricos.
- `static.go`: estáticos, fallback SPA, assets hash com cache longo/index no-cache, CSP/nosniff/referrer-policy.
- `auth_test.go` e `server_test.go`: testes iniciais passam (ver validações abaixo), mas cobertura precisa ser ampliada.

### Fotos

- `internal/media/service.go`: LRU 2000 entradas/64MiB, TTL sucesso6h/ausência1h/erro1min, fila64, quatro workers e ticker compartilhado350ms; leitura de ranking não chama Telegram.
- `internal/telegram/avatar.go`: GetChat/GetUserProfilePhotos/GetFile, download backend somente api.telegram.org HTTPS, sem redirect, timeout/limite2MiB e MIME raster permitido. Sem token no browser ou erro retornado.
- Cache é de foto em memória; nenhuma tabela nova. Endpoint 202 pendente/204 ausente; frontend usa fetch autenticado e Blob URL.
- SafeAPICaller deixou de registrar err.Error() no debug de erro de transporte para evitar URL/token nos logs.
- **Ainda faltam testes de cache/source/429/redaction.** Revisar se a estratégia atual satisfaz cooldown global e deduplicação sob evicção: o código inicial não deve ser presumido completo.

### Startup e bot

- `internal/config/web.go`: WEB_ADDR, alias WEBHOOK_LISTEN_ADDR com conflito recusado, MINIAPP_LAUNCH_URL, MINIAPP_SECRET obrigatório, INITDATA_MAX_AGE e MIGRATION_TIMEOUT.
- `internal/app/app.go`: migrar/verificar antes de listener; HTTP único; atomic readiness e handler webhook; construir bot depois do listener; supervisão/cancelamento/shutdown.
- `cmd/bot/main.go` foi simplificado para config/log/signal/app.Run; `--dev` permite assets ausentes sem bypass de autenticação.
- `internal/telegram/shared_http.go` + alterações em bot.go: modo webhook no servidor compartilhado e callback de readiness; defer para drenar dispatcher.
- `SetMiniAppURL` e markup URL comum em respostas de ranking grupo/privado (também caminho compartilhado pós-partida). Ausência de URL omite botão; startup avisa.
- `migrations.go`: somente mensagem/comentário atualizados; sem alteração dos SQLs ou semântica.
- Teste de Initialize fail-closed existe; testes de startup real/listener/webhook/concurrency ainda insuficientes.

### Frontend

- Criado `web/package.json`, package-lock gerado por npm install, tsconfig estrito, Vite/Tailwind/Vitest, ESLint, index e scripts/build.mjs.
- React19, Router7, Query5, Vite7, Tailwind4; versões resolvidas no lock. Node24. npm avisou que ESLint9 está fora de suporte; avaliar atualização compatível e rerodar gates, sem ignorar o aviso.
- `web/embed.go` usa `all:dist`; `.keep` versionável, dist gerado ignorado; build recria placeholder.
- `src/api/client.ts`: fetch com initData em Authorization, DTOs string de pontos.
- `src/lib/score.ts`: BigInt, milhar/decimal pt-BR, singular Legacy.
- `src/lib/telegram.ts`: ready/expand, insets/viewport, BackButton; feature detection.
- `components/Ranking.tsx`: cápsulas, SVG medalhas top3, card/link só grupo, foto/fallback, skeleton, retry, score.
- `hooks/useRanking.ts`: infinite query por sistema/rota, cursor, detecção de duplicação/posição, refresh de período usando datas backend.
- `pages/Rankings.tsx`, `App.tsx`, `main.tsx`: global e detalhe com query params preservados; browser externo amigável; empty/error/loading.
- `styles.css`: primeira implementação da composição do mockup; **não foi feita inspeção por screenshot/browser nem aceite visual**. Adaptar cores somente pela imagem, não pelo rótulo textual do plano.
- `src/test/setup.ts` existe; **ainda não foram escritos os testes frontend da matriz**.
- `.gitignore` passou a ignorar bin, node_modules, dist gerado e coverage, preservando dist/.keep.

## 6. Validações observadas, sem inferir resultados

Antes da implementação (auditoria): `go test -count=1 ./...` passou.

Durante implementação:

1. Primeira compilação detectou campo/método `key` conflitante e import telego ausente. Corrigidos (`keyBytes`, import).
2. `go test ./internal/app ./internal/httpapi ./internal/telegram` passou naquele ponto (app/httpapi ainda sem testes nessa execução).
3. Depois de adicionar testes: `go test ./internal/httpapi ./internal/app ./internal/ranking` **passou**.
4. `npm --prefix web run build` **passou**, incluindo tsc: 89 módulos, JS ~305.56kB (~96.55kB gzip), CSS ~11.46kB naquele build.
5. `npm --prefix web run lint` falhou com exhaustive-deps no useCallback de Rankings.tsx. Corrigi desestruturando fetchNextPage. Reexecução foi iniciada, mas **não foi possível recuperar o resultado após troca de sessão/permissões**; rerodar.
6. `git diff --check` passou ao gerar esta passagem (não verifica novos arquivos ainda não rastreados).
7. PostgreSQL integration, race, suite Go completa após mudanças, debugcards, vet, Make, Docker e testes frontend **não foram concluídos**.

Não declarar a feature completa baseada nesses gates parciais.

## 7. Ambiente e ferramentas temporárias

- Linux arm64, Go1.26.3, CGO_ENABLED=0. Node/npm/Docker ausentes inicialmente no PATH.
- Baixei Node24.21.0 oficial, extraí em `/tmp/unobotgo-tools.7nJqZf/node-v24.21.0-linux-arm64`.
- SHA256 do tar.xz conferido com SHASUMS256 oficial: `6ad1325edbdb5649c379b75a237147a666c95d4f9ae8d340fef2d1575d289ad2`.
- Para npm se o diretório ainda existir:

```bash
export PATH=/tmp/unobotgo-tools.7nJqZf/node-v24.21.0-linux-arm64/bin:$PATH
```

- npm install gerou lock e node_modules; não instalou ferramentas globais. npm ci ainda deve ser validado.
- PostgreSQL18.6 está disponível via binários Termux. Já havia servidor no socket default, mas conexões read-only falharam por roles inexistentes (`u0_a395` e `postgres`). Não alterar roles/dados desse servidor nem usar credenciais de produção.
- Para isolamento, iniciei criação de **cluster descartável novo** em `/tmp/unobotgo-pg.6rta9c/data`, usuário `unobot_test`, auth trust local, porta55439, bind127.0.0.1, socket `/tmp/unobotgo-pg.6rta9c`, log `server.log` nesse diretório.
- Comando iniciado:

```bash
initdb -D /tmp/unobotgo-pg.6rta9c/data -U unobot_test -A trust --no-locale
pg_ctl -D /tmp/unobotgo-pg.6rta9c/data -l /tmp/unobotgo-pg.6rta9c/server.log -o '-p 55439 -h 127.0.0.1 -k /tmp/unobotgo-pg.6rta9c' start
```

- **Resultado final não recuperado**: última saída era initdb selecionando shared_buffers. Inspecionar diretório/log/processo; não presumir servidor iniciado nem executar initdb sobre diretório parcialmente existente. Se estiver pronto, DSN exclusivamente desse cluster: `postgres://unobot_test@127.0.0.1:55439/postgres?sslmode=disable`.
- Os testes existentes usam schemas aleatórios e removem somente seus próprios schemas. Não rodar com DATABASE_URL de produção. Cluster temporário deve ser parado ao concluir, sem apagar diretórios sem aprovação.
- Docker não disponível na auditoria. Race pode exigir CGO_ENABLED=1; limitação histórica ThreadSanitizer VMA39 vs48 deve ser documentada exatamente se reaparecer, não mascarada.
- Permissões mudaram durante a execução para sandbox workspace-write. Comando normal falhou com `error building bubblewrap command: app-server socket directory has an unsupported host mount ...`. Consulta Git fora do sandbox foi autorizada. Se continuar ocorrendo, usar mecanismo de aprovação/escalation; não contornar permissões.
- Sessions antigas (6359 initdb, 93951 lint) não puderam ser recuperadas na passagem; podem não existir mais. Não usar IDs antigos como prova de sucesso.

## 8. Próximos passos recomendados

1. Revalidar dev/status/diff/log, ler plano integral e mockup. Não perder alterações atuais.
2. Atualizar no plano apenas o estado histórico de aprovação que ainda aparece em seções finais: cabeçalho já diz aprovado, mas existem textos antigos “aprovação global pendente” e condição de parada de planejamento. Registrar que implementação foi pausada para passagem, não concluída.
3. Rerodar gofmt, Go focal e lint/typecheck frontend. Inspecionar código novo com cuidado antes de expandir.
4. Terminar preparação do PostgreSQL isolado e executar novos e antigos testes integration. Corrigir queries/fixtures a partir de erros reais. Ainda não existe prova real de SQL global.
5. Medir EXPLAIN (ANALYZE, BUFFERS) com fixture representativa. Só criar 0008 se índice justificar; nunca editar migrations aplicadas.
6. Completar segurança/lifecycle/cache e testes. Particularmente: middleware auth com referências cruzadas/expiradas, 409 na virada, media e limites, bot startup/shutdown, listener falho, concorrência migrations.
7. Escrever matriz frontend completa: defaults, sistemas/abas, links e não-links, detalhe/voltar, formato/máscara, loading/empty/error, ausência initData, paginação/virada, avatar.
8. Revisar frontend em browser/screenshots comparando os três painéis do mockup, incluindo nomes longos, scores int64 grandes, telas estreitas e fonts ampliadas. Não homologar visual sem comparação.
9. Implementar **Makefile, scripts/dev.mjs, Dockerfile.v2 multi-stage Node/Go, compose e CI** — ainda NÃO modificados. Manter runtime distroless/nonroot e multiarch. Não executar publish.
10. Atualizar .env.example, docs públicos pertinentes, README, docs de build/branching, .agent/memory/memory.md, decisions.md e context.md. Ainda não foram atualizados nesta implementação.
11. Executar todos os gates exigidos abaixo, revisar diff e gerar relatório final objetivo com limitações.
12. Só mover plano para done quando implementação realmente concluída; homologação Telegram continua do usuário. Sem commit/push.

## 9. Pontos de revisão conhecidos no código inicial

Estes são riscos para investigar, não afirmações de que já foram corrigidos:

- Query SQL global ainda não executada em PG; validar tipos dos parâmetros, CTE, collation, empty page/header e keyset em todos os casos.
- Fallbacks/ordenação e a CTE compartilhada precisam passar testes mensais/desempate antigos.
- Cache de avatar usa entrada pending como deduplicação; testar evicção durante refresh e cancelamento. Cooldown global de 429 ainda precisa revisão além do SafeAPICaller existente.
- API retorna erro genérico corretamente, mas observabilidade segura de falhas de repository pode precisar melhoria sem expor URLs/token/IDs públicos.
- Frontend recebe refs com validade1h; testar refetch/reabertura de detalhe expirado e virada mensal. Atualmente detalhe com409 mostra retry/voltar, não renovação transparente da referência.
- Infinite scroll detecta duplicação/posição, mas não é snapshot entre requests; documentar e testar dados vivos.
- App e página chamam useTelegram; revisar listeners/BackButton para evitar lifecycle duplicado.
- Janela de fetch avatar e retries pode terminar antes de uma fila fria longa; testar recuperação, expiracão e ausência normal.
- CSS usa grid com score nowrap em boa parte das larguras; testar números muito grandes/nomes e evitar overflow sem recalcular pontos.
- `internal/app.Initialize` é testável, porém testes atuais não provam sozinhos todo o ciclo real de Run. A construção do dispatcher foi mantida depois do listener; provar que nenhuma falha de migration inicia workers.
- Caminho webhook antigo continua no pacote; preservar regressões e validar novo caminho compartilhado em separado.
- Dist gerado atual pode estar desatualizado em relação à última correção TS. Rebuild obrigatório antes de validar artefato.
- Teste integration novo contém algumas chamadas setup sem checagem explícita de erro; melhorar antes do aceite.
- Sem revisão final de dependências/audit npm, sem comparação visual, sem provas Docker.

## 10. Gates finais exigidos

```bash
go test -count=1 ./...
go test -race ./...
go vet ./...
go build ./...
go test -tags debugcards ./...
go vet -tags debugcards ./...
go build -tags debugcards ./...
go test -count=1 -tags integration ./internal/storage/postgres/...
git diff --check
npm --prefix web ci
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
npm --prefix web run build
make web-build
make build
make check
docker build -f Dockerfile.v2 -t unobotgo:miniapp-local .
```

Integração exige TEST_DATABASE_URL exclusivamente local/de teste. Docker multiarch e inspeção do runtime sem node/npm/source também exigidos quando ferramenta disponível. Não usar `|| true` para encobrir falha. Nunca `--push`.

## 11. Entrega esperada ao usuário ao finalizar

Relatório completo: arquitetura final, arquivos/interfaces, consultas e índices/EXPLAIN, startup/lock/checksums, initData, avatares/cache, embed/build, UI aderente ao mockup, botão Telegram, testes com resultados reais, limitações ambientais, config para homologação e git status. Confirmar explicitamente nenhum commit/push/main/deploy/migration de produção. Deixar tudo no working tree para homologação manual dos 15 itens do plano.

## Prompt curto para a próxima IA

> Continue a implementação aprovada do Mini App de ranking global do UnoBotGO, somente na dev. Leia AGENTS.md, `.agent/miniapp-implementation-handoff_2026-09-30.md` e o plano em `.agent/plans/approved/miniapp-ranking-global_2026-09-30_11-20.md`. Preserve o working tree: há implementação parcial não commitada. O mockup anexado é a fonte de verdade visual. Não faça commit/push, não altere main, não faça deploy nem migrations em produção. Comece verificando o estado real e os testes pendentes; conclua o plano e entregue relatório para homologação manual, sem presumir que o código parcial já está validado.

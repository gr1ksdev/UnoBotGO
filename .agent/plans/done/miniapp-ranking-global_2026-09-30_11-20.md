# Plano: Mini App de ranking global do UnoBotGO

Status: APROVADO pelo usuário; implementação em andamento exclusivamente no working tree dev. Sem commit, push, publicação, deploy ou migrations em produção. O mockup é a fonte de verdade das cores; descrições textuais de azul/vermelho são observações da imagem, não requisitos independentes que possam prevalecer sobre ela.

## Pedido do usuário

Adicionar Mini App Telegram de ranking global do mês corrente, com universos Updated/Legacy separados, listas de grupos e players, detalhe do grupo, fotos e IDs mascarados. Manter bot, API, React e migrations no mesmo projeto, com frontend compilado embutido e apenas um executável em produção. Antes de implementar: auditar, salvar plano e parar para aprovação.

## Objetivo

Entregar, depois da aprovação, `bin/unobotgo`, construído normalmente durante o build. Em runtime: config → PostgreSQL → advisory lock → verificar/aplicar migrations → liberar lock → iniciar HTTP/API/Mini App → iniciar bot/polling/webhook/workers. Se migration falhar, encerrar com exit code diferente de zero, sem iniciar qualquer funcionamento parcial. Reutilizar ranking mensal e regras homologadas; preservar comandos de grupo/privado, /config e gameplay. Node/npm somente no desenvolvimento/build.

## Contexto atual — auditoria em 2026-09-30

### Git e trabalho existente

- Branch confirmada: `dev`, sincronizada com `origin/dev` em `c86556b` (`feat(ranking): implement private monthly ranking, migration 0007 and /config UX improvements`).
- `git status --short --branch`: `## dev...origin/dev`, sem arquivos modificados ou novos antes deste plano.
- `git diff`, `git diff --stat` e `git diff --cached`: vazios.
- Anteriores: `d434ffd` ranking mensal; `87f624d` correção de fixture de Trocar Mãos; `93dc473` desempate; `64bb97d` ranking visível.
- Portanto ranking privado e /config existem e estão commitados; não há conflito pendente identificado. Revalidar status/diff imediatamente após aprovação, porque isso pode mudar.
- `main` local: `6eea6c1399d287b58116d5be16a1807238653397`; apenas lida. Históricos dev/main independentes, promoção por árvore pública, sem merge direto.
- Nenhum checkout, reset, restore, stash, commit, push ou alteração de referência foi realizado.

### Stack, entrypoints e runtime

- Go `1.26.3`, pgx/v5 `5.11.0`, telego `1.10.0`; `golang.org/x/sync` já consta nas dependências.
- V2 inicia em `cmd/bot/main.go`; o código V1 continua na raiz. Não mover/reorganizar o legado nesta entrega.
- Startup atual abre PostgreSQL e executa apenas `VerifySchema`, com timeout total de 10s. O operador precisa executar `cmd/migrate` antes.
- `cmd/migrate` já chama `Store.Migrate`, com timeout de 1 minuto, sem depender de token Telegram.
- `telegram.New` cria dispatcher e inicia workers; por isso a construção do bot também deve ficar DEPOIS das migrations.
- Polling é padrão. Webhook cria seu próprio `net/http.Server` em `Bot.runWebhook`, usa `WEBHOOK_LISTEN_ADDR` (default :8080), path de `WEBHOOK_URL`, `/healthz`, segredo, limite de body, dedupe e fila.
- Hoje não existe servidor de API/SPA em polling. Não existe router externo ou necessidade de framework HTTP pesado.
- Supervisão HTTP atual precisa ser revista: depois do registro do webhook, `runWebhook` aguarda principalmente cancelamento; o novo coordenador deve tratar falhas tardias do listener como falhas do processo.

### Migrations e dados reais

| Migration | Responsabilidade |
|---|---|
| 0001_groups | group_configs e configuração |
| 0002_results | completed_games, completed_game_players, player_group_stats |
| 0003_known_users | usuários observados por grupo |
| 0004_imports | staging de import, fora do escopo |
| 0005_insufficient_eligible_players | status de resultado sem elegíveis suficientes |
| 0006_monthly_ranking | player_group_monthly_stats + backfill |
| 0007_group_title | group_configs.title + índice user/month |

- Próximo número livre observado: **0008**, a conferir novamente antes de criar qualquer migration.
- `Migrate` já embute SQL, ordena arquivos, calcula SHA-256, abre uma transação, adquire `pg_advisory_xact_lock(71870101)`, compara ledger/checksums, aplica pendências, registra e commita. Rollback/commit liberam o lock automaticamente.
- `VerifySchema` exige exatamente versões/checksums conhecidos. Existem testes de aplicação idempotente, checksum alterado, migrações concorrentes e rollback.
- Não há necessidade de adicionar outro advisory lock nem substituir o migrador. Comentários/erro que mandam executar cmd/migrate precisam refletir o startup automático.
- `player_group_monthly_stats`: PK `(chat_id,user_id,month_start)`, score_units bigint não negativo, ranking_system, completed_games, wins, display_name, last_finished_at e updated_at.
- Índices: `(chat_id,month_start,score_units DESC,user_id)` e `(user_id,month_start)`. Não existe índice começando por mês/sistema para a leitura global.
- Histórico: completed_games guarda ChatID, sistema, finished_at e scoring_status; participantes guardam posição/status/went_out. PKs por GameID e `(game_id,user_id)`; nenhuma coluna de avatar.
- `RecordCompletedGame` grava stats totais e mensais na mesma transação, apenas para elegíveis; GameID/hash preserva idempotência. **Não alterar esse fluxo.**
- `group_configs.title` é observado em comandos e my_chat_member; fallback atual Telegram é `Grupo <ChatID>`. Esse fallback precisa de variante mascarada na API/UI para não vazar ID completo.
- `groups.Config` tem campo Title, mas o SELECT/scan geral de configuração não o preenche; o ranking privado já lê title por JOIN direto. Para o Mini App, projetar o título no DTO de leitura, sem depender desse scan nem alterar /config desnecessariamente.

### Ranking mensal e serviços

- `internal/ranking/time.go` centraliza America/Sao_Paulo, MonthStart, MonthDateString, MonthName e tzdata embutida. Reutilizar; nenhum cron/reset físico.
- `ranking.Service` tem relógio injetável `Now` e leituras de grupo e do usuário. APIs existentes recebem instante no repositório.
- `ListGroupRanking` usa uma consulta com CTE latest, mês, posição elegível e ordem: score DESC, última posição ASC, conclusão DESC, UserID ASC. Limite 512 é de apresentação Telegram, **não pode limitar o detalhe global permanentemente**.
- A CTE atual usa expressão date_trunc/AT TIME ZONE em finished_at. Para uma leitura paginada indexável, compartilhar predicado equivalente por intervalo `[início do mês, início do próximo mês)` calculado pelo helper canônico, provando equivalência em testes de fronteira.
- `/ranking` privado usa o From.ID autenticado pelo Telegram, consulta grupos em uma query e separa totais Updated/Legacy. Preservar.
- `RenderGroupRanking` e o renderer privado têm limite de 4000 unidades UTF-16. Preservar; frontend terá componentes próprios, usando a mesma semântica de dados.
- `FormatScore` usa inteiros, mas ainda não adiciona agrupadores de milhar. Definir especificação compartilhada com fixtures Go/TS para 2.840 pts e 2.840,00 pts, sem mudar apresentação Telegram sem necessidade.

### Telegram, frontend e build

- Telego instalado oferece GetChat, GetUserProfilePhotos, GetFile e FileDownloadURL. A interface BotAPI local ainda não expõe esses métodos; preferir interface pequena separada para avatares.
- InlineKeyboardButton possui URL e WebApp; o próprio comentário da versão instalada restringe WebApp ao privado. Direct link é o mecanismo proposto em ambos os contextos (ver referências oficiais).
- Na auditoria inicial não existiam frontend nem mockup. Agora o usuário forneceu `mockup_de_rankings_uno_em_iphones-2.png`, presente como arquivo não rastreado e referência visual oficial. Preservar integralmente a imagem; não editar/substituir. O sticker existente não é referência deste Mini App.
- Makefile atual usa `go run .` e docker compose do V1. Dockerfile da raiz e docker-compose.yml também apontam para V1. Não tratar esses targets como o fluxo oficial V2 atual.
- Dockerfile.v2 já usa builder no BUILDPLATFORM, GOOS/GOARCH e distroless static-debian12:nonroot, com um executável `/unobot`. Workflows usam explicitamente esse arquivo.
- CI dev: Go + PostgreSQL17 + Docker multiarch sem push. Workflow main valida e publica apenas em push na main. Preservar esses gatilhos/permissões.
- Ambiente de auditoria: Linux arm64, CGO_ENABLED=0; Go/Make disponíveis; Node, npm e Docker não encontrados no PATH. Não instalar ferramentas nesta etapa.

### Verificação de base executada

`go test -count=1 ./...`: PASSOU nesta auditoria, incluindo V1 e V2. Não houve acesso ao banco de produção. Não reexecutei migrations, integração PostgreSQL, frontend, Docker ou race nesta etapa de planejamento. A limitação ThreadSanitizer/VMA foi observada em turnos anteriores; memória mais recente também registra execuções race aprovadas em outro contexto. Reavaliar o ambiente durante a implementação, sem presumir sucesso ou falha.

## Arquivos analisados

- AGENTS.md; .agent/context.md; trechos atuais de .agent/memory/memory.md.
- go.mod; cmd/bot/main.go; cmd/migrate/main.go.
- internal/config/config.go; .env.example.
- internal/ranking/{service.go,time.go,eligibility.go} e testes de calendário.
- internal/storage/postgres/{migrations.go,results.go,ranking.go,groups.go,migrations_integration_test.go,ranking_monthly_integration_test.go}.
- SQL 0006/0007 e schema/índices de resultados e stats anteriores.
- internal/telegram/{bot.go,transport.go,client.go,dispatch.go,commands.go,ranking.go} e testes relacionados.
- Tipos/métodos de telego v1.10.0 no module cache, sem modificar dependências.
- Makefile; Dockerfile; Dockerfile.v2; docker-compose.yml; .dockerignore; .gitignore.
- .github/workflows/{dev-ci.yml,main-container.yml}; docs/{branching.md,build.md}.

## Estratégia de implementação

### 1. Arquitetura e entrypoint único

Manter `cmd/bot` como caminho fonte por compatibilidade; o artefato oficial passa a se chamar `bin/unobotgo` (no container `/unobotgo`). Renomear o diretório para cmd/unobotgo não agrega comportamento e exigiria mudanças extras em scripts/documentos.

Extrair coordenação para `internal/app` testável:

1. Carregar configuração, validar secrets/URLs/assets necessários e instalar signal context.
2. Conectar PostgreSQL com timeout de conexão separado.
3. Chamar Migrate com timeout apropriado: adquirir advisory lock existente, verificar ledger/checksums e aplicar pendências na transação; commit/rollback libera o lock. Não construir o binário em runtime.
4. Executar VerifySchema como verificação final após aplicação; a verificação necessária à aplicação já acontece sob lock no migrador. Qualquer falha encerra sem listener, bot ou workers iniciados.
5. Montar ranking service, media service e handlers após persistência pronta. Separar construção do bot do início de workers para cumprir HTTP antes de bot/polling/webhook/workers.
6. Abrir UM listener HTTP, registrar API/static/media/health e webhook quando configurado. Readiness inicialmente false; webhook retorna 503 até o bot estar pronto.
7. Iniciar bot/registro Telegram/polling ou webhook; ativar readiness quando o transporte estiver pronto.
8. Qualquer falha de startup/HTTP/bot cancela os demais, drena workers, encerra HTTP com timeout e fecha pool. main faz os.Exit apenas depois de run retornar e executar defers.

`cmd/migrate` pode continuar como ferramenta de desenvolvimento/diagnóstico sem token; não será copiado para a imagem nem necessário no deploy. Simulador e V1 também ficam fora do runtime.

Unificar o servidor webhook: separar preparação/registro/transporte e expor handler ao coordenador. Manter segredo, body limit, dedupe, filas, scheduler e comportamento polling. Migrar testes de lifecycle; não manter dois listeners na mesma porta.

Compatibilidade: WEB_ADDR assume endereço HTTP; se ausente, aceitar WEBHOOK_LISTEN_ADDR existente como alias. Se ambos explicitamente diferirem, falhar com erro claro, sem ignorar configuração. Reservar prefixos /api, /assets, /healthz, /readyz. WEBHOOK_URL precisa de path não conflitante, por exemplo /telegram; URLs antigas com path raiz exigem ajuste explícito documentado, nunca alteração silenciosa de domínio.

Migrations concorrentes seguras NÃO tornam o gameplay em memória multi-instância. Operação do bot continua singleton por token. Não propor balanceamento de sessões de jogos nesta feature.

### 2. HTTP e autenticação Mini App

- stdlib net/http ServeMux; transport HTTP separado de ranking/media/application.
- API e frontend da mesma origem; nenhuma política CORS aberta. Vite terá proxy apenas em desenvolvimento.
- Frontend lê `Telegram.WebApp.initData` original, mantém em memória e envia em `Authorization: tma <initData>` a cada fetch da API/media. Nunca querystring, localStorage ou logs.
- Backend valida HMAC-SHA256 pelo algoritmo oficial para bot: parse URL-encoded sem reserializar JSON, rejeitar chaves duplicadas, separar hash, ordenar campos, usar linhas key=value; chave derivada com WebAppData e token, comparação em tempo constante. Não confundir esse mecanismo com Ed25519 de validação por terceiros. Incluir testes com campos opcionais/signature presentes conforme algoritmo oficial.
- Verificar auth_date inteiro, prazo default 1h configurável, tolerância máxima de 30s para relógio futuro; exigir user válido somente após validação. Limitar header/payload para evitar consumo ilimitado.
- Não confiar em user_id enviado em corpo/query nem em initDataUnsafe. Não inferir membership de chat_instance; global é legível a usuários autenticados do Mini App, conforme visão de todos os grupos.
- Invalidade/expiração → 401 genérico. Frontend mostra orientação para reabrir pelo Telegram; não há login por senha nem bypass de produção. Browser externo sem initData mostra “Abra o Ranking Global pelo Telegram.”
- Assets/SPA públicos sem dados; API e fotos protegidas. Readiness/health sem segredo ou detalhes de banco.
- HTTP: ReadHeaderTimeout 5s, ReadTimeout/WriteTimeout 15s, IdleTimeout 60s; context SQL por request com timeout menor. Tamanho de headers limitado; logs sem Authorization/token/URLs de download Telegram.
- Header de segurança/caching compatível com Telegram Web/Desktop: nosniff, referrer-policy, CSP permitindo apenas assets próprios e script Telegram necessário. Não bloquear embedding do cliente Telegram com X-Frame-Options DENY indiscriminado.
- TTL de initData não equivale a impedir replay dentro da janela; API é de leitura, usar rate limit por usuário autenticado e concorrência limitada. Não inventar mecanismo de refresh da assinatura Telegram.

### 3. API e contratos propostos

| Método/path | Comportamento |
|---|---|
| GET /api/v1/rankings/groups?system=updated&limit=50&cursor=... | Grupos do mês/sistema, total por grupo e cursor |
| GET /api/v1/rankings/players?system=updated&limit=50&cursor=... | Usuários agregados entre grupos, sem perfil |
| GET /api/v1/rankings/groups/{group_ref}?system=updated&limit=50&cursor=... | Cabeçalho/total do grupo e jogadores mensais paginados |
| GET /api/v1/media/{avatar_ref} | Foto interna autenticada ou fallback/ausência |
| GET /healthz | Liveness mínima |
| GET /readyz | Readiness do processo/persistência/transporte |

Resposta comum: `month_start` YYYY-MM-DD, `month_name`, `month_ends_at`, `server_time`, `timezone`, `system`, `items`, `next_cursor`. Não aceitar mês histórico no endpoint. Sistema ausente assume updated; sistema inválido retorna 400.

Item de grupo: position, key opaca estável, group_ref opaco, name, masked_id, score_units, avatar_url interna. Item de player: position, key opaca, name, masked_id, score_units, avatar_url; **sem href/perfil**. Detalhe acrescenta total do grupo e lista de jogadores. Não serializar structs de storage diretamente.

**Precisão APROVADA pelo usuário:** Go mantém Units/int64. JSON transmite `score_units` como string decimal de inteiro, por exemplo `"284000"`; TypeScript interpreta com BigInt e formata por divisão/resto, sem Number/float. Isso evita perda de precisão de int64 no JavaScript. Representação decimal/string segura aprovada; nunca usar Number para int64 arbitrário nem float para pontos. Fixture de contrato comum cobre Legacy centésimos/100 e Updated com duas casas, separador milhar ponto, decimal vírgula e singular/plural. SUM(bigint) retorna numeric no PostgreSQL: validar faixa antes de converter a int64, falhar explicitamente em overflow, nunca truncar/arredondar.

IDs: helper usa representação decimal sem sinal para últimos quatro dígitos; IDs menores exibem todos os dígitos disponíveis com prefixo ••••, sem completar com informação falsa e sem overflow em abs(MinInt64). Nome vazio no Mini App: `Grupo ••••8462` / `Jogador ••••7953`; não reutilizar fallback que contém raw ID.

**Referências opacas APROVADAS pelo usuário:** MINIAPP_SECRET ou mecanismo seguro equivalente. UI mostra somente `ID ••••1234`. Identificadores técnicos: chaves HMAC estáveis por entidade/contexto para React/deduplicação; referências de grupo/avatar e cursores cifrados/autenticados com AES-GCM, versionados, tipados e com escopo. Utilizar MINIAPP_SECRET separado, derivando subchaves por finalidade. Assim não mandar IDs completos nem escondê-los apenas com base64. Referências não substituem autenticação. URL/ref inválida ou entidade fora do mês/sistema retorna 404/400 sem revelar raw ID. Rotação do segredo invalida referências/cursores, que são refeitos ao recarregar a lista.

### 4. SQL, reutilização e paginação

- Nova interface de leitura global/paginada no pacote ranking, sem adicionar SQL a handlers; manter a interface de escrita intacta. Service usa relógio já injetável e valida system/limit/cursor.
- Grupos: filtrar mensal por month_start e ranking_system; GROUP BY chat_id; SUM(score_units), MAX(last_finished_at) como last_activity_at, título por JOIN. Ordenar total DESC, atividade DESC, nome ASC, ChatID ASC.
- Players: mesma restrição antes da agregação; GROUP BY user_id; somar todos os grupos daquele sistema; atividade MAX(last_finished_at). Escolher nome da linha mensal com last_finished_at mais recente (ela já representa participação elegível); empate de timestamps usa ChatID como estabilidade técnica. Nunca identificar pelo nome. Ordenar total DESC, atividade DESC, nome ASC, UserID ASC.
- Nomes globais são escolhidos dentro do universo/mês selecionados; não puxar nome/atividade de outro sistema para influenciar a classificação.
- Nome/colação usados no ORDER BY e cursor devem coincidir; definir collation C explícita para ordenação técnica previsível de nomes Unicode, sem tratar nome como identidade.
- Detalhe: extrair a query base mensal/elegível e a ordenação já usadas pelo grupo para um helper compartilhado. `ListGroupRanking` continua wrapper para Telegram (até 512); novo método paginado aceita sistema e limite e utiliza a MESMA CTE/predicado/ordenação. Não ordenar em JS ou carregar tudo para paginar no handler.
- Caso sistema solicitado não corresponda ao grupo/estatística válida, não converter: retornar não encontrado naquele universo. Manter proteções contra dados incompatíveis.
- Pontos zero com completed_games válidos entram. Abandonos não geram stats; latest de desempate continua excluindo-os. Grupos sem nenhuma participação pontuada mensal não aparecem.
- Keyset cursor, default 50, máximo 100; pedir limit+1 para descobrir continuidade. Cursor inclui versão, tipo de lista, mês, sistema, chaves completas de ordenação e prazo curto (5min). Validação no servidor impede reutilização entre sistemas/grupos/endpoints. Nada de OFFSET crescente ou páginas numeradas.
- SQL calcula posição ordinal sobre a relação ordenada antes do filtro de continuação. Em conjunto sem mudanças, páginas reproduzem posições 1..N sem gaps artificiais.
- Ranking é vivo: entre requisições os resultados podem mudar. Não fingir snapshot histórico aplicando updated_at <= cursor em stats mutáveis. Cada página é um snapshot SQL consistente; entre páginas, frontend detecta IDs repetidos/posições incompatíveis e reinicia a lista, além de refetch completo ao retomar app/refresh. Cursor expira rápido; não manter transação aberta durante scrolling. Documentar possibilidade de atualização da lista sob jogos concorrentes.
- Virada do mês: backend rejeita cursor de outro mês com 409 `ranking_period_changed`, retorna metadados atuais; frontend descarta páginas anteriores. month_ends_at + server_time guiam invalidação, com nova confirmação no backend; nunca decidir mês pela timezone local.
- SQL da página/cabeçalho do detalhe deve compartilhar snapshot (uma instrução/CTE ou transação read-only curta). Quantidade de queries limitada e independente do número de linhas; nenhum GetChat/GetChatMember para nomes no ranking.

### 5. Migrations necessárias

**Nenhuma nova tabela de ranking, coluna de score, backfill ou migração de nomes é necessária.** Avatares serão inicialmente cacheados em memória, sem schema.

Possível única migration de índices: `0008_global_ranking_indexes.up.sql`, somente se os planos/fixtures confirmarem necessidade. Candidatos justificados pela estrutura:

- player_group_monthly_stats `(month_start,ranking_system,chat_id,user_id)` para recortar o universo mensal global; índices existentes começam por chat/user.
- completed_games `(chat_id,ranking_system,finished_at DESC,game_id DESC) WHERE scoring_status='scored'` para selecionar histórico elegível mensal do detalhe, após usar predicado temporal indexável.

Antes de criar: EXPLAIN (ANALYZE, BUFFERS) em banco local com múltiplos meses/sistemas e volume representativo, comparar planos, custo de escrita e tamanho. Se índices não trouxerem benefício, não criar 0008. Não criar índice largo com nomes/bytes de avatar por conveniência.

Não usar CREATE INDEX CONCURRENTLY dentro do migrador transacional existente. Índice normal sob migration demanda janela de startup; medir e documentar. Não alterar SQL 0001–0007 ou checksums. Aprovação deste plano não autoriza executar migrations em produção.

### 6. Fotos, proxy e cache

- Interface pequena `AvatarSource`: adapter Telegram usa GetChat.Photo.SmallFileID para grupos; GetUserProfilePhotos(limit=1) e tamanho adequado para usuários; GetFile + download apenas no backend.
- Endpoints de ranking não esperam Telegram. Retornam referências internas, e a UI mostra fallback imediatamente. Foto carregada sob demanda apenas quando card se aproxima da viewport.
- Cache LRU limitado (proposta: 2000 metadados, 64MiB de imagens), TTL positivo 6h, ausência 1h, falha transitória 1min; limites testáveis em configuração interna. Sem tabela, disco persistente ou serviço externo nesta versão.
- singleflight por entidade; refresh em fila limitada, até 4 workers e taxa global conservadora de chamadas Telegram. Respeitar 429/retry_after e cooldown; fila cheia preserva fallback, sem bloquear ranking nem criar goroutine ilimitada.
- Cache miss frio inevitavelmente exige chamadas para fotos ainda desconhecidas; o objetivo é amortizar entre renders/usuários e limitar concorrência, não prometer zero chamadas em primeiro acesso. Reinício do processo pode esfriar cache.
- Enquanto resolve: resposta pendente curta com Retry-After e poucas tentativas na UI; ausência confirmada retorna 204, erro transitório não quebra lista. Imagem existente pode ser reaproveitada dentro do prazo de validade; após expiração, revalidar privacidade/disponibilidade.
- Fotos autenticadas: fetch com Authorization e Blob URL local, liberada ao desmontar; `<img loading="lazy">` usa apenas Blob/internal reference, nunca URL com token. Sem initData no URL para compensar a limitação de headers de img.
- Download só de endpoint Telegram conhecido, sem aceitar URL arbitrária do cliente. Limites de bytes (ex.: 2MiB)/tempo/tipo; não retransmitir HTML/SVG remoto arbitrário. ETag e cache privado curto, compatíveis com autenticação.
- Revisar sanitização: SafeAPICaller atual registra err.Error() em debug; erros de rede podem conter URL. Garantir que novo caminho de media e qualquer erro propagado não registrem token/URL sensível, com testes de redaction. Não devolver erros upstream ao browser.
- Foto indisponível/restrição Telegram/bot removido do grupo são condições normais: iniciais/ícone local, sem alterar ranking ou consultar cada membro.

### 7. Frontend, visual e navegação

- `web/`: React + TypeScript estrito + Vite + Tailwind + TanStack Query + React Router; Node 24 LTS e npm. Sem Next/SSR/runtime JS em produção. Versões compatíveis fixadas no package-lock durante implementação.
- Rotas `/` e `/groups/:groupRef`; query params `system=updated|legacy` e `tab=groups|players`. Defaults updated/groups. Voltar preserva sistema e contexto de lista/scroll. Players não possuem link/onClick/perfil.
- Query keys incluem endpoint, sistema, referência e período conhecido; troca de sistema cancela/ignora requests anteriores. Não mostrar dados Legacy enquanto header diz Updated nem usar placeholderData cruzado entre sistemas.
- Organização: `src/api` contratos/fetch; `src/lib` score/Telegram; `src/hooks` rankings/avatar; `src/components` cards/segmented/skeleton/empty/error; `src/pages` GlobalRankingPage/GroupRankingPage; App e router.
- Layout obrigatório: composição do mockup oficial, painel mobile centralizado (max-width inicial ~560px, calibrado pela referência), fundo claro, cards brancos, bordas grandes, sombras suaves e respiro. Header GLOBAL em gradiente azul, como na imagem; DETALHE em vermelho. A referência oficial substitui a descrição anterior de header global vermelho. Controles Updated/Grupos ativos vermelhos e Players ativo azul. Primeiro colocado amarelo suave, segundo neutro/prateado e terceiro bronze/pêssego suave. Alinhamento flex/grid, sem tabela.
- Avatar circular com dimensão reservada; tipografia legível, nomes Unicode quebram/ajustam sem vazar ID. Topo de detalhe com avatar maior, nome, máscara, total e mês; lista abaixo com mesmo padrão.
- Skeletons com geometria dos cards; empty state por grupos/players; erro em card com Tentar novamente; erro de próxima página mantém itens existentes e ação para retry.
- Infinite scroll via IntersectionObserver, com botão acessível Carregar mais como alternativa; nunca lista inicial de milhares.
- Telegram bridge: ready/expand; viewport estável; safe/content safe area com fallback CSS env; BackButton sincronizado com rota; remover listeners ao desmontar. Tema claro explícito e contraste controlado mesmo em Telegram dark; sem segundo tema completo.
- Acessibilidade: controles semânticos, aria-selected/labels, foco visível, botão voltar, prefers-reduced-motion; sem biblioteca visual pesada ou fonte remota obrigatória.
- **Referência oficial de produto:** `mockup_de_rankings_uno_em_iphones-2.png`, anexada duas vezes (mesma referência). Reproduzir composição, hierarquia, proporções, cards, espaçamentos, arredondamentos, sombras, header, segmented controls, avatars, medalhas, destaque do primeiro e disposição nome/ID/pontos. Não é inspiração genérica.
- Não substituir por dashboard administrativo, Material UI genérico, tabela tradicional, sidebar ou design criado do zero. Adaptar somente o necessário para responsividade, Telegram, dados reais, acessibilidade e Android/iOS/Desktop.
- Molduras de iPhone, notch, barra de status e fundo promocional externo não serão desenhados como UI falsa. Não fixar Setembro, nomes, fotos ou pontos de exemplo. Calendário decorativo, sem navegação mensal.

#### Componentes visuais principais

| Componente proposto | Correspondência obrigatória com o mockup |
|---|---|
| AppShell e tokens CSS | Superfície clara, painel mobile, tipografia, raios, sombras, cores UNO e safe areas reais |
| GlobalRankingHeader | Gradiente azul, título branco com mês backend, calendário discreto e navegação coerente com Telegram |
| RankingSystemSwitch | Cápsula clara grande Updated/Legacy, seleção vermelha, proporções e padding da referência |
| RankingTabSwitch | Grupos/Players no painel branco arredondado; Grupos ativo vermelho, Players ativo azul |
| RankingList e RankingCard | Posição à esquerda, avatar circular, nome sobre ID secundário, pontuação à direita; mesmos espaçamentos, raios e sombras |
| RankBadge | Medalhas dourada/prateada/bronze com número interno e fita, fiéis à imagem; SVG/CSS local, somente top 3; depois `4.`, `5.` etc., sempre posições únicas |
| Avatar | Geometria reservada nas linhas, versão grande no detalhe, foto real ou fallback consistente |
| MaskedIdentity e Score | Nome destacado, `ID ••••1234` acinzentado, pontos azul-escuro à direita com unidade menor; formatação inteira exata |
| GroupHero | Topo vermelho, voltar, avatar grande à esquerda, nome/ID/total/mês à direita, painel translúcido e detalhes discretos de cartas UNO |
| GroupRankingSection | Painel claro arredondado abaixo do hero, título “Ranking interno do grupo · <mês>” e mesma lista |
| RankingSkeleton, EmptyState e ErrorState | Estados integrados à geometria e linguagem visual aprovada, retry acessível |

Extrair tokens/proporções da referência antes de construir telas. Comparar screenshots das três telas (Grupos, Players e detalhe) lado a lado com o mockup, usando fixtures determinísticas. Verificar densidade da lista, alinhamento dos pontos, tamanhos dos avatars, controles e top 3. Documentar somente adaptações necessárias para nomes longos, valores grandes, fontes ampliadas e safe area. Não gerar outro mockup para substituir o aprovado. Testes funcionais não substituem aceite visual manual.

### 8. go:embed e checkout limpo

- `web/embed.go` no pacote web; `//go:embed all:dist`, expondo fs.FS com fs.Sub. Padrão all permite um placeholder técnico oculto sem bundle versionado.
- Versionar apenas `web/dist/.keep` técnico vazio; ignorar demais arquivos gerados. Build frontend limpa gerados preservando/recriando esse placeholder por script Node, inclusive no tratamento de erro; não deixar npm build apagar um arquivo tracked permanentemente.
- O Go compila em checkout limpo para testes; porém startup de produção verifica presença de index.html/assets reais e falha claramente se o build frontend não aconteceu. Não servir placeholder como Mini App funcional.
- Modo dev explícito (`--dev`) permite API com Vite sem dist pronto e uma indicação de desenvolvimento na rota estática; **não** desabilita initData ou migrations.
- Fluxos oficiais make build/preview/Docker sempre geram dist antes do Go. Teste do artefato deve executar binário em diretório sem web/dist externo, provando embed.
- SPA fallback somente para GET/HEAD de navegação fora de /api, health, webhook e assets. Asset/API inexistente retorna 404, não index.html.
- Assets Vite hash: cache longo immutable; index.html revalidável/no-cache; API private/no-store. Produção sem source maps públicos ou scripts contendo secrets.

### 9. Botão e configuração

- `🌐 Ranking Global` como InlineKeyboardButton.URL usando direct link Telegram configurado, tanto no grupo quanto privado. Sem WebApp em grupo; sem alias/comando global textual.
- Construir markup compartilhado para respostas bem-sucedidas/vazias de /ranking; conteúdo das mensagens preservado. Por padrão aplicar também ao ranking automático porque usa o mesmo caminho de envio, registrando essa escolha na homologação.
- MINIAPP_LAUNCH_URL ausente: aviso único no startup e botão omitido; nunca mandar botão quebrado. URL inválida: configuração recusada. Não inventar bot username/short_name nem configurar BotFather automaticamente.

| Configuração | Proposta |
|---|---|
| WEB_ADDR | :8080; alias compatível com WEBHOOK_LISTEN_ADDR se não informado |
| MINIAPP_LAUNCH_URL | direct link HTTPS t.me do Mini App; opcional para botão |
| TELEGRAM_INITDATA_MAX_AGE | 1h, positivo |
| MINIAPP_SECRET | chave base64 de 32 bytes para referências/cursores; sem default inseguro |
| MIGRATION_TIMEOUT | 2m inicial, positivo; ajustar após medir migrations/backfill |

TOKEN, DATABASE_URL e configuração webhook existentes permanecem. Não criar env para cada detalhe visual/cache na primeira versão. Vite usa paths relativos e proxy `/api` para 127.0.0.1:8080 por default. Domínio HTTPS e configuração BotFather são dados de homologação, não hardcoded no código. MINIAPP_SECRET precisa ser provisionado pelo operador; não será escrito em arquivo versionado.

### 10. Makefile, Docker e CI

Targets oficiais:

| Target | Ação |
|---|---|
| help | Comandos e pré-requisitos |
| dev | Prepara dependências e supervisiona backend + Vite |
| dev-back | Backend dev com API/migrations e autenticação real |
| dev-web | Vite dev + proxy |
| web-build | npm ci + typecheck/build para web/dist |
| preview | web-build + backend servindo dist embutido |
| build | npm ci → npm run build → go build → bin/unobotgo |
| run | Executa bin/unobotgo existente |
| start | build + run |
| test | Testes Go normais e frontend, sem integrações externas implícitas |
| test-race | Race Go |
| vet | Go vet |
| check | Frontend completo, Go normal/debugcards/race/vet/build/diff e integração real |
| clean | Remove SOMENTE binários/dist gerados e caches locais definidos; preserva .keep, fonte, .env e dados PG |

`make check` exige TEST_DATABASE_URL e não finge integração aprovada se estiver ausente. Limitação do race deve gerar falha/relatório explícito, não `|| true`. Docker multiarch fica como gate separado quando Docker/buildx disponíveis, além de check.

Supervisão dev: pequeno script Node com módulos nativos (sem ferramentas globais adicionais); compila Go para artefato dev e inicia diretamente esse processo e o processo Vite. Tratar SIGINT/SIGTERM/exit de qualquer filho, encerrando os dois e preservando exit code. Evitar cascata go run/npm/shell que deixe netos órfãos. Scripts npm obrigatórios: dev, build, typecheck, lint, test (Vitest run, sem watch no CI).

Manter simulator. Targets legados local/db/up/down/logs devem ser documentados/remapeados explicitamente para não rodar V1 por engano. Proposta: Dockerfile.v2 continua canônico, compose app passa a apontar explicitamente para ele; Dockerfile raiz V1 permanece identificado como legado, sem reorganizar sua fonte.

Dockerfile.v2: stage Node24 no BUILDPLATFORM → npm ci/build; stage Go no BUILDPLATFORM recebe somente dist gerado e compila CGO=0 para TARGETOS/TARGETARCH; stage distroless nonroot recebe apenas `/unobotgo`. Sem migrador separado, Node, npm, source frontend, toolchain ou credenciais. Certificados e tzdata disponíveis (tzdata Go já embutida). Expor porta HTTP; nenhum volume de avatar necessário.

.dockerignore: excluir node_modules, dist local (usar COPY --from do stage Node), bin, reports, secrets e artefatos internos. Preservar fontes web necessárias ao primeiro estágio.

CI dev: setup Node24 + npm ci/lint/typecheck/test/build antes de qualquer teste/build Go que precise dos assets; setup Go atual; normal/race/vet/build/debugcards; PostgreSQL real e integração; Docker multiarch sem push. Repetir preparação de assets no job integration, se importar application/web. Atualizar a cópia de main-container.yml **na dev** para o futuro fluxo público, preservando gatilhos/permissões existentes; não tocar na branch main nem disparar publicação. Public tree futura deve incluir web e scripts necessários, nunca .agent/node_modules/dist gerado. Nenhuma promoção nesta tarefa.

## Arquivos que poderão ser modificados

Existentes:

- cmd/bot/main.go; cmd/migrate/main.go somente se precisar compartilhar bootstrap/diagnóstico.
- internal/config/config.go e config_test.go; .env.example.
- internal/storage/postgres/migrations.go e testes (mensagens/preflight se necessário, sem editar SQL aplicado).
- internal/ranking/service.go, time.go se necessário acrescentar limite superior do mês e testes.
- internal/storage/postgres/ranking.go e testes mensais/desempate, extraindo consulta comum sem regressão.
- internal/telegram/bot.go, transport.go, ranking.go, client.go e respectivos testes; handlers mínimos para URL configurada.
- Makefile, Dockerfile.v2, docker-compose.yml, .dockerignore, .gitignore.
- .github/workflows/dev-ci.yml e cópia main-container.yml na dev.
- README.md, docs/build.md, docs/v2-telegram.md, docs/m7-persistence.md, docs/branching.md (allowlist futura), documentação técnica nova docs/miniapp.md.
- .agent/memory/memory.md, decisions.md e context.md somente após aprovação/execução das decisões; mover plano approved/done conforme workflow.

Novos, organização aproximada:

- internal/app/{app.go,app_test.go,startup_integration_test.go}.
- internal/httpapi/{server.go,auth.go,rankings.go,media.go,static.go,references.go,*_test.go}.
- internal/ranking/{global.go,pagination.go,*_test.go} e fixtures compartilhadas.
- internal/storage/postgres/{global_rankings.go,global_rankings_integration_test.go}; migration 0008 de índices apenas se validada.
- internal/media/{service.go,cache.go,*_test.go}; internal/telegram/avatar.go e testes.
- web/embed.go, web/dist/.keep, web/package.json, package-lock.json, vite.config.ts, tsconfig*.json, eslint config, index.html, src e tests; scripts locais de build.
- scripts/dev.mjs para supervisão e, se útil, utilitário de clean restrito a gerados.

Não alterar engine/gameplay, fórmula, escrita de resultados, políticas de abandono, SQL 0001–0007 ou staging de import. Ajustes além dessa lista exigem justificativa de integração registrada, sem apagar trabalho existente.

## Passos detalhados — fases A a L

### A. Auditoria e arquitetura final

Auditoria concluída e mockup oficial recebido. Após aprovação global, revalidar Git/schema e preservar a imagem fornecida. Mover plano para approved e registrar decisões/arquitetura. Preparar fixtures e mapa do ciclo de vida. Aprovações técnicas individuais não autorizam implementação.

### B. Startup único e migrations automáticas

Extrair internal/app, reutilizar Migrate/advisory lock, configurar timeout, provar fail-fast antes de construir bot/listener. Preservar cmd/migrate dev. Testar concorrência/ledger/rollback.

### C. HTTP único e initData

Unificar listener e webhook; adicionar health/readiness, middleware HMAC, referências opacas e cancelamento. Testar autenticação/falhas/startup/shutdown antes de dados reais de ranking.

### D. API global/paginação/detalhe

Implementar services/repositories/DTOs, agregações separadas, calendário backend, keyset e detalhe compartilhado; comparar com Telegram em PG real. Medir EXPLAIN; criar índice novo somente se justificado.

### E. Cache/proxy de avatares

Adapter telego separado, cache positivo/negativo, fila limitada/singleflight, media autenticado e fallback. Nenhuma chamada por linha na query de ranking. Testar vazamento de tokens e saturação.

### F. Scaffold frontend e embed

Criar stack definida, npm lock/scripts, TypeScript estrito, Router/Query/Telegram bridge, proxy e embed/placeholder. Build limpo reproduzível; autenticação ausente com estado amigável.

### G. Implementação visual

Implementar os componentes da seção 7 e três telas comparando diretamente com o mockup oficial. Preservar header global azul, detalhe vermelho, cápsulas, medalhas e composição das linhas. Screenshots com fixtures e adaptações necessárias documentadas. Validar estados, Android/iPhone/Desktop, safe area, acessibilidade e navegação de volta; não substituir por redesign.

### H. Botão no Telegram

Adicionar URL configurável às respostas de ranking grupo/privado; omitir quando ausente e preservar UTF-16/layout/permissões. Testar URL comum em grupo, nunca web_app incompatível.

### I. Makefile

Implementar targets curtos, supervisor dev, dependências/builds determinísticos, preview e limpeza restrita. Documentar diferença de comandos V1 antigos.

### J. Docker e CI

Stages Node/Go/distroless, multiarch, clean checkout e frontend antes do Go. Atualizar pipelines na dev, sem push/publicação. Verificar conteúdo real de imagem e execução não root.

### K. Verificação completa

Executar matriz abaixo; rever diff/schema/Git, registrar riscos, testes falhos/indisponíveis sem mascarar. Documentar mudanças e instruções locais. Nenhum commit/push.

### L. Homologação manual

Entregar working tree dev e roteiro ao usuário. Não declarar homologado por testes automáticos. Aguardar aceite explícito; publicação/produção exigem outra autorização.

## Riscos

- Migrations automáticas exigem privilégios DDL no usuário de startup e timeout compatível com backfill/índices; DB de produção não será tocado nesta execução.
- Lock de migrations não distribui estado das partidas; duas instâncias ativas do mesmo bot continuam fora do modelo atual.
- Unificação HTTP afeta webhook; proteger paths, ready-state e ciclo de shutdown para evitar updates antes da inicialização ou porta duplicada.
- JavaScript perde precisão de inteiros grandes; string decimal + BigInt evita float. SUM SQL também precisa checagem de faixa.
- Tratar month_start DATE corretamente, sem convertê-lo como meia-noite UTC e mudar mês por acidente. Próximo mês por calendário, nunca por duração fixa.
- Nomes de grupos sem título hoje revelam ChatID no fallback; criar fallback da API mascarado e nunca reutilizar cegamente o texto Telegram.
- Sem índice global inicial, agregações podem ler muitos registros. Keyset limita transferência, mas não elimina custo de agregação; medir em PostgreSQL com volume realista.
- Paginação de dados vivos não é snapshot histórico; invalidar conjunto diante de inconsistência/mudança de mês e documentar comportamento.
- Fotos não são garantidas pelas permissões Telegram. Cache frio/429/reinício não pode travar API nem gerar tempestade de requests.
- Cadastro do Mini App/HTTPS/short_name no BotFather é externo e necessário à homologação real; não há dado de domínio fornecido.
- Fidelidade ao mockup recebido exige comparação visual das três telas; responsividade e acessibilidade admitem ajustes pontuais, não redesign. Node/npm/Docker não estavam disponíveis na auditoria inicial.
- Go embed e dist ignorado podem quebrar checkout limpo se placeholder/build não forem verificados.
- `VerifySchema` antigo recusa versões novas: rollback de binário após migration 0008 precisa estratégia compatível, não apenas substituir executável sem análise.

## Impactos esperados

- Ranking global visual acessível por botão no Telegram, sem mensagem gigante.
- Mesmo ranking mensal e pontos atuais, sem conversão Legacy/Updated; detalhe consistente com /ranking.
- Uma origem HTTP e um binário de produção, inicialização de schema automática e segura.
- Build passa a exigir Node/npm além de Go, mas container final continua mínimo sem Node/npm.
- Mudanças restritas a dev; nenhum artefato de agente promovido à main.

## Compatibilidade

- Linux/macOS: Go, Node24/npm, Make e PostgreSQL; supervisor trata sinais/filhos.
- Windows: mesmos runtimes; Make via ambiente compatível; scripts Node evitam depender de bash para supervisão principal. Testar finalização dos processos nesse sistema quando disponível.
- Docker: distroless nonroot, linux/amd64 e linux/arm64; só binário/certificados/runtime mínimo.
- CI/CD: fixtures PostgreSQL isoladas, assets gerados antes do Go, lock npm. Ausência de ferramenta local registrada; nenhum gate silenciosamente ignorado.

## Como testar

### Backend/API — matriz obrigatória

1. Grupos Updated/Legacy isolados; total correto por ChatID e sistema.
2. Players Updated/Legacy; mesmo UserID em vários grupos somado uma vez, nome da participação elegível mais recente do universo.
3. Score zero aparece; abandono, N<2, pending e mês anterior não entram indevidamente.
4. Virada America/Sao_Paulo (UTC do mês seguinte ainda pode pertencer ao anterior); sessão/scroll atravessando meia-noite renova lista.
5. Empates globais por atividade/nome/ID; posições únicas; detalhe por placement/data/UserID igual ao Telegram.
6. Máscaras de IDs positivos/negativos/curtos; raw IDs ausentes em JSON/fallback/erros/URLs visíveis. Referências/cursor adulterados/trocados de escopo recusados.
7. Valores muito grandes e >2^53; JSON string inteira, nenhum cálculo com float; overflow int64 explícito.
8. initData válido, alterado, expirado, futuro, malformado, duplicado, sem user; comparação constante e payload não confiável não usado.
9. Foto disponível/ausente/privada, TTL/negative cache, singleflight, fila cheia, 429, timeout, limite de bytes, conteúdo inválido, credenciais ausentes no output/log.
10. Paginação 50/limite inválido/última página, scope e expiração de cursor, ordenação em páginas, mudanças concorrentes; quantidade de queries independente de quantidade de itens.
11. SQL real, não só mocks: agregações com dois meses/dois sistemas, títulos/nome atualizado, jogos idempotentes/falha no commit; EXPLAIN e contagem de chamadas Telegram zero durante listagem.
12. SPA deep link, asset inexistente, API 404 sem HTML, Cache-Control correto, dist ausente no startup de produção e embed real fora da árvore fonte.
13. /ranking grupo/privado e /config preservados; botão ausente sem URL, URL correta nos dois contextos; markup sem web_app em grupo.

### Startup/migrations

1. Schema completo: inicializa sem reaplicar.
2. Migration pendente: aplicada antes de listener/dispatcher/polling/webhook.
3. Checksum/version desconhecida: falha e nenhum bot iniciado.
4. Migration com erro: rollback/exit não zero, sem funcionamento parcial.
5. Duas chamadas concorrentes: advisory lock serializa; teste adicional mantém lock em sessão para provar que segundo startup espera sem iniciar bot.
6. Cancelamento/timeout do lock: encerra corretamente, sem lock abandonado.
7. Falha HTTP/bot depois de iniciar: cancelamento e cleanup simétricos, readiness false.

### Frontend — Vitest + React Testing Library

Cobrir todos os 18 cenários solicitados: default Updated, alternância de sistema, default Grupos, troca de abas, render grupos/players, player não clicável, grupo clicável, detalhe/voltar, máscara, Updated/Legacy, empty/error/skeleton, detalhe completo, sistema preservado e initData ausente. Acrescentar respostas fora de ordem na troca de abas/sistemas, paginação/virada de mês, avatar fallback, BigInt >2^53, foco/BackButton e limpeza de Blob URLs/listeners.

### Build

```bash
make web-build
make build
go build ./...
go build -tags debugcards ./...
```

### Testes e gates finais (depois de implementar)

```bash
go test -count=1 ./...
go test -race ./...
go vet ./...
go test -tags debugcards ./...
go vet -tags debugcards ./...
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
docker buildx build --platform linux/amd64,linux/arm64 -f Dockerfile.v2 --output type=oci,dest=/tmp/unobotgo-miniapp.oci .
```

Usar TEST_DATABASE_URL somente de banco local/de teste isolado. Integrações de startup adicionais devem rodar com sua tag/pacote. Se CGO estiver desligado, tentar CGO_ENABLED=1 para race; registrar mensagem exata se VMA bloquear. Não substituir gate falho por sucesso presumido.

Inspeção de imagem por docker create/export ou conteúdo OCI, sem pressupor shell no distroless: verificar usuário nonroot, entrypoint único, ausência de node/npm/source/.env/.agent, frontend servido pelo binário com assets externos removidos do ambiente de execução. Nenhum comando com --push.

### Execução e homologação manual

```bash
make dev
make preview
make run
```

Configurar apenas banco/token de teste. Para integração real Mini App, fornecer HTTPS acessível pelo Telegram e direct link cadastrado, sem provisionamento/deploy automático nesta tarefa. Validar os 15 itens do pedido: /ranking grupo preservado, botão, abertura Updated/Grupos, alternâncias, ordem, detalhe, voltar, players sem clique, foto/fallback, máscara, pontos iguais ao banco e mês backend correto. Adicionar privado /ranking, Legacy, browser externo, safe area/dark e cursor na virada do mês. Não executar comandos que iniciem o bot real durante a mera auditoria.

## Rollback

Antes de qualquer publicação, alterações permanecem no working tree e podem ser revertidas seletivamente após nova instrução, preservando trabalho do usuário. Não usar reset/restore/stash automático.

Sem migration nova, voltar à versão anterior exige apenas reverter código/config/build quando autorizado. Se um índice 0008 for aplicado em ambiente de teste, não editar/apagar ledger: a versão anterior de VerifySchema rejeita migração desconhecida. Usar forward fix/binary compatível ou recriar somente banco descartável; estratégia de produção exige autorização separada. Não planejar down destrutivo.

## Esclarecimentos incorporados e aprovação

1. **Design oficial recebido:** `mockup_de_rankings_uno_em_iphones-2.png` é requisito de produto, não inspiração. Componentes e validação visual na seção 7.
2. **Precisão APROVADA:** score_units string decimal segura na API, BigInt no frontend quando necessário; nunca Number para int64 arbitrário nem float para pontos.
3. **Referências opacas APROVADAS:** MINIAPP_SECRET ou mecanismo seguro equivalente; UI somente `ID ••••1234`.
4. **Startup esclarecido:** binário construído no build. Runtime config → PostgreSQL → advisory lock → verify/apply migrations → release lock → HTTP/API/Mini App → bot/polling/webhook/workers. Falha de migration encerra sem operação parcial.
5. **HTTPS/BotFather entendido:** domínio e direct link serão configurados posteriormente para homologação real; sem provisionamento/deploy nesta etapa.
6. **Aprovação explícita concedida pelo usuário:** implementação executada exclusivamente no working tree da branch `dev`. Nenhuma alteração na main, nenhum commit, push, publicação de container, deploy ou migration em produção.

### Estado Git nesta atualização

Branch dev. Implementação completa no working tree aguardando homologação manual do usuário. Imagem de mockup preservada intacta. Sem commits ou push realizados.

Demais regras de produto já estão definidas: todos os grupos/players participantes visíveis a usuários autenticados do Mini App, sem opt-in por grupo nesta entrega, sem perfil, sem histórico navegável, sem mistura de sistemas.

## Observações e referências verificadas

- Botão web_app inline é restrito ao privado; direct link abre Mini App em qualquer chat. Plano usa URL comum do Telegram em ambos: [Bot API — InlineKeyboardButton](https://core.telegram.org/bots/api#inlinekeyboardbutton) e [Mini Apps — Direct Links](https://core.telegram.org/bots/webapps#direct-link-mini-apps).
- HMAC e auth_date: [validação oficial de initData](https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app). Bridge/safe area: [Telegram Mini Apps](https://core.telegram.org/bots/webapps).
- Node24 consta como LTS na auditoria: [Node releases](https://nodejs.org/en/about/previous-releases). Build frontend: [Vite](https://vite.dev/guide/).
- Regras de embed, diretórios ocultos e padrão all: [Go embed](https://pkg.go.dev/embed).
- Não usar skills de imagem para inventar substituto do mockup: preservado o design e paleta do mockup oficial.

## Condição de entrega desta etapa

Implementação completa finalizada no working tree local. Todos os testes Go, testes PostgreSQL reais de integração, suites frontend e gates de build/lint executados com sucesso. Entregar relatório completo para homologação manual do usuário, sem commit, sem push, sem deploy ou migrations em produção.

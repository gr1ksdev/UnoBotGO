# Exibição de Nome e Avatar do Usuário na aba Perfil com Conversão para Anônimo — 2026-10-02 (somente dev; pronto para homologação)

- Pedido do usuário aprovado no plano: `nome-avatar-perfil-anonimo_2026-10-02_13-41.md`.
- **Exibição do Perfil do Usuário no Mini App**:
  - `web/src/lib/telegram.ts`: expandido o tipo de `TelegramApp.initDataUnsafe` para incluir `user?: TelegramUser` com tipagem para `first_name`, `last_name`, `username` e `photo_url`.
  - `web/src/pages/Profile.tsx`:
    - Resgata os dados do usuário autenticado no Telegram.
    - Se a privacidade estiver desativada (`anonymous: false`): exibe o nome real completo, username `@username` (ou fallback) e a foto de perfil do Telegram em tag `<img>` dentro da caixa de avatar (com fallback para iniciais caso não tenha foto ou falhe no carregamento).
    - Se a privacidade for ativada (`anonymous: true`): converte imediatamente o nome para `"Anônimo"`, o subtítulo para `"Modo anônimo ativado • Oculto no ranking"` e o avatar para a silhueta neutra (`.avatar-anonymous` com `.icon-anonymous`), removendo a renderização de foto e iniciais.
    - O botão switch continua salvando a preferência via `PUT /api/v1/me/privacy` e revalidando TanStack Query.
- **Estilos CSS**:
  - `web/src/styles.css`: adicionada estilização para `.profile-header-info`, `.profile-avatar-box`, `.profile-status-text` com suporte a truncate responsivo e alinhamento centralizado dos metadados.
- **Validação e Testes**:
  - `web/src/pages/Profile.test.tsx`: testes unitários e de integração de frontend cobrindo:
    1. Renderização do nome real, username e imagem de perfil do usuário.
    2. Conversão imediata para nome `"Anônimo"` e ícone neutro de silhueta ao ativar o toggle switch.
    3. Restauração do nome real e da imagem ao desativar o modo anônimo.
    4. Fallback de iniciais quando o usuário não possui `photo_url`.
    5. Tratamento de erro na API de persistência.
  - Validação completa aprovada: `make check` (100% de testes unitários do Go e Vitest, testes de integração PostgreSQL, lints, typecheck, builds e git diff).
- Regras de isolamento: zero commit, zero push, mantido no working tree da dev.

---

# Correção de posicionamento do footer no Perfil e sessão de privacidade no /config — 2026-10-02 (somente dev; pronto para homologação)

- Pedido do usuário aprovado no plano: `corrigir-footer-perfil-e-config-privacidade_2026-10-02_13-28.md`.
- **Correção do Footer no Perfil (Mini App)**:
  - Causa raiz: `--bottom-nav-gap`, `--bottom-nav-safe` e `--bottom-nav-height` estavam restritos a `.global-view`. Na aba de perfil (`.profile-view`), a expressão `bottom: calc(var(--bottom-nav-gap) + var(--bottom-nav-safe))` tornava-se inválida sem fallbacks, fazendo com que o WebKit/Safari descartasse a propriedade para `bottom: auto; top: auto;`, renderizando a barra no topo/cabeçalho.
  - Correção:
    - Declarados valores padrão com fallbacks em `:root` para `--bottom-nav-height: 60px;`, `--bottom-nav-gap: 12px;`, `--bottom-nav-safe: max(env(safe-area-inset-bottom, 0px), var(--telegram-bottom, 0px));`, `--bottom-nav-frame: 10px;`.
    - Estendido o seletor para `.global-view, .profile-view`, aplicando a reserva de `padding-bottom` e altura segura em ambas as telas.
    - Incorporados fallbacks embutidos em `.bottom-navigation` (`bottom: calc(var(--bottom-nav-gap, 12px) + var(--bottom-nav-safe, 0px));`) e `.bottom-nav-item` (`height: var(--bottom-nav-height, 60px);`).
    - Adicionada a classe `global-view` ao contêiner raiz de `ProfilePage` (`<main className="app-shell global-view profile-view">`) e reset de scroll via `window.scrollTo(0, 0)` no mount.
- **Sessão de Privacidade no Menu `/config` do Grupo (Telegram)**:
  - Renderização textual (`RenderGroupConfig`): adicionada a linha `<b>Privacidade no ranking:</b> %s` (`Público` / `Anônimo`) e um 3º bloco explicativo (`<blockquote><b>🌐 Público</b>...</blockquote>` ou `<blockquote><b>🔒 Anônimo</b>...</blockquote>`), mantendo a separação padronizada por `────────────`.
  - Teclado inline (`makeGroupConfigButtons`): adicionada a 3ª linha de botões `[ Público ]  [ Anônimo ]` com indicação `✅` no estado ativo, associados aos callbacks `cfg_privacy_public_<chatID>` e `cfg_privacy_anon_<chatID>`.
  - Tratamento de Callbacks (`handleConfigCallback`): adicionado o caso `"privacy"` no switch de ações para alternar entre `public` e `anon`, invocando `svc.SetRankingPrivate`, respeitando a autorização restrita (`groups.CanConfigureUser`), atualizando a mensagem dinamicamente via `EditMessageText` e respondendo com toast explicativo.
- **Testes e Validações**:
  - `internal/telegram/renderer_test.go`: atualizada a suíte com verificação dos 3 cabeçalhos, 3 blockquotes e 2 separadores para estados público e anônimo.
  - `internal/telegram/config_test.go`: validadas as 3 linhas de botões, alternância dinâmica de privacidade via callbacks e bloqueio de usuários não autorizados.
  - Validação completa aprovada: `make check` (100% de testes unitários, testes de integração PostgreSQL, vitest, lints, typecheck, builds e git diff).
- Regras de isolamento: zero commit, zero push, mantido no working tree da dev.

---

# Privacidade e Modo Anônimo no Ranking Global — 2026-10-02 (somente dev; pronto para homologação)

- Pedido do usuário aprovado no plano: `privacidade-modo-anonimo-ranking_2026-10-02_12-52.md`.
- Camada pura de apresentação/privacidade:
  - Não remove dados do banco de dados.
  - Não altera identidade interna (`user_id`, `chat_id`, nomes e fotos originais persistem inalterados).
  - Não interfere no gameplay das partidas.
  - Não altera fórmulas, pontuações, somas de `score_units`, buckets mensais ou critérios de ordenação e desempate.
- Arquitetura de persistência e Migration 0009 (`0009_ranking_privacy.up.sql`):
  - `group_configs.ranking_private` boolean NOT NULL DEFAULT false.
  - Tabela `user_privacy_settings (user_id bigint PRIMARY KEY CHECK (user_id > 0), ranking_private boolean NOT NULL DEFAULT false)`.
  - Toggle atômico implementado no PostgreSQL para grupos e usuários (`NOT ranking_private`).
  - Preservação estrita das colunas de migrações anteriores: `groupColumns` mantido nos 7 campos históricos da 0001, mantendo compatibilidade total com testes de migração histórica.
- Comando `/privacidade`:
  - No privado do bot: altera a privacidade global do usuário remetente (`msg.From.ID`).
  - Em grupos: altera a privacidade do grupo (`chat_id`), exigindo autorização restrita de configuração via `groups.CanConfigureUser` (administradores ou instalador do bot que ainda seja membro ativo) e menção obrigatória `@bot`.
- Regras de projeção e anonimização server-side:
  - Global de Grupos: grupo anônimo projeta `Name: "Grupo anônimo"`, `MaskedID: ""`, `Avatar: ""` e `Anonymous: true`. Preserva `GroupRef` opaco para permitir visualização de detalhes.
  - Global de Players: usuário anônimo projeta `Name: "Anônimo"`, `MaskedID: ""`, `Avatar: ""` e `Anonymous: true`.
  - Detalhe do Grupo: se o grupo for anônimo, tanto o cabeçalho quanto todos os jogadores naquele grupo detalhado são projetados como `"Anônimo"`. Se o grupo for público, cada jogador respeita individualmente sua própria privacidade de usuário.
  - Proteção de Mídia: `/api/v1/media/{ref}` verifica anonimato do grupo ou usuário via `PrivacyChecker` e retorna imediatamente HTTP 204 No Content se anônimo, sem realizar chamadas upstream ao Telegram ou expor bytes no cache.
- Mini App Telegram e 3ª aba Perfil:
  - Navegação inferior estendida para 3 abas: `Grupos | Players | Perfil`.
  - Preservado o design de vidro líquido deslizante (`liquid glass`) da barra de navegação com transições fluidas (0%, 100%, 200%).
  - Tela `/profile` com toggle switch acessível (`role="switch"`), feedback dinâmico de salvamento e mutação TanStack Query com invalidação de cache.
  - Componentes de ranking atualizados com ícone neutro estilizado de anônimo para evitar requisições de imagem ou iniciais quando anônimo.
- Todas as validações aprovadas: `make check` (100% de testes unitários e de integração PostgreSQL, vitest, lints, typechecks, build e diff check).
- Regras de isolamento: zero commit, zero push, mantido no working tree da dev.

---

# Refinamento de navegação e cabeçalho do Telegram Mini App — 2026-10-02 (somente dev)

- Pedido do usuário aprovado no plano: `refinar-navegacao-header-telegram_2026-10-02_13-35.md`.
- Eliminação de controles redundantes e duplicações visuais:
  - **Ranking Global**: Removido o botão/seta customizado `<button className="back-button">` do DOM. A raiz não possui rota anterior interna; o fechamento da janela pertence exclusivamente ao chrome nativo do Telegram. O topo exibe apenas o título centralizado e o calendário.
  - **Detalhe do Grupo**: Ao executar dentro do Telegram (onde `BackButton` nativo está disponível via `window.Telegram.WebApp.BackButton`), a seta customizada do hero vermelho é ocultada. A navegação de retorno é delegada integralmente ao `BackButton` nativo do Telegram.
  - **Fallback Web**: Caso a aplicação seja executada fora do Telegram (ambiente de testes/dev sem `BackButton` nativo), o botão customizado de voltar é renderizado discretamente no slot esquerdo do detalhe.
- Centralização dos títulos e layout em Grid:
  - `.title-bar` migrado para CSS Grid simétrico em 3 colunas: `grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr); align-items: center;`.
  - Slots laterais com classes `.title-bar-left` e `.title-bar-right` garantem que o elemento central (`h1`) fique matematicamente no centro da viewport.
  - Removido o compensador artificial `padding-right: 36px` de `.detail-header .title-bar h1`.
  - Ajustes responsivos de fonte para telas estreitas (<= 360px e <= 310px) evitando quebra indesejada ou overflow horizontal.
- Integração de cores com a barra nativa do Telegram:
  - Sincronização dinâmica mantida via `setHeaderColor`: `#073b82` no Ranking Global e `#99121f` no Detalhe de Grupo, restaurando `#073b82` ao retornar para a raiz.
  - Confirmado que a API do Telegram Mini Apps não permite injetar títulos customizados de texto arbitrário dentro da barra nativa do cliente. A solução correta adotada é a continuidade visual perfeita entre o chrome nativo e a primeira faixa da aplicação sem navegação duplicada.
- Ciclo de vida e cleanup do BackButton:
  - `useTelegram` gerencia o desregistro com `offClick` e ocultação com `hide()` ao navegar de volta para o global ou ao desmontar o componente.
  - Testes cobrem ausência de acúmulo de listeners em navegações sucessivas (`global -> detalhe -> global -> detalhe -> global`).
- Todos os elementos preexistentes aprovados preservados: cards, avatares, scores, cards UNO decorativos no hero, onda SVG, marquee, filtros e TanStack Query cache.
- Validações completas: `npm run lint`, `npm run typecheck`, `npm run test` (49 testes passando), `npm run build`, `go test ./...`, `go vet ./...`, `make build`, `make test` e `git diff --check`.

---

# Publicação na branch main — 2026-10-02 (commit 6feef1d)

- Promoção da árvore pública V2 aprovada em `dev` para o histórico independente de `main`:
  - Commit em `main`: `6feef1d` (`feat(v2): release Mini App, monthly ranking, unified app and V2 updates`).
  - Árvore pública limpa e estrita, sem arquivos internos (`.agent/`, `AGENTS.md`, `.reports/`, `codemaps/`, `*.go` na raiz, `*.png` na raiz, `docker-compose.yml`, `Dockerfile` legado ou `docs/v2-audit.md`).
  - Inclui Mini App (`web/`), HTTP API, proxy de mídia com cache LRU, migrations 0006/0007/0008 com autostartup em `internal/app`, comandos endereçados `@bot`, alias `/join`, CLI `cmd/devseed` e workflow multi-arquitetura atualizado.
  - Validações completas aprovadas localmente antes do push: `npm ci`, `npm test`, `npm build`, `go test -race`, `go vet`, `go build`, `git diff --check`, `public-tree` regex check e testes de integração PostgreSQL.
  - Push realizado com sucesso para `origin/main`, acionando o build de container multi-arquitetura OCI e publicação no GHCR (`ghcr.io/gr1ksdev/unobotgo:latest`).

---

# Correção do CI PostgreSQL após migration 0008 — 2026-10-01 (branch dev)

- Causa da falha no CI:
  - O teste `TestListGroupRankingAccumulationIsolationAndHistory` em `internal/storage/postgres/ranking_integration_test.go` criava o grupo 43 via `s.GetOrCreateGroupConfig(ctx, 43)` sem configurar explicitamente o sistema para `Legacy`.
  - Com a introdução da migration `0008_default_ranking_updated.up.sql`, novos grupos são criados por padrão com `ranking_system = 'updated'`.
  - Como o teste gravava uma partida de sistema `Legacy` no grupo 43, `RecordCompletedGame` detectava a incompatibilidade entre o sistema do grupo (`Updated`) e o do resultado (`Legacy`), retornando `ranking.ErrNeedsProductDecision`.
  - O teste foi ajustado para chamar `s.SetRankingSystem(ctx, 43, groups.Legacy)` logo após a criação do grupo 43, expressando formalmente a intenção do teste de testar isolamento multissistema com grupos em sistemas distintos.
- Cobertura adicional da migration 0008 em `internal/storage/postgres/groups_integration_test.go`:
  - **Caso A (novos grupos)**: assume `Updated` por padrão em todas as vias de criação.
  - **Caso B (grupo Legacy existente pré-0008)**: permanece rigorosamente `Legacy` e com mesma revisão após a migration 0008.
  - **Caso C (grupo Updated existente pré-0008)**: permanece rigorosamente `Updated` e com mesma revisão após a migration 0008.
  - **Idempotência**: reaplicação de `Migrate` preserva todos os dados e revisão inalterados.
- Auditoria da migration 0008: confirmada alteração exclusiva de default (`ALTER COLUMN ranking_system SET DEFAULT 'updated'`), sem qualquer `UPDATE` em linhas existentes.
- O alias `/join` não tem qualquer relação com o PostgreSQL e permaneceu inalterado.
- Todas as validações aprovadas: `go test -race -tags integration ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `make check` e `git diff --check`.

---

# Massa fictícia de homologação para o Mini App de Ranking Global — 2026-09-30 (somente dev; pronto para homologação)

- Pedido do usuário aprovado no plano: `massa-ficticia-homologacao-miniapp_2026-09-30_22-40.md`.
- Arquitetura implementada em `internal/devseed` e `cmd/devseed`:
  - **Namespace e Separação Cirúrgica**:
    - Chat IDs reservados para grupos do seed: faixa negativa `[-990000000999, -990000000001]`.
    - User IDs reservados para jogadores do seed: faixa positiva `[9900000001, 9900000999]`.
    - Game IDs reservados: prefixo `devseed_game_`.
    - Limpeza cirúrgica com `make clean-miniapp-seed` baseada nessas restrições exatas, sem tocar dados reais e sem `TRUNCATE`.
  - **Proteção Fail-Closed Contra Produção**:
    - Exige explicitamente `APP_ENV=development` ou `ALLOW_DEV_SEED=1`.
    - Validação de `DATABASE_URL`: rejeita URLs vazias ou hosts de produção.
    - Operação inteiramente transacional (rollback automático em erro).
  - **Massa Determinística Completa**:
    - **Mês Corrente Dinâmico**: calculado via `ranking.MonthStart` em `America/Sao_Paulo`.
    - **Atualizado**: 80 grupos e 180 jogadores (superando o limite de paginação de 50). Nomes curtos, médios, emojis, acentos, 1 grupo com nome gigante e 1 grupo sem título (`Grupo ••••0005`), 1 jogador sem nome (`Jogador ••••0017`).
    - **Legado**: 65 grupos e 140 jogadores completamente segregados do Atualizado (com sobreposição intencional de UserIDs sem compartilhamento de pontuação).
    - **Casos Especiais**: Grupo grande com 60 membros para scroll profundo, grupo pequeno com 2 membros, grupo principal ("UNO da Galera") com scores idênticos para testar desempate por `last_placement` (LucasZ à frente de Mariana) e `last_completed_game_at`.
    - **Multi-Grupo e Histórico**: Jogador Freddy presente em 5 grupos com score acumulado alto (~25.000,00 pts) e display name atualizado para "Freddy UNO".
  - **Idempotência**:
    - Execuções consecutivas de `make seed-miniapp` limpam fixtures anteriores do namespace e reinserem os dados exatos determinísticos sem duplicar scores.
    - `make clean-miniapp-seed` é um no-op seguro se executado repetidamente.
- Verificações completas: testes de segurança e integração no PostgreSQL, 35 testes vitest, Go vet, build e git diff limpos.
- Regras de isolamento: zero commit, zero push, main intocada, sem deploy/migration em produção.

---

# Refinamento visual das cartas de UNO no hero de detalhe — 2026-09-30 (somente dev; pronto para homologação)

- Pedido do usuário aprovado no plano: `refinamento-cartas-hero-detalhe_2026-09-30_14-15.md`.
- Fonte de verdade visual: `mockup_de_rankings_uno_em_iphones.png`.
- Ajustes executados em `web/src/pages/Rankings.tsx`, `web/src/styles.css` e `web/src/App.test.tsx`:
  - 3 cartas físicas de UNO grandes em leque realista:
    - Vermelha em primeiro plano (`z-index: 3`, rotação suave `3deg`, `bottom: 10px`, `right: -32px`).
    - Amarela no meio (`z-index: 2`, rotação `18deg`, `bottom: 44px`, `right: -36px`), corpo e elipse branca visíveis.
    - Verde atrás (`z-index: 1`, rotação `34deg`, `bottom: -22px`, `right: -28px`), corpo e elipse branca visíveis.
  - Proporções físicas de baralho: `clamp(92px, 25vw, 106px)` x `clamp(144px, 39vw, 166px)`, borda branca nítida de 3.5px, border-radius de 12px, elipses centrais com brilho sutil e sombras projetadas em camadas (`-4px 6px 18px rgba(0, 0, 0, 0.42)`).
  - Transbordamento lateral (clipping): `.group-hero` com `overflow: visible;` permitindo que as cartas passem pela borda do hero até a lateral direita do `.app-shell`, onde a borda física da tela as corta naturalmente com zero scroll horizontal. A parte inferior é naturalmente sobreposta pela curva do painel branco de ranking interno.
  - Legenda atualizada de `"Total do grupo no mês de {month}"` para `"Total em {month}"`, com asserção correspondente atualizada em `web/src/App.test.tsx`.
  - Hierarquia e legibilidade: Avatar e `.group-summary` garantidos com `z-index: 10`, mantendo textos de nome, ID, pontos e legenda sempre acima e 100% legíveis.
- Verificações completas: lint, typecheck, 20 testes de frontend em vitest, build de produção em `web/dist`, 16 pacotes Go testados e `git diff --check` sem pendências.
- Regras de isolamento: zero commit, zero push, main intocada, sem deploy/migration em produção.

---

# Terceira passada de fidelidade visual e densidade mobile (390x844px) — 2026-09-30 (somente dev; pronto para homologação)

- Pedido do usuário aprovado no plano: `fidelidade-visual-mockup-rankings-passo-3_2026-09-30_13-38.md`.
- Fonte de verdade visual: `mockup_de_rankings_uno_em_iphones.png` (viewport nativo 390x844px do iPhone).
- Ajustes finos executados em `web/src/styles.css`, `web/src/pages/Rankings.tsx`:
  - Tipografia: nomes ampliados para `16.5px` peso 750 marinho escuro (`#081534`), score para `18px` peso 800 tabular nums, títulos para `18px` peso 750, IDs em `13px` peso 500.
  - Cards: altura mínima de `80px`, padding `12px 16px 12px 12px`, sombras e bordas ultra sutis (`border: 1px solid rgba(226, 232, 240, 0.7)`).
  - Avatares: ampliados para `56px` x `56px` nos cards com borda branca 2px de alta definição.
  - Fundo: clareado para `#f8fafd`, eliminando aspecto acinzentado e reforçando o contraste dos cards `#ffffff`.
  - Header Global: gradiente profundo com iluminação rica (`#073b82` a `#2280ea`).
  - Tela de Detalhes: hero em carmesim acetinado profundo (`#660a14` a `#90111e`), avatar de `104px`, nome com `23px` peso 800, score de `32px`, e cartas UNO decorativas amarela e verde fanned out com bordas brancas sólidas e elipses centrais cortadas pela lateral direita.
- `web/dist` recompilado e pronto para execução com `go run -tags debugcards ./cmd/bot`.
- Regras de isolamento: zero commit, zero push, main intocada, sem deploy/migration em produção.

---

# Segunda passada de fidelidade visual da Mini App de ranking — 2026-09-30 (somente dev; pronto para homologação)

- Pedido do usuário aprovado no plano: `fidelidade-visual-mockup-rankings_2026-09-30_13-10.md`.
- Fonte de verdade visual: `mockup_de_rankings_uno_em_iphones.png`.
- Ajustes executados em `web/src/styles.css`, `web/src/components/Ranking.tsx` e `web/src/pages/Rankings.tsx`:
  - Header Global: gradiente azul royal vibrante (`linear-gradient(180deg, #0947ba 0%, #0d54c7 40%, #156ddf 80%, #1a75ec 100%)`).
  - Controles segmentados: contêineres cápsula `rounded-full` claros. Ativo `Atualizado`, `Legado` e `Grupos` em vermelho (`#ea2328`), ativo `Players` em azul elétrico (`#0262f6`), inativos em texto ardósia `#4b5770` sem fundo.
  - Cards de ranking: proporções ampliadas (altura 76px, radius 22px), topo 1 em amarelo suave quente com medalha dourada e 1 branco, topo 2 em prata suave com medalha prateada e 2 branco, topo 3 em bronze/pêssego suave com medalha bronzeada e 3 branco. Posições 4+ limpas com número sem ponto final (16px negrito).
  - Tipografia: nomes escuros em marinho (#081534), ID com 5 bullets (`ID •••••8462`), score tabular com `pts` semibold.
  - Tela de detalhe: header e hero em carmesim rico (`linear-gradient(180deg, #700d18 0%, #9e1322 35%, #cf1e2c 70%, #9e1322 100%)`), avatar de 92px com borda branca de 4px, nome com 21px negrito, score grande 28px, cartas UNO amarela e verde estilizadas no canto inferior direito e título com ícone 👥 (`UsersIcon`).
- Zero alterações de backend: regras de persistência, APIs, autenticação e dados intocados.
- Regras de isolamento: zero commit, zero push, main intocada, sem deploy/migration em produção.

---

# Mini App de ranking global do UnoBotGO — 2026-09-30 (somente dev; pronto para homologação manual)

- Pedido do usuário aprovado no plano: `miniapp-ranking-global_2026-09-30_11-20.md`.
- Arquitetura implementada:
  - Frontend SPA (React 19, TypeScript estrito, Vite, Tailwind v4, TanStack Query, React Router) em `web/`.
  - Compilação via `make web-build` para `web/dist`, embutido diretamente no Go via `web/embed.go` (`//go:embed all:dist`).
  - Executável único `bin/unobotgo` gerado por `make build`.
  - Startup orquestrado em `internal/app/app.go`: ordem estrita config → PostgreSQL → advisory lock transacional (71870101) → Migrate → VerifySchema → release lock → listener HTTP unificado (API, estáticos, webhook) → bot dispatcher e workers. Falha de migração encerra imediatamente com fail-closed.
- Segurança e integridade de dados:
  - Autenticação via `Authorization: tma <initData>`, validada por HMAC-SHA256 oficial em tempo constante em `internal/httpapi/auth.go`.
  - IDs de Telegram mascarados: interface exibe somente `ID ••••1234`.
  - Referências técnicas, cursores de paginação keyset e URLs de avatar utilizam tokens criptografados e autenticados com AES-GCM (derivados de `MINIAPP_SECRET`).
  - Precisão decimal: `score_units` transmitido como string no JSON e manipulado via `BigInt` no TypeScript, evitando perda de precisão em inteiros > 2^53.
  - Período mensal corrente calculado no backend em `America/Sao_Paulo`. Requisições com mês divergente retornam 409 `ranking_period_changed`.
- Cache de fotos de avatar:
  - Cache LRU em memória em `internal/media/service.go` (máx 2000 entradas e 64 MiB), rate limit por ticker de 350ms e workers dedicados.
  - Downloads protegidos em `internal/telegram/avatar.go` restritos à `api.telegram.org` HTTPS com redação total de tokens/URLs em logs e erros.
- Interface visual e paridade de produto:
  - Layout e estilos em `web/src/styles.css` aderentes ao mockup oficial (`mockup_de_rankings_uno_em_iphones-2.png`).
  - Header global em gradiente azul; detalhe do grupo em gradiente vermelho com hero, avatar ampliado, pontuação total e cartas UNO decorativas.
  - Medalhas top 3 com SVG customizado (ouro, prata, bronze), posições únicas e fundos destacados (place-1 ouro suave, place-3 bronze suave).
  - Segmented controls de universos (Atualizado/Legado) e abas (Grupos/Players) com estados ativo vermelho/azul.
  - Cards de grupos clicáveis (abrem `/groups/:groupRef`); cards de players não clicáveis.
  - Navegação de volta e integração com BackButton nativo do Telegram.
- Botão inline no Telegram:
  - Respostas de `/ranking` em grupos e no privado agora incluem o botão inline `🌐 Ranking Global` apontando para o Direct Link /ranking gerado pelo username do getMe via URL comum do Telegram (configuração atual desde 2026-10-01).
- Regras de isolamento: zero commit, zero push, main intocada, sem deploy/migration em produção.

---

# Melhoria visual e explicativa do comando /config — 2026-09-30 (somente dev; em homologação)

- Pedido do usuário aprovado no plano: `melhoria-ux-config_2026-09-29_23-57.md`.
- Formatação de `/config` enriquecida em `RenderGroupConfig` (`internal/telegram/renderer.go`):
  - Inclui exatamente dois blocos de citação (`<blockquote>...</blockquote>` em HTML).
  - Blockquote 1 (modo ativo):
    - Clássico: `<b>🎮 Clássico</b>\nRegras padrão do bot, sem as combinações extras do modo Caseiro.`
    - Caseiro: `<b>🎮 Caseiro</b>\nPermite combinações extras entre cartas de compra, como +4 sobre +2 e +2 da cor escolhida sobre +4.`
  - Separador em linha própria estritamente entre os dois blockquotes: `────────────`.
  - Blockquote 2 (sistema de ranking ativo):
    - Legado: `<b>🏆 Legado</b>\nTodos os jogadores elegíveis, exceto o último colocado, recebem +1 ponto.`
    - Atualizado: `<b>🏆 Atualizado</b>\nA pontuação varia conforme a colocação: quanto melhor a posição, mais pontos o jogador recebe.`
  - Rodapé em duas linhas: `Selecione abaixo para alterar.\nAs mudanças afetarão apenas as próximas partidas criadas.`.
- Reatividade dinâmica: tanto a abertura inicial (`handleConfig`) quanto os callbacks de modo (`cfg_mode`), ranking (`cfg_rank`) e reabertura (`cfg_open`) já utilizam `RenderGroupConfig`, fazendo com que as edições de mensagem reflitam instantaneamente a nova seleção e removam os resumos anteriores.
- Permissões, callbacks, regras de gameplay/ranking e migrations preservados sem alterações.
- Regras de isolamento: zero commit, zero push, main intocada, sem deploy/migration em produção.

---

# Ranking mensal privado (/ranking no privado) — 2026-09-29 (somente dev; em homologação)

- Pedido do usuário aprovado no plano: `ranking-privado_2026-09-29_22-50.md`.
- Bifurcação de `/ranking`:
  - No PRIVADO: consulta as pontuações mensais do próprio usuário remetente (`msg.From.ID`) por grupo e exibe seções separadas para `Atualizado` e `Legado`. Nunca aceita UserID por texto/parâmetro.
  - Em GRUPOS: comportamento 100% preservado (ranking competitivo do grupo com medalhas e desempates).
- Separação estrita dos sistemas:
  - Seções independentes para `Atualizado` (com centésimos `pt-BR`) e `Legado` (com pontuação inteira `pt/pts`), cada uma com seu próprio `Total · ...` calculado via soma inteira de `score_units`.
  - Nunca soma os dois sistemas em um total único.
  - Seções vazias não são exibidas.
  - Usuário sem nenhuma participação elegível no mês recebe mensagem amigável: `Você ainda não possui partidas pontuadas neste mês.`.
- Título dos Grupos e Migração 0007:
  - Nova migration `internal/storage/postgres/migrations/0007_group_title.up.sql`: adiciona coluna `title text NOT NULL DEFAULT ''` na tabela `group_configs` e índice `player_group_monthly_stats(user_id, month_start)`.
  - Método `ObserveGroupTitle(ctx, chatID, title)` atualiza o título do grupo de forma não destrutiva e condicional (`WHERE EXCLUDED.title <> '' AND group_configs.title IS DISTINCT FROM EXCLUDED.title`), sem sobrescrever com vazio/nulo.
  - Títulos são observados automaticamente em comandos de grupo e no evento `HandleMyChatMember`.
  - Fallback determinístico: se o grupo não possuir título gravado, exibe `Grupo <ChatID>` (ex.: `Grupo -1004477538462`).
- Consulta única sem N+1:
  - `ListUserMonthlyRankings` em `internal/storage/postgres/ranking.go` executa uma única query buscando todas as linhas de `player_group_monthly_stats` unidas com `group_configs` para o mês corrente (`America/Sao_Paulo`).
  - Ordenação determinística: por sistema, `score_units DESC`, `last_finished_at DESC`, `nome do grupo ASC`, `chat_id ASC`.
  - Jogadores com score 0 elegível aparecem normalmente. Abandonos definitivos continuam fora do ranking.
- Limite de mensagem e segurança Telegram:
  - `RenderUserMonthlyRankings` respeita o limite de 4000 unidades UTF-16, truncando grupos ordenadamente com indicação `… e mais X grupos.` (ou `1 grupo.`) e mantendo o `Total` acumulado com o valor real de todos os grupos do sistema.
  - Escape HTML rigoroso nos títulos de grupos.
  - Sem botões inline, ranking global ou WebApp nesta etapa.
- Regras de isolamento: zero commit, zero push, main intocada, sem deploy/migration em produção.

---

# Ranking mensal do grupo — 2026-09-29 (somente dev; em homologação)

- Pedido do usuário aprovado no plano: `ranking-mensal_2026-09-29_21-35.md`.
- Transformação do ranking acumulado em RANKING MENSAL do mês calendário vigente.
- Timezone canônico: `America/Sao_Paulo` (incorporado via `_ "time/tzdata"` para disponibilidade universal). A virada ocorre estritamente às 00:00:00 do dia 01 de cada mês em Brasília. Uma partida concluída pertence integralmente ao mês do seu `finished_at`.
- Sem resets físicos, sem jobs cron ou deleção de tabelas: o histórico anterior permanece 100% preservado no PostgreSQL.
- Nova migration `internal/storage/postgres/migrations/0006_monthly_ranking.up.sql`:
  - Cria tabela `player_group_monthly_stats` com chave primária `(chat_id, user_id, month_start)` e índice `(chat_id, month_start, score_units DESC, user_id)`.
  - Executa backfill idempotente agregando partidas pontuadas anteriores de `completed_games` e `completed_game_players`.
- Persistência atômica: `RecordCompletedGame` atualiza `player_group_stats` (acumulado all-time) e `player_group_monthly_stats` (mês corrente) na mesma transação atômica. Idempotência por `GameID` retorna `AlreadyPersisted: true` sem duplicar pontuação.
- Consulta e Desempate: `ListGroupRanking` agora aceita `at time.Time`, consulta `player_group_monthly_stats` e restringe a CTE de desempate (`latest`) exclusivamente às partidas pontuadas daquele mês. Partidas de meses anteriores não influenciam o critério de desempate do mês vigente.
- Renderização Telegram: `RenderGroupRanking` exibe o cabeçalho `🏆 Ranking do grupo · <NomeDoMês>` (ex.: `Setembro`, `Outubro`) e, quando o mês não tiver partidas, a mensagem amigável `Ainda não há partidas pontuadas neste mês.`.
- Regras de isolamento: zero commit, zero push, main intocada, sem deploy/migration em produção.

---

# Correção de intermitência em testes de Trocar Mãos (Swap Hands) — 2026-09-29 (somente dev)

- Pedido do usuário aprovado no plano: `corrigir-intermitencia-swap-hands_2026-09-29_13-26.md`.
- Causa raiz: o helper de teste `readyToSwap` em `internal/telegram/swap_test.go` realizava compras e passes sucessivos até encontrar a carta `SwapHands`, sem descartar nenhuma carta. Quando o embaralhamento aleatório (`rand.Shuffle`) posicionava a carta nas últimas 6 posições da pilha (posições 88 a 93 de 94 cartas), restavam menos de 7 cartas no `DrawPile`. Testes subsequentes que realizavam `uno.JoinGame` (`TestKeepHandInlineFlowStaleColorAndMultigroup:117` e `TestSwapInlineTargetDepartureInvalidatesColor:224`) falhavam intermitentemente com `uno.ErrDeckEmpty` ("not enough drawable cards"), pois entradas tardias exigem a compra obrigatória de 7 cartas pela regra do UNO.
- Correção implementada:
  - No helper `readyToSwap` (`internal/telegram/swap_test.go`), adicionado controle de tentativas (até 5) com `ChatID` isolado a cada tentativa (`ChatID(-901 - int64(attempt)*10)`).
  - Rastreamento da quantidade de compras (`cardsDrawn`): se `SwapHands` for encontrada com `cardsDrawn <= 80`, a partida é aceita e retornada (garantindo pelo menos 14 cartas no `DrawPile`).
  - Se a pilha for excessivamente drenada (`cardsDrawn > 80`), a tentativa é descartada e uma nova partida reembaralhada é iniciada, garantindo probabilisticamente mais de 99% de sucesso em até 2 tentativas e eliminando totalmente a escassez de cartas para `JoinGame`.
- Validações:
  - Stress tests repetidos 100 vezes: `go test -count=100 -run '^TestKeepHandInlineFlowStaleColorAndMultigroup$' ./internal/telegram` e `go test -count=100 -run '^TestSwapInlineTargetDepartureInvalidatesColor$' ./internal/telegram` passaram com 100% de sucesso (zero falhas).
  - Suíte completa normal (`go test -count=1 ./...`) e com race detector (`go test -count=1 -race ./...`) 100% aprovadas.
  - `go vet ./...`, `go build ./...` e `git diff --check` aprovados.
  - Zero alterações em código de produção, regras de jogo ou branch `main`.

---

# Correção do desempate — 2026-09-29

- Autorização posterior explícita: “faça o commit e o push pra dev”. Exceção à suspensão registrada abaixo, restrita a esta entrega na dev. Push inclui o commit local 64bb97d e a correção; main/deploy/publicação de container continuam fora do escopo.

- Regra operacional obrigatória do usuário: não executar git commit/push/amend/rebase; alterações somente no working tree da dev até autorização explícita FUTURA. Preservar HEAD 64bb97d e main 6eea6c1.
- Decisão anterior 1/1/3 obsoleta. Agora score DESC, última colocação elegível ASC, conclusão da última partida elegível DESC, UserID ASC. Cada jogador usa sua própria última partida; não precisam ter jogado juntos.
- Sem schema novo: CTE DISTINCT ON seleciona completed_game_players.position + completed_games.finished_at por jogador, apenas scored e participação com placement/status/went_out elegíveis. GameID DESC estabiliza escolha se há partidas distintas do mesmo usuário no mesmo timestamp. Join com stats antes do limite; NULLS LAST se referência ausente.
- Renderer compartilhado apenas enumera posições 1..N. Pontos continuam em stats, nenhum recálculo/alteração de escrita/fórmula/elegibilidade. UTF-16/layout/comando público preservados.
- Integração real PostgreSQL18.6 local passou: score primário, placements 1/2/3/5, data, UserID, partidas pessoais, histórico ausente, 3000/3000/0, inversão após novas vitórias individuais, duplicata, abandono, pending/N<2 e commit diferido rejeitado.
- Gates normal, debugcards (segunda execução), vet/build padrão e tagged, diff check aprovados. Primeira debugcards falhou em TestSwapInlineTargetDepartureInvalidatesColor, optional_swap_test.go:224, “not enough drawable cards”; reproduzido 4/20 na cópia da base 4f25eeb. Arquivos desse teste/fixture idênticos à base 64bb97d e intocados. Não ocultar intermitência. Race CGO=1 falhou por ThreadSanitizer VMA 39, suportado 48.
- Nome “.”: observedName concatena FirstName/LastName, stats preserva nome observado e renderer só escapa HTML. Teste confirma preservação do ponto. Não foi consultado o banco de homologação para confirmar aquele UserID; não alterar nomes nesta tarefa.
- Documentação pública e plano atualizados; sem publicação/deploy/migration de produção. Aguardar homologação do usuário.

---

# Histórico: ranking acumulado visível — 2026-09-29 (empates substituídos acima)

- Pedido autorizou implementação direta após auditoria, sem push/main/container/deploy/produção. Base dev 4f25eeb; main preservada em 6eea6c1399d287b58116d5be16a1807238653397.
- Novo `ranking.ReadRepository`, `ranking.GroupRanking` e `ranking.Service.ListGroupRanking`; Store lê somente player_group_stats do ChatID, ordenando score_units DESC/user_id. Config, contagem e prefixo de até 512 em um snapshot SQL. Incompatibilidade de sistema em qualquer linha recusa leitura. Escrita, fórmulas, elegibilidade e migrations inalteradas.
- `ranking.FormatScore` compartilha inteiros Legacy e centésimos Updated; `RenderGroupRanking` atende /ranking e pós-commit. Empates exatos 1/1/3, UserID somente estabilidade. Limite 4000 unidades UTF-16 após entidades HTML, linhas completas e total omitido; sem truncar nomes.
- Fechamento pontuado envia resultado com ganhos e ranking histórico em duas mensagens, substituindo resumo redundante; paths comando/inline/timeout mantêm tokens invalidados. Falhas/AlreadyPersisted/N<2 preservam comportamento de finalização sem falso anúncio. Retry existente preservado; Telegram continua sem outbox.
- /ranking público no grupo, ajuda/menu; privado orienta grupo, sem dados informa vazio, falha de leitura informa indisponibilidade. Nomes persistidos, sem GetChatMember. Abandonado sem posição/score na mensagem: “fora do ranking”; histórico anterior continua intacto.
- Testes reais PostgreSQL 18.6 local isolado passaram: A/B 1500/1500/0, histórico ausente, grupo isolado, idempotência, abandono, limite de consulta, incompatibilidade além do prefixo e trigger diferido que força falha no COMMIT.
- Gates finais: go test -count=1 ./..., debugcards, vet/build normal e debugcards e diff check passaram. Race com CGO=1 não executável: ThreadSanitizer VMA 39, requer 48; não declarar aprovado.
- Primeiras execuções normal/debugcards falharam em TestKeepHandInlineFlowStaleColorAndMultigroup, optional_swap_test.go:117 (“not enough drawable cards”). Reproduzido na base 4f25eeb extraída em /tmp, com -count=20 (4 execuções falharam). Teste preexistente não alterado; últimas execuções completas passaram. Não confundir com falha V1 ou regressão de ranking.
- Homologação Telegram pelo usuário ainda pendente; roteiro em docs/v2-telegram.md. Artefatos .agent ficam somente na dev.

---

# Correção de observação de usuário no /config e callbacks — 2026-09-28 (somente dev)

- Pedido do usuário aprovado no plano: `corrigir-observacao-config_2026-09-28_22-26.md`.
- Causa raiz da falha em `ObserveGroupUser`: em grupos novos (onde `/novo` ainda não havia sido executado), `group_configs` não possuía registro para aquele `chat_id`. A tabela `known_group_users` possui chave estrangeira `chat_id REFERENCES group_configs(chat_id)`. Em `handleConfig` e `handleConfigCallback`, `ObserveGroupUser` era chamado antes da resolução/criação da configuração, resultando em erro 23503 (violação de foreign key).
- Correção implementada:
  - Em `internal/telegram/commands.go:handleConfig`, `svc.Repository.GetOrCreateGroupConfig` é chamado antes de `ObserveGroupUser`, garantindo que o registro do grupo exista no banco de dados com os defaults (`Classic` + `Legacy`).
  - Em `internal/telegram/callbacks.go:handleConfigCallback`, a mesma garantia foi aplicada antes de invocar `ObserveGroupUser`.
  - Se `GetOrCreateGroupConfig` falhar (ex: indisponibilidade do banco), a observação de usuário é omitida de forma segura, impedindo inserção de registros órfãos ou erros de FK.
  - Regras de autorização (`CanConfigureUser`, `SetDefaultGameMode`, `SetRankingSystem`), instalador registrado (`installed_by_user_id`), defaults e regras de `/novo` permanecem 100% preservadas.
  - Schemas e migrations intactos: a chave estrangeira foi mantida e respeitada.
- Testes e regressões:
  - Adicionado suporte a verificação de foreign key no mock de repositório de usuários em `internal/telegram/config_test.go`.
  - Testes de regressão adicionados cobrindo primeiro `/config` por admin, primeiro `/config` por usuário comum (bloqueado na autorização mas observado) e callbacks de configuração em grupo novo.
  - Teste de integração PostgreSQL `TestObserveGroupUserRequiresGroupConfigFK` adicionado em `internal/storage/postgres/users_integration_test.go` validando o comportamento real da restrição de chave estrangeira.
  - Validações aprovadas: `go test -count=1 ./...`, `go test -tags debugcards ./...`, `go vet ./...`, `go build ./...`, `git diff --check` e testes de integração PostgreSQL `go test -count=1 -tags integration ./internal/storage/postgres/...`.

---

# Encerramento da Milestone M7 e Adiamento de Import de Ranking Antigo — 2026-09-27 (somente dev)

- Status final: Milestone M7 CONCLUÍDA e HOMOLOGADA NO TELEGRAM REAL.
- Escopo entregue e homologado no Telegram real:
  1. Persistência PostgreSQL com pool `pgx/v5` e migrações versionadas (`cmd/migrate`) com lock consultivo e checksums.
  2. Configuração por grupo persistente (`group_configs`) com defaults não-bloqueantes (`Classic` + `Legacy`).
  3. Snapshot imutável de configuração por partida no momento da criação, mantendo `/novo classico` e `/novo caseiro` como overrides de partida única.
  4. Cálculo e concessão determinística de ranking em centésimos (`half-up`) para os sistemas Legacy e Updated.
  5. Regras de elegibilidade: concluintes válidos entram em N; jogadores abandonados (`/sair` definitivo) ficam fora do ranking, não contam em N e recebem +0,00; reentrantes que concluem pontuam normalmente conforme colocação.
  6. Fechamento síncrono atômico no encerramento da partida, com idempotência por `(game_id, canonical_hash)` e mensagem dedicada de anúncio de pontos pós-commit enviada apenas após confirmação transacional.
  7. Interface de configuração Telegram `/config` com botões inline interativos, autorização para administradores e instalador ativo, e captura de transição real de instalação/reentrada via `my_chat_member`.
  8. Proteção contra troca de ranking (`ErrNeedsProductDecision`): bloqueio de alternância entre Legacy e Updated quando o grupo já possui histórico acumulado, preservando a integridade estatística.
- Cenários homologados no Telegram real:
  - Legacy N=2 e N=3.
  - Updated N=2 (1º=+10,00, 2º=+0,00) e N=3 (1º=+10,00, 2º=+5,00, 3º=+0,00).
  - Abandono definitivo com exclusão de N e menção `(fora do ranking)`.
  - Saída seguida de reentrada com pontuação normal.
  - `/config`, seleção de modos/rankings, e recusa amigável de troca com histórico.
- Import de ranking antigo: DEFERRED. "Old ranking import is deferred to a future milestone."
  - Parser e reconciliação pura em `internal/rankingimport` e tabelas de staging em migration `0004` retidos como fundação.
  - Nenhum comando de importação criado, nenhum score importado aplicado, nenhuma conversão assumida.
- Artefatos internos: `.agent/` e `AGENTS.md` são exclusivamente de desenvolvimento na `dev` e não devem ser promovidos para a `main`.

---

# Correção de blefe em +4 sobre +2 no Caseiro e reentrada na mesma partida — 2026-09-26 (somente dev)

- Pedido do usuário aprovado no plano: `corrigir-blefe-caseiro-e-reentrada_2026-09-26_13-48.md`.
- Blefe em +4 como counter de +2 no Caseiro:
  - `internal/uno/game.go` identifica especificamente `stackedOnDrawTwo := s.DrawCounter > 0 && top.Rank == DrawTwo && s.Rules.StackWildDrawFourOnTwo`.
  - Nesses casos, `DrawFourChallengeable = false` e `bluffing = false` são passados via `ColorChoice` para a fase de escolha de cor e para o estado da engine (`State.DrawFourChallengeable`).
  - Em `choose()`, `s.PendingBluff` e `s.DrawFourChallengeable` só são preenchidos se `pending.DrawFourChallengeable` for verdadeiro.
  - A ação `CallBluff` em `takeAction` rejeita com `ErrInvalidAction` se `!s.DrawFourChallengeable || s.PendingBluff == nil`, impedindo ações forçadas ou callbacks com tokens obsoletos.
  - Na camada de visão (`internal/game/views.go`), `CanCallBluff` exige `state.DrawFourChallengeable`, garantindo que botões/stickers de desafio não sejam exibidos para o próximo jogador.
  - O blefe de +4 normal fora de stacking permanece 100% inalterado e o modo Clássico (`ClassicRules`) não é afetado.
- Reentrada de jogador após `/sair` vs Colocação:
  - Jogador que usou `/sair` sem ter colocado (`HasPlacement(id) == false` e status `Left`) pode reentrar na partida ativa via `/entrar` caso a sala esteja aberta.
  - O retorno reutiliza exatamente o algoritmo existente de late join (compra 7 novas cartas da pilha com `g.draw(s, 7)` e entra na cauda lógica da ordem de turnos). O registro histórico em `s.Players` é reutilizado em vez de duplicado, preservando os invariantes de `s.Validate()`.
  - Jogador que já conquistou colocação (`WentOut` / presente em `Placements`) fica definitivamente impedido de reentrar (`uno.ErrAlreadyFinished`), exibindo `"🏁 Você já terminou esta partida e não pode entrar novamente."`.
  - Precedência rigorosa no Join (`internal/game/service.go`): Placed (`ErrAlreadyFinished`) -> Ativo (`ErrAlreadyJoined`) -> Trancado (`ErrRoomLocked`) -> Admissão.
  - Se a sala estiver trancada (`RoomLocked`), tanto novo jogador quanto jogador que deu `/sair` recebem `"🔒 Esta partida está trancada e não aceita novos jogadores."`. Ao destrancar (`/destrancar`), a reentrada funciona normalmente.
  - Unicidade garantida: nenhum `PlayerID` aparece duplicado em `Placements` nem em `Players`.
- Testes automatizados cobrem os cenários 1 a 5 de blefe e testes de ciclo de vida/reentrada com salas abertas e trancadas.

---

# Resolução do sticker cinza de Trocar cartas — 2026-09-26

- Plano aprovado com "sim": `corrigir-sticker-cinza-troca-maos_2026-09-26_00-15.md`.
- Causa raiz de `DOCUMENT_INVALID`: o `file_id` anterior (`CAACAgEAAxkBAAER8aRqtlf6ZtRKfAj02K5AnlVcRz_W_AACVAcAAkaGsEXgXGCANqlQKz0E`) era inválido para a Bot API do bot (`400 wrong file_id` em `getFile`), fazendo com que qualquer `InlineQueryResultCachedSticker` com esse ID causasse erro 400 no `answerInlineQuery`.
- Solução: upload de `assets/stickers/swap_hands_grey.png` via `sendSticker` diretamente pelo bot para o chat do usuário autorizado (`7595607953`), gerando o `file_id` válido e autenticado `CAACAgEAAxkDAAMoarc5AnTTNQ_W6bTz1yaQVlhRR20AAtwHAAJGhrhFYMGxG-e10Xc9BA` (`file_unique_id: AgAD3AcAAkaGuEU`, confirmado com HTTP 200 no `getFile`).
- O mapeamento em `StickersGrey["swap_hands"]` foi atualizado com o novo `file_id`.
- Removido o contorno textual (`InlineQueryResultArticle`) de `internal/telegram/inline.go`; a carta indisponível volta ao fluxo nativo de `InlineQueryResultCachedSticker` com prefixo `grey_` e sem token de jogada.
- Testes unitários atualizados em `internal/telegram/swap_test.go`; validações de build e vet com e sem tag aprovadas.
- Homologado com sucesso pelo usuário no Telegram real ("deu certo agora"), com o sticker cinza renderizando perfeitamente sem erros.

---

# Correção de falso tópico — 2026-09-25

- Usuário confirmou grupo comum sem tópicos e aprovou plano corrigir-falso-topico_2026-09-25_23-55.md.
- HandleMessage e HandleReset agora bloqueiam apenas IsTopicMessage. MessageThreadID isolado pode indicar thread comum e não é motivo de bloqueio.
- Regressões: /novo, /entrar e /reset em thread comum; reset não autorizado permanece recusado; tópicos reais com ID zero/não zero seguem bloqueados sem mutação; /dar tagged funciona em thread comum.
- Testes internal/telegram e build ./... passaram com e sem debugcards; git diff --check aprovado. Homologação no grupo afetado pendente.
- Sem commit, push ou deploy. Alterações anteriores preservadas.

---

# Comando descartável /dar — 2026-09-25

- Plano revisado aprovado com "sim": comando-dar-carta_2026-09-25_23-44.md.
- Disponível exclusivamente com tag debugcards. Ativar: `go run -tags debugcards ./cmd/bot` ou `go build -tags debugcards -o /tmp/unobot-debugcards ./cmd/bot`.
- Autorização exclusiva: Telegram ID 7595607953, validado no serviço. Outros usuários no grupo são ignorados sem revelar o comando. Não aparece em ajuda ou menus.
- Sintaxe: /dar troca, /dar coringa, /dar +4, /dar <vermelho|azul|verde|amarelo> <0..9|+2|pular|inverter>.
- Responder a mensagem do alvo ativo; sem resposta, entregar para o próprio solicitante. Usuário autorizado pode ser observador ao entregar a outro participante.
- Move primeira cópia do monte ou, se faltar, do descarte do fundo ao topo excluindo topo. Nunca cria cartas nem retira de mãos alheias. Recusa indisponibilidade.
- Somente TakingTurn, sem DrawCounter nem PendingBluff. Preserva turno, prazo, direção, cor e DrawnCardID; revisão incrementa uma vez. Troca apenas no caseiro.
- Implementação em debugcards.go e testes tagged nos pacotes uno, game e telegram. Código normal só tem hook em commands.go e stub debugcards_disabled.go. GiveCard não existe nas APIs compiladas sem tag.
- Desativar: substituir por build sem tag e reiniciar (partidas em memória são perdidas no reinício). Docker/pipeline atuais já compilam sem tag.
- Remover definitivamente: retirar hook de commands.go e arquivos debugcards*.go dos três pacotes, preservando histórico local. O código-fonte continua visível a quem acessa o repositório.
- Validações: go test ./..., go test -tags debugcards ./..., build/vet de ambas variantes, race com tag em uno/game/telegram e git diff --check passaram. go list confirmou exclusão da implementação tagged no build padrão.
- Correção anterior de DOCUMENT_INVALID preservada. Sem commit, push ou deploy nesta etapa.

---

# Correção de abertura da mão com Trocar cartas indisponível — 2026-09-25

- Usuário autorizou corrigir diretamente, sem criar plano.
- Relatos: DOCUMENT_INVALID somente para jogador com Trocar cartas, após desafio de +4 perdido pelo adversário e após +2; voltou a abrir depois de mudar a mesa.
- Mitigação: SwapHands indisponível volta a InlineQueryResultArticle, prefixo grey_, sem token. Sticker colorido jogável preservado. ID/asset cinza retidos para investigação, sem envio no menu.
- Causa suspeita: documento do sticker cinza rejeitado no inline; ainda não confirmada por reprodução real.
- Teste de regressão exige artigo, ausência do documento cinza e nenhuma ação ao selecionar.
- Validações aprovadas: testes TestSwap, suíte internal/telegram, build de cmd/bot e git diff --check. Homologação real pendente.

---

# Trocar cartas no modo caseiro — 2026-09-25

- Aprovação explícita: usuário respondeu "sim" ao plano trocar-cartas-caseiro_2026-09-24_23-58.md.
- CaseiroRules habilita AllowSwapHands: baralho padrão de 109 cartas, com uma SwapHands sem cor (c109); ClassicDeck/BotRules continuam com 108.
- Sticker: CAACAgEAAxkBAAER8VtqteJsR8-zG10NFeLTIZyxuZYsBQACBwkAAkkSsEU562tb90Ja3D0E.
- PlayCard descarta a carta e abre ChoosingPlayer. ChoosePlayer/TargetID troca todas as cartas restantes com outro participante ativo e avança o turno mantendo cor, ordem e direção.
- Restrições aprovadas: não finalizar com a carta, não jogar sobre coringa e não responder penalidades. UNO é anunciado após a troca para cada envolvido com uma carta.
- Autor não sai nem tem turno pulado por timeout enquanto escolhe; demais podem sair/entrar e revisões antigas são recusadas. Cancelamento e encerramento por saída funcionam na fase nova.
- Serviço autoriza a ação inline e expõe PlayerChooserID sem mãos alheias. Telegram reutiliza tokens pessoais de uso único e revisão; menu lista nomes/contagens. A carta indisponível usa o sticker cinza `CAACAgEAAxkBAAER8aRqtlf6ZtRKfAj02K5AnlVcRz_W_AACVAcAAkaGsEXgXGCANqlQKz0E`, com resultado `grey_` sem token de ação.
- O asset fonte da variante indisponível está em `assets/stickers/swap_hands_grey.png` (PNG RGBA, 342×512, 206642 bytes). O Telegram confirmou o sticker como estático; testes garantem que o resultado cinza não altera revisão nem turno.
- Alternar modo no lobby reconstrói apenas o deck padrão. State.CustomDeck preserva decks de WithDeck entre mudanças e serialização; regra incompatível com deck especial injetado é recusada.
- Simulador escolhe a menor mão ativa (desempate por ordem), descreve escolha/troca e contabiliza HandSwaps.
- Validações aprovadas: go test ./..., go build ./..., go vet ./..., go test -race ./internal/uno ./internal/game ./internal/telegram ./internal/simulation e git diff --check.
- Simulações seed 20260924, 4 jogadores: caseiro terminou em 119 ações (incluindo troca), clássico em 37 ações. Relatórios em .reports/simulations/.
- Homologação visual/disponibilidade do file_id no Telegram real pendente. Nenhum deploy ou commit realizado nesta tarefa.
- Sticker cinza registrado e enviado ao usuário `7595607953`; validação desta correção: testes Telegram focados 20 vezes, `go test ./...`, `go vet ./...`, `go build ./...` e `git diff --check` aprovados.

---

# Memória atual — simulador local de partidas — 2026-09-23

- Pedido aprovado: ambiente de testes na `dev`, organizado em milestones, com escolha de jogadores/modo, bots autônomos e relatório final.
- Ajuste posterior de 2026-09-24: o relatório passou a incluir início, fim, tempo total de execução e histórico completo de preparação e jogadas, com revisão, ação, todos os eventos públicos da engine e estado da mesa após cada etapa. A linha do tempo resumida de especiais foi preservada.
- Implementação:
  - `cmd/simulator`: interface interativa e flags `--players`, `--mode`, `--seed`, `--max-actions`, `--output` e `--quiet`.
  - `internal/simulation`: configuração, estratégia determinística, runner, diagnósticos, estatísticas e Markdown.
  - Quantidade aceita: 2–10. Modos: Clássico=`uno.BotRules()` e Caseiro=`uno.CaseiroRules()`.
  - A mesma seed reproduz embaralhamento, dealer e decisões dos bots.
  - Estratégia usa `Game.CanPlay`; empilha quando possível, desafia +4 quando não pode responder, escolhe a cor predominante, compra e passa pela API real.
  - Estado validado após cada ação; rejeição, invariante, cancelamento ou limite gera diagnóstico e relatório parcial.
  - Relatório registra colocações, ações, compras, maior mão, UNO, especiais, empilhamentos, penalidades e blefes, com explicação contextual.
- Uso: `make simulator` ou `go run ./cmd/simulator --players 4 --mode caseiro --seed 20260924`.
- Relatórios padrão ficam em `.reports/simulations/` e são ignorados pelo Git.
- Validações normais aprovadas; o race detector não é suportado pelo linker C do ambiente Termux/Android atual.

---

# Memória atual — correção de autorização do CallBluff no Service — 2026-09-23

- Pedido do usuário: "ao tentar fazer o blefe: ⚠️ Mezi: Jogada não aceita: invalid action. Abra Suas cartas novamente.; nao precisa criar plano para correcao"
- Causa identificada: em `internal/game/service.go`, a função de autorização `authorize` verificava os tipos de ação recebidos inline. Faltava incluir `uno.CallBluff` no switch (`case uno.PlayCard, uno.DrawCard, uno.PassTurn, uno.ChooseColor, uno.CallBluff:`), fazendo com que a validação caísse no `default: return uno.ErrInvalidAction` antes de encaminhar a ação para a engine `uno.Game`.
- Correção:
  - Adicionado `uno.CallBluff` ao caso de ações inline autorizadas em `internal/game/service.go`.
  - Adicionado teste de regressão `TestService_CallBluffAuthorized` em `internal/game/service_test.go`.
- Testes: 100% aprovados sem regressões.

---

# Memória anterior — seleção de modo no lobby e travamento pós-início — 2026-09-23

- Usuário aprovou o plano: "sim". Registro: `selecao-modo-lobby_2026-09-23_20-55.md`.
- Substituição do botão no Lobby:
  - Durante `Phase == uno.Lobby`, o botão inline da mensagem do grupo agora exibe a seleção de modo (`[ ✅ 🎻 Clássico ]  [ 🏠 Caseiro ]` ou `[ 🎻 Clássico ]  [ ✅ 🏠 Caseiro ]`), em vez de "Suas cartas".
  - O responsável/criador da partida (`view.OwnerID`) pode alternar livremente entre os modos Clássico e Caseiro clicando nos botões diretamente na mensagem.
  - Ao alternar, o texto do lobby (`Regras: Clássico` ou `Regras: Caseiro`) e os botões inline são atualizados imediatamente.
- Bloqueio após início da partida:
  - Ao iniciar com `/iniciar`, `view.Phase` transita para `TakingTurn` e o botão passa a ser `[ 🃏 Suas cartas ]`.
  - Qualquer clique residual em botões de modo de mensagens anteriores é rejeitado no servidor com alerta `⚠️ A partida já foi iniciada, não é possível alterar o modo!`.
  - Tentativas de alteração por não-responsáveis são rejeitadas com alerta `⚠️ Apenas o responsável pela partida pode alterar o modo.`.
- Engine e Service:
  - `uno.SetRules ActionType = 11` e `uno.RulesChanged EventType = "rules_changed"`.
  - Método `s.SetRules(ctx, actor, gameID, rules)` em `Service`.
  - Regra validada: só permitida durante `Phase == uno.Lobby` e autorizada apenas para `entry.ownerID`.
- Testes 100% aprovados.

---

# Memória atual — correção da penalidade cruzada Caseiro — 2026-09-24

- Relato confirmado: `+2 → +4` mantinha 2 durante a escolha de cor, mas `choose` substituía o contador por 4.
- Correção: ao resolver um `+4` empilhável, somar `Pending.DrawCount` ao `DrawCounter` existente.
- Resultado: `+2 → +4 = 6`; `+2 → +4 → +2 = 8`; a compra remove todas as oito cartas pendentes.
- A correção de soma não mudou o inventário então vigente; depois, a feature Trocar cartas passou a acrescentar uma `SwapHands` somente ao Caseiro, totalizando 109 cartas contra 108 do Clássico.
- Ajuste aprovado em 2026-09-25: no Caseiro, `+4 → +4` passa a ser recusado. `+2 → +4` e o `+2` da cor escolhida sobre `+4` permanecem válidos. `BotRules()`/modo Clássico continua permitindo `+4 → +4`.
- A linha textual `Direção: ...` foi removida do estado público; as setas entre jogadores continuam mostrando o sentido atual.

---

# Memória atual — comandos privados e ajuda — 2026-09-25

- `/start` privado foi separado de `/help`: boas-vindas curtas, sem V2/Golang, com link para adicionar o bot a um grupo.
- O botão usa `https://t.me/<username>?startgroup=true`, com username preenchido por `GetMe` no startup.
- Como o Telegram entrega esse deep link no grupo como `/start@bot true`, o payload `true` recebe uma mensagem de orientação e não tenta iniciar partida. `/start` sem payload mantém o alias histórico de `/iniciar`.
- `/help` apresenta comandos em `<blockquote>`, usa o username atual nas instruções inline e credita a versão brasileira em Go (Golang) baseada no `@unopybot`.
- `/ajuda` e `/kill` continuam aceitos como aliases; `/start` continua compatível como alias de `/iniciar` em grupos.
- Os menus do Telegram são separados nos escopos padrão, privado e grupos.

---

# Memória atual — publicação limpa e multi-arquitetura — 2026-09-24

- Pedido aprovado: promover a versão V2 atual para a árvore pública de `main` e publicar também para ARM64.
- O build container passa a usar `linux/amd64,linux/arm64` tanto na validação de `dev` quanto na publicação da `main`.
- O builder roda em `$BUILDPLATFORM` e gera binário estático conforme `$TARGETOS/$TARGETARCH`, evitando executar o toolchain Go ARM64 por emulação.
- A promoção da `main` preserva o histórico independente e usa allowlist. Relatórios de simulação, `.agent`, fontes V1 e artefatos locais não entram na árvore pública.
- Publicação concluída: `dev` em `c052195`, `main` em `d7ecdb5`. Os workflows de árvore pública e container passaram; o manifesto GHCR `latest` foi verificado com variantes `linux/amd64` e `linux/arm64`.

---

# Memória atual — recuperação e reset por grupo — 2026-09-24

- Pedido: evitar que uma falha ou ação lenta deixe um grupo permanentemente sem resposta e fornecer um comando que restaure apenas aquele grupo.
- Plano aprovado: `recuperacao-e-reset-por-grupo_2026-09-24_05-14.md`, aprovado com “pode sim”.
- `/reset` é admitido antes da fila normal e processado numa fila de recuperação própria. A deduplicação de updates continua compartilhada por polling e webhook.
- Autorização: responsável da partida ou membro confirmado via `GetChatMember` como creator/administrator. Sem partida, somente administrador. Privado, tópico e remetente anônimo são rejeitados.
- Cada chat possui geração/contexto. O reset cancela a geração anterior, descarta tarefas antigas e cria uma fila dedicada limpa para aceitar `/novo` imediatamente mesmo com o shard original saturado.
- `Service.ResetChat` apaga partida ativa, runtime, índices, histórico do chat e retorna GameIDs para invalidação no TokenStore. O reset administrativo sem estado é idempotente.
- Workers de chat, inline e recuperação recuperam panic e registram categoria, chat e stack trace, sem registrar payload, cartas, token inline ou token do bot.
- Testes específicos cobrem saturação, cancelamento, tarefas obsoletas, isolamento entre chats, panic, autorização, contexto expirado, remoção dos índices/histórico/tokens, deduplicação e nova partida após reset.
- Validação: `go test ./...`, 20 repetições de game/telegram, `go vet ./...`, `go build ./...`, gofmt e `git diff --check` passaram. `go test -race` ficou indisponível no Termux/Android por falha do toolchain CGO/linker e deve rodar no CI Linux.
- Limite operacional: Go não mata goroutines à força. O cancelamento depende de operações cooperarem com context; a geração e a tombstone impedem que o trabalho antigo volte a controlar o estado resetado.

---

# Memória anterior — restituição de blefe (+4) e ordenação de cartas da V1 — 2026-09-23

- Pedido do usuário: "o blef foi removido, quando um player joga um +4 coringa, nao da pro usuario solicitar o blefe. tbm qurria que as cartas fossem em ordem igual na v1, elas estao espalhadas, vermelhas entre verdes e virce versa. nao precisa criar plano"
- Registro de plano: `restituir-blefe-e-ordenacao-cartas_2026-09-23_20-25.md`.
- Blefe do +4 Coringa (Call Bluff):
  1. No momento do descarte do WildDrawFour, a engine verifica se o jogador possuía cartas da cor ativa na mão antes do descarte (`p.Bluffing = true`).
  2. Ao selecionar a cor do +4 (`ChooseColor`), a engine registra a intenção e a pendência de blefe em `State.PendingBluff = &BluffInfo{Actor: actor, Target: target, Bluffing: bluffing}`.
  3. A view pública expõe `CanCallBluff: true` para o jogador alvo (vítima do +4) enquanto `PendingBluff` estiver ativo.
  4. Na interface inline do Telegram, é exibido o botão com sticker `option_bluff` ("BQADBAADygIAAl9XmQABJoLfB9ntI2UC").
  5. Ao solicitar o blefe (`uno.CallBluff`):
     - Se o autor do +4 blefou: "Blefe pego! <bluffer> recebeu X cartas!" (o autor compra a penalidade acumulada em `DrawCounter`).
     - Se o autor do +4 não blefou: "<bluffer> não blefou! <challenger> recebeu X cartas!" (a vítima compra `DrawCounter + 2` cartas).
     - O turno passa ao próximo jogador após a vítima.
  6. Se a vítima comprar cartas ou jogar normalmente (ou se o jogo avançar), o `PendingBluff` é limpo.
- Ordenação de cartas na mão:
  - Implementada ordenação idêntica à V1 (`sortHand`): cores agrupadas em Vermelho (0), Azul (1), Verde (2), Amarelo (3), e Especiais/Coringas (+4 e Wild) ao final (99).
  - Dentro de cada cor, ordenadas por valor numérico (0 a 9) seguido de cartas de ação (Skip, Reverse, DrawTwo).
  - Aplicada tanto no resumo textual da mão quanto na paginação de stickers inline.
- Todos os testes unitários e de integração passaram com 100% de sucesso.

---

# Memória anterior — restauração das regras de jogo da V1 — 2026-09-23

- Usuário aprovou o plano: "sim". Registro: `restaurar-regras-v1_2026-09-23_19-55.md`.
- Regras originais da V1 restauradas na engine (`internal/uno`):
  1. `NoWildFinish: true`: Proibido bater o jogo com carta especial (Wild / Coringa ou +4). Quando o jogador tem apenas 1 carta e ela é especial, a carta fica indisponível para jogada.
  2. `NoWildOnWild: true`: Proibido jogar Coringa sobre Coringa (Wild ou +4 sobre outro Wild ou +4 no topo do descarte, exceto quando respondendo/empilhando penalidade permitida pelas regras do modo).
  3. `AllowWildDrawFourAlways: true`: +4 livre para jogar a qualquer momento, sem a restrição da Mattel de verificar se o jogador tem a cor ativa na mão.
  4. `FreePlayAfterDraw: true`: Ao comprar 1 carta voluntariamente, a vez não passa compulsoriamente se ela não for jogável. O jogador pode descartar qualquer carta válida que possua na mão ou clicar em passar a vez.
- Configuração de modos:
  - `BotRules()` (Clássico V1): ativa todas as 4 regras acima, além de `StackDrawTwo: true`, `StackWildDrawFour: true`, `NumberedStart: true` e `AllowLateJoin: true`.
  - `CaseiroRules()` (Caseiro V1): herda `BotRules()` e ativa o cruzamento de penalidades (`StackWildDrawFourOnTwo: true` e `StackDrawTwoOnWildFour: true`).
  - `ClassicRules()`: preserva o modo estrito Mattel sem essas regras de casa para suites oficiais.
- Toda a camada visual e de textos do Telegram (HTML, formatação, botões, stickers, reação festiva 🥳 no UNO) permaneceu 100% inalterada.

---

# Memória anterior — correção +4 coringa e cache de cartas — 2026-09-23

- Usuário aprovou o plano: "sim". Registro: `corrigir-coringa-mais-quatro-e-cache-cartas_2026-09-23_19-25.md`.
- +4 Coringa não pula automaticamente o próximo jogador (`StackWildDrawFour: true` em `BotRules()`): ao jogar +4 e escolher a cor, a penalidade é acumulada em `DrawCounter` e o próximo jogador recebe a vez normalmente (sem execução de penalidade forçada e sem pular o turno). O jogador pode contra-atacar com outro +4 ou recolher as cartas voluntariamente com a opção "Comprar X cartas".
- Invalidação de cache local do cliente Telegram: o botão `🃏 Suas cartas` agora é gerado dinamicamente com a revisão da partida (`g_<GameID>_<revision>`), impedindo que o cliente do Telegram reaproveite resultados cacheados de jogadas anteriores ou de outros jogadores.
- Parse flexível de inline query: `HandleInlineQuery` extrai `GameID` mesmo com o sufixo de revisão `_<revision>` (`strings.SplitN(queryBody, "_", 2)[0]`).
- Visão de espectador durante `ChoosingColor`: para jogadores que não são o autor da escolha da cor, exibe exclusivamente o aviso de espera e o resumo de suas próprias cartas, retornando imediatamente sem expor artigos de cor nem misturar stickers cinzas.

---

# Memória histórica — milestone corretiva V2 — 2026-09-23

- Usuário aprovou o plano: "Implement the plan.". Registro: milestone-corretiva-v2_2026-09-23_14-12.md.
- Sintoma esclarecido pelo usuário: "Esta partida não está disponível ou você não está participando dela". Causa rastreada: confirmação final anexava Suas cartas incondicionalmente; engine/serviço já estavam encerrados.
- Corrigida apresentação terminal, invalidação dos tokens existentes ao vencer/sair encerrando, resposta a query histórica e mensagem de rejeição sem convite de continuação após fechamento.
- Corrida independente corrigida: scheduler mutava antes de enfileirar notificação. Agora candidato revisionado é revalidado e aplicado dentro da fila do chat, junto ao envio. Nenhum I/O sob lock do manager.
- Clássico Telegram = BotRules (placements/stack +2); não confundir com ClassicRules (FirstWinner). Usuário escolheu preservar efeitos terminais atuais do stacking, sem nova compra automática.
- UserCache guarda nomes; Renderer.PlayerLink escolhe destino usando view posterior. GetMe configura BotID antes de updates. Cor pendente mantém UserID real do chooser, mesmo com mão vazia.
- g_<GameID> visível preservado. Telegram InlineQuery/ChosenInlineResult não fornecem chat_id; não usar estado global de última partida nem confiar em chosen.Query para roteamento. Token one-use mantém binding user/game/chat/action/card/color/revision.
- Novos testes: matriz determinística de seis cartas finais, dois modos/direções, 3–4 participantes; prazos/candidatos concorrentes e encerrados; finais nos dois transportes com API mockada; barreiras de fila sem sleeps; targets/HTML, contexto multigrupo, cursor e replays.
- Validações aprovadas: go test ./..., go test -race ./..., go build ./..., go vet ./..., git diff --check. Cenários selecionados de integração/scheduler/contexto passaram 20 execuções.
- Homologação visual em Android/iOS/Desktop e Telegram real não executada. Sem commit, push, merge ou promoção.

---

# Memória atual — 2026-09-15 — M3

- Aprovação explícita: “Aprovado. Implemente a Milestone 3 conforme este plano.”
- Plano: telegram-mvp-v2_2026-09-15_14-05.md.
- Entry point V2 criado em `cmd/bot/main.go`, totalmente desacoplado do `main.go` legado.
- Concorrência de chat: 8 filas particionadas por ChatID (`hash(ChatID) % 8`) com 32 slots de capacidade para serializar comandos e jogadas da mesma partida.
- Concorrência inline: 4 workers dedicados lendo de fila com capacidade 64 para responder queries inline sem bloquear nem ser bloqueados por operações de chat.
- Backpressure seguro: tarefas descartadas sem mutação quando canais saturam.
- TokenStore privado: tokens aleatórios de 128 bits base64url gerados por `crypto/rand`. Consumo atômico sob mutex, TTL de 2 minutos (configurável), limite global (20.000) e por usuário (512) com evicção FIFO e limpeza oportunista. Invalidação imediata em caso de cancelamento/saída.
- Bypass omitempty telego v1.10.0: `InlineRequestConstructor` emitindo explicitamente `cache_time:0`, `is_personal:true` e `next_offset:""` para `answerInlineQuery`.
- `SafeAPICaller`: retry estrito de apenas 1 vez para HTTP 429 se `retry_after <= 5s`; timeouts e erros de rede não têm retry automático; URLs sanitizadas para nunca vazar tokens nos logs.
- Comandos suportados: `/novo`, `/entrar`, `/iniciar`, `/cancelar` (alias `/kill`), `/sair`, `/estado`, `/ajuda` (alias `/start` em grupos/privado).
- Matching de comandos case-insensitive (`strings.EqualFold(targetBot, h.botUsername)`): aceita `/novo@UnoGoBrBot` e `/novo@unogobrbot`.
- Regra de lobby no `/novo`: criador NÃO é inscrito automaticamente (permanece responsável observador com 0 inscritos até enviar `/entrar`).
- Stickers limpos no chat: remoção de botões de validação nos stickers jogados e eliminação de edições de markup.
- Teclado inline de jogo simplificado: `makeGameButtons` gera exclusivamente o botão `🃏 Suas cartas` (removido botão redundante `🔄 Atualizar estado`).
- Mensagem de estado mais enxuta: removido cabeçalho `🃏 UnoBotGO` de `RenderPublicState` e eliminada a contagem de cartas `(X cartas)` dos jogadores na lista pública (mantendo apenas o alerta `⚠️ UNO!` para quem estiver com 1 carta).
- Permissão de `/iniciar` flexibilizada: qualquer participante do chat pode usar `/iniciar` quando houver pelo menos 2 inscritos no lobby (paridade com V1). O comando `/cancelar` continua restrito ao responsável/owner da partida.
- Modo inline em grade/carrossel horizontal: exclusão de artigos de texto no fluxo de cartas durante a partida; apenas `InlineQueryResultCachedSticker` são emitidos.
- Empilhamento de +2 (`StackDrawTwo: true` em `BotRules()`): jogar +2 soma 2 a `DrawCounter` e passa a vez ao próximo jogador sem pular. O próximo jogador pode contra-atacar com outro +2 ou comprar a penalidade acumulada (passando a vez). Enquanto `DrawCounter > 0`, apenas cartas +2 são jogáveis.
- Seletor de cor estilo V1 limpo: na fase `ChoosingColor`, o jogador da vez recebe 4 artigos de cor ("Escolha sua cor") e 1 artigo de resumo das cartas ("Cartas (toque para estado do jogo):"), retornando imediatamente sem stickers cinzas misturados.
- Grito de UNO separado com reação festiva: quando um jogador atinge 1 carta na mão (`uno.UnoAnnounced`), o bot envia uma mensagem dedicada no grupo e adiciona a reação festiva `🥳` via `SetMessageReaction`.
- Carta inicial sempre numérica (`NumberedStart: true` em `BotRules()`): a partida nunca inicia com cartas de ação (+2, Skip, Reverse, Wild), garantindo início limpo sem penalidades na largada e todos com 7 cartas.
- Envio do sticker da carta virada: no `/iniciar`, o bot envia o sticker da primeira carta do topo no grupo antes da mensagem de status.
- Aceite manual do Telegram: testes automatizados 100% aprovados; roteiro de homologação manual detalhado em `docs/v2-telegram.md`.

---

## Memória histórica da M2 (2026-09-15)

- Aprovação explícita: “Aprovado. Implemente a Milestone 2 conforme este plano revisado.”
- Plano vigente: camada-aplicacao-v2_2026-09-15_13-30.md; anterior preservado como histórico.
- Owner é metadata administrativa independente de inscrição; somente owner inicia/cancela na M2.
- Owner que sai/obtém colocação transfere ao turno resultante ou primeiro ativo no lobby.
- Lobby sem sucessor conserva owner observador e permanece aberto até cancelamento explícito.
- Último Wild mantém participação ativa até escolher cor; WentOut/Left não recebem mão privada.
- Service aceita Actor autenticado pelo adapter; nunca confiar em Actor vindo diretamente de payload.
- Ordem de locks: entry -> índices; lookup solta índices antes de esperar entry. Nenhum I/O externo sob locks.
- Publicação de índices sempre termina após Apply aceito, mesmo com cancelamento de contexto.
- Histórico final público FIFO: padrão100, zero desativa; sem mãos/runtime, perdido ao reiniciar.
- Sem MemoryRepository nesta milestone; future recovery deve validar State+metadata e reconstruir índices.
- M1 preserva proibição de reentrada e compra não jogável passa turno automaticamente.
- Ver docs/v2-application.md antes de implementar M3. Escopo M2 não inclui adapter/tokens.

---

## Memória histórica anterior

# Memória atual — 2026-09-14

## V2 Milestone 1

- Usuário aprovou o plano com "Implement the plan.".
- Decisões confirmadas: +4 ilegal bloqueado inicialmente; UNO automático;
  múltiplos grupos com seleção explícita futura; preservar colocações e entrada tardia.
- Engine nova em internal/uno; V1 permanece executável na raiz.
- IDs físicos, snapshot com cópia profunda, Restore validado, Apply atômico por cópia,
  revision estrita e erros via errors.Is. Snapshots contêm mãos privadas e ficam no servidor.
- Injeção de deck/shuffler permite testes determinísticos; runtime RNG não é persistido.
- Sem empilhamento Classic; somente carta comprada pode ser jogada após compra.
- Policy FirstWinner ou Placements; dez participantes registrados, sem reentrada;
  entrada tardia opcional; cancelamento/saída distinguem motivo de término.
- Falta de cartas retorna ErrDeckEmpty sem mutação parcial; nenhuma carta é fabricada.
- Game não é thread-safe: manager da M2 será responsável pelo mutex privado por partida.
- Testes incluem 40 partidas determinísticas completas e regras/segurança/inventário/recovery.
- Conferir docs/v2-rules.md e docs/v2-audit.md antes das próximas milestones.

## Correções de contexto V1

- PostgreSQL já é obrigatório no startup V1 e guarda ranking/modo por grupo.
- Inline atual usa HTTP próprio com cache_time=0 explícito; telego tem omitempty.
- IDs atuais são aparência:índice; não existe validação AntiCheat/revision no handler atual.
- InlineQuery e ChosenInlineResult não trazem chat_id. Não inferir destino por
  inline_message_id. A seleção deve ficar vinculada à partida na emissão do resultado.
- Passar no race detector atual não cobre os caminhos concorrentes defeituosos do V1.

---

## Histórico anterior (preservado; consultar correções acima)

# Memória do Projeto - UnoGoBot

## Stack
- **Linguagem:** Go
- **Biblioteca Telegram:** telego (github.com/mymmrac/telego)
- **Estado:** Em memória (sem banco de dados)

## Arquitetura
- `main.go` — Entry point, inicializa bot e long polling
- `config.go` — Constantes de configuração (token, waiting_time, etc.)
- `card.go` — Definição de cartas, cores, valores, especiais, stickers
- `deck.go` — Baralho (shuffle, draw, dismiss, fill)
- `player.go` — Jogador (lista duplamente ligada em anel)
- `game.go` — Estado do jogo (regras, turnos, efeitos)
- `gamemanager.go` — Gerenciador de múltiplos jogos
- `results.go` — Construção de resultados inline
- `actions.go` — Ações do jogo (jogar carta, comprar, pular, etc.)
- `inline.go` — Handlers de inline query + chosen inline result
- `commands.go` — Handlers de comandos (/novo, /entrar, etc.)
- `errors.go` — Erros customizados

## Convenções
- Comandos em português (/novo, /entrar, /sair, etc.)
- Modo inline via stickers (usando file_ids do projeto jh0ker/mau_mau_bot)
- Jogador implementado como lista duplamente ligada em anel
- `sync.Mutex` por jogo para concorrência
- Context全局 compartilhado (`botCtx`)

## Decisões técnicas
- Usar telego ao invés de python-telegram-bot (migração Python → Go)
- Estado em memória (leve, sem dependências externas)
- Long polling (sem necessidade de webhook para dev)
- Stickers do projeto original (CARDS_CLASSIC_COLORBLIND)
- Roteamento proativo do contexto de jogo ativo (UserIDCurrent) no início de turnos e interações em grupo, mantendo o inline query livre de parâmetros de chat expostos, com leituras de mapas globais sob Mutex e sufixo de anti-cheat nos IDs dos resultados inline para invalidar o cache do Telegram de forma nativa e idêntica ao repositório original. Suporte a inversão de rotação de turnos (Reversed) no Game e redução do escopo de Mutex em inline handlers para evitar deadlocks de concorrência.

## Problemas conhecidos
- Stickers podem não funcionar se o pacote de stickers original for alterado
- Modo de jogo "text" não implementado completamente (sempre usa stickers)
- Bot precisa de `/setinline` e `/setinlinefeedback` no BotFather

## Histórico de correções (21/06/2026)
- **Cache:** `CacheTime = 1` em vez de 0 — telego usa `omitempty` que omite 0 do JSON; Telegram default 300s.
- **Anti-cheat:** Result IDs com timestamp `time.Now().UnixNano()` para invalidar cache do cliente.
- **Parsing anti-cheat:** Aceita formato 2 ou 3 partes (`<id>:<anticheat>` ou `<id>:<timestamp>:<anticheat>`).
- **Regras de cartas:** `cardPlayable` reescrita igual ao Python original (bloqueia +4 em +2 c/ draw_counter > 0, special-on-special, etc).
- **EndGame:** Adicionado `EndGameByGame` para encerrar quando jogador já saiu. `game.Started = false` setado antes de `afterAction`.
- **Notificação anti-cheat:** Mensagem enviada ao jogador se ação expirou.
- **Ordenação:** Cartas no inline agora ordenadas por cor (r, b, g, y) e valor.
- **Seletor de cores:** Sem duplicação de emoji; `Title` mostra nome da cor.
- **displayName:** `@@` corrigido para `@`.
- **Cores da mão:** Mostra emojis das cores disponíveis durante escolha de cor.

- M6: Bot suporta `TELEGRAM_MODE=polling|webhook`; polling é default.
- Webhook usa `WEBHOOK_URL` condicional e servidor compartilhado `WEB_ADDR`; segredo derivado por contexto exclusivo e drop fixo false (configuração atual desde 2026-10-01), com `/healthz` liveness e dedupe em memória por UpdateID.


# Revisão de status do projeto — 2026-09-26

- Pedido autorizou auditoria seguida de alterações documentais, commit/push dev/main e tentativa de default main. Plano: status-projeto_2026-09-26_01-00.md.
- Bases auditadas: dev ea22a1e341bd55b440033a8deae3eb2a78b64453; main 4a635918e40cdacbc91662cb06ac30d8b94faf81. Sem ancestral comum.
- docs/project-status.md distingue implementação, testes, aceite real, main, experimental e design. README corrigido minimamente (título, link, /iniciar e status webhook).
- Apenas dev: SwapHands/stickers/simulador correspondente, +4 sobre +4 recusado no Caseiro, direção textual removida, comandos privados, correção de falso tópico, debugcards. Simulador/reset/penalidade cruzada já estavam na main.
- Aceite cinza confirmado não implica aceite completo da troca. Menções visuais e falso tópico mantêm homologação pendente. Polling recomendado; webhook experimental pelo relato operacional fornecido, sem atribuir causa.
- Codemaps em codemaps/ descrevem V1 (2026-06-25), não são mapa atual da V2. Não foram copiados para documentação pública. Docs técnicos também contêm seções históricas; código e atualizações recentes têm precedência.
- Testes, vet, build e diff check aprovados em ambas; testes debugcards aprovados na dev. Race local bloqueado: CGO=0 inicialmente; com CGO=1, ThreadSanitizer unsupported VMA range (39, exige 48). Base CI main validada/publicada; acompanhar CI dos novos pushes.
- Promoção preparada em worktree independente: somente README.md e docs/project-status.md. Script exato public-tree aprovado; nenhuma alteração de gameplay ou feature promovida.
- Default consultada por API/ls-remote: dev. gh ausente, sem GH_TOKEN/GITHUB_TOKEN ou configuração gh; SSH Git não fornece API administrativa. Mudança indisponível neste ambiente. Comando manual: gh repo edit gr1ksdev/UnoBotGO --default-branch main.
- API pública: branches protected=false, rulesets=[]; detalhes de proteção retornam 401. Workflows usam branches explícitas; nenhuma alteração de proteção/workflow necessária nesta revisão.


# Gameplay UX Polish — 2026-09-26 (somente dev)

- Pedido autorizou auditoria curta, implementação, commit e push dev; proibiu main/promoção/merge. Base badf81c; main fd011ab.
- Auditoria: join da engine já insere na cauda lógica (antes do atual no sentido positivo, depois no negativo). Não foi reproduzido corte real de turno. Order é físico; renderer antigo podia sugerir entrada no meio. Preservado algoritmo da engine e adicionados testes com avanços reais, ambos sentidos, Reverse, placements e múltiplas entradas.
- Renderer apresenta ordem a partir do atual, no sentido vigente, com → significando próximo da sequência. Ações/efeitos/resultados separados; cor apenas em topo sem cor; penalidade preservada; colocações com medalhas; um título de encerramento. PlayerLink, escaping e UNO preservados.
- managedGame.locked pertence à sessão. Service.SetLocked exige owner/chat, inclusive owner observador; não usa privilégio de ChatAdmin. Join verifica admissão sob entry.mu. Projeções públicas incluem Locked; nenhuma revisão de engine/timer/token é alterada pelo lock.
- Nova sessão aberta; operação idempotente em lobby/jogo; owner transferido mantém controle. Resumo fechado conserva metadata, mas não aceita lock/unlock; reset descarta sessão. Não existe persistência de sessão para restaurar.
- /trancar e /destrancar adicionados aos comandos de grupo e ajuda. Nenhum handler/token/contexto inline foi alterado.
- Homologação real pendente, main não publicada. Usuário fará aceite no Telegram antes de qualquer promoção.

- Validação desta milestone: test/vet/build normais e com debugcards aprovados; diff check aprovado. Race local bloqueado por VMA 39 (exige 48), após habilitar CGO; verificação via CI após push.

# M7 em implementação — 2026-09-27
- Plano aprovado: m7-persistencia-ranking_2026-09-27_13-33.md, agora em approved.
- Usuário removeu worker/fila de resultados e outbox do plano. Fechamento síncrono após GameFinished; commit antes da mensagem de pontos. Nenhum snapshot de engine/mãos no DB.
- M7.1: pgx/v5, DATABASE_URL obrigatório em cmd/bot, cmd/migrate explícito, migration ledger checksum/advisory lock. Primeiro schema: group_configs Classic/Legacy.
- Testes de integração exigem TEST_DATABASE_URL, schemas temporários isolados. Decisões de produto continuam bloqueadas.

- M7.2: internal/groups define defaults/repository/autorização; postgres GetOrCreate preserva Classic/Legacy. /novo consulta config uma vez; overrides não alteram default. Session snapshot de ranking/revision em game permanece independente de DB, lobby mantém seletor existente. API de modo valida permissão no serviço. Setup Telegram/troca de sistema ainda pendentes de UX/política; nenhum novo comando criado.

- M7.3 foundation: centésimos int64/half-up; immutable public DTO + lifecycle counters; completed results held independently of history/reset until COMMIT. Telegram finalization synchronous only at closure, 10s timeout, no workers/outbox. Result policy blank persists needs_product_decision with NULL score and no stats; no competitive policy enabled. Atomic/idempotent Store test uses explicitly test-only policy, not runtime. GameID+canonical hash rejects conflicting retry, rollback on mismatched ranking.

- M7.4: known_group_users por (chat_id,user_id), username nullable, atualização monotônica. /novo observa criador em DB (fora de gameplay); comandos/chosen inline observam participantes somente em RAM e resultado flush na mesma transação de encerramento. Não scrapeia membros nem acessa DB por jogada. Unicode preservado; nomes duplicados não mesclam IDs.

- M7.5 foundation: ranking_imports/entries por IDs, UNIQUE(chat,source_hash); parser usa último sufixo inteiro e preserva Unicode/invisíveis/duplicatas/linhas inválidas. Reconciliation exata conservadora; múltiplas entradas reivindicando mesmo UserID ou múltiplos candidatos => ambiguous, sem autolink. Staging transacional idempotente não aplica pontos. Sem conversão Updated ou UI/import oficial.

- Revisão M7: rollback usa contexto independente com prazo de 3s, para cancelamento da operação não impedir cleanup e conexão quebrada não prender finalização indefinidamente. Testes PostgreSQL/race e checks locais continuam passando.

- Continuação M7 aprovada: N = placements válidos de concluintes. Abandonados/lobby fora de N, sem posição artificial, zero; late/reentry sem punição se concluiu. Departure permitido se N>=2; N<2 sem concessão, cancelled excluído. internal/ranking.Prepare trabalha numa cópia, policy completed-placements-v1. Engine não alterada. Testes reais do serviço usam deck determinístico e ações normais para confirmar todos os ciclos.

- Storage da elegibilidade: migration 0005 amplia apenas scoring_status com insufficient_eligible_players. Não reescreve dados/checksums anteriores. Todos os avaliados têm score_units numérico (zero para abandonos/N<2); stats só para elegíveis quando N>=2. participant_count continua sendo total auditado, não N. Integração PostgreSQL 17/race confirmou N2/N3/N8/N7/N6/departure/N0/N1, idempotência concorrente, rollback após sete stats, retry e preservação de pending históricos (repreparação conflitante não recontabiliza).

- Telegram e anúncio de ranking pós-commit:
  - Finalização síncrona nos 3 pontos de encerramento (`inline.go`, `commands.go`, `bot.go` timeout).
  - Mensagem final compacta é enviada primeiro; o callback `notify` é retornado apenas se o commit confirmou `commit.Scored == true` e `!commit.AlreadyPersisted`.
  - Disparo de `notifyPoints` envia a mensagem adicional com ordenação (colocados primeiro em ordem de colocação, depois participantes sem colocação com `(fora do ranking)` ordenados por UserID), escape HTML dos nomes, Legacy em inteiros e Updated em decimal formatado com vírgula e 2 casas (`half-up`).
  - N<2 ou falhas de commit não disparam a mensagem de pontos; em caso de falha de banco, o resultado é mantido na memória do `game.Service` para retry síncrono via `RetryPendingResults`. Retries já persistidos (`AlreadyPersisted`) não reenviam a mensagem de pontuação. Commit `b56322e`.

# M7 — UX Telegram de Configuração de Grupo — 2026-09-27 (somente dev)
- Plano aprovado: `telegram-group-config-ux_2026-09-27_16-50.md` com dois ajustes do usuário:
  1. Troca Legacy ↔ Updated bloqueada se houver histórico/scores incompatíveis acumulados (`ErrNeedsProductDecision`), preservando a configuração anterior e informando o usuário com alerta sem quebrar a UI.
  2. `installed_by_user_id` só é atualizado quando `my_chat_member` comprovar transição real de instalação/reentrada (`left`/`kicked` -> `member`/`administrator`) usando o ator fornecido pelo update.
- Implementação:
  - `internal/groups`: `SetRankingSystem`, `SetInstalledBy`, `CanConfigureUser`, `RecordInstallation` e `ErrNeedsProductDecision`.
  - `internal/storage/postgres`: `SetRankingSystem` com verificação transacional de conflito em `player_group_stats` e `completed_games`, e `SetInstalledBy`.
  - `internal/telegram`:
    - `Renderer.RenderGroupConfig` e `Renderer.RenderGroupWelcome`.
    - `makeGroupConfigButtons` e `makeGroupWelcomeButtons`.
    - `lookupMembershipAPI` e `parseMembership` para mapear status telego para `groups.Membership` (fail closed em erro).
    - `bot.go`: adicionado `my_chat_member` a `allowedUpdates`, despachado via `dispatcher.EnqueueChat`, registro de `/config` no menu de grupos.
    - `commands.go`: tratamento de `/config` com verificação de autorização (admin atual ou instalador ainda membro), `HandleMyChatMember` filtrando transições reais e registrando instalador.
    - `callbacks.go`: tratamento de callbacks `cfg_` (`cfg_open_`, `cfg_mode_`, `cfg_rank_`), revalidação a cada clique, atualização do status e edição limpa da mensagem.
    - `KnownGroupUser` atualizado em `/config`, callbacks `cfg_` e `my_chat_member`.
  - Suíte completa em `internal/telegram/config_test.go` cobrindo todos os 23 cenários + regra de conflito de histórico.

# Trocar Mãos opcional — 2026-09-28
- Implementado sobre bce47d0 após aprovação explícita: alvo/manter → cor via Inline Mode, transferência somente no ChooseColor. KeepHand/HandKept; revisões r+1/r+2/r+3. Última carta termina sem escolhas e sem trocar mãos.
- Testes novos em uno/game/telegram optional_swap_test.go: quatro cores, dois/três jogadores, ambas direções, mãos/IDs, snapshot/replay, última carta, UNO, autenticação, revisão, replay, multigrupo, alvo saindo, concorrência de cor e resultado M7. Testes existentes preservados/adaptados; relatório conta HandsSwapped real.
- go test -count=1 ./... e variante debugcards, vet/build normais/debugcards, diff check aprovados. Race não executável localmente: CGO padrão 0; CGO_ENABLED=1 falha ThreadSanitizer VMA 39 (requer 48). Não declarar race aprovado. Integração com PostgreSQL real não reexecutada nesta correção sem alterações de banco.
- Simulador: Caseiro 4 players seed 20260924 (126 ações), Clássico 2 players seed 20260928 (23 ações), concluídos; relatórios somente /tmp.
- Homologação Telegram pendente; nenhum push nem promoção. main permanece 62fc344. A documentação geral de status possuía referências de publicação antigas; esta atualização documenta o novo fluxo, não reaudita toda a matriz.


# Refino visual do detalhe do grupo — 2026-09-30

- Somente dev, working tree para homologação; sem commit/push/deploy/migration.
- Hero sem card interno: group-hero não tem border, background, border-radius ou shadow. Header compacto 192px com dados usuais, sem insets Telegram. Avatar 64px (56px em largura estreita), score 26px, duas cartas 32x50px em coluna reservada; decoração removida abaixo de 310px.
- Painel com raio 20px e sobreposição 12px; linhas 62px, avatar 38px, medalhas 23x30px, top 3 com cores suaves. Estilos compactos restritos a detail-view.
- Nome pontual: observedName concatena FirstName/LastName do Telegram; results.go persiste DisplayName e global_rankings.go retorna nomes não vazios sem filtrar pontuação. TestRankingPreservesObservedDotName confirma preservação intencional de '.'. Sem consulta a registro de produção. Fallback 'Jogador' somente na apresentação frontend de jogadores com nome vazio ou somente pontuação/espaços/controles. Preservados grupos, nomes Unicode/emoji, IDs, persistência e regras competitivas.
- Validação Node 24.21.0: lint/typecheck/test (31 testes)/build aprovados. Go test -count=1 ./... e go build ./... aprovados após build frontend. git diff --check aprovado. Testes PostgreSQL não garantem execução de integrações sem banco configurado.
- Browser local Brave via Playwright temporário, API/Telegram simulados: larguras 280/320/360/390/480px, nomes extensos e score int64 máximo sem overflow horizontal. Em 390x844, nove linhas completas usuais; oito com score extremo. Abaixo de 310px scores usam linha adicional. Capturas em /tmp/unobot-visual-validation/detail-390.png e detail-320-stress.png.
- Mockup local inspecionado, porém anexos novos não disponíveis na conversa. Mockup antigo tem card; instrução atual de removê-lo prevalece. Homologação real no celular permanece com usuário.


# Recuperação das proporções da referência — 2026-09-30

- A rodada compacta anterior foi REPROVADA. A referência agora disponível em `Ranking do Grupo em Estilo UNO.png` é autoritativa; objetivo atual é fidelidade de composição/escala, não maximizar quantidade de rows visíveis. Não reutilizar os números compactos anteriores como especificação.
- Hero integrado mantido, sem card interno. Em 390/430 CSS px: header 286px antes da sobreposição e sem insets nativos, avatar 96px, nome 28px, score 44px, ID 15px, período 14px. Sheet raio 36px, overlap 22px, padding superior 24px; título 17px/ícone 22px.
- Três cartas físicas de 112x174px com rotações e sombras, parcialmente fora da viewport à direita e abaixo da sheet; overflow decorativo intencional contido no header. Removida coluna de pequenos ícones.
- Rows 88px em dados usuais, avatares 56px e medalhas 30x40px; top 1 dourado suave, top 2/3 neutros e sombras leves. Max-width 480px mantido no desktop sem scale interno. Ajustes específicos de largura estreita preservam leitura, e score excepcionalmente grande quebra linhas em vez de reduzir toda a interface.
- Apenas styles.css e o markup decorativo/título do hero alterados nesta rodada. Fallback Jogador da rodada anterior intacto; sem mudanças em API/backend/ranking/auth/group_ref/score/paginação.
- Validação Node 24.21.0: lint, typecheck, 31 testes frontend, build e diff check aprovados. Go test -count=1 ./... e go build ./... aprovados após regenerar assets embed; sem migrations.
- Brave/Playwright temporário com API/Telegram simulados: 280/320/360/390/430/1280px; nomes longos e int64 máximo sem overflow de conteúdo. No desktop: avatar 96px, score 44px, rows 88px, iguais ao mobile prioritário. Browser Back navega para global. Sem insets reais do Telegram nestes testes.
- Comparação lado a lado (mesma largura de imagem): /tmp/unobot-visual-validation/comparacao-proporcoes.html e comparacao-proporcoes.png. Capturas proportions-390.png, proportions-430.png e proportions-desktop.png. Referência inclui barra nativa; capturas usam dados simulados e fallback de avatar. Homologação visual permanece com usuário.
- Somente dev, sem commit/push/deploy; referência fornecida preservada como arquivo untracked. Plano: restaurar-proporcoes-referencia_2026-09-30_20-41.md.


# Polimento do hero com artes reais — 2026-09-30

- Rota /groups/:groupRef: App.tsx → RankingsPage, com Avatar/Score/RankingCard e novo HeroCards. Estilos permanecem em styles.css, sem alterações em API/backend/SQL/auth/navegação.
- Assets locais prévios: somente swap_hands_grey.png. Artes coloridas reais estavam no mapa Stickers de internal/telegram/stickers.go; recuperadas via getFile read-only (g_0/y_0/r_0), sem mensagens nem mudanças na integração, e copiadas intactas para web/src/assets/cards/{green,yellow,red}-zero.webp. Origem documentada no README desse diretório. Total ~31KB, WebP alpha 342x512. Vite emite três arquivos estáticos versionados; nenhuma chamada ao Telegram ou credencial para decoração em runtime.
- Removidos spans/ovais e cores genéricas CSS. HeroCards renderiza imgs decorativas com alt vazio, aria-hidden, dimensões intrínsecas e draggable=false. Composição mantém três cartas reais, width 128px e height auto (~192px), rotações distintas, sombras alpha, clipping à direita e pela sheet; fora do caminho de interação.
- Avatar ampliado 96→108px, deslocado 4px acima e com sombra suave. Grid ajustado para 108px + conteúdo, gap 12px, reserva 64px à direita e padding externo 22px. Nome 30px, ID 16px, score 46px/unidade 23px e período 15px. Header usual 304px sem insets; sem card interno. Em <=360px avatar 92px, score 40px e decoração reposicionada. Rows 88px, top 1 e estrutura da lista preservados; sombra dourada ajustada levemente.
- Checks finais com Node 24.21.0: lint/typecheck/test (31 testes)/build/diff check aprovados; go test -count=1 ./... e go build ./... aprovados após regenerar embed.
- Browser Brave com API/Telegram simulados, assets reais do build/dev: 280/320/360/390/430/1280px, nomes longos e int64 máximo sem overflow de conteúdo. Onde há decoração, bounding box do red card não invade summary. Abaixo de 310px cartas ocultas. Confirmados Updated/Legacy, header de autenticação, navegação de volta preservando system/tab e carregamento autenticado de avatar com imagem de teste.
- Mesma leitura de dados via useRanking/client preservada e coberta por testes e fixtures; nenhum banco/ranking de produção consultado. Aceite real no Telegram permanece com usuário.
- Capturas: /tmp/unobot-visual-validation/real-cards-390.png, real-cards-430.png, real-cards-desktop.png e real-cards-with-avatar-390.png (dados e avatar de teste). Só dev; nenhum commit/push/main/deploy/migration. Plano polir-hero-assets-reais_2026-09-30_21-08.md.


# Lapidação final: avatar e onda — 2026-09-30

- Estrutura/identidade geral e cards foram aprovados pelo usuário antes desta rodada; não redesenhar nem retomar objetivos de densidade.
- Alterações de produto limitadas a styles.css e novo assets/hero-wave.svg. Avatar 108→116px (92→100px <=360), sem mudar borda/sombra/fallback. Grid recebe padding-left6 e gap reduzido4; avatar deslocado6px, resumo10px à direita. Cartas reais intactas, apenas offset8px à direita para acompanhar a reserva de texto.
- Sheet usa pseudo-elemento ::before com máscara SVG alpha, altura24px, viewBox480x24 e preserveAspectRatio none. Dois picos assimétricos (y3/y5) e vale y17, variação máxima21px. Contorno superior arredondado antigo substituído pela onda; margin-top e padding da sheet intactos. Surface horizontal com os mesmos tons é compartilhada com a máscara para evitar seam; sombra reta do topo removida. Overlap de1px da máscara/body evita gaps.
- Hero usual continua304px sem insets. Browser comparou snapshots da lista antes/depois em320/360/390/430/1280: posições, alturas, larguras, tipografia, backgrounds, sombras, radius e gaps idênticos, incluindo título. Avatar cresceu8 e deslocou6; resumo deslocou10 em todas essas larguras. Não há mudança de altura/layout por causa da onda.
- Browser Brave com API/Telegram/avatars de teste: larguras280–430 e desktop, nomes longos/int64 máximo, sem overflow de conteúdo ou sobreposição do red-card com resumo quando visível. Confirmados Legado/Atualizado, auth headers, carregamento de foto e back com system/tab preservados. Sem consulta a ranking de produção.
- Node24.21.0: lint/typecheck/test(31)/build aprovados; git diff --check aprovado. go build ./... aprovado para embed; nenhum Go alterado, portanto go test não repetido nesta rodada. Build CSS inclui prefixos -webkit-mask e CSP existente permite imagens data:, sem mudar backend.
- Capturas /tmp/unobot-visual-validation/final-wave-{390,430,desktop}.png e final-wave-with-avatar-390.png; dados/avatar simulados. Plano lapidar-avatar-onda_2026-09-30_21-34.md concluído; nova homologação visual com usuário. Somente dev, sem commit/push/main/deploy/migration.


# Navegação principal inferior — 2026-09-30

- Estrutura visual aprovada preservada; mudança exclusivamente frontend/UI/navegação na dev, sem commit/push/main/deploy/backend/API/banco.
- RankingsPage remove Segmented Tipo de ranking do ranking-panel e monta BottomNavigation somente global, fora dos branches loading/error/empty. Atualizado/Legado continua no header. system/tab em searchParams continuam única fonte de verdade; React Router Link aponta para /?system=...&tab=groups|players, preservando deep links e refresh, sem estado duplicado nem novas queries.
- Novo BottomNavigation usa ícone UsersIcon existente e SVG de pessoa na mesma abordagem, labels e aria-current=page. Ativo com peso, indicador pequeno e fundo de ícone; cores vermelho grupos/azul players, inativo cinza. Links têm foco de teclado e área de toque60px.
- Barra fixed com max-width480 centralizada, branco98%, borda/sombra suaves. Altura de conteúdo60px +1px border +safe bottom. global-view usa padding-bottom60+safe+16px; safe=max(env(safe-area-inset-bottom), --telegram-bottom) e safe lateral com as variáveis existentes. Desktop>=500: bottom20, bordas inferiores32, igual container centralizado; mobile bottom0.
- No detalhe não há nav nem padding extra. Back visual, fallback de erro e Telegram BackButton retornam sempre tab=groups preservando system, inclusive deep links com tab=players. useTelegram recebe opção manageBackButton; root App não disputa propriedade com Page, corrigindo root hide após Page show em abertura direta. initData/autenticação/eventos/queries/backend intactos.
- Quatro testes de navegação adicionados e existentes adaptados para links: defaults, sistema preservado na troca, URL/deep link, root não oculta BackButton do detalhe, loading/error/empty, retorno e cleanup. Total35 testes aprovados.
- Node24.21.0: lint/typecheck/test(35)/build aprovados; git diff --check aprovado; go build ./... aprovado para assets embed. Nenhum Go alterado, go test não repetido nesta rodada.
- Browser Brave com API/Telegram simulados:320/390/430/1280px e safe bottom0/34, nav largura min(viewport,480), alinhada ao app, altura61/95, último card43px acima da barra no scroll final; sem overflow. Também validado teclado Enter, refresh, URLs/system, retorno UI/Telegram, footer oculto no detalhe, loading/empty/error e paginação6 itens.
- Capturas /tmp/unobot-visual-validation/bottom-nav-players.png, bottom-nav-mobile-safe.png e bottom-nav-desktop.png. Sem consulta a ranking de produção. Working tree anterior preservado; plano bottom-navigation-rankings_2026-09-30_21-45.md concluído; homologação visual com usuário.


# Navegação em pílula de vidro — 2026-09-30

- Usuário pediu barra separada, pílula com estética Liquid Glass; interpretado como flutuante/descolada das bordas, ainda acessível ao rolar. Mudança só em styles.css; links/estado/BackButton/detail intactos.
- Cápsula max340px, margem lateral mínima20px, radius999, altura70px (toques60/frame10). Distância inferior12px +safe, e20px adicionais de margem externa desktop. Safe area fica fora da cápsula, sem deformar altura; reserva global98px+safe protege conteúdo.
- Vidro CSS inspirado: gradient translúcido, backdrop blur22/saturate180%, borda branca/reflexo e sombras suaves. Seleção como subpílula clara com ícone/cor/peso; indicador linear anterior removido. Fallback branco96% quando backdrop-filter não suportado.
- Node24.21.0: lint/typecheck/test35/build aprovados; git diff --check aprovado; go build ./... aprovado para embed. Nenhum Go alterado.
- Browser fixtures320/390/430/1280px com safe0/34: caps280/340px, nav dentro do app, safe externa, sem overflow e último card44px acima da pílula no scroll final. Verificados Enter, refresh, system/tab, retornos UI/Telegram, footer oculto no detalhe, loading/error/empty e paginação.
- Capturas /tmp/unobot-visual-validation/glass-pill-players.png, glass-pill-mobile-safe.png e glass-pill-desktop.png. Somente dev/working tree; sem commit/push/main/deploy/backend/API/ranking/auth. Plano pilula-liquid-glass_2026-09-30_21-58.md concluído; homologação visual pendente.

## Liquid Glass — 2026-09-30
- BottomNavigation preserva rotas e safe area; material web usa backdrop real com filtro SVG e mapa de deslocamento gerado por canvas apenas no resize.
- Documentação Apple consultada: adopting-liquid-glass e HIG Materials. Não é a API nativa SwiftUI/UIKit; compatibilidade óptica validada no Brave/Chromium, sem homologação Safari/WebView real.
- Alternativas: blur CSS, superfície opaca sem suporte e preferências de contraste/transparência; redução de movimento respeitada.

- Polimento da seleção: gradiente cinza translúcido 18%/10% e blur 3px em .glass-selection; acessibilidade mantém superfície sólida sem blur. Lint/typecheck/35 testes/build/diff-check aprovados.

## Nomes longos — 2026-09-30
- ScrollingName compartilhado no hero e cards de grupos/players mede overflow com ResizeObserver e fonte carregada. Apenas nomes que não cabem animam para a esquerda com pausa e retorno.
- Uma única cópia do nome, title completo e foco em textos longos. prefers-reduced-motion desativa animação e permite scroll manual.
- Validação: lint/typecheck/37 testes/build/diff-check aprovados; browser confirmou deslocamento, nomes curtos estáticos, resize, navegação e ausência de overflow em 390/430px.

## Simplificação de configuração V2 — 2026-10-01
- Plano simplificar-config-env_2026-10-01_13-11 aprovado explicitamente. Só dev/working tree, sem commit/push/deploy/main.
- Loader único externo com6variáveis normais e WEBHOOK_URL somente webhook. MINIAPP_SECRET aceita só Base64 padrão com32bytes. Políticas antes configuráveis agora constantes; variáveis antigas/alias não são lidos.
- HMAC-SHA256 para secret_token usa domínio/protocolo/versionamento unobotgo/v2/telegram-webhook-secret/v1 e saída43caracteres Base64URL sem padding. Testes pinam contexto, formato, estabilidade, rotação, não reutilização da chave master e headers403/200.
- getMe único fornece link ranking; botão preservado, username@ normalizado. Sem URL externa do Mini App na configuração.
- Validações aprovadas: go test ./..., go test -race ./..., go vet ./..., go build ./..., build cmd/bot, make check completo (37testes frontend, debugcards e PostgreSQL isolado), compose config quiet e git diff --check.
- make check inicialmente detectou falha preexistente de fixtures de desempate: partidas setembro2026 consultadas via time.Time{} (mês atual outubro). Reproduzida no HEAD d17162c exportado em /tmp; corrigido apenas ranking_tiebreak_integration_test.go para consultar mês fixo das fixtures, sem regras/SQL/schema. Segunda execução completa passou.
- Banco de testes em container efêmero exclusivo na porta15433; nenhum dado do banco existente alterado. Node24 instalado temporariamente após SHA256 oficial; npm ci reportou2vulnerabilidades moderadas preexistentes, lockfile/dependências não alterados.

# Fullscreen nativo do Mini App — 2026-10-01

- `App` inicializa o Telegram WebApp e solicita fullscreen uma vez, apenas com suporte à API 8.0 e quando ainda não ativo. `expand()` mantém fallback para clientes incompatíveis/recusa.
- O cabeçalho próprio permanece; fechar/menu flutuantes pertencem ao Telegram. Insets do dispositivo e do conteúdo são respeitados e atualizados nos eventos fullscreen.
- Navegação não solicita fullscreen novamente; BackButton, autenticação, API e ranking preservados. Cor dos controles nativos acompanha azul global/vermelho detalhe.
- Implementação somente dev, sem commit/push/deploy. Homologação real no celular pendente.
# Detalhe do grupo — 2026-10-01

- Removida a legenda redundante `Total em <Mês>` do hero a pedido do usuário. O mês permanece no título do ranking interno; pontuação e período da API preservados.


# Configuração V2 — 2026-10-01

- Config único: TOKEN, TELEGRAM_MODE, DATABASE_URL, TURN_TIMEOUT, WEB_ADDR, MINIAPP_SECRET e WEBHOOK_URL condicional. Config/Web separados e políticas externas removidos; defaults fixos centralizados.
- Secret webhook HMAC-SHA256 com contexto unobotgo/telegram/webhook-secret/v1 e Base64 URL sem padding; setWebhook recebe token derivado e header segue validado em tempo constante. Drop false; URL pública permanece necessária.
- Link https://t.me/<bot>/ranking gerado no getMe único; BotFather continua responsável por URL HTTPS/short name ranking.
- Frontend preservado integralmente contra snapshot inicial. Testes focados/config/Telegram e go test ./... passaram. Race default falhou por CGO=0, make check recusou TEST_DATABASE_URL ausente; demais verificações interrompidas e canceladas pelo usuário antes de commit/push autorizado na dev.

## Migrations: consolidação fail-closed — 2026-10-01
- Plano consolidar-migrations-startup_2026-10-01_13-51 aprovado. Base49400ba, dev, sem commit/push/main/deploy; working tree estava limpo exceto plano.
- Auto-run já existia; reutilizado Store.Migrate/VerifySchema. Removido CLI, corrigidas instruções manuais. SQL0001..0007 não alterados.
- Ledger validado integralmente antes de pendências (checksum, versões desconhecidas e prefixo ordenado). Erros/logs indicam migration/estágio, preservam errors.Is/As e suprimem dados sensíveis dos detalhes do driver. Sem alterações em operationError de persistência.
- Lock transacional71870101 e transação única do lote preservados. Initialize aplica config.MigrationTimeout2m e callback funcional recebe contexto original da aplicação.
- PostgreSQL efêmero exclusivo porta15433: vazio, reaplicação, parcial, rollback SQL/lote, sequência não transacional provando ausência de execução antes da validação, cancelamento SQL, timeout do lock com recuperação, dois pools executando exatamente uma vez, startup real Store + HTTP mock e falhas sem callback.
- go test ./..., go test -race ./..., go vet ./..., go build ./..., make check, make build, integração race e git diff --check aprovados. Frontend45testes, lint/typecheck/build preservados. Sem refs operacionais ao CLI, salvo registros históricos .agent; nenhum frontend/config/SQL/domain alterado.
- npm ci informou2vulnerabilidades moderadas já existentes e aviso de ESLint; dependências/lockfile preservados. Nenhuma validação bloqueada, nenhum serviço Telegram real iniciado ou banco aplicativo alterado.


# Alias e endereçamento — 2026-10-01

- /join@bot e /entrar@bot usam o mesmo handler de entrada. Comandos de grupo sem @bot ou dirigidos a outro bot são ignorados antes de respostas de identidade/tópico/reset. Privado sem sufixo preservado; callbacks/inline seguem existentes.
- groups.Defaults agora usa Updated; migration 0008 altera exclusivamente DEFAULT SQL. Nenhum grupo existente convertido e nenhuma migration aplicada em produção. Testes SQL de migração e caminhos de criação adicionados; execução depende de TEST_DATABASE_URL.

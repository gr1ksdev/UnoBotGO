# Mini App de ranking global — 2026-09-30 (somente dev; pronto para homologação)

- Adicionado Mini App Telegram integrado ao binário Go único (`bin/unobotgo`), servindo API, SPA estático e webhook na mesma porta/origem (`WEB_ADDR`, padrão `:8080`).
- Frontend moderno em `web/`: React 19, TypeScript estrito, Vite 7, Tailwind v4, TanStack Query 5 e React Router 7. Compila para `web/dist` e é embutido via `//go:embed all:dist` em `web/embed.go`.
- Startup orquestrado em `internal/app`:
  - PostgreSQL advisory lock transacional (`71870101`).
  - Execução e verificação de migrações (`Migrate` + `VerifySchema`) antes de abrir listeners ou iniciar bot/workers.
  - Fail-closed total: falhas de migration encerram com exit code não zero sem abrir qualquer serviço parcial.
- Segurança e integridade:
  - Autenticação por HMAC-SHA256 em tempo constante do `Authorization: tma <initData>`.
  - Ocultação de raw IDs: interface exibe `ID ••••1234`. Referências de grupo, cursores keyset e URLs de avatar são seladas com AES-GCM (usando `MINIAPP_SECRET`).
  - Precisão matemática: `score_units` como string decimal no JSON e `BigInt` no frontend, sem perda de precisão float.
  - Mês corrente calculado no backend em `America/Sao_Paulo`. Requisições com mês divergente retornam 409 `ranking_period_changed`.
- Cache de fotos de avatar:
  - Cache LRU em memória (`internal/media/service.go`) até 2.000 fotos e 64 MiB com workers dedicados e rate limit via ticker de 350ms.
  - Sanitização de logs sem vazamento de tokens ou URLs internas do Telegram.
- Regras operacionais: sem commits, sem push, sem alterações na branch `main`, sem deploy/migration em produção.

---

# Ranking mensal privado (/ranking no privado) — 2026-09-29 (somente dev; em homologação)

- O comando `/ranking` agora bifurca por contexto de chat:
  - Chat PRIVADO: exibe a pontuação mensal do próprio usuário remetente (`msg.From.ID`) por grupo no mês calendário vigente (`America/Sao_Paulo`).
  - GRUPO / SUPERGRUPO: mantém 100% inalterado o ranking competitivo do grupo (com medalhas e desempates por colocação/tempo).
- Apresentação e separação de sistemas:
  - Seções independentes para `Atualizado` e `Legado`, cada uma com seu próprio `Total · ...` calculado via soma inteira de `score_units` (`ranking.Units`).
  - Nunca combina as pontuações dos dois sistemas em um total único. Seções vazias são omitidas.
  - Para meses sem partidas pontuadas: `Você ainda não possui partidas pontuadas neste mês.`.
- Título dos grupos e Migração 0007:
  - Migration `0007_group_title.up.sql` adiciona coluna `title text NOT NULL DEFAULT ''` na tabela `group_configs` e índice `player_group_monthly_stats(user_id, month_start)`.
  - Método `ObserveGroupTitle(ctx, chatID, title)` atualiza o título do grupo de forma não destrutiva (`WHERE EXCLUDED.title <> '' AND group_configs.title IS DISTINCT FROM EXCLUDED.title`), sem sobrescrever com vazio/nulo.
  - Títulos observados automaticamente em comandos de grupo e no evento `HandleMyChatMember`.
  - Fallback determinístico: se o grupo não possuir título gravado, exibe `Grupo <ChatID>` (ex.: `Grupo -1004477538462`).
- Consulta única sem N+1:
  - `ListUserMonthlyRankings` em `internal/storage/postgres/ranking.go` executa uma única query SQL unindo `player_group_monthly_stats` e `group_configs`, com ordenação determinística por sistema, `score_units DESC`, `last_finished_at DESC`, `nome do grupo ASC` e `chat_id ASC`.
  - Score 0 elegível é exibido normalmente. Abandonos definitivos ficam fora do ranking.
- Limite de mensagem e segurança Telegram:
  - `RenderUserMonthlyRankings` respeita o limite de 4000 unidades UTF-16, truncando grupos ordenadamente com indicação `… e mais X grupos.` (ou `1 grupo.`) e mantendo o `Total` acumulado com o valor real de todos os grupos do sistema.
  - Escape HTML rigoroso nos títulos de grupos.
  - Sem botões inline, ranking global ou WebApp nesta etapa.
- Regras operacionais: sem commits, sem push, sem alterações na branch `main`, sem deploy/migration em produção.

---

# Ranking mensal do grupo — 2026-09-29 (dev; em homologação)

- O ranking ativo exibido pelo comando `/ranking` e pelas mensagens automáticas pós-partida passa a ser um RANKING MENSAL do mês corrente, em vez de cumulativo perpétuo.
- Timezone canônico: `America/Sao_Paulo` (com `_ "time/tzdata"` para compatibilidade estática em qualquer runtime/container). A virada do mês é estritamente às 00:00:00 do dia 01 do mês em Brasília. Uma partida concluída pertence integralmente ao mês do seu `finished_at`.
- Sem resets físicos, cron jobs ou comandos de purga: o histórico anterior permanece íntegro no banco de dados.
- Tabela `player_group_monthly_stats` criada pela migration `0006_monthly_ranking.up.sql` com chave composta `(chat_id, user_id, month_start date)` e índice para ranking `(chat_id, month_start, score_units DESC, user_id)`.
- Backfill na migration 0006 realiza agregação 100% exata e idempotente de `completed_games` e `completed_game_players` (jogos scored e jogadores elegíveis).
- `RecordCompletedGame` mantém atomicidade: atualiza `player_group_stats` e `player_group_monthly_stats` na mesma transação. Idempotência por `GameID` retorna `AlreadyPersisted: true` sem duplicar pontuação mensal.
- `ListGroupRanking` agora aceita `at time.Time`, busca o bucket do mês em `player_group_monthly_stats` e restringe a CTE de desempate (`latest`) exclusivamente às partidas do mês em análise. Resultados de meses anteriores não influenciam o desempate do mês corrente.
- `RenderGroupRanking` exibe o cabeçalho `🏆 Ranking do grupo · <NomeDoMês>` (ex.: `Setembro`, `Outubro`) e, quando o mês não tiver partidas, a mensagem `Ainda não há partidas pontuadas neste mês.`.
- Regras operacionais: sem commits, sem push, sem alterações na branch `main`, sem deploy/migration em produção.

---

# Desempate atualizado — 2026-09-29 (dev; homologação pendente)

- Autorização posterior do usuário: “faça o commit e o push pra dev”. Libera commit desta correção e push exclusivo da dev, incluindo 64bb97d já local. Não autoriza main, deploy ou publicação de container; não implica confirmação de homologação.

- Regra operacional: NÃO executar commit, push, amend ou rebase sem autorização explícita em mensagem futura. HEAD deve permanecer 64bb97d durante esta homologação.
- A decisão anterior de empate competitivo foi substituída: posições sempre únicas, score_units DESC → última colocação elegível ASC → finished_at dessa partida DESC → user_id ASC.
- Última participação individual vem de completed_games JOIN completed_game_players, filtrando scored e elegibilidade; DISTINCT ON por user_id, finished_at DESC/game_id DESC. JOIN com stats antes do LIMIT, sem N+1, migration ou duplicação de campos.
- Renderer compartilhado usa índice+1. Abandono, pending e N<2 não substituem a última referência; idempotência/transação mantidas. Score continua exclusivamente em player_group_stats.
- PostgreSQL real validou 3000/3000/0 e mudança de ordem após jogos individuais; normal/debugcards/vet/build passaram (debugcards teve intermitência preexistente, registrada no plano). Race bloqueado por VMA39/48.
- Nome “.” preservado; caminho usa nome observado/persistido diretamente, sem evidência de corrupção. Valor real do usuário em homologação não consultado.

---

# Histórico: ranking acumulado visível — 2026-09-29 (regra de empate substituída acima)

- Após commit pontuado: resultado da partida com ganhos e ranking histórico em mensagens separadas. /ranking usa o mesmo RenderGroupRanking, disponível a qualquer membro do grupo.
- Application: ranking.Service → ranking.ReadRepository → postgres.Store.ListGroupRanking → player_group_stats. Leitura limitada (512), total exato/config no mesmo snapshot; ordenação score_units DESC,user_id. Sem nova fonte de verdade ou alteração da transação homologada.
- Ranking histórico inclui participantes ausentes da última partida; empate competitivo por score exato (1/1/3), formato Legacy inteiro/Updated centésimos, medalhas top 3 e números “4.” em diante. Abandono definitivo explícito sem posição/pontos na mensagem.
- Mensagem até 4000 unidades UTF-16, preservando nomes/linhas completos e informando omitidos. Sem chamadas individuais Telegram ou callbacks de paginação.
- Testes PostgreSQL reais passaram em instância local isolada. Suites normal/debugcards e vet/build passaram; race bloqueado por VMA 39/48. Intermitência preexistente do teste de Trocar Mãos reproduzida na base e registrada no plano.
- Sem push, main, publicação de container, deploy ou migrations de produção; aceite Telegram depende do usuário.

---

# Encerramento formal da Milestone M7 (Persistência, Configuração e Ranking) e Adiamento de Import — 2026-09-27 (somente dev)

- A Milestone M7 foi encerrada formalmente no escopo atual da branch `dev`, devidamente testada e homologada manualmente no Telegram real.
- Status da M7: CONCLUÍDA E HOMOLOGADA NO TELEGRAM REAL (escopo de import antigo adiado).
- Escopo homologado no Telegram real:
  - Sistema Legacy: N=2 e N=3.
  - Sistema Updated: N=2 (1º=+10,00, 2º=+0,00); N=3 (1º=+10,00, 2º=+5,00, 3º=+0,00).
  - Abandono definitivo: jogador que deu `/sair` sem voltar fica fora do ranking, não entra em N, recebe +0,00 pontos e demais concluintes são pontuados normalmente.
  - Saída + reentrada: jogador que usou `/sair`, reentrou via `/entrar` e concluiu a partida pontuou normalmente conforme sua colocação.
  - Configuração de Grupo (/config): botões inline, seleção de Modo (Clássico/Caseiro) e Ranking (Legado/Atualizado), permissões de admin/instalador via `my_chat_member`, defaults não bloqueantes (`Classic` + `Legacy`), e bloqueio de alternância de ranking com histórico acumulado (`ErrNeedsProductDecision`).
- Import de ranking antigo: classificado explicitamente como DEFERRED ("Old ranking import is deferred to a future milestone"). A fundação técnica (`internal/rankingimport` e tabelas de staging) foi preservada sem criação de comandos, conversões arbitrárias ou aplicação de scores.
- Política de artefatos internos: arquivos em `.agent/` e `AGENTS.md` são de controle interno de desenvolvimento e NUNCA devem ser promovidos para a branch pública `main`.

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
- `DOCUMENT_INVALID` resolvido definitivamente: o ID cinza anterior gerava erro 400 no `getFile`.
- O bot gerou o novo ID `CAACAgEAAxkDAAMoarc5AnTTNQ_W6bTz1yaQVlhRR20AAtwHAAJGhrhFYMGxG-e10Xc9BA` via `sendSticker` diretamente ao usuário `7595607953`.
- `internal/telegram/inline.go` não possui mais fallback textual; a carta usa `InlineQueryResultCachedSticker` com ID cinza e sem token de ação de jogo.
- Testes `internal/telegram` e build com e sem tag passaram 100%.
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

---

# Contexto atual — penalidade cruzada Caseiro (2026-09-24)

- No modo Caseiro, o `+4` jogado sobre penalidade `+2` preserva o contador anterior: após a escolha de cor, `DrawCounter` passa de 2 para 6.
- A cadeia `+2 → +4 → +2` acumula 8 e a compra consome a penalidade completa.
- Desde 2026-09-25, o Caseiro recusa `+4 → +4`. A resposta cruzada `+4 → +2` permanece válida quando o `+2` corresponde à cor escolhida; o modo Clássico mantém `+4 → +4`.
- Esta regra não altera inventário: o Clássico permanece com 108 cartas e o Caseiro com 109 por causa da única `SwapHands`.

# Contexto atual — direção no estado Telegram (2026-09-25)

- `RenderPublicState` não imprime mais a linha textual `Direção: ...`.
- O sentido continua indicado na lista **Jogadores em jogo** pelos separadores `➡️` e `⬅️`, inclusive após Reverse.

# Contexto atual — comandos privados Telegram (2026-09-25)

- `/start` no privado possui apresentação curta sem versão/stack, indica `/help` e mostra `➕ Adicionar a um grupo`.
- O deep link usa o username retornado por `GetMe`; reiniciar o processo absorve mudanças feitas no BotFather.
- O payload `startgroup=true` é tratado como confirmação de adição no grupo e orienta `/novo`/`/help`; não aciona o alias `/iniciar`.
- `/help` usa blockquote para documentar comandos e termina com a origem brasileira em Go (Golang), baseada no `@unopybot`. `/ajuda` permanece como alias.
- Menus registrados por escopo: padrão `/help`; privado `/start`, `/help`; grupos comandos de partida e `/help`.

---

# Contexto atual — publicação multi-arquitetura (2026-09-24)

- O container V2 é compilado para `linux/amd64` e `linux/arm64` nos workflows de `dev` e `main`.
- `Dockerfile.v2` executa o builder em `$BUILDPLATFORM` e faz cross-compile estático com `TARGETOS`/`TARGETARCH`; o estágio distroless final é específico da plataforma.
- A `main` mantém histórico e árvore pública independentes. Promoções usam allowlist e excluem `.agent`, `.reports`, V1, arquivos locais e workflow exclusivo de desenvolvimento.
- As tags `latest` e `sha-<commit>` do GHCR representam manifestos OCI multi-arquitetura.

---

# Contexto atual — recuperação e reset por grupo (2026-09-24)

- A branch `dev` oferece `/reset` para recuperar somente o grupo afetado, sem reiniciar o processo nem tocar em outros chats.
- O comando entra por uma fila de recuperação independente (2 workers, capacidade 16), mesmo quando a fila normal está cheia.
- Depois da autorização como responsável atual ou creator/administrator confirmado pelo Telegram, o dispatcher cancela o contexto anterior, incrementa a geração e cria uma fila limpa dedicada ao chat.
- Tarefas antigas são descartadas pela geração. Todos os workers de chat, inline e recuperação possuem barreira de `recover` com stack trace sem payloads ou tokens.
- `game.Service.ResetChat` remove atomicamente a sessão ativa, índices de chat/jogadores, histórico retido e runtime; referências antigas retornam `ErrGameReset`.
- O adapter invalida todos os tokens dos jogos removidos. Administrador pode repetir reset sem estado; membro comum não pode.
- Chats privados, tópicos e remetentes anônimos são recusados. O comando está registrado no Telegram, ajuda e README.
- Cancelamento de contexto é cooperativo; código externo que ignore context pode continuar em sua goroutine, mas fica isolado da nova geração e do estado removido.

---

# Contexto atual — simulador local de partidas (2026-09-23)

- A branch `dev` possui um simulador separado em `cmd/simulator`, apoiado pelo pacote `internal/simulation` e pela API pública de `internal/uno`.
- Execução interativa solicita 2–10 jogadores e modo `classico`/`caseiro`; flags permitem seed, limite de ações, saída e modo silencioso.
- `classico` mapeia para `uno.BotRules()` e `caseiro` para `uno.CaseiroRules()`, preservando os modos efetivamente expostos pelo Telegram.
- Baralho e decisões automáticas compartilham RNG derivado da seed. A seed é registrada para reprodução da jogabilidade.
- Cada ação captura revisão, resumo anterior/posterior e eventos; todo snapshot aceito passa por `State.Validate()`.
- Relatório Markdown registra resultado, estatísticas gerais/por jogador, jogadas especiais explicadas e diagnósticos. Saída padrão: `.reports/simulations/`, ignorada pelo Git.
- O simulador não importa Telegram, não usa `.env`, rede ou banco, e não altera regras de produção.

---

# Contexto atual — milestone corretiva V2 (2026-09-23)

- Implementação na dev, a partir de b08522d, aprovada pelo usuário com "Implement the plan.".
- Engine mantém gameplay e encerra por placements quando resta um jogador, sem TurnChanged terminal.
- Telegram usa PublicGameView para decidir teclados. Closed/Finished não oferecem Suas cartas; refresh remove markup; botões históricos de resumo retido informam partida encerrada.
- Service.ExpiredTurns descobre candidatos sem mutar; AutoSkipTurn revalida jogador, GameID/ChatID, revision, fase e prazo sob lock. Scheduler aplica e notifica na mesma tarefa do chat. AutoSkipExpired continua disponível como wrapper síncrono.
- turnStarted é zerado no encerramento; leituras de final/runtime usam mutex da partida.
- Renderer recebe BotID após GetMe. PlayerLink usa UserID real só de CurrentTurn/ColorChooserID; demais nomes, lobby e encerrado apontam ao bot. Eventos usam view posterior. Sem BotID válido: texto escapado.
- Contexto inline g_<GameID> preservado: não é token de ação; necessário para abertura contextual inequívoca entre grupos. Query vazia continua com mão única/seletor.
- Regra terminal de stacking preservada por decisão explícita: sem compra extra automática, sem novo turno; contador final mantido.
- Polling/webhook compartilham pipeline. Sem mudanças em Docker, CI, ranking, campeonato ou estratégia de branches. /estado já existia na base; não foi introduzido.
- Testes, race detector, build, vet e diff --check aprovados. Homologação visual real do Telegram pendente; detalhes em docs/v2-telegram.md.

---

# Contexto atual — V2 Milestone 3 (2026-09-15)

- M3 implementada: Playable Telegram MVP em `cmd/bot`, `internal/config` e `internal/telegram`.
- Executável independente `./cmd/bot`, consumindo `internal/game.Service` sem tocar no executável V1.
- Dispatcher com 8 filas particionadas por ChatID para processar mensagens, comandos e confirmações em ordem estrita.
- 4 workers dedicados para responder consultas inline de forma não bloqueante.
- Bounded queues (32 por chat, 64 inline) provendo backpressure seguro.
- TokenStore privado em memória: tokens imprevisíveis de 128 bits (base64url) de uso único para ações e cursores opacos para paginação (> 45 cartas).
- Custom `telegoapi.RequestConstructor` (`InlineRequestConstructor`) para serializar explicitamente `cache_time:0`, `is_personal:true` e `next_offset:""` contornando o `omitempty` da telego v1.10.0.
- Custom `telegoapi.Caller` (`SafeAPICaller`) com retry limitado para HTTP 429 (até 5s) e sanitização estrita de URLs para não vazar bot token nos logs.
- Comandos em grupos: `/novo`, `/entrar`, `/iniciar`, `/cancelar` (`/kill`), `/sair`, `/estado`, `/ajuda`.
- Sem ranking, Match, persistência externa, modos extras ou filtros avançados nesta milestone.
- Documentação técnica e roteiro de homologação manual em `docs/v2-telegram.md` e `README.md`.

---

## Registro histórico da M2 (2026-09-15)

- M2 implementada em `internal/game`; Service é a API do adapter M3.
- Manager privado possui runtime UNO e mutex por partida; lock global só protege índices/resumos.
- Create não inscreve responsável. Owner observador pode Start/Cancel; dealer é participante ativo.
- Engine só recebeu exceção de participação para Start/Cancel, preservando solicitante verdadeiro.
- Índices chat/jogador são de sessões abertas/participação ativa, sem current game global.
- PlayerView só expõe mão do próprio Actor confiável; PublicView não contém snapshots/mãos.
- Encerramento descarta runtime e mantém somente resumos públicos FIFO (100 por padrão, configurável).
- MemoryRepository adiado: snapshots transitórios, sem segunda fonte de verdade ou recovery implementado.
- Sem Telegram V2, tokens, ranking, Match, timers ou backend persistente nesta entrega; V1 intacto.
- Contrato atual: `docs/v2-application.md`; regras: `docs/v2-rules.md`; auditoria histórica: `docs/v2-audit.md`.

---

## Registro histórico da M1 (decisões futuras abaixo foram atualizadas pela M2)

# Contexto atual — V2 Milestone 1 (2026-09-14)

- O executável V1 permanece na raiz, em `package main`, com telego v1.10.0.
- V1 exige PostgreSQL para ranking/configuração; somente partidas são efêmeras em RAM.
- `internal/uno` é a nova engine independente de Telegram, SQL, ambiente e relógio.
- Actions transacionais por cópia, revision estrita, cartas físicas com IDs, snapshots serializáveis.
- `ClassicRules`: primeiro vencedor, sem entrada tardia. `BotRules`: colocações e entrada tardia.
- Ambos usam regras Classic das cartas, UNO automático e bloqueio de +4 ilegal.
- Manager/locks por partida, MemoryRepository e serviço serão Milestone 2; adapter V2 será M3.
- Engine é de dono único: chamadas ao mesmo Game devem ser serializadas pelo futuro manager.
- Documentação autoritativa: docs/v2-rules.md e docs/v2-audit.md.
- O módulo Go continua github.com/malbs/UnoGoBot; V1 não foi migrado nem removido.

---

## Contexto histórico do V1 (pode conter informações superadas)

# Contexto do Projeto - UnoGoBot

## Stack e Ferramentas
- **Linguagem**: Go (1.20+)
- **Biblioteca**: `github.com/mymmrac/telego` para interações com a API do Telegram.
- **Configurações**: Gerenciadas via variáveis de ambiente carregadas pelo `godotenv`.
- **Persistência**: Totalmente em memória (ram). Sem banco de dados relacional ou chave-valor persistente.

## Objetivos e Requisitos
1. Permitir que múltiplos grupos iniciem e joguem partidas de UNO de forma concorrente e isolada.
2. Usar o inline mode do Telegram com Stickers para uma experiência gráfica de exibição e seleção de cartas.
3. Manter a integridade de concorrência usando locks (`sync.Mutex`) no gerenciador de jogos e estados de jogo individuais.

## Padrões Internos
- **Listas Circulares**: Os jogadores são encadeados em um anel usando referências de ponteiros `Next` e `Prev`. Isso facilita a alteração de turnos (`Turn`) e efeitos de inversão (`Reverse`).
- **Ponteiros Globais**: O `GameManager` rastreia o jogo ativo atual de cada usuário (`UserIDCurrent`) e a lista de jogos ativos de cada usuário (`UserIDPlayers`) para saber como direcionar as requisições que chegam sem ChatID (como as queries inline).
- **Parâmetros de Contexto**: A inline query utiliza parâmetros de string (como o ID do chat) passados via switch do botão "Suas cartas" para manter o alinhamento de contexto no ambiente multi-grupo.


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

# M7 — fundação iniciada (2026-09-27)
- cmd/bot requer DATABASE_URL, ping e schema atualizado antes do transporte; cmd/migrate aplica SQL versionado sem TOKEN.
- internal/storage/postgres usa pgx/v5; internal/uno e internal/game continuam sem imports de PostgreSQL.
- Sem fila de resultados/outbox por decisão explícita. Finalização futura deve aguardar transação antes da pontuação Telegram.
- Plano aprovado com decisões competitivas/UX ainda pendentes. Não declarar M7 completa.

- M7.2: group_configs alimenta /novo; explicit overrides continuam. PublicGameView/managedGame carregam Snapshot (ranking_system/config_revision) por valor. Engine e ações de gameplay não importam storage. groups.Service verifica associação atual para configuração; setup Telegram ainda não integrado.

- M7.3 foundation: completed_games + completed_game_players + player_group_stats. Resultado oficial público extraído antes de descarte da engine e retido em memória independente do FIFO/reset. Hooks de fechamento inline/saída/timeout aguardam RecordCompletedGame; erro retém DTO. Sem política competitiva aprovada: runtime persiste needs_product_decision (score NULL, sem stats). Calculador e transação pontuada testados com policy exclusiva de teste. Nenhuma mensagem de pontos ainda.

- M7.4: metadata de nomes observada por comandos/chosen inline é RAM durante gameplay; finalization persiste conhecidos juntamente com resultado. /novo pode gravar criador diretamente por ser criação/configuração. Import futuro lista conhecidos por UserID sem scrape.

- M7.5 groundwork: internal/rankingimport parser/reconciliation sem SQL; postgres staging versionado/auditável. Fonte é Legacy e nenhuma operação de aplicação/reset/conversão foi criada.

# M7 — UX Telegram de Configuração de Grupo (2026-09-27, somente dev)
- Comando único `/config` em grupos para administradores atuais e instalador do bot (se ainda membro do grupo). Usuários comuns ou instalador que saiu têm acesso recusado com mensagem amigável (fail closed).
- Interface inline com botões para alternar Modo padrão (`Clássico` / `Caseiro`) e Sistema de Ranking (`Legado` / `Atualizado`), com marcação visual do item ativo.
- Setup é totalmente opcional e não bloqueia o início de partidas: defaults do sistema são `Classic` + `Legacy`.
- Snapshot por partida preservado: `/novo` consome a configuração do grupo no momento da criação. Mudanças posteriores de configuração não afetam partidas em andamento. `/novo classico` e `/novo caseiro` continuam operando como overrides exclusivos daquela partida sem alterar a `GroupConfig`.
- Transição de ranking protegida: troca `Legacy` ↔ `Updated` é bloqueada caso o grupo já contenha pontuações ou partidas pontuadas no sistema anterior (`groups.ErrNeedsProductDecision`), emitindo alerta explicativo no callback e preservando a configuração anterior sem conversão, reset ou rankings paralelos.
- Suporte a `my_chat_member`: detecta transição real de instalação/reentrada (`left`/`kicked` -> `member`/`administrator`), registra `installed_by_user_id` a partir do ator e envia mensagem curta de boas-vindas com botão `[ ⚙️ Configurar ]`. Updates de status irrelevantes (promoções/demissões de cargo do bot) não disparam mensagem.
- Registro monotônico de `KnownGroupUser` integrado em `/config`, callbacks `cfg_` e `my_chat_member`.

# Trocar Mãos opcional — 2026-09-28 (dev, sem push)
- Base bce47d0. PlayCard → ChoosingPlayer → ChoosePlayer/KeepHand → ChoosingColor → ChooseColor. Revisão por ação (r+1/r+2/r+3).
- ColorChoice.SwapHands/SwapTarget guardam decisão. Zero representa manter; mãos e turno só mudam na cor. Inline/tokens existentes, sem callbacks novos.
- Última carta: completePlay imediato, sem escolhas/troca; conserva cor anterior se partida continua. Exceção só para SwapHands em NoWildFinish; +4/Wild intactos.
- Alvo que sai durante cor invalida seleção, reabre ChoosingPlayer; tokens anteriores stale. Timeout pendente segue sem intervenção.
- M7/config/ranking/storage preservados. Novo fluxo aguarda homologação Telegram; não publicado na main.


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

### Material da navegação
`useLiquidGlass.ts` gera mapa da lente nas bordas da pílula. `BottomNavigation.tsx` aplica filtro ao backdrop e mantém seleção derivada da URL. Sem dependências novas, backend ou API.

# Configuração V2 simplificada — 2026-10-01

- Runtime carrega apenas `internal/config.Config`: TOKEN, TELEGRAM_MODE, DATABASE_URL, TURN_TIMEOUT, WEB_ADDR, MINIAPP_SECRET e WEBHOOK_URL condicional. `Web`/`LoadWeb` removidos; cmd/bot chama um loader único.
- Políticas fixas em internal/config/defaults.go: logs info, histórico100, inline TTL2m/global20000/user512, initData1h e migration startup2m. Descarte de updates pendentes fixo false em todos os caminhos de transporte.
- Webhook preserva setWebhook em cada startup, URL pública explícita HTTPS com caminho dedicado e comparação constante do header. Secret = Base64URL sem padding de HMAC-SHA256 com master decodificado32bytes e contexto unobotgo/v2/telegram-webhook-secret/v1. Rotação do master atualiza secret no próximo startup; master nunca enviado/logado.
- Bot.Run gera Direct Mini App https://t.me/<username>/ranking com getMe já existente; shortname ranking constante. URL HTTPS externa pertence ao BotFather.
- Variáveis antigas em registros anteriores de .agent são históricas, não instruções de instalação atuais. .env real preservado; ferramentas devseed/migrate e V1 continuam com suas configurações específicas fora do runtime V2.

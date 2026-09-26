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

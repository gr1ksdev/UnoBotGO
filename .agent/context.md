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

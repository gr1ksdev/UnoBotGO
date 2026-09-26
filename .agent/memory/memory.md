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
- Webhook usa `WEBHOOK_URL`, `WEBHOOK_SECRET`, `WEBHOOK_LISTEN_ADDR` e `WEBHOOK_DROP_PENDING_UPDATES` (default false), com `/healthz` liveness e dedupe em memória por UpdateID.


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

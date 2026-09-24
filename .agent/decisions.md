# Decisões

# Decisão: Autorização da ação CallBluff no Service da aplicação

## Data
2026-09-23

## Contexto
Ao desafiar o blefe de um +4 Coringa através do sticker inline `option_bluff`, a jogada era rejeitada com o erro `invalid action` (`⚠️ <jogador>: Jogada não aceita: invalid action. Abra Suas cartas novamente.`).

## Decisão tomada
Incluir `uno.CallBluff` no switch de autorização de ações inline em `internal/game/service.go` (`case uno.PlayCard, uno.DrawCard, uno.PassTurn, uno.ChooseColor, uno.CallBluff:`), permitindo que a ação seja devidamente autenticada e enviada à engine `uno.Game`.

## Motivo
Garantir que a ação de blefe não seja descartada prematuramente pela camada de aplicação antes de alcançar as regras da partida.

## Impacto
O blefe agora é processado normalmente pelo bot no Telegram.

---

# Decisão: Seleção de modo no lobby e travamento pós-início

## Data
2026-09-23

## Contexto
Ao enviar o comando `/novo`, a mensagem do lobby exibia o botão "🃏 Suas cartas", mesmo sem nenhuma carta distribuída aos jogadores. O usuário solicitou que esse botão fosse substituído pela seleção de modo (Clássico ou Caseiro) e que essa seleção permanecesse disponível até o início da partida com `/iniciar`, momento a partir do qual a alteração de modo não é mais permitida.

## Decisão tomada
1. Na engine (`internal/uno`):
   - Adicionar `SetRules ActionType = 11` e `RulesChanged EventType = "rules_changed"`.
   - Adicionar campo `Rules Rules` em `Action`.
   - No `Game.Apply`, permitir a ação `SetRules` exclusivamente quando `Phase == Lobby`, atualizando as regras e emitindo `RulesChanged`. Se já iniciado, rejeitar com `ErrGameStarted`.
2. No serviço (`internal/game`):
   - Autorizar `uno.SetRules` no mesmo chat para o responsável (`actor.PlayerID == entry.ownerID`).
   - Adicionar método `s.SetRules(ctx, actor, gameID, rules)`.
3. No teclado do Telegram (`internal/telegram/commands.go`):
   - Em `makeGameButtons`, durante `Phase == uno.Lobby`, retornar botões inline de seleção de modo: `[ ✅ 🎻 Clássico ]  [ 🏠 Caseiro ]` (ou vice-versa com base nas regras ativas).
   - Durante as fases de jogo ativo (`TakingTurn`, `ChoosingColor`), retornar exclusivamente `[ 🃏 Suas cartas ]`.
4. No handler de callbacks (`internal/telegram/callbacks.go`):
   - Tratar prefixos `mode_classic_` e `mode_caseiro_`.
   - Validar que a partida ainda está em `Phase == Lobby`. Se já iniciada, exibir alerta no Telegram informando que o jogo já começou e o modo não pode ser alterado.
   - Validar que o solicitante é o responsável pela partida (`view.OwnerID`). Se não for, alertar que apenas o responsável pode alterar.
   - Atualizar as regras via `s.SetRules` e editar em tempo real o texto do lobby (`RenderLobby`) e os botões (`makeGameButtons`).

## Motivo
Eliminar o botão prematuro "Suas cartas" durante o lobby, proporcionar uma experiência intuitiva e rápida de configuração de modo diretamente no grupo e garantir que as regras da partida fiquem travadas após o início.

## Impacto
Interface mais clara e organizada no lobby, sem confusão para novos jogadores e com total garantia de imutabilidade das regras após o início do jogo.

---

# Decisão: Restituição do blefe no +4 Coringa e ordenação das cartas da mão estilo V1

## Data
2026-09-23

## Contexto
O usuário relatou que a opção de blefe havia sumido quando um jogador descartava +4 Coringa, impedindo a vítima de desafiar o blefe. Além disso, as cartas da mão no teclado inline apareciam espalhadas e desordenadas (ex: vermelhas misturadas com verdes).

## Decisão tomada
1. Criar a ação `uno.CallBluff` e o evento `uno.BluffCalled` na engine.
2. Na engine (`internal/uno`):
   - Ao jogar `WildDrawFour`, verificar se o descarte foi um blefe (se o jogador possuía cartas da cor ativa na mão antes de jogar o +4) e guardar `Bluffing bool`.
   - Ao escolher a cor (`ChooseColor`), criar `s.PendingBluff = &BluffInfo{Actor: actor, Target: target, Bluffing: bluffing}`.
   - Tratar `CallBluff`: se teve sucesso (o autor blefou), o autor recebe a penalidade de compra `DrawCounter`; se não teve sucesso (não blefou), o desafiante recebe `DrawCounter + 2` cartas. Em ambos os casos a penalidade é aplicada e a vez passa para o próximo jogador.
   - Limpar `PendingBluff` caso qualquer outra ação seja executada.
3. Expor `CanCallBluff bool` na `PublicGameView` de `internal/game/views.go`.
4. No Telegram inline (`internal/telegram/inline.go`):
   - Ordenar a mão do jogador (`sortHand`) seguindo o critério da V1: Vermelho -> Azul -> Verde -> Amarelo -> Coringas (+4 e Wild), ordenados numericamente e depois por ação internamente.
   - Quando `view.Public.CanCallBluff` for verdadeiro e for a vez da vítima, anexar o sticker de blefe `option_bluff` ("BQADBAADygIAAl9XmQABJoLfB9ntI2UC").
5. No Telegram renderer (`internal/telegram/renderer.go`):
   - Renderizar o resultado da ação de blefe ("Blefe pego!" ou "<jogador> não blefou!").

## Motivo
Restauração integral da regra clássica de desafio de blefe do +4 Coringa e melhoria ergonômica da visualização de cartas na mão do jogador, garantindo organização visual limpa e paridade total com a V1.

## Impacto
Mecânica de blefe do UNO restabelecida com fidelidade à V1 e cartas perfeitamente agrupadas por cor e valor na interface inline.

---

# Decisão: Restauração integral das regras de cartas da V1 na engine V2

## Data
2026-09-23

## Contexto
O usuário solicitou o retorno de todas as regras de jogabilidade de cartas originais da V1, preservando 100% dos textos, layouts e formatações da V2 atual. Na V1 existiam comportamentos clássicos específicos como não bater com Coringa, proibição de jogar Coringa sobre Coringa, +4 livre sem restrição de cartas na mão, e a mecânica de compra livre onde o turno não passa compulsoriamente se a carta comprada for incompatível.

## Decisão tomada
1. Adicionar quatro novas flags configuráveis na struct `Rules` da engine (`internal/uno`):
   - `NoWildFinish`: impede que um jogador vença/feche o jogo descartando uma carta especial (Wild ou WildDrawFour) como sua última carta.
   - `NoWildOnWild`: impede jogar Coringa sobre Coringa (Wild sobre Wild, +4 sobre Wild, etc.) na rodada comum quando não estiver respondendo a empilhamento de penalidade.
   - `AllowWildDrawFourAlways`: permite descartar +4 a qualquer momento, sem a restrição oficial da Mattel de verificar se o jogador tem a cor ativa na mão.
   - `FreePlayAfterDraw`: após comprar uma carta voluntariamente, não passa a vez compulsoriamente; o jogador pode descartar qualquer carta válida de sua mão ou acionar a ação `PassTurn`.
2. Habilitar todas essas flags em `BotRules()` (modo Clássico do bot) e em `CaseiroRules()` (modo Caseiro do bot).
3. No `CaseiroRules()`, manter adicionalmente as regras de cruzamento de penalidades (`StackWildDrawFourOnTwo` e `StackDrawTwoOnWildFour`), respeitando a paridade com a V1.
4. Manter `ClassicRules()` sem essas flags para compatibilidade estrita do manual Mattel nos testes de conformidade.
5. Preservar inalterada toda a camada de apresentação em HTML, formatações, queries dinâmicas, menções seguras e reação festiva `🥳` no UNO.

## Motivo
Fidelidade total à experiência de jogo consolidada na versão 1 do UnoBotGO, mantendo a estabilidade arquitetural e a segurança de concorrência da V2.

## Impacto
Jogabilidade idêntica ao bot clássico, sem quebras visuais e com suite de testes 100% aprovada.

---

# Decisão: Empilhamento de +4 coringa e invalidação de cache de cartas

## Data
2026-09-23

## Contexto
Após jogar um +4 Coringa, o bot na engine V2 aplicava imediatamente a compra de 4 cartas e pulava compulsoriamente a vez do próximo jogador, tirando a interatividade e a possibilidade de rebater (+4 sobre +4). Além disso, os botões inline estáticos com `g_<GameID>` faziam com que o cache do cliente do Telegram entregasse a visão de seleção de cores do primeiro jogador quando outro jogador abria o botão "Suas cartas".

## Decisão tomada
1. Adicionar `StackWildDrawFour: true` em `BotRules()`, permitindo que ao escolher a cor do +4 a penalidade seja acumulada em `DrawCounter` e o próximo jogador receba a sua vez normalmente para rebater com outro +4 ou recolher as cartas voluntariamente.
2. Anexar a revisão atual da partida no botão `🃏 Suas cartas` (`g_<GameID>_<revision>`), tornando a query string sempre dinâmica a cada turno/jogada para invalidar o cache local dos clientes do Telegram.
3. Permitir parse de query inline flexível ignorando sufixos após o `GameID`.
4. Durante `ChoosingColor`, exibir exclusivamente o aviso de espera e o resumo das próprias cartas para jogadores espectadores/não-escolhedores, sem expor seletores de cor nem stickers.

## Motivo
Eliminar a sensação de pulo de turno automático e garantir que cada jogador veja com fidelidade suas próprias cartas no Telegram.

## Impacto
Fluxo de jogo do +4 alinhado ao comportamento esperado do bot, com controle completo de turnos e sem vazamento de opções ou cache compartilhado entre usuários.

---

# Decisão: Correção de estado global e concorrência V1

## Contexto
O bot mantinha estado global em memória e apresentava problemas de cartas fantasmas ao limpar lobbies não iniciados (vazamento de referências) e exibição incorreta de cartas em inline query ao jogar em múltiplos chats devido à falta de contexto do chat nas requisições inline.

## Decisão tomada
1. Reverter a exibição de ChatID no input do inline query, mantendo a caixa de texto limpa.
2. Sincronizar o ponteiro global de jogo ativo `UserIDCurrent[userID]` de forma proativa e invisível ao usuário:
   - A cada mensagem ou comando recebido do usuário no grupo (em `handleMessage`), caso ele tenha um player associado àquele chat.
   - A cada transição de turno no jogo (ao iniciar a partida, ao passar o turno após uma jogada inline, ao kickar, skippar ou quando um jogador sai).
3. Encapsular a limpeza de referências de jogadores de um jogo na função privada `removeGamePlayers` do `GameManager` e utilizá-la tanto no encerramento normal/forçado quanto na limpeza de lobbies não iniciados (`CleanGames`).
4. Implementar getters thread-safe (`GetCurrentPlayer` e `GetPlayersForUser`) para ler os mapas globais sob Mutex.
5. Adotar o mecanismo de `AntiCheat` idêntico ao repositório original `jh0ker/mau_mau_bot` para invalidar naturalmente o cache local de inline queries no Telegram:
   - Anexar o sufixo `:AntiCheat` nos IDs dos resultados inline (ex: `r_7:0`).
   - Extrair, validar e incrementar o `player.AntiCheat++` no `handleChosenInlineResult`.
6. Corrigir a rotação de turno (`Turn`) e listagem de jogadores (`Players`) em `game.go` para navegar no sentido anterior (`.Prev`) quando `Reversed` for `true`.
7. Otimizar a liberação de locks de `game.Lock()` no `handleChosenInlineResult` para ser realizada de forma antecipada (antes de chamadas de rede do Telegram e do `UpdateCurrentPlayer`), prevenindo deadlocks.

## Motivo
Garante a integridade do estado e evita o descompasso na sincronização do inline query sem expor IDs internos no campo de texto de digitação do Telegram. A liberação antecipada de locks previne deadlocks de Mutex e otimiza a latência. A correção em game.go alinha a rotação ao comportamento real de jogo UNO.

## Impacto
O bot agora é estável em cenários com múltiplos jogos paralelos e limpa totalmente a memória ao cancelar lobbies, sem deixar resíduos de cartas ou sessões fantasma para os usuários. A carta reverse agora muda de fato a rotação do jogo e não causa travamentos concorrentes.

# Decisão: fronteira da engine V2 e políticas de rodada

## Data
2026-09-14

## Contexto
O usuário aprovou a implementação da Milestone 1 após auditoria do V1. Escolheu
bloquear +4 ilegal inicialmente, manter UNO automático e preservar continuidade
por colocação/entrada tardia. Inline multigrupo usará seleção explícita futuramente.

## Decisão tomada
Criar internal/uno sem Telegram, SQL ou estado global. Game encapsula State, ações
são aplicadas numa cópia e só confirmadas após validação; revision incrementa uma
vez por ação aceita. Snapshot/Restore fazem cópia profunda, e cartas físicas têm
IDs. Regras Classic são independentes das políticas FirstWinner/Placements e
AllowLateJoin. Dez registrados por jogo, sem reentrada; falta de cartas falha
atomicamente. Cor ativa não modifica a carta Wild. Challenge fica para depois.

## Motivo
Permitir testes determinísticos e futura troca de interface, eliminar acoplamento
e corrupção parcial, e preservar a dinâmica desejada sem chamar as adaptações de
regras oficiais. A implementação V1 não é substituída durante construção da engine.

## Impacto
Nova engine com biblioteca padrão apenas. M2 adicionará serviço, índices,
MemoryRepository e mutex privado por jogo. A engine requer acesso serializado;
Estado não serializa locks/RNG. M3 conectará Telegram, snapshots privados serão
convertidos em views autorizadas, tokens vincularão resultados a usuário e partida.
Ranking/Match não entram na engine. PostgreSQL do V1 permanece intacto.

## Documentação
- docs/v2-rules.md: API, diferenças de regras, atomicidade e limites.
- docs/v2-audit.md: arquitetura V1, riscos e fatos verificados na Bot API.


# Decisão — M2: aplicação, administração independente e ownership único

## Data
2026-09-15

## Contexto
M1 possui engine de dono único e snapshots privados completos. Usuário aprovou
M2 com criador não participante, gestão independente, múltiplos chats por jogador
e retenção de somente resumos públicos dos últimos 100 jogos encerrados.

## Decisão tomada
Service exportado e manager privado em internal/game. Runtime único por partida,
mutex privado e índices publicados em seção curta sob lock global. Não criar
MemoryRepository: não há responsabilidade distinta sem persistência real.
Start/Cancel usam solicitante real, autorizado pelo OwnerID no serviço; exceção
mínima da guarda de participação na M1, mantendo dealer ativo e validações de regras.
Views não expõem snapshots. PlayerView não recebe target: somente mão do Actor.
Encerramento libera índices/runtime e arquiva só view pública com FIFO configurável.

## Motivo
Evitar duas fontes de verdade, falsificação do autor administrativo, inscrição
implícita, vazamento de mãos e serialização desnecessária entre partidas.

## Impacto
M3 consumirá Service e fará autenticação/seleção multigrupo. Sem tokens ou Telegram
nesta entrega. Context cancelado após aceitação não interrompe publicação. Recovery
futuro exige envelope State+metadata e reconstrução de índices, fora de I/O sob locks.
Engine permanece independente; V1 e dependências/configuração permanecem intactos.

## Documentação
- docs/v2-application.md: API, autorização, lifecycle, locking, privacidade e limites.
- docs/v2-rules.md: contrato administrativo mínimo atualizado da M1.

# Decisão — M3: Playable Telegram MVP, concorrência e tokens de uso único

## Data
2026-09-15

## Contexto
A Milestone 3 conecta a aplicação V2 ao Telegram sem acoplamento de engines ou banco de dados externo. O V1 sofria com descompasso de cache (`omitempty` omitindo `cache_time:0`), concorrência não serializada por chat, locks retidos em I/O de rede e identificação frágil de partidas inline.

## Decisão tomada
1. Executável V2 em `cmd/bot/main.go` consumindo `internal/game.Service`.
2. Dispatcher com 8 filas particionadas por `ChatID` (buffer 32) para mensagens e jogadas, e 4 workers separados (buffer 64) para consultas inline.
3. Tokens criptográficos de 128 bits (`TokenStore`) para vincular cada resultado inline a uma ação de revisão estrita, consumidos atomicamente e invalidados por evento ou tempo.
4. `InlineRequestConstructor` para contornar `omitempty` na serialização de `answerInlineQuery`, emitindo `cache_time:0` e `is_personal:true`.
5. `SafeAPICaller` com sanitização de logs (nunca expor tokens ou URLs autenticadas) e retry limitado exclusivo para HTTP 429.
6. Isolamento total entre V1 e V2: nenhum código legado é alterado.

## Motivo
Garantir concorrência estritamente ordenada por grupo, respostas inline imediatas e não bloqueantes, segurança contra anti-cheat/repetição e conformidade com as regras de confidencialidade e integridade do bot.

## Impacto
O V2 torna-se jogável de ponta a ponta no Telegram. A engine e o serviço mantêm zero dependência de transporte. Sem persistência externa, ranking ou Match nesta milestone.

## Documentação
- docs/v2-telegram.md: arquitetura detalhada e roteiro de homologação manual.
- README.md: visão geral e comandos.

# Decisão: Paridade V1 de empilhamento +2, seletor de cor limpo e grito de UNO

## Data
2026-09-15

## Contexto
Na jogabilidade do V1, jogar um `+2` não pulava o oponente imediatamente; transferia o turno para que o jogador pudesse ver suas cartas e rebater com outro `+2` (acumulando a penalidade) ou comprar as cartas acumuladas. No seletor de cor, o jogador recebia 4 opções de cor limpas e um 5º artigo de resumo da mão, sem stickers cinzas misturados. Por fim, o anúncio de UNO devia ser enviado em mensagem dedicada no grupo com reação festiva 🥳.

## Decisão tomada
1. Adicionada regra `StackDrawTwo: true` a `uno.BotRules()` e campo `DrawCounter int` em `uno.State`.
2. Em `internal/uno/game.go`:
   - Quando `s.Rules.StackDrawTwo && card.Rank == DrawTwo`: `s.DrawCounter += 2` e a vez passa para o próximo jogador sem skip.
   - Enquanto `s.DrawCounter > 0`: apenas cartas `DrawTwo` podem ser jogadas; se o jogador comprar, recebe `s.DrawCounter` cartas de uma vez, zera o contador e o turno avança.
3. Em `internal/telegram/inline.go`:
   - Na fase `ChoosingColor`, se o jogador for o selecionador da cor, retorna exclusivamente 4 artigos de cor ("Escolha sua cor") e 1 artigo de resumo das cartas da mão ("Cartas: ..."), retornando imediatamente sem misturar stickers cinzas.
   - Na fase `TakingTurn` com `DrawCounter > 0`, o sticker de compra exibe "Comprando X cartas" e as cartas não-+2 da mão ficam desabilitadas (cinzas).
   - Ao detectar `uno.UnoAnnounced` em `HandleChosenInlineResult`, o bot envia uma mensagem separada no grupo (`<link> <b>Gritou UNO!</b>`) e aplica a reação `🥳` via `SetMessageReaction`.
4. Interface `BotAPI` estendida com `SetMessageReaction(ctx context.Context, params *telego.SetMessageReactionParams) error`.

## Motivo
Garantir paridade completa com a dinâmica clássica apreciada pelos jogadores no V1, eliminando a frustração de perder o turno compulsoriamente sem poder visualizar a mão ou rebater com outro +2, mantendo a interface inline limpa na escolha de cores e celebrando o grito de UNO com a reação festiva solicitada.

## Impacto
Controle total do jogador preservado, interface do seletor alinhada ao V1 e suporte nativo à reação festiva via Bot API sem introduzir regressões ou alterar a separação arquitetural da engine.

# Decisão: Simplificação de mensagens, remoção do botão de atualizar e liberação de /iniciar

## Data
2026-09-15

## Contexto
O usuário solicitou remover o botão "🔄 Atualizar estado" do teclado inline nas mensagens do jogo, remover a linha de cabeçalho `🃏 UnoBotGO`, ocultar a contagem de cartas `(X cartas)` dos jogadores nas mensagens públicas da mesa e permitir que qualquer membro do chat possa dar `/iniciar` (e não apenas quem criou a partida via `/novo`), mantendo `/cancelar` exclusivo do responsável.

## Decisão tomada
1. Em `internal/game/service.go`: autorização de `uno.StartGame` atualizada para exigir apenas que o autor esteja no chat da partida (`actor.ChatID != 0`), sem exigir que seja `entry.ownerID`. O comando `uno.CancelGame` permanece restrito exclusivamente ao `entry.ownerID`.
2. Em `internal/telegram/commands.go`: `makeGameButtons` simplificado para conter apenas o botão `🃏 Suas cartas`, eliminando o botão `🔄 Atualizar estado`.
3. Em `internal/telegram/renderer.go`:
   - Removido cabeçalho `🃏 <b>UnoBotGO</b>\n\n` de `RenderPublicState`.
   - Na lista "Jogadores em jogo:", removida a contagem `(X cartas)`. Se o jogador estiver com 1 carta, exibe `⚠️ <b>UNO!</b>`.
   - Em `RenderLobby`, atualizado o texto para `Use /iniciar para começar!`.

## Motivo
Atender à preferência do usuário por mensagens mais compactas e limpas no grupo, manter a privacidade das mãos (sem expor a contagem exata de cartas de cada um a cada lance) e trazer paridade com o V1 onde qualquer membro podia iniciar a partida quando houvesse quórum.

## Impacto
Mensagens no chat do Telegram ficam mais limpas, diretas e com menos botões desnecessários, melhorando a experiência mobile dos usuários.



# Decisão

## Data
2026-09-16

## Contexto
A Milestone 6 adiciona Webhook como transporte alternativo ao long polling.

## Decisão tomada
Usar um pipeline único de updates, servidor `net/http` interno com validação de `X-Telegram-Bot-Api-Secret-Token`, `SetWebhook` explícito em todo startup webhook, `DeleteWebhook(false)` ao iniciar polling e deduplicação em memória por `UpdateID`.

## Motivo
Preservar a jogabilidade existente, aplicar alterações de segredo sem operação manual e evitar processamento duplicado sem adicionar infraestrutura persistente.

## Impacto
Webhook exige URL HTTPS pública terminada externamente; a deduplicação é perdida após reinício.


# Decisão: encerramento e menções contextuais do V2

## Data
2026-09-23

## Contexto
A confirmação final oferecia acesso a uma mão indisponível. O scheduler podia publicar timeout fora de ordem. Todos os nomes apontavam ao próprio jogador. O usuário aprovou o plano corretivo e decidiu preservar a regra terminal existente de stacking.

## Decisão tomada
- Preservar a engine; teclados e erros seguem a view atual/terminal. Invalidar tokens existentes no encerramento natural ou por saída, mantendo validação server-side como autoridade.
- Separar descoberta de candidatos de timeout da aplicação; GameID, ChatID, jogador, revision, fase e prazo são revalidados sob mutex. Mutação e mensagem executam na mesma fila de chat. Limpar turnStarted ao encerrar.
- Centralizar destino das menções em Renderer.PlayerLink: UserID real somente para responsável atual, BotID nos demais casos. Usar view resultante em confirmações/eventos; obter BotID de GetMe antes do ingresso.
- Manter g_<GameID> visível, separado dos tokens de ação one-use. Não há contexto oculto equivalente no botão Inline Mode. Não adicionar aliases, mudar formato ou usar seleção global por usuário.
- Preservar penalidades imediatas, cor pendente e stacking terminal atual (sem nova compra automática nem chance de rebater). Não modificar gameplay, infraestrutura ou transportes.

## Motivo
Corrigir os caminhos que oferecem ou anunciam turnos obsoletos sem inventar outra fonte de estado nem sacrificar segurança/multigrupo. A política de menções precisa refletir o responsável posterior à ação, não seu autor anterior.

## Impacto
Final sem convite para jogar; candidates antigos tornam-se no-op; mesma implementação em polling e webhook. Novas mensagens apontam ao bot exceto pelo jogador responsável. Mensagens históricas não são reescritas em massa. Aceitação visual de tg://user?id=<BotID> continua pendente em clientes reais, conforme roteiro documentado.

# Decisão: simulador local desacoplado dos transportes

## Data
2026-09-23

## Contexto
Era necessário executar partidas automáticas com quantidade e modo escolhidos pelo operador, detectar falhas e explicar cartas especiais sem depender do Telegram ou duplicar as regras da engine.

## Decisão tomada
Criar `internal/simulation` sobre a API pública de `internal/uno` e expô-lo por `cmd/simulator`. Usar uma seed única para embaralhamento e decisões, validar o snapshot após cada ação e gerar relatório Markdown a partir de ações, eventos e estados resumidos. Manter o simulador fora de `internal/game` e `internal/telegram`.

## Motivo
A engine é a fonte de verdade das regras e já oferece ações transacionais, snapshots, `CanPlay`, shuffler injetável e validação estrutural. Um adaptador local separado testa esse contrato sem credenciais, rede, banco ou efeitos no bot em produção.

## Impacto
Partidas de 2–10 bots nos modos Clássico e Caseiro podem ser reproduzidas por seed. Relatórios ficam em `.reports/simulations/`, fora do Git. O simulador cobre lógica de jogo; transporte, stickers, callbacks e filas Telegram permanecem fora de seu escopo.
# Decisão: recuperação isolada e geração por grupo

## Data
2026-09-24

## Contexto
O bot legado em Python podia deixar um grupo lento ou sem respostas após uma ação desconhecida. Na V2, grupos compartilham workers particionados e um comando colocado na fila comum não conseguiria recuperar um shard saturado.

## Decisão tomada
Processar `/reset` em uma fila administrativa independente. Após autenticar o responsável ou administrador do grupo, cancelar o contexto anterior, avançar a geração do chat e encaminhar novos trabalhos para uma fila dedicada limpa. Remover atomicamente no serviço a partida ativa, índices, histórico e runtime daquele chat, tombstonar referências antigas e invalidar os tokens retornados. Proteger todas as classes de worker com recuperação de panic.

## Motivo
A via de recuperação precisa continuar acessível quando o caminho comum falha e precisa isolar ações antigas sem reiniciar o bot inteiro ou interromper outros grupos. A autorização via Telegram evita que um membro comum use a limpeza para sabotar partidas.

## Impacto
O grupo pode criar uma nova partida imediatamente após o reset. Tarefas da geração anterior são descartadas e o estado removido não pode ser republicado por referências antigas. O mecanismo não recupera processo morto, indisponibilidade global da API ou código externo que ignore cancelamento; esses casos ainda dependem do supervisor do processo e dos timeouts de rede.

---

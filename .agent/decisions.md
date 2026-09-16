# Decisões

## Data
2026-06-21

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

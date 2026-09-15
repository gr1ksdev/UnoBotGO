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

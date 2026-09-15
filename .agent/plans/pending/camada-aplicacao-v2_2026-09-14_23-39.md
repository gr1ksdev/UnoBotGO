# Plano: camada-aplicacao-v2 — Milestone 2

## Pedido do usuário
Planejar a aplicação entre o futuro adapter Telegram e internal/uno, com ownership
único, concorrência por partida, índices consistentes e views privadas/públicas.
Não implementar antes da aprovação. Não modificar V1 nem redesenhar a M1.

## Objetivo
Criar internal/game como única API consumida pela M3, sem Telegram, tokens,
persistência externa, ranking, Match, timers ou novas regras de UNO.

## Contexto atual
M1 nos commits d7af8f8 e 680d42b. Game.Apply faz cópia/validação/commit atômico e
revision estrita, mas requer acesso serializado, inclusive Snapshot/CanPlay.
Snapshot contém mãos privadas. Players conserva registrados; Order conserva os
ativos. WentOut/Left deixam a rotação; Finished pode manter jogadores Playing.
Último Wild aguarda escolha antes da colocação, ainda participando da rodada.
CancelGame/StartGame exigem autor Playing. LeaveGame do último jogador em lobby
produz lobby vazio, não Finished. Nenhum desses comportamentos será alterado.
Base verificada: go test -race ./... passou nesta análise; árvore inicialmente limpa.

## Arquivos analisados
- docs/v2-audit.md, docs/v2-rules.md
- Todos os arquivos Go de internal/uno, incluindo testes e exemplo
- .agent/context.md, .agent/memory/memory.md, .agent/decisions.md
- Commits d7af8f8 e 680d42b (escopo e histórico)

## Premissas propostas, sujeitas à aprovação deste plano
Foram solicitadas preferências ao usuário; ainda sem resposta no momento do registro.
1. Criador entra atomicamente no lobby e assume sua gestão.
2. Somente responsável ativo inicia/cancela. Ao sair/obter colocação, responsabilidade
   passa ao jogador atual; em lobby, ao primeiro ID restante em Order.
3. Guardar últimos 100 resultados públicos encerrados, configurável por construtor.
   Não guardar mãos/runtime encerrados. Limite zero desabilita retenção; negativo é erro.
4. Regras são explícitas no CreateRequest; zero de uno.Rules significa Classic como
   na M1. M3 poderá fornecer uno.BotRules para preservar a dinâmica escolhida.

## Ownership e ausência de Repository
Service exposto; manager e managedGame privados no mesmo package. Não haverá API
pública que devolva *uno.Game, uno.State, ponteiros internos ou callbacks com engine.

Service possui manager. Manager possui managedGame por GameID; cada managedGame
possui mutex privado, uma *uno.Game, metadata de chat/criador/responsável e marcador
de fechamento. A engine é a única fonte do estado ativo. Snapshot é cópia transitória
para autorização, projeções e comparação de participantes; não é armazenado em paralelo.

Não criar MemoryRepository/internal/storage nesta milestone: duplicaria engine e
snapshot sem oferecer persistência. Índices e resumos são projeções reconstruíveis,
jamais uma segunda fonte para autorizar jogadas. Encerramento substitui runtime por
view pública final imutável; não mantém snapshot integral.

Recovery futuro: carregar envelope confiável (uno.State + metadata da aplicação),
validar/Restore antes de disponibilizar serviço, reconstruir índices de partidas
não encerradas e participantes ativos, rejeitar IDs/chats ativos duplicados. I/O
ocorrerá fora dos locks. Contrato de save durável/CAS será decidido com backend real;
a M2 não implementa Restore público, checkpoints, Save/Get fictícios ou export de mãos.

## API concreta

```go
type ChatID int64

type Actor struct {
    PlayerID uno.PlayerID
    ChatID   ChatID // zero somente para consulta privada/inline sem contexto de grupo
}

type CreateRequest struct {
    ChatName string
    Rules    uno.Rules
}

type Outcome struct {
    View   PublicGameView
    Events []uno.Event
}

func NewService(options ...Option) (*Service, error)
func WithHistoryLimit(limit int) Option

func (s *Service) Create(ctx context.Context, actor Actor, req CreateRequest) (Outcome, error)
func (s *Service) Apply(ctx context.Context, actor Actor, id uno.GameID, action uno.Action) (Outcome, error)
func (s *Service) PublicView(ctx context.Context, id uno.GameID) (PublicGameView, error)
func (s *Service) PlayerView(ctx context.Context, actor Actor, id uno.GameID) (PlayerGameView, error)
func (s *Service) FindChatGame(ctx context.Context, chat ChatID) (GameSummary, error)
func (s *Service) FindPlayerGames(ctx context.Context, actor Actor) ([]GameSummary, error)
```

Apply atende Join/Leave/Start/Play/Draw/Pass/ChooseColor/Cancel da engine, sem oito
wrappers redundantes. Action.PlayerID deve coincidir com Actor.PlayerID; nunca
reescrever silenciosamente um autor divergente. Start exige DealerID explícito.
Create gera GameID opaco aleatório de 128 bits codificado em hex (crypto/rand),
fora dos locks, e aplica Join do criador (revision 0 -> 1) antes da publicação.
Colisão detectada retorna erro sem substituir jogo; não reutilizar GameIDs.

Actor é identidade/contexto autenticado pelo adapter confiável, não payload do
jogador. A M2 não autentica credenciais nem consulta membros/admins do Telegram.
Create/Join/Leave/Start/Cancel exigem ChatID não zero e correspondente ao jogo.
Play/Draw/Pass/ChooseColor aceitam ChatID zero para o futuro inline; se fornecido,
deve corresponder. A engine valida participação/turno/posse/revision/regras.

PlayerView não recebe targetPlayerID: só retorna mão de Actor.PlayerID, participante
ativo daquele jogo. Mesmo responsável não vê mão alheia. FindPlayerGames lista
apenas jogos do próprio Actor.PlayerID; ChatID, quando fornecido, não muda a seleção
multigrupo. PublicView/FindChatGame fornecem apenas dados públicos a callers internos
confiáveis; a entrega a um chat concreto será responsabilidade do adapter.

Erros de aplicação: ErrInvalidArgument, ErrForbidden, ErrGameNotFound,
ErrNoActiveGame, ErrChatOccupied, ErrGameClosed, ErrNotParticipant e ErrIDConflict.
Preservar erros da engine para errors.Is. Não mascarar revision stale com retry
ou atualização automática. Erros retornam Outcome vazio.

## Views e privacidade

```go
type PublicPlayer struct {
    ID        uno.PlayerID
    Status    uno.PlayerStatus
    CardCount int
    Active    bool
}

type CloseReason string // completed, departure, cancelled, empty_lobby

type PublicGameView struct {
    GameID          uno.GameID
    ChatID          ChatID
    ChatName        string
    CreatorID       uno.PlayerID
    OwnerID         uno.PlayerID
    Revision        uint64
    Phase           uno.Phase
    Rules           uno.Rules
    CurrentTurn     uno.PlayerID
    Direction       int
    ActiveColor     uno.Color
    TopCard         *uno.Card // nil no lobby; cópia própria
    ColorChooserID  uno.PlayerID
    Players         []PublicPlayer // todos registrados, em ordem de registro
    Order           []uno.PlayerID // rotação atual da engine
    Placements      []uno.Placement
    Closed          bool
    CloseReason     CloseReason
}

type CardView struct {
    Card     uno.Card
    Playable bool
}

type PlayerGameView struct {
    Public      PublicGameView
    PlayerID    uno.PlayerID
    Hand        []CardView
    DrawnCardID uno.CardID // somente do próprio jogador, se aplicável
}

type GameSummary struct {
    GameID   uno.GameID
    ChatID   ChatID
    ChatName string
    Revision uint64
    Phase    uno.Phase
}
```

Playable calculado por Game.CanPlay sob o mesmo lock da Snapshot; nenhum matching
é reimplementado no serviço. Hand mantém ordem da engine; ordenação/filtros ficam
M4. Não projetar State, Player completo, Cards catálogo, DrawPile, DiscardPile ou
Pending completo. Copiar slices/ponteiros na construção e na devolução de arquivos
públicos retidos. Events são copiados; CardsDrawn continua apenas quantidade.
As views pública e privada de uma consulta/Outcome compartilham a mesma revision.

## Protocolo de locking e publicação

Estruturas privadas:
- indexMu sync.RWMutex: byID (referência privada + GameSummary), byChat e byPlayer.
- byChat: ChatID -> GameID apenas aberto; byPlayer: PlayerID -> conjunto de GameID ativos.
- entry.mu sync.Mutex: engine, metadata mutável e resultado final da partida.
- FIFO global de GameIDs encerrados, limitada por HistoryLimit.

1. Buscar referência por GameID sob indexMu.RLock; liberar antes de esperar entry.mu.
2. Obter entry.mu; conferir fechamento, contexto, autorização e revision.
3. Executar Game.Apply, Snapshot e projeção local sem indexMu. Não acessar outra engine.
4. Em sucesso, mantendo entry.mu, obter indexMu.Lock brevemente e publicar: diff dos
   participantes, summary/revision, eventual remoção de byChat/byPlayer e retenção final.
5. Liberar indexMu e só então entry.mu; retornar Outcome já copiado.

Nunca adquirir entry.mu segurando indexMu. Nunca manter locks de duas partidas.
Não chamar rede, disco, logger externo, callback de consumidor ou factory sob locks.
Factories/RNG de testes serão dependências privadas; shuffler da engine deve
continuar local, sem I/O, como na M1.

Apply pode ter atualizado runtime antes do passo 4, mas esse estado permanece
inacessível sob entry.mu. PublicView/PlayerView aguardam publicação completa.
FindChatGame/FindPlayerGames leem somente resumos publicados sob indexMu: podem
observar antes ou depois da operação concorrente, nunca dados parcialmente publicados.
FindPlayerGames retorna lista coerente num instante, ordenada por ChatID/GameID;
não promete que ela continuará atual até a próxima chamada. Revision trata essa janela.

Create constrói engine e autojoin fora do lock global; check definitivo de chat
ocupado e inserção em todos os índices ocorrem numa única seção crítica. Dois
candidatos concorrentes produzem um sucesso e um ErrChatOccupied; candidato perdedor
nunca é publicado. Nenhuma operação lenta de engine ocorre sob indexMu.

Contexto: verificar ctx.Err antes do trabalho e após adquirir lock, antes de Apply.
Mutex padrão não é interrompível por contexto; checar novamente ao adquiri-lo.
Após ação aceita, concluir publicação e retornar sucesso, mesmo se contexto for
cancelado nesse intervalo; não deixar engine/índices divergentes nem fingir rollback.

## Lifecycle e cleanup

| Evento | Índices/comportamento |
|---|---|
| Create | Criador já registrado/ativo; chat reservado; revision 1. |
| Join | Publicar somente se engine aceitar; adicionar jogo ao conjunto do autor. |
| Start | Manter membros; publicar fase/revision atualizadas. |
| Leave | Remover somente vínculo daquele jogo; preservar outros chats do jogador. |
| Colocação | WentOut sai de byPlayer, permanece em Public.Players/Placements. |
| Último Wild pendente | Continua ativo para escolher cor, mesmo com mão vazia. |
| Finished/Cancel | Remover chat e todos os vínculos de jogador; OwnerID zero. |
| Último sai do lobby | Fechar sessão da aplicação: Closed=true, empty_lobby; Phase permanece Lobby. |

Ativo = sessão aberta && State.Phase != Finished && Player.Status == Playing.
Não inferir só por quantidade de cartas nem só por status Playing.

Lobby vazio não fabrica ação/evento GameFinished nem altera regras da engine. O
Outcome traz PlayerLeft e Closed/empty_lobby; M3 terá um sinal explícito de cleanup.

No fechamento, produzir view final, descartar engine e reter só essa projeção pública
por GameID. Limite padrão 100 resumos; FIFO por ordem de encerramento, sem timers.
Evicção remove apenas byID de registro já encerrado, sob indexMu, sem lock de outra
partida. Um caller que já obteve a referência lê resultado final imutável/ErrGameClosed.
Consultas posteriores à evicção retornam ErrGameNotFound. Sem retenção de mãos.
Histórico não é durável; no restart é perdido. PlayerView encerrada retorna
ErrGameClosed e não fornece nem a própria mão residual. PublicView continua válida
enquanto resumo retido. Uma nova partida pode ocupar imediatamente o mesmo chat.

## Estratégia de implementação
Manter um único package interno, Service exportado e manager/entries privados.
Separar autorização, ownership/índices e projeção por arquivos. Não criar interfaces
públicas de Manager/Repository ou dependências externas. Helpers de autorização
chamados dentro da operação serializada são puros e privados, evitando TOCTOU.
Factories para cenários determinísticos são privadas e nunca expostas ao adapter.

## Passos detalhados
1. Tipos/API/erros, constructor e projeções públicas/privadas sem snapshots expostos.
2. Ownership privado, IDs e Create atômico com índices e autojoin.
3. Apply serializado com autorização, atualização de índices, owner e Outcome coerente.
4. Lookups por chat/jogador, lifecycle, resumo final limitado e lobby vazio.
5. Testes reais de concorrência/privacidade e documentação da API para M3.
6. Atualizar documentos de memória após implementação, validar e revisar diff.

## Arquivos que poderão ser modificados
Criar em internal/game: service.go, manager.go, views.go, errors.go, doc.go,
service_test.go, manager_test.go, views_test.go, concurrency_test.go.
Criar docs/v2-application.md. Atualizar trecho de roadmap de docs/v2-rules.md para
registrar decisão de adiar Repository, além de .agent/context.md,
.agent/memory/memory.md e .agent/decisions.md. Não modificar internal/uno, V1,
go.mod/go.sum, Docker ou configuração.

## Mudanças necessárias na M1
Nenhuma identificada. Exigência de participante ativo para gestão é atendida por
autojoin e transferência de responsável; lobby vazio é fechamento da sessão, não
mudança de regra. Erros, revision e snapshots existentes são suficientes.

## Riscos
- Actor precisa vir de adapter confiável; serviço não prova identidade de Telegram.
- Lista de jogos é uma fotografia, não autorização permanente de jogada inline.
- Histórico público é limitado e efêmero; não prometer recovery real nesta entrega.
- Revisões simultâneas tornam ações stale, inclusive joins; não fazer retry silencioso.
- Evitar inversão de locks e I/O externo em helpers executados sob entry.mu.

## Impactos esperados
API consumível pela M3 sem mãos alheias, sem current game por usuário e sem
compartilhamento mutável. Uma partida lenta não trava execução de outra.
Mudanças de índice e views tornam-se observáveis no mesmo commit da aplicação.
Nenhuma alteração da experiência V1 em execução nesta milestone.

## Compatibilidade
Go e biblioteca padrão existentes. Linux local; código portátil macOS/Windows.
Docker e CI/CD existentes permanecem intactos; sem serviços necessários para testes.

## Como testar

### Cenários funcionais
- Create, join, start, leave, cancel, lookups e erros de argumentos/contexto.
- Um usuário em vários chats, sem ponteiro current; ordenação determinística da lista.
- Revision stale não muda engine/view/índices e não produz eventos.
- Owner transferido em saída/colocação, não em erro; não-owner não inicia/cancela.
- Finished com jogadores ainda Playing remove todos dos índices ativos.
- Wild final pendente mantém autor ativo até escolha; saída preserva regra da M1.
- Lobby vazio libera chat sem evento de vitória/término fictício da engine.
- Novo jogo no mesmo chat após fim; ação velha não toca jogo novo.
- Retenção final 0/1/100, FIFO, cópias independentes, evicção sem remover jogo aberto.
- Falha de criação/factory ou ação não deixa índices parciais.

### Concorrência real
Barreiras de canais/WaitGroup, sem time.Sleep ou assertions de latência:
- Criações simultâneas no mesmo chat: um sucesso.
- Ações não terminais com mesma revision: um sucesso, demais ErrStaleRevision.
- Dois jogos e mesmo usuário atualizando índices simultaneamente: sem lost update.
- Join/leave/colocação/cancel/fim concorrendo com lookups/views: projeções coerentes.
- Bloquear shuffler de teste de A por canal e completar operação em B antes de
  desbloquear A: prova de ausência de execução da engine sob lock global.
- Corrida entre encerramento e Create no mesmo chat: nunca dois jogos abertos.
- Contexto cancelado antes de Apply: nenhum efeito; cancelamento após commit não
  interrompe atualização de índices. Timeouts só como watchdog contra deadlock.

### Privacidade e cópias
- Estrutura/JSON de PublicGameView não contém Hand/State/pilhas/cartas privadas.
- PlayerView do autor A só contém mão de A; sem parâmetro para pedir mão de B.
- Actor A com Action.PlayerID B é recusado; responsável não recebe privilégio de mão.
- Não participante/inativo/encerrado não acessa view privada.
- CardsDrawn sem CardID; nenhuma API exportada retorna uno.State ou *uno.Game.
- Mutar Hand, Players, Order, Placements, TopCard, Events e listas retornadas não
  altera consultas subsequentes, inclusive concorrentes e resumos encerrados.

### Build
```bash
go build ./...
```
### Testes
```bash
gofmt -w internal/game
go test ./...
go test -race ./...
go vet ./...
git diff --check
```
### Execução
M2 via testes/exemplo local, sem Telegram/token/banco. V1 segue sem alteração.

## Rollback
Reverter somente commits da M2, preservando engine/M1, V1 e histórico dos planos.

## Observações
Este arquivo contém planejamento apenas. Não é aprovação de implementação.
Após aprovação explícita, mover pending -> approved; implementar incrementalmente;
registrar resultados e mover para done. Não criar commits de implementação agora.

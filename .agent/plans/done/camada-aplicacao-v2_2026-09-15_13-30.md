# Plano: camada-aplicacao-v2 — Milestone 2 (revisão 2026-09-15)

Substitui, sem apagar ou sobrescrever, camada-aplicacao-v2_2026-09-14_23-39.md.
Esta é a versão vigente para aprovação de implementação.

## Pedido do usuário
Planejar a aplicação entre o futuro adapter Telegram e internal/uno, com ownership
único, concorrência por partida, índices consistentes e views privadas/públicas.
Não implementar antes da aprovação. Não modificar V1 nem redesenhar a M1.
Separar completamente responsabilidade administrativa e participação, conforme
instrução do usuário de 2026-09-15; responsável pode iniciar/cancelar sem jogar.

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
produz lobby vazio, não Finished. A exigência genérica de autor Playing para
StartGame/CancelGame conflita concretamente com a separação solicitada: será
removida apenas dessas duas ações, sem alterar regras de cartas ou dealer.
Lobby vazio continuará aberto; o responsável existe independentemente dos jogadores.
Base verificada: go test -race ./... passou nesta análise; árvore inicialmente limpa.

## Arquivos analisados
- docs/v2-audit.md, docs/v2-rules.md
- Todos os arquivos Go de internal/uno, incluindo testes e exemplo
- .agent/context.md, .agent/memory/memory.md, .agent/decisions.md
- Commits d7af8f8 e 680d42b (escopo e histórico)

## Decisões confirmadas pelo usuário
1. Criador se torna responsável sem entrar como jogador. Create publica lobby
   vazio na revision 0, sem evento PlayerJoined e sem vínculo em byPlayer.
2. Responsável pode iniciar/cancelar sem jogar. Participação exige Join explícito;
   responsabilidade, por si só, nunca autoriza mão privada nem jogadas de cartas.
3. Ao sair/obter colocação, responsável que participava transfere gestão ao jogador
   atual; em lobby, ao primeiro ID restante em Order. A transferência não altera
   mãos, participantes, ordem ou revision adicional da engine. Responsável que
   nunca entrou conserva gestão; não transferir só por não estar em Order.
4. Se o responsável participante sair do lobby e não houver sucessor, conserva
   gestão como observador; lobby permanece aberto e pode ser cancelado por ele.
   Se a rodada terminar, fechar normalmente e limpar responsável conforme lifecycle.
5. Guardar últimos 100 resultados públicos encerrados, configurável por construtor.
   Não guardar mãos/runtime encerrados. Limite zero desabilita retenção; negativo é erro.
6. Regras são explícitas no CreateRequest; zero de uno.Rules significa Classic como
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
reescrever silenciosamente um autor divergente. PlayerID identifica o solicitante
real; nas ações administrativas ele não implica participação.
Start é autorizado pelo OwnerID sob entry.mu, independentemente de Players/Order.
Se DealerID vier zero, o Service seleciona o primeiro participante de Order; se
vier explícito, deve pertencer a Order. Com menos de dois participantes, recusar
com ErrNotEnoughPlayers sem efeitos. O dealer enviado à engine é sempre participante
ativo; não se substitui Action.PlayerID pelo dealer. A engine também valida dealer
ativo, fase, mínimo de jogadores e revision.
Cancel é autorizado pelo OwnerID e enviado à engine com o autor verdadeiro, inclusive
em lobby sem jogadores. Não há cancelamento sintético na aplicação nem autor fictício.
Create gera GameID opaco aleatório de 128 bits codificado em hex (crypto/rand),
fora dos locks, e publica NewGame na revision 0, sem executar Join.
Colisão detectada retorna erro sem substituir jogo; não reutilizar GameIDs.

Actor é identidade/contexto autenticado pelo adapter confiável, não payload do
jogador. A M2 não autentica credenciais nem consulta membros/admins do Telegram.
Create/Join/Leave/Start/Cancel exigem ChatID não zero e correspondente ao jogo.
Play/Draw/Pass/ChooseColor aceitam ChatID zero para o futuro inline; se fornecido,
deve corresponder. A engine valida participação nas ações de jogador e valida
turno/posse/revision/regras. Start/Cancel não exigem participação do solicitante.
Na M2, gestão é autorizada somente pelo OwnerID; não aceitar um booleano IsAdmin
fornecido pelo cliente. Permissões administrativas externas poderão ser resolvidas
pelo adapter/política confiável na milestone de administração, fora de locks e sem
conceder participação automaticamente. Esta entrega não implementa consulta de admins.

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

type CloseReason string // completed, departure, cancelled

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

Create constrói engine e metadata do responsável fora do lock global; check definitivo de chat
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
| Create | Responsável definido; zero jogadores; chat reservado; revision 0; byPlayer vazio. |
| Join | Publicar somente se engine aceitar; adicionar jogo ao conjunto do autor. |
| Start | Manter membros; publicar fase/revision atualizadas. |
| Leave | Remover somente vínculo daquele jogo; preservar outros chats do jogador. |
| Colocação | WentOut sai de byPlayer, permanece em Public.Players/Placements. |
| Último Wild pendente | Continua ativo para escolher cor, mesmo com mão vazia. |
| Finished/Cancel | Remover chat e todos os vínculos de jogador; OwnerID zero. |
| Último sai do lobby | Manter lobby aberto e chat reservado; nenhum vínculo em byPlayer; gestão permanece independente. |

Ativo = sessão aberta && State.Phase != Finished && Player.Status == Playing.
Não inferir só por quantidade de cartas nem só por status Playing.

Lobby vazio não é condição de fechamento, nem na criação nem após saída dos jogadores.
O Outcome da saída traz PlayerLeft, Phase=Lobby e Closed=false. Cancelamento explícito
pelo responsável usa CancelGame na engine e produz revision/evento/Finished normais.
Responsável observador localiza a sessão por chat/GameID; FindPlayerGames continua
sendo exclusivamente índice de participação, não índice de gestão. Não criar índice
administrativo adicional nem misturar owner com Public.Players.
Transferência de OwnerID é consequência da saída/colocação aceita do próprio owner,
com sucessor quando houver, e é publicada junto aos índices sob o mesmo protocolo.
Não implementar uma ação nova de transferência manual nesta milestone.

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
2. Ajuste mínimo/testes da M1 para Start/Cancel por solicitante não participante.
3. Ownership privado, IDs e Create atômico com metadata de owner, sem autojoin.
4. Apply serializado com autorização, atualização de índices, owner e Outcome coerente.
5. Lookups por chat/jogador, lifecycle, resumo final limitado e lobbies vazios abertos.
6. Testes reais de concorrência/privacidade e documentação da API para M3.
7. Atualizar documentos de memória após implementação, validar e revisar diff.

## Arquivos que poderão ser modificados
Criar em internal/game: service.go, manager.go, views.go, errors.go, doc.go,
service_test.go, manager_test.go, views_test.go, concurrency_test.go.
Criar docs/v2-application.md. Atualizar trecho de roadmap de docs/v2-rules.md para
registrar decisão de adiar Repository e a autorização administrativa separada, além
de .agent/context.md, .agent/memory/memory.md e .agent/decisions.md.
Ajustar somente internal/uno/game.go (guarda de participação), action.go (contrato)
e game_test.go (regressões administrativas). Não modificar V1, go.mod/go.sum,
Docker ou configuração. Preservar a auditoria histórica da M1 como registro do estado anterior.

## Mudanças necessárias na M1 — motivo concreto e limite
Game.Apply atualmente exige Status=Playing para toda ação exceto JoinGame, antes
do switch. Isso impede início/cancelamento pelo responsável observador, mesmo
quando a aplicação autorizou corretamente. O usuário proibiu inscrição automática
e autor fictício; adaptar somente a aplicação não resolve esse contrato.

Exceção explícita na guarda de participação: JoinGame, StartGame e CancelGame.
- StartGame e CancelGame conservam o Action.PlayerID real e positivo do solicitante.
- A aplicação autoriza OwnerID/contexto antes de chamar a engine.
- Start mantém validação existente de fase, mínimo de jogadores, DealerID ativo,
  distribuição e revision. Nenhuma identidade do responsável é inserida em Players.
- Cancel mantém a transição normal para Finished, incremento de revision e evento,
  inclusive em lobby vazio. Engine não conhece OwnerID/admin/chat nem autenticação.
- Leave/Play/Draw/Pass/ChooseColor continuam exigindo participante Playing; acesso
  privado da aplicação continua exigindo participação ativa.
- Não renomear Action.PlayerID nem criar novas hierarquias de ações nesta milestone;
  documentar que ele identifica solicitante, e não prova inscrição/participação.
- Não modificar State, regras de cartas, pontuação/colocações ou interfaces existentes.

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
- Owner observador inicia com dois participantes e não é inscrito, recebe mão ou
  entra em byPlayer; pode cancelar lobby vazio e rodada sem participar.
- Serviço fornece dealer ativo; início sem mínimo ou com dealer inválido falha sem efeitos.
- Não-owner participante não inicia/cancela. Autor real preservado até Game.Apply;
  teste com factory/shuffler local e estado sem owner, sem fabricar identidade.
- Owner transferido em saída/colocação, não em erro ou por mera ausência em Order;
  transferência só muda metadata e não inscreve/remove jogadores.
- Sem sucessor no lobby vazio, conservar gestão do owner observador.
- Finished com jogadores ainda Playing remove todos dos índices ativos.
- Wild final pendente mantém autor ativo até escolha; saída preserva regra da M1.
- Create vazio reserva chat na revision 0, não produz PlayerJoined e não cria vínculo
  do owner em byPlayer. Rejeitar segunda criação nesse chat.
- Lobby fica aberto após última saída; cancelamento explícito libera chat e produz
  Finished/evento/revision via engine. Não fabricar empty_lobby nem fechamento automático.
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
- Não participante/inativo/encerrado não acessa view privada, inclusive owner observador.
- CardsDrawn sem CardID; nenhuma API exportada retorna uno.State ou *uno.Game.
- Mutar Hand, Players, Order, Placements, TopCard, Events e listas retornadas não
  altera consultas subsequentes, inclusive concorrentes e resumos encerrados.

### Regressões da engine
- Start com solicitante externo e dealer ativo: aceita, sem inscrever solicitante.
- Start com dealer externo/ausente ou menos de dois participantes: recusa atomicamente.
- Cancel com solicitante externo positivo: aceita em lobby vazio e jogo iniciado.
- Autor zero/negativo, revision stale e ação após Finished continuam recusados.
- Solicitante externo ainda não pode Leave/Play/Draw/Pass/ChooseColor.
- Todos os testes de regras, invariantes e partidas completas da M1 permanecem passando.

### Build
```bash
go build ./...
```
### Testes
```bash
gofmt -w internal/game internal/uno/game.go internal/uno/action.go internal/uno/game_test.go
go test ./...
go test -race ./...
go vet ./...
git diff --check
```
### Execução
M2 via testes/exemplo local, sem Telegram/token/banco. V1 segue sem alteração.

## Rollback
Reverter somente commits da M2, inclusive ajuste administrativo limitado da engine,
restaurando comportamento da M1; preservar V1 e histórico dos planos.

## Observações
Este arquivo contém planejamento apenas. Não é aprovação de implementação.
Após aprovação explícita, mover pending -> approved; implementar incrementalmente;
registrar resultados e mover para done. Não criar commits de implementação agora.

## Aprovação e conclusão — 2026-09-15

Aprovação explícita recebida: “Aprovado. Implemente a Milestone 2 conforme este
plano revisado.” Plano movido de pending para approved antes da implementação;
agora concluído e movido para done. A versão anterior permanece preservada.

### Entrega

- Service, manager privado, índices, autorização e views em internal/game.
- Owner separado de participação; dealer ativo sem falsificar solicitante.
- Mutex por partida; publicação atômica das projeções; testes reais de concorrência.
- Lifecycle/transferência, descarte de runtime e histórico público FIFO configurável.
- Documentação API/locking/recovery futuro, memória e decisões atualizadas.
- Ajuste M1 somente na guarda Start/Cancel, contrato Action e regressões.
- V1, dependências, Docker e configuração intactos. Nenhuma integração M3.

### Validação executada

- gofmt aplicado; gofmt -l sem resultados nos arquivos envolvidos.
- go build ./...: passou.
- go test ./...: passou, incluindo regressões V1/M1.
- go test -race ./...: passou.
- go vet ./...: passou.
- git diff --check e git diff --cached --check: passaram.
- Cobertura por statements: internal/game 95,1%; internal/uno 91,8%; agregada dos
  dois packages 92,9%. Profile gerado fora do repositório em
  /tmp/unobotgo-v2-coverage.out.
- Diff completo revisado: ownership, ordem de locks, projeções/privacidade,
  lifecycle/evicção, autorização, testes e documentação; escopo conferido.

### Commits de implementação

- 47cc695 — fix(uno): allow authorized observer requests for start and cancel
- e3645ca — feat(game): add concurrent application service and private player views
- Registro de conclusão/memória/planos em commit documental subsequente.

### Divergências

Nenhuma divergência arquitetural ou expansão do escopo aprovado. MemoryRepository
foi omitido conforme decisão do plano revisado. Durante testes, fixtures foram
corrigidas para respeitar a proibição de reentrada e a passagem automática após
compra não jogável da M1; essas regras da engine não foram alteradas.

### Limites mantidos

Sem armazenamento durável/recovery; restart perde sessões e resumos. Actor deve
vir de adapter confiável; autenticação Telegram e tokens pertencem às próximas
milestones. Cancelamento de contexto não interrompe espera por mutex nem desfaz
ação aceita. Testes/build locais em Linux; outros sistemas não foram executados.

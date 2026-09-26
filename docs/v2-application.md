# UnoBotGO V2 — Milestone 2

## Atualização — controle de entradas (2026-09-26, somente dev)

`managedGame.locked` é metadata de sessão protegida por `entry.mu`, exposta como
`Locked` nas views e no resumo. `Service.SetLocked` autoriza exclusivamente o
owner no chat correto, mesmo quando ele não é jogador. Começa aberto e funciona
no lobby e durante o jogo; chamadas repetidas são idempotentes. Administrador de
chat não recebe essa permissão apenas por ser administrador.

`Service.Apply(JoinGame)` verifica o lock sob o mesmo mutex da mutação da engine:
retorna `ErrRoomLocked` sem modificar estado quando a sala está trancada.
SetLocked não altera revision da engine, mãos, turno ou prazo; jogadores atuais
continuam jogando. A transferência existente de owner transfere essa permissão.

Partidas encerradas recusam alterações. O resumo final conserva a informação;
reset descarta a sessão e toda partida nova começa aberta. Não existe restauração
durável de sessão atualmente: snapshots de `uno.State` não contêm metadata de
admissão e não são snapshots completos de `internal/game`.


## Troca de mãos — 2026-09-25

`ChoosePlayer` é autorizada como ação inline, vinculada ao `Actor.PlayerID` real,
à partida e à revisão. A engine valida turno, fase e alvo ativo; o serviço aplica
a troca sob o mutex da partida. `PublicGameView.PlayerChooserID` identifica quem
escolhe, enquanto `Players` fornece os alvos ativos e suas contagens públicas.
Cada `PlayerView` passa a refletir somente a nova mão do próprio solicitante.
Nenhum ID de carta trocada é publicado em eventos ou na view pública.

`ChoosingPlayer` não é elegível para timeout, assim como a escolha de cor.
Após confirmar a troca, o próximo turno recebe um prazo novo. Seleções antigas
são recusadas pela revisão, mesmo quando a carta ainda existe na partida.


> Atualização de 2026-09-23: o contrato de timeout e encerramento vigente está
> descrito abaixo. As seções da M2 preservam o contexto histórico.

## Timeout e encerramento — milestone corretiva

A engine continua responsável por `GameFinished`. O serviço publica a view final,
remove os índices ativos, descarta o runtime privado e zera `turnStarted` na mesma
operação protegida pelo mutex da partida. Não há timer individual nem turno do
último jogador após encerramento.

O scheduler usa duas operações:

- `ExpiredTurns(ctx, timeout) []ExpiredTurn`: descoberta somente de leitura;
  candidato contém `GameID`, `ChatID`, `PlayerID` e `Revision`.
- `AutoSkipTurn(ctx, candidate, timeout) (Outcome, bool)`: revalida candidato,
  fase, prazo e encerramento sob o mutex, antes de aplicar `SkipTurn`.
  `false` significa que não houve ação nem deve haver mensagem.

Candidatos são dados internos do scheduler confiável, nunca input de jogadores.
O adapter enfileira execução e envio da mensagem na mesma tarefa do chat utilizada
pelas jogadas. Não deve aplicar um timeout fora da fila e enfileirar somente sua
notificação: esse padrão permitiria anunciar um turno antigo após a vitória.
Candidatos duplicados, antigos, cancelados, de jogos removidos ou de um jogo
anterior no mesmo chat não alteram o estado. Saturação da fila não aplica a ação.

`AutoSkipExpired(ctx, timeout)` permanece como wrapper síncrono das duas operações
para consumidores sem fila. O bot usa as operações separadas. Leituras de runtime,
`final` e prazo usam exclusivamente o mutex da partida; `indexMu` só protege os
índices e a cópia de referências.

Regras de cartas preservadas: o Clássico do Telegram usa `BotRules` (placements e
stacking de +2), enquanto `ClassicRules` da engine usa primeiro vencedor. Wild/+4
aguardam escolha de cor antes da colocação. Penalidades imediatas continuam sendo
aplicadas antes do término. Com stacking terminal, não há nova compra automática,
chance de rebater ou turno adicional; o contador final é preservado como antes.


`internal/game` fornece a camada de aplicação entre adapters futuros e
`internal/uno`. Não inicia Telegram, não substitui o executável V1 e não depende
de banco, tokens inline, ranking, Match ou timers.

## API

Construir uma instância de `game.Service` com `game.NewService(options...)` e
compartilhá-la entre handlers. O serviço é seguro para chamadas concorrentes;
não copiar a instância nem utilizar seu valor zero.

| Método | Contrato |
|---|---|
| `Create(ctx, Actor, CreateRequest)` | Cria lobby vazio; criador torna-se responsável, sem inscrição. |
| `Apply(ctx, Actor, GameID, uno.Action)` | Autoriza e serializa ação; retorna `Outcome{View, Events}`. |
| `PublicView(ctx, GameID)` | Estado público atual ou resumo encerrado retido. |
| `PlayerView(ctx, Actor, GameID)` | Apenas mão do próprio participante ativo autenticado. |
| `FindChatGame(ctx, ChatID)` | Resumo da única sessão aberta no chat. |
| `FindPlayerGames(ctx, Actor)` | Sessões de participação ativa do autor, ordenadas por ChatID/GameID. |
| `ResetChat(ctx, Actor)` | Remove sessão ativa e histórico do chat; exige responsável da partida ou administrador autenticado pelo adapter. |

`CreateRequest` contém `ChatName` e `uno.Rules`; regras zero equivalem a Classic.
`WithHistoryLimit(n)` configura retenção de encerrados: padrão 100; zero desativa;
valor negativo é erro. IDs são 128 bits aleatórios de `crypto/rand`, codificados
em hexadecimal. O serviço não aceita IDs de criação fornecidos pelo consumidor;
colisão com um registro existente é recusada, sem substituição.

Exemplo de fluxo (erros devem ser tratados pelo adapter):

```go
service, err := game.NewService(game.WithHistoryLimit(100))
if err != nil { return err }
owner := game.Actor{PlayerID: 99, ChatID: -100}
lobby, err := service.Create(ctx, owner, game.CreateRequest{Rules: uno.ClassicRules()})
if err != nil { return err }
current := lobby
for _, id := range []uno.PlayerID{1, 2} {
    current, err = service.Apply(ctx, game.Actor{PlayerID: id, ChatID: -100},
        lobby.View.GameID, uno.Action{
            Type: uno.JoinGame, PlayerID: id, Revision: current.View.Revision,
        })
    if err != nil { return err }
}
started, err := service.Apply(ctx, owner, lobby.View.GameID, uno.Action{
    Type: uno.StartGame, PlayerID: owner.PlayerID, Revision: current.View.Revision,
})
if err != nil { return err }
// started.View é pública. O responsável 99 continua sem participar.
_ = started
```

## Responsável, solicitante e participante

`Actor` é identidade/contexto vindo de um adapter **confiável**. A camada não
verifica credenciais Telegram. Nunca construir Actor a partir de um ID alegado no
payload cliente; o adapter deverá autenticá-lo. `Action.PlayerID` deve coincidir
com o solicitante real; divergência retorna `ErrForbidden`. `Actor.ChatAdmin`
também é uma afirmação confiável do adapter, preenchida somente após consultar a
função administrativa do usuário no chat.

- Create exige PlayerID positivo e ChatID não zero.
- Join/Leave exigem contexto do chat correspondente.
- Start/Cancel exigem contexto do chat e `Actor.PlayerID == OwnerID`.
- Play/Draw/Pass/ChooseColor/ChoosePlayer/CallBluff aceitam ChatID zero para o futuro inline; qualquer
  ChatID fornecido precisa corresponder ao jogo.
- ResetChat aceita o responsável da partida ou `ChatAdmin`; nenhum dos dois recebe
  acesso a mãos por causa dessa autorização.

O responsável pode iniciar/cancelar sem participar, inclusive cancelar lobby vazio.
Start escolhe `Order[0]` como dealer quando `DealerID` é zero. Dealer explícito
precisa ser ativo; mínimo de dois participantes permanece obrigatório. Não se
inscreve o owner nem se substitui o solicitante pelo dealer.

**Ajuste mínimo M1:** `Game.Apply` dispensa participação apenas para Join, Start e
Cancel; Start/Cancel conservam solicitante positivo, revision e validações de
fase/dealer/mínimo. Autorização administrativa pertence ao serviço. Nenhuma regra
de cartas, State ou assinatura da engine mudou.

Quando um owner participante sai ou obtém colocação, o sucessor é o jogador do
turno resultante; no lobby, o primeiro de Order. Sem sucessor em lobby vazio, o
responsável permanece como observador. Owner que já administra sem participar
não é transferido por mudanças de outros jogadores. Transferência não altera
participação e acompanha a mesma revision da ação, sem ação artificial adicional.

## Ownership, locking e atomicidade

O Service possui um manager privado. Cada `managedGame` possui exclusivamente a
instância runtime de `uno.Game`, metadata e mutex privado. Não há singleton ou
ponteiro de "current game" por usuário.

O manager mantém, sob `indexMu`, três projeções:

- GameID → referência privada + resumo publicado;
- ChatID → GameID aberto;
- PlayerID → conjunto de GameIDs com participação ativa.

Protocolo de Apply:

1. Localizar referência sob `indexMu.RLock`, soltando-o antes de esperar o mutex
   da partida. Nenhum caminho adquire mutex de partida segurando indexMu.
2. Sob mutex da partida, validar fechamento, autorização, contexto e revision.
3. Executar engine, snapshots transitórios e projeções sem lock global.
4. Ainda sob mutex da partida, adquirir indexMu por uma seção curta e publicar
   participantes, owner, resumo/revision, liberação do chat e eventual histórico.
5. Soltar indexMu, depois mutex da partida; retornar cópias independentes.

Readers de índices usam exclusivamente resumos publicados; readers de views
aguardam mutex da partida. Assim ninguém observa runtime novo com índices ainda
parciais. Lookups podem observar antes ou depois de Apply concorrente. Uma lista
é coerente num instante, mas pode ficar antiga antes da próxima chamada: a revision
continua obrigatória e não há retry silencioso.

Create constrói candidato fora dos locks e publica o lobby vazio numa única seção
crítica após verificar ocupação/colisão. Apenas um Create concorrente no mesmo
chat vence. Nenhuma operação de engine usa lock global; não há I/O externo nem
callbacks de consumidor sob locks. Shuffler da engine é local, sem I/O.

Contexto é verificado na entrada e após espera por lock, antes de mutar. Mutex
padrão não é interrompível: cancelamento não promete retorno imediato durante
espera. Após engine aceitar a ação, o serviço termina publicação e retorna sucesso
mesmo se ctx for cancelado nesse intervalo; não simula rollback de ação aceita.

`ResetChat` usa espera de lock sensível ao contexto. Quando autorizado, marca o
runtime removido como resetado, apaga a sessão ativa, todos os índices de jogadores
e todos os resumos históricos do chat em uma única seção protegida. Referências
antigas passam a retornar `ErrGameReset` e não podem publicar novamente nos
índices. Partidas de outros chats permanecem intactas. Para administradores, o
reset sem estado é idempotente; sem partida ativa, um usuário comum não possui
autoridade implícita para limpar o chat.

## Lifecycle e histórico

| Transição | Roteamento e retenção |
|---|---|
| Create | Chat reservado, revision 0, nenhum jogador inscrito. |
| Join | Autor entra em byPlayer somente após ação aceita. |
| Start | Mesmos participantes; fase/revision publicadas. |
| Leave | Remove somente vínculo daquele jogo; histórico de participante permanece. |
| Colocação | WentOut sai de roteamento, permanece em Players/Placements públicos. |
| Último Wild pendente | Mão zero ainda ativa para escolher cor; sem transferência prematura. |
| Lobby fica vazio | Continua aberto, gestão preservada; cancelamento é explícito. |
| Finish/Cancel | Remove chat e todos os vínculos ativos; OwnerID zero; runtime descartado. |

Ativo significa jogo aberto e `Player.Status == uno.Playing`; quantidade de cartas
sozinha não determina participação. Engine mantém registros de quem saiu e não
permite reentrada na mesma rodada. M2 não altera essas regras.

Somente a view pública final é retida; engine, mãos residuais, catálogo e pilhas
não ficam no arquivo. FIFO global por ordem de encerramento mantém os últimos N
resumos, sem atingir jogos abertos. `PublicView` funciona enquanto retido;
`PlayerView`/Apply retornam `ErrGameClosed`. Após evicção, novas consultas retornam
`ErrGameNotFound`. Referência já obtida por chamada concorrente continua segura e
pode retornar o resumo final, mesmo após evicção. Novo lobby pode ocupar o mesmo
chat imediatamente após fechamento; ação antiga não altera o novo lobby.

## Privacidade e preparação inline

`PublicGameView` traz metadata, fase, turno, cor, carta superior, ordem, colocações
e contagens/status. Não contém State, mãos, catálogo nem pilhas. `PlayerGameView`
contém essa projeção e somente a mão de Actor.PlayerID. Não existe parâmetro para
pedir mão alheia, nem exceção para owner. `CardView` usa identidade física e
aparência de `uno.Card`; `Playable` vem de `Game.CanPlay` sob o mesmo lock/revision.
`DrawnCardID` privado só aparece para o autor do turno quando aplicável.

Consultas não alteram engine/revision. Slices e ponteiros retornados são cópias,
inclusive arquivos públicos encerrados. Eventos são fatos públicos da engine;
`CardsDrawn` tem quantidade, nunca IDs das cartas compradas.

A M3 poderá consultar FindPlayerGames: zero → vazio, um → PlayerView, vários →
seleção explícita. Owner observador localiza gestão por chat, sem entrar no índice
de participação. Tokens/autenticação Telegram e renderer ainda não existem.

## Storage e recovery futuro

Não há MemoryRepository: duplicaria snapshots e runtime sem persistência real.
Manager é o único dono do runtime; snapshots privados são transitórios. Índices e
resumos são projeções, não fontes paralelas de autorização.

Recovery futuro poderá carregar envelope confiável com metadata da aplicação e
`uno.State`, validar via Restore e reconstruir índices antes de disponibilizar o
serviço, recusando chats/IDs duplicados. I/O deverá ocorrer fora dos locks; protocolo
de gravação durável/CAS será definido com backend real. M2 não expõe snapshots,
Save/Get artificiais ou recovery público. Reiniciar perde sessões e histórico.

## Erros e validação

Erros de aplicação: `ErrInvalidArgument`, `ErrForbidden`, `ErrGameNotFound`,
`ErrNoActiveGame`, `ErrChatOccupied`, `ErrGameClosed`, `ErrNotParticipant`,
`ErrIDConflict`, `ErrGameReset`. Erros de regras/revision são preservados para `errors.Is`.
Falhas de Apply retornam Outcome vazio e não alteram estado/índices.

```bash
go build ./...
go test ./...
go test -race ./...
go vet ./...
go test -coverprofile=/tmp/unobotgo-v2-coverage.out ./internal/game ./internal/uno
git diff --check
```

Testes usam posições validadas determinísticas, múltiplas goroutines, barreiras e
canais; timeouts somente detectam deadlock. Cobrem disputa de revision/chat,
partidas independentes, leitores durante colocações/fechamento, privacidade,
cópias, limites de histórico, cancelamento de contexto e falhas atômicas.

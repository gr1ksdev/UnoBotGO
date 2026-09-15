# UnoBotGO V2 — engine (Milestone 1)

A mão do jogador é privada e acessada digitando @usernamebot no campo de mensagem do Telegram.

Esta milestone entrega apenas `internal/uno`. O adapter Telegram ainda é o V1;
nenhum comando passa a usar a engine nova automaticamente. `go run .` continua
executando o V1, com seu token e PostgreSQL. A engine pode ser utilizada sem ambos.

## API e responsabilidades

```go
import "github.com/malbs/UnoGoBot/internal/uno"

g, err := uno.NewGame("round-1", uno.BotRules())
// Tratar err antes de continuar.
r, err := g.Apply(uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: 0})
// Tratar err; r.Revision passa a ser a versão para a próxima ação.
_ = r
```

- `NewGame(id, rules, options...)` cria um lobby com inventário Classic de 108 cartas.
- `Apply(Action) (Result, error)` é a única entrada para mutações.
- `StartGame` exige `DealerID` de um participante ativo e ao menos dois jogadores.
- `Snapshot()` fornece uma cópia profunda serializável; **contém mãos privadas**.
- `Restore(state, shuffler)` valida e copia um snapshot confiável do servidor.
- `CanPlay(playerID, cardID)` usa as mesmas regras de `PlayCard`, sem efeitos colaterais.
- `CanPlayDrawFour(hand, activeColor)` verifica isoladamente a restrição de cor.
- `WithDeck(cards)` injeta inventário e ordem; a primeira carta do slice é a próxima compra.
- `WithShuffler(func([]CardID))` permite testes determinísticos. Usar função vazia
  para preservar a ordem. O shuffler deve apenas permutar IDs, sem guardar o slice.

O estado guarda IDs físicos em mãos/pilhas, um catálogo de cartas, jogadores
registrados, ordem ativa, direção, cor ativa, carta comprada no turno, pendência de
cor, revision e colocações. Não há ponteiros de jogador em anel, mutex, timer,
configuração global, Telegram, SQL ou rede no estado.

O serviço futuro autenticará o autor e autorizará início/cancelamento/gestão.
A engine verifica participação, fase, turno, posse e regras. Não interpreta IDs
como credenciais. Snapshot nunca deve ser aceito de um jogador ou enviado inteiro
para um renderer público: a Milestone 2 construirá views específicas por usuário.

### Atomicidade e concorrência

Cada ação aceita incrementa revision uma vez, inclusive entrada/saída do lobby,
compra, passagem e escolha de cor. Revision diferente da atual é recusada. Uma
falha retorna `Result{}` e erro reconhecível por `errors.Is`, sem alterar estado,
inventário ou revision. A engine aplica a ação em uma cópia, valida e só então
substitui o estado. RNG é dependência runtime: sua sequência interna não é
revertida numa falha nem serializada. Deck/pilhas já materializados são preservados.

A mesma instância de `Game` requer acesso serializado, inclusive para consultas.
A Milestone 2 adicionará mutex privado por partida no manager. Engines distintas
não compartilham estado de jogo. Não há promessa de persistência durável nesta
entrega; snapshots demonstram que recuperação futura é possível.

## Regras de cartas

Base: [manual Classic Mattel W2085](https://service.mattel.com/instruction_sheets/W2085.pdf).
Distribuição de sete cartas, em rodadas a partir do jogador após o dealer.
Combinação por cor ou rank; Wild escolhe cor. Compra voluntária é permitida:
somente a carta comprada pode ser jogada em seguida. Se ela não for jogável, o
turno avança; se for, o jogador pode jogá-la ou passar. Não pode comprar novamente.

+2 e +4 fazem o alvo comprar e perder o turno, sem empilhar. +4 exige ausência da
cor ativa na mão inteira, inclusive após compra. Cor fica no estado da rodada,
nunca na carta física. Efeitos da última carta são resolvidos antes do término.

Na abertura, Skip pula o primeiro jogador; +2 também cobra duas; Reverse faz o
dealer começar na direção inversa; Wild pede cor ao primeiro jogador sem avançar
seu turno. +4 é devolvido à pilha: procura-se a próxima carta elegível e embaralha-se
o restante; a busca é finita para decks de teste sem carta inicial elegível.

[Regra a dois da Mattel](https://service.mattel.com/instruction_sheets/J3723-0824.pdf):
Reverse equivale a Skip, retornando a vez ao autor. Não são importadas as cartas
extras específicas daquela edição.

## Adaptações e políticas explícitas

| Política | ClassicRules | BotRules |
|---|---|---|
| Encerramento | Primeiro a zerar | Continua por colocação até restar um |
| Entrada durante o jogo | Não | Sim |
| Regras das cartas | Classic | Classic |
| Anúncio de UNO | Automático | Automático |
| +4 ilegal | Bloqueado | Bloqueado |

`Rules` contém apenas `EndPolicy` e `AllowLateJoin`; não há flags fictícias para
modos ainda inexistentes. `BotRules` preserva a dinâmica de participação escolhida
pelo usuário, não todos os desvios de regra do V1. Empilhamento, proibição de
terminar com Wild e proibição de Wild sobre Wild não foram portados.

- Até dez participantes **registrados** por jogo, inclusive quem saiu/terminou.
  Não há reentrada com o mesmo ID; isso evita ganhar repetidamente na mesma rodada.
- Entrada tardia compra sete cartas atomicamente e ocupa o assento imediatamente
  anterior ao atual na direção do jogo, mantendo o turno e o alvo de +4 pendente.
- Quem zera recebe `PlayerWon` e colocação, sai da rotação e permanece no histórico.
- Último restante recebe colocação com `WentOut=false`; não há falsa emissão de
  `PlayerWon`. Ranking futuro deve distinguir esse resultado de zerar a mão.
- Saída devolve cartas abaixo do topo do descarte e ajusta o turno sem pular
  indevidamente. O último restante por saída termina com motivo `departure`.
- Autor de escolha de cor pendente precisa resolvê-la antes de sair. Outros podem
  sair; se sair o alvo do +4, o próximo ativo assume o alvo. Se só restar o autor,
  encerra-se por saída e limpa-se a pendência.
- Último Wild aguarda escolha de cor antes de confirmar colocação e efeito.
- Cancelamento é permitido a um participante pela engine, sujeito à autorização
  externa da aplicação. Termina sem atribuir nova vitória e limpa a pendência.
- UNO é anunciado automaticamente quando uma jogada deixa uma carta, conforme
  escolha do usuário; denúncia e penalidade por esquecimento ficam fora da engine inicial.

### Falta de cartas

Reciclar somente o descarte abaixo da carta superior. Toda compra verifica a
quantidade disponível antes de executar. Se nem a reciclagem puder satisfazer a
compra, `ErrDeckEmpty` preserva a ação inteira. Não há compra parcial nem criação de
cartas. Essa política explícita para exaustão total também vale para +2, escolha
após +4, distribuição e entrada tardia. Num estado sem cartas suficientes, pode
ser necessário sair/cancelar pela camada de aplicação; a engine não inventa uma
regra de empate nem um baralho novo. A pendência após +4 continua recuperável.

## Eventos e privacidade

Eventos públicos incluem entrada/saída, início, carta jogada, quantidade comprada,
pulo, direção, cor, turno, UNO, colocação e término. `CardsDrawn` não inclui IDs ou
valores das cartas. Um resultado representa uma única revision mesmo quando
contém vários eventos. O renderer deve tratar `GameFinished` sem publicar um novo
turno. Ranking futuro consumirá resultados de maneira idempotente por jogo e
revision; não existe event sourcing aqui.

## Challenge e próximos passos

Challenge não foi implementado, por escolha do usuário. +4 ilegal é bloqueado.
`ColorChoice` guarda autor, alvo, cor anterior e quantidade, e a restrição pura
pode ser reutilizada. Challenge real exigirá uma fase de contestação e captura de
evidência **no momento da jogada**, além das penalidades; nunca durante uma query.

M2: manager/serviço, views privadas, locks e MemoryRepository. M3: Telegram seguro,
seleção multigrupo, tokens opacos, configuração, logs, shutdown, README e Docker.
M4: refinamento inline/filtros. M5: Match, ranking, modos e administração.
M6: persistência durável se necessária. Match/MD e pontuação acumulada não pertencem
às regras de uma rodada.

## Verificação

```bash
gofmt -w internal/uno
go build ./...
go test ./...
go test -race ./...
go vet ./...
```

Testes determinísticos cobrem regras, abertura, efeitos finais, erros sem mutação,
IDs físicos, descarte, políticas de participação/colocação, snapshots e recuperação.
Incluem 40 partidas completas com seeds fixas. Testes concorrentes da mesma partida
serão responsabilidade do manager na M2. Nenhum teste conecta Telegram ou PostgreSQL.

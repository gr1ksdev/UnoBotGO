# Plano: comando-dar-carta

## Pedido do usuário
Adicionar comando que entrega uma carta escolhida a um jogador. Somente o usuário Telegram 7595607953 pode executar. Retirar uma cópia do monte ou descarte e recusar quando não houver cópia disponível.

## Objetivo
Disponibilizar /dar na V2 com autorização no serviço e transferência transacional de uma carta existente, sem duplicar inventário nem retirar cartas de outra mão.

## Contexto atual
Comandos de grupo passam por CommandHandler, fila do chat, Service e engine UNO. A engine aplica mutações por cópia com revision estrita e valida a conservação de cartas. Não existe ação de entrega dirigida. A correção textual da carta de troca indisponível está na árvore de trabalho e deve ser preservada.

## Arquivos analisados
- internal/telegram/commands.go
- internal/telegram/bot.go
- internal/telegram/inline.go
- internal/game/service.go
- internal/game/manager.go
- internal/game/views.go
- internal/uno/action.go
- internal/uno/card.go
- internal/uno/game.go
- internal/uno/state.go
- internal/uno/rules.go
- internal/config/config.go
- .agent/context.md
- .agent/plans/pending/adicionar-comando-depuracao_2026-06-21_13-31.md (histórico V1; não atende este pedido)

## Arquivos que poderão ser modificados
- internal/telegram/commands.go, renderer.go e testes de comandos
- internal/game/service.go e testes
- internal/uno/action.go, game.go, event.go, errors.go e testes
- README.md, docs/v2-telegram.md e docs/v2-application.md
- .agent/context.md, .agent/memory/memory.md, .agent/decisions.md e este plano

## Estratégia de implementação
Comando de grupo /dar <carta>, uma carta por chamada. Em resposta a mensagem de usuário, esse usuário é o destinatário; sem resposta, o próprio solicitante. Respostas a mensagens de bot, remetente anônimo ou participante inativo são recusadas, sem redirecionamento silencioso.

Sintaxe aceita, sem distinção de maiúsculas:
- /dar troca
- /dar coringa
- /dar +4
- /dar <vermelho|azul|verde|amarelo> <0..9|+2|pular|inverter>
- Aceitar sufixo @nome_do_bot usando parser existente. Argumentos extras/inválidos retornam ajuda curta.

Disponível nos modos clássico e caseiro, respeitando o inventário e as regras do modo: troca somente no caseiro. Apenas em TakingTurn, sem DrawCounter positivo nem PendingBluff; recusar durante seleção de cor/jogador, lobby e encerramento para não interferir em efeitos ainda pendentes. O comando não joga a carta, não passa turno e não reinicia prazo; mantém DrawnCardID. Jogabilidade após entrega continua regida por CanPlay.

Buscar a primeira cópia compatível na ordem atual do monte. Se não existir, buscar do fundo ao topo do descarte, excluindo sempre o topo. Mover seu ID para o fim da mão do alvo, preservando a ordem das demais cartas. Sem cópia disponível, retornar erro e manter todo o estado e revision.

## Passos detalhados
1. Após aprovação, mover plano para approved.
2. Acrescentar GiveCard ao fim de ActionType. Reutilizar CardID para carta física e TargetID para destinatário, permitindo esses campos somente nas ações correspondentes. Engine verifica origem disponível, fase, pendências e alvo ativo; solicitante administrativo não precisa jogar a partida.
3. Acrescentar evento CardGranted com destinatário e ID da carta concedida. A confirmação no grupo informa apenas a carta entregue e destinatário, nunca o restante da mão.
4. Adicionar Service.GiveCard com Actor, GameID, alvo, Color e Rank. Validar identidade 7595607953 e ChatID do jogo no serviço. Sob lock da partida, selecionar cópia, aplicar ação com revisão atual e publicar via fluxo existente. Proteger também acesso direto via Service.Apply ao novo tipo de ação; não confiar só no handler.
5. Usar constante única de autorização no pacote game; não criar configuração para habilitar outros usuários. Dono da partida ou administrador sem esse ID não ganha acesso.
6. Implementar handler /dar com resolução do jogo pelo chat e destinatário por ReplyToMessage.From ou solicitante. Negar privado, bots, remetente anônimo e tópicos conforme regras existentes. Atualizar cache de nome do destinatário apenas com dados da mensagem.
7. Responder sucesso com nome, carta e botão Suas cartas. Não gerar token inline de GiveCard nem registrar /dar no menu público de comandos; documentar a sintaxe administrativa no README/docs.
8. Testar autorização, seleção de cartas, conservação de inventário, recusa sem mutação, privacidade da resposta e invalidação de seleções anteriores pela revisão.
9. Executar validações e documentar resultado; atualizar memória e decisões e mover plano para done. Preservar mudanças anteriores; sem commit, push ou deploy neste pedido.

## Riscos
- Entrega alterar evidência de blefe ou alvo pendente: recusar enquanto existirem efeitos pendentes.
- Usuário comum falsificar destinatário ou ação: autorização no serviço, Actor real e alvo revalidado na engine.
- Remover carta do topo ou duplicar IDs: mover apenas uma cópia do monte/descarte elegível e validar conservação.
- Seleções inline já abertas ficam antigas após alteração: revisão incrementa uma única vez, seguindo proteção atual.

## Impactos esperados
- Usuário 7595607953 consegue preparar mãos para testes por comando no grupo.
- Nenhuma carta é criada e nenhuma mão de terceiro é usada como origem.
- Sem alterações de dependências, configuração ou persistência.

## Compatibilidade
- Linux, macOS e Windows: somente lógica Go, sem comandos específicos de plataforma.
- Docker: mesmo executável V2 e configuração.
- CI/CD: preservar pipeline e validar testes/build existentes.

## Como testar

### Build
```bash
go build ./...
go vet ./...
```

### Testes
```bash
go test ./...
go test -race ./internal/uno ./internal/game ./internal/telegram
git diff --check
```
Cenários: ID autorizado como participante/observador; dono/admin não autorizado; chat incorreto; reply e autoentrega; destinatário ausente/inativo; parsing numéricas/especiais e erros; prioridade monte; fallback descarte; exclusão do topo/mãos; carta indisponível; troca no clássico; escolha/penalidade/blefe pendentes; conservação das cartas, direção, turno, prazo e marcador de compra; incremento único de revisão; menus antigos rejeitados.

### Execução
Homologar em bot de teste configurado: iniciar partida, responder a jogador com /dar amarelo +2 e /dar troca, conferir nova mão. Tentar com outro usuário e confirmar recusa. Não executar bot de produção automaticamente.

## Rollback
Reverter somente alterações desta feature por revisão específica, preservando a correção anterior do menu inline e o histórico dos planos. Cartas já transferidas permanecem cartas válidas do inventário.

## Observações
Preferências confirmadas: ID exclusivo 7595607953; obter do monte/descarte e recusar falta de cópia. Defaults propostos: nome /dar, alvo por resposta ou próprio autor, uma carta por chamada, apenas turno normal sem efeito pendente e comando fora do menu público. Implementação aguarda aprovação deste plano conforme AGENTS.md; a dispensa anterior de plano referia-se à correção do sticker.


## Revisão: comando descartável via build de teste

O usuário escolheu explicitamente a segunda opção: comando presente somente em build de teste. Esta revisão substitui as decisões anteriores sobre integração permanente de GiveCard, novos eventos e documentação pública. As regras de autorização, sintaxe, destinatário, origem das cartas e fases permitidas permanecem as descritas acima.

### Isolamento e ativação
- Toda implementação e testes específicos ficarão em arquivos `debugcards*.go`, com `//go:build debugcards`.
- Build padrão, Dockerfile.v2 e pipeline de publicação permanecem sem a tag. Nenhuma mudança no Docker ou workflow é necessária.
- Ativar localmente: `go run -tags debugcards ./cmd/bot` ou `go build -tags debugcards -o /tmp/unobot-debugcards ./cmd/bot`.
- Para desativar no processo em execução, substituir o executável por build normal e reiniciar. Não há desligamento instantâneo por chat; partidas são efêmeras e o reinício perde as partidas em memória.
- Não registrar /dar no menu, /help ou README público. Registrar instruções somente em `.agent/memory/memory.md` e neste plano. Código-fonte continua inspecionável; ausência no build normal não promete sigilo do repositório.
- No build de teste, usuários diferentes de 7595607953 recebem o mesmo comportamento de comando desconhecido (sem resposta nem divulgação da ferramenta). Serviço continua rejeitando explicitamente atores não autorizados.

### Integração mínima
1. Telegram: inserir uma chamada a `handleDebugCommand(ctx, msg, cmdName, fields)` antes do switch de comandos de grupo, depois das validações de identidade/tópico/chat. Implementação em `internal/telegram/debugcards.go`; stub `internal/telegram/debugcards_disabled.go` com `//go:build !debugcards` sempre retorna false e não contém nome do comando nem ID do usuário.
2. O handler tagged processa somente /dar. No arquivo tagged ficam parser, exemplos, resolução de destinatário e mensagens. Fora do grupo, comportamento genérico existente permanece. Não modificar renderer nem registro de comandos.
3. Serviço: método `Service.GiveCard` em `internal/game/debugcards.go`, disponível apenas com a tag. Recebe Actor, GameID, destinatário, Color e Rank; verifica usuário exclusivo e chat correspondente, obtém lock da partida, valida contexto, chama engine e publica pelo manager existente. Constante de ID fica somente nesse arquivo. Sem novo tipo em Service.Apply.
4. Engine: método `Game.GiveCard(revision, target, color, rank)` em `internal/uno/debugcards.go`, retorna Result e Card concedida. Verificar revisão/overflow, fase TakingTurn sem penalidade/blefe, combinação válida de cor/rank, regra da troca e alvo ativo. Clonar estado, procurar no monte primeiro e depois descarte excluindo topo, mover uma cópia, validar, incrementar revisão uma vez e efetivar. Emitir apenas o evento existente CardsDrawn com Count=1 para destinatário, sem expor carta no evento. A carta retornada ao serviço serve à confirmação administrativa. Não alterar ActionType, Action, State, EventType nem Game.Apply.
5. A confirmação informa a carta entregue e o destinatário, sem publicar restante da mão. Não muda turno, prazo, sentido, cor, penalidade ou marcador de compra. Seleções antigas ficam inválidas pela revisão.
6. Código padrão recebe apenas o hook no handler e seu stub; o restante é removível por arquivo. Para apagar definitivamente: remover hook e arquivos `debugcards*.go` dos três pacotes, incluindo testes. Preservar histórico dos planos/memória.

### Arquivos previstos nesta revisão
- internal/telegram/commands.go (hook único)
- internal/telegram/debugcards.go, debugcards_disabled.go e testes com tags correspondentes
- internal/game/debugcards.go e debugcards_test.go
- internal/uno/debugcards.go e debugcards_test.go
- .agent/context.md, .agent/memory/memory.md, .agent/decisions.md e este plano
- Sem alterações nos arquivos públicos de documentação previstos na versão inicial.

### Validação das duas variantes
```bash
go test ./...
go test -tags debugcards ./...
go test -race -tags debugcards ./internal/uno ./internal/game ./internal/telegram
go build ./...
go build -tags debugcards ./...
go vet ./...
go vet -tags debugcards ./...
git diff --check
```
- Teste sem tag: /dar não produz efeito nem resposta no grupo, inclusive para o ID autorizado; não aparece no menu nem ajuda.
- Testes com tag: cobertura funcional e de autorização prevista no plano original, incluindo não proprietário autorizado, proprietário não autorizado, fases pendentes e conservação de inventário.
- Conferir `go list` com e sem tag: implementação e APIs GiveCard ausentes dos GoFiles padrão, presentes somente com debugcards.
- Preservar correção do sticker já existente na árvore de trabalho. Sem commit, push ou deploy automático.

### Status
Preferência de build isolado confirmada pelo usuário. Plano revisado aguarda aprovação para implementação.


## Conclusão da revisão
- Aprovada explicitamente pelo usuário com "sim" e implementada em 2026-09-25.
- APIs GiveCard, parser, autorização e testes isolados com debugcards; build normal contém somente hook e stub.
- Testes completos, build e vet passaram nas duas variantes; race passou com tag em uno/game/telegram. go list confirmou seleção correta de arquivos. git diff --check passou.
- Instruções de uso/desativação/remoção registradas em memória e contexto locais. Sem registro público do comando, commit, push ou deploy. Correção anterior do sticker preservada.

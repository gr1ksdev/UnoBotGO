# Plano: milestone corretiva V2

## Pedido do usuário
Corrigir encerramento visual com dois jogadores, auditar o contexto inline e usar UserID real somente no jogador responsável pela ação atual.

## Objetivo
Respeitar o lifecycle publicado pela engine, serializar timeout com jogadas e centralizar menções contextuais sem mudar gameplay nem segurança multigrupo.

## Contexto atual
Base dev b08522d. A engine já encerra quando resta um jogador, sem TurnChanged. O handler inline anexa Suas cartas à confirmação final; clicar produz a mensagem relatada de partida indisponível. O scheduler aplica AutoSkip fora da fila do chat, permitindo publicar um turno antigo após encerramento. A leitura de final na descoberta não usa o mutex do campo. GetMe fornece BotID, mas FormatLink usa sempre o ID do jogador.

## Arquivos analisados
- internal/uno/{game,state,event,rules}.go e testes
- internal/game/{service,manager,views}.go e testes
- internal/telegram/{bot,commands,inline,callbacks,renderer,tokens,dispatch,transport}.go e testes
- cmd/bot/main.go, docs/v2-{rules,application,telegram}.md
- .agent/context.md, .agent/memory/memory.md, .agent/decisions.md

## Arquivos que poderão ser modificados
- internal/game/service.go, manager.go e testes
- internal/telegram/bot.go, commands.go, inline.go, callbacks.go, renderer.go e testes/mocks
- Testes em internal/uno (sem alteração planejada na engine)
- docs/v2-application.md, docs/v2-telegram.md e registros .agent

## Estratégia de implementação
Teclados recebem PublicGameView e não oferecem ações após Closed/Finished; edições removem explicitamente markup. Queries históricas recebem resposta terminal quando o histórico existe, senão indisponibilidade genérica. Erros de jogada consultam contexto atual antes de oferecer continuação; encerrar invalida tokens do jogo.

Separar descoberta de candidatos vencidos (GameID, ChatID, jogador, revision) e execução sob lock. Scheduler enfileira execução e mensagem na mesma tarefa do chat; revalidar prazo, identidade, revision e fase. Preservar AutoSkipExpired como wrapper compatível. Zerar turnStarted ao encerrar e corrigir leitura concorrente.

Renderer recebe BotID após GetMe, antes de ingressos. Helper separa nome exibido de destino: lobby/encerrado -> bot; TakingTurn -> UserID só de CurrentTurn; ChoosingColor -> responsável pela cor. Confirmações e eventos usam view posterior. Sem contexto -> bot; sem BotID válido -> texto escapado. Preservar HTML, nomes e usernames.

Inline continua g_<GameID>, distinto de tokens de ação de 128 bits, one-use, TTL e binding completo. Telegram não oferece contexto oculto equivalente em switch_inline_query*. Queries vazias já abrem uma partida ou seletor; não garantem origem multigrupo. Manter formato e navegação. Alternativa futura: base64url dos mesmos 128 bits, ainda visível. Não usar estado global de última partida.

## Passos detalhados
1. Registrar aprovação, mover plano para approved.
2. Implementar descoberta/execução segura de timeout e limpeza de prazo.
3. Corrigir teclados, respostas terminais e invalidação no encerramento.
4. Injetar BotID e substituir menções diretas por política contextual.
5. Adicionar matriz de regressão de cartas finais, scheduler, tokens, renderer e ambos transportes.
6. Executar testes, race, build e vet; documentar resultados e mover plano para done.

## Riscos
- Timeout precisa revalidar candidato após espera na fila; saturação não pode mutar estado.
- Não confundir ClassicRules (FirstWinner) com Clássico Telegram (BotRules/Placements).
- Botões históricos continuam acessíveis e devem falhar sem convite de continuação.
- Links Telegram precisam homologação visual em clientes; testes validam HTML/targets.

## Impactos esperados
- Confirmação final sem próxima jogada ou Suas cartas.
- Nenhum AutoSkip ou mensagem de timeout de turno encerrado.
- Somente responsável atual recebe link com UserID real.
- Contexto inline e gameplay preservados.

## Compatibilidade
- Linux, macOS, Windows: sem novas dependências; validação local não equivale a homologação multiplataforma.
- Docker e CI/CD: nenhuma alteração.
- Webhook e polling: mesmo pipeline compartilhado, sem lógica de gameplay por transporte.

## Como testar
### Build
```bash
go build ./...
go vet ./...
```
### Testes
```bash
go test ./...
go test -race ./...
```
Matriz 2 jogadores: numérica, Reverse, Skip, +2, +4 e Wild em BotRules/Caseiro; eventos, placements e sem próximo turno. 3+ preserva placements/direções. ClassicRules preserva FirstWinner. Efeitos imediatos e escolha de cor continuam iguais; stacking terminal não cobra compra extra. Testes de fechamento limpam runtime/índices/prazo. Candidatos obsoletos, duplicados, cancelados, nova partida e fila saturada não mutam. Barreiras/canais validam ordenação. Inline: zero/uma/múltiplas partidas, queries contextuais/vazias, botões antigos, TTL, one-use, stale revision, usuários e partidas isoladas. Renderer: todos os estados, eventos, escape HTML e IDs distintos. Transportes produzem resultados equivalentes.
### Execução
```bash
go run ./cmd/bot
```
Somente homologação manual posterior em ambiente de teste, alternando polling/webhook; não iniciar bot real automaticamente.

## Rollback
Reverter alterações da milestone sem apagar planos e memória histórica. Sem operações destrutivas, commit, push, merge ou promoção nesta tarefa.

## Observações
- Aprovado pelo usuário: "Implement the plan." em 2026-09-23.
- Decisão explícita: preservar comportamento atual de penalidades finais com stacking, sem compra adicional. Preservar contador final existente e não criar turno executável.
- /estado já existe na dev analisada; não adicionar nem ampliar.
- Fontes: https://core.telegram.org/bots/api#inlinekeyboardbutton, #inlinequery, #choseninlineresult, #formatting-options, #user. Links tg://user?id aceitos em HTML; User inclui bots; apresentação sujeita ao cliente e regras de menção.
- Base passou go test ./..., go test -race ./..., go build ./... e go vet ./... durante análise.


## Resultado da implementação — 2026-09-23

Concluída na dev, sem commit/push/merge. Alterações concentradas no service/manager, scheduler, renderer e handlers Telegram; engine e TokenStore preservados. Testes Telegram adicionais reunidos em internal/telegram/lifecycle_test.go, além dos arquivos previstos.

Validações aprovadas: go test ./..., go test -race ./..., go build ./..., go vet ./... e git diff --check. Os cenários selecionados de finais nos dois transportes, ordenação de timeout, isolamento de contexto e cursores passaram 20 execuções. Matriz exata de cartas/efeitos usa fixtures determinísticas na engine; integração Telegram alcança o final via API pública real do serviço, com Telegram mockado e sincronização por canais.

Documentação, decisões e memória atualizadas. Homologação visual em Telegram real não foi executada; roteiro em docs/v2-telegram.md. Não houve bot real iniciado, modificação em Docker/CI ou mudança de regras.

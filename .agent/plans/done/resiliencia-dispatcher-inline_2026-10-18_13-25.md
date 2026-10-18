# Plano: resiliencia-dispatcher-inline

## Pedido do usuário
O bot parou de responder e ficou offline após uma tentativa de jogar via inline em um grupo (`chat_id=-1003565223332`), gerando o aviso de saturação da fila: `time=2026-10-18T12:20:39.875Z level=WARN msg="chat queue saturated, dropping task" chat_id=-1003565223332 worker_idx=4`. O usuário pediu para checar o que acontece e criar um plano para solucionar essas fragilidades.

## Objetivo
Eliminar a suscetibilidade a bloqueios permanentes e perda silenciosa de jogadas no Telegram:
1. Proteger os workers do `Dispatcher` com timeout delimitado por tarefa (`ChatTaskTimeout`), evitando que goroutines ou chamadas de I/O bloqueadas travem o worker indefinidamente.
2. Ampliar a capacidade das filas de chat particionadas (`ChatQueueCapacity`) para absorver picos de tráfego normais sem descartar tarefas precocemente.
3. Adicionar estorno atômico de token de ação inline (`ReleaseAction`) no `TokenStore`, garantindo que se o agendamento no dispatcher for recusado (fila cheia ou shutdown), o token volte ao estado não consumido para permitir nova tentativa pelo usuário/Telegram.

## Contexto atual
- O `Dispatcher` particiona chats em 8 workers fixos (`abs(chatID) % 8`). O grupo `-1003565223332` cai no `worker_idx=4`.
- A capacidade de fila `ChatQueueCapacity` é de apenas 32 itens por partição.
- Cada worker processa tarefas sequencialmente utilizando `task.fn(task.ctx)`, onde `task.ctx` não possui timeout individual de tarefa (herda apenas o cancelamento global do dispatcher).
- Se qualquer tarefa travar ou sofrer latência prolongada, o worker não esvazia a fila. Uma vez cheia (32 itens), novas mensagens e comandos são sumariamente descartados no `default` do `select`.
- No `InlineHandler.HandleChosenInlineResult`, o token de ação da carta jogada é marcado como consumido via `h.tokens.ConsumeAction(tokenStr, actorID)`. Se `h.dispatcher.EnqueueChat` falhar por saturação da fila, a tarefa é descartada, mas o token permanece como consumido (`tok.Consumed = true`), impedindo que o jogador refaça a jogada ou que o Telegram reenvie o evento.

## Arquivos analisados
- `internal/telegram/dispatch.go`
- `internal/telegram/dispatch_test.go`
- `internal/telegram/tokens.go`
- `internal/telegram/tokens_test.go`
- `internal/telegram/inline.go`
- `internal/telegram/bot.go`
- `internal/telegram/bot_test.go`

## Arquivos que poderão ser modificados
- `internal/telegram/dispatch.go`
- `internal/telegram/dispatch_test.go`
- `internal/telegram/tokens.go`
- `internal/telegram/tokens_test.go`
- `internal/telegram/inline.go`

## Estratégia de implementação
1. **Definição de timeouts de execução de tarefa:**
   - Adicionar constante `ChatTaskTimeout = 20 * time.Second` e `RecoveryTaskTimeout = 15 * time.Second` em `internal/telegram/dispatch.go`.
   - Nos loops dos workers (`start` e `runDedicatedChat`), envolver a execução de `task.fn` com `taskCtx, cancel := context.WithTimeout(task.ctx, ChatTaskTimeout)` e `defer cancel()`.
   - No worker de recuperação (`recoveryQueue`), envolver com `taskCtx, cancel := context.WithTimeout(d.ctx, RecoveryTaskTimeout)`.
   - Caso o timeout estoure, o contexto cancela operações ativas de rede (Telegram API) e banco de dados, liberando o worker para a próxima tarefa da fila.

2. **Aumento da capacidade de amortecimento de fila:**
   - Elevar `ChatQueueCapacity` de 32 para 128 (quadruplicando a margem para rajadas de mensagens sem desperdício de memória).
   - Elevar `InlineQueueCap` de 64 para 128.
   - Elevar `RecoveryQueueCapacity` de 16 para 32.

3. **Mecanismo de estorno/rollback de token de ação:**
   - Implementar método `ReleaseAction(tokenStr string, actorID uno.PlayerID) bool` em `internal/telegram/tokens.go`:
     - Sob `s.mu.Lock()`, se o token existir, pertencer ao `actorID`, estiver com `tok.Consumed == true` e ainda sem `ResultSummary`, restaura `tok.Consumed = false`.
   - Em `internal/telegram/inline.go` (`HandleChosenInlineResult`):
     - Se `accepted := h.dispatcher.EnqueueChat(...)` retornar `false`, invocar `h.tokens.ReleaseAction(tokenStr, actorID)`.
     - Logar `WarnContext` informando que o enfileiramento foi recusado e o token liberado.
     - Retornar `false` para indicar que a atualização não foi processada.

4. **Testes automatizados:**
   - Adicionar teste unitário em `tokens_test.go` validando o comportamento de `ReleaseAction` (estorno com sucesso, recusa se usuário incorreto ou se já finalizado com resultado).
   - Adicionar teste unitário em `dispatch_test.go` validando que uma tarefa que excede o timeout não impede a execução da tarefa seguinte no mesmo worker.
   - Garantir que todos os testes existentes continuem passando (`go test -count=1 ./...`).

## Passos detalhados
1. Criar `ReleaseAction` no `TokenStore` (`internal/telegram/tokens.go`) e cobrir com testes unitários em `tokens_test.go`.
2. Atualizar constantes de capacidade (`ChatQueueCapacity = 128`, etc.) e implementar `ChatTaskTimeout` / `RecoveryTaskTimeout` nos loops dos workers em `internal/telegram/dispatch.go`.
3. Adicionar teste em `internal/telegram/dispatch_test.go` verificando liberação do worker após cancelamento por deadline/timeout.
4. Integrar `ReleaseAction` em `internal/telegram/inline.go` quando `EnqueueChat` falhar.
5. Executar `go test -count=1 ./...` e `go vet ./...` para validação rigorosa.

## Riscos
- **Tarefas canceladas por timeout:** Tarefas que realmente precisem de mais de 20s (improvável para comandos e jogadas de Telegram) serão interrompidas com context cancelled. Mitigação: 20s é muito generoso (o dobro do timeout de 10s usado em queries e requisições individuais).
- **Consumo de memória pelo aumento da fila:** Uma fila de 128 ponteiros de `chatTask` em 8 partições consome menos de 50 KB de RAM, risco desprezível.

## Impactos esperados
- O bot não ficará mais travado permanentemente se uma chamada externa ou requisição de rede congelar em uma partição.
- Rajadas de mensagens em grupos não causarão saturação prematura da fila do chat.
- Jogadas inline descartadas por sobrecarga temporária não perdem o token, permitindo que novas tentativas funcionem sem erros de "token já consumido".

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -count=1 ./internal/telegram/...
go test -count=1 ./...
```

### Execução
Executar o bot e submeter tarefas concorrentes e simulação de timeouts para garantir esvaziamento contínuo das filas.

## Rollback
Reverter as alterações em `internal/telegram/dispatch.go`, `tokens.go` e `inline.go` via `git checkout`.

## Observações
As alterações são restritas à camada de transporte/resiliência do Telegram (`internal/telegram`), sem alterar regras de negócio de UNO ou esquemas do banco de dados.

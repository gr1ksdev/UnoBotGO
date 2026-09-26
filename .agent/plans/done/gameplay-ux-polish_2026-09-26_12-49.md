# Plano: gameplay UX polish

## Pedido do usuário
Auditoria curta e implementação na dev de ordem de late join, trancar/destrancar e mensagens compactas. Commit e push dev autorizados explicitamente. Main não pode ser alterada.

## Objetivo
Garantir sequência lógica de turnos, bloquear novas entradas por decisão do owner e padronizar apresentação sem alterar outras regras.

## Contexto atual
HEAD badf81c. State.Order contém IDs ativos em ordem física; next usa índice + passos * Direction. Reverse inverte Direction (a dois produz Skip); finalizados saem de Order. join já insere antes do atual no sentido positivo e depois no negativo, portanto na cauda lógica. Renderer mostra o slice físico e pode sugerir ordem diferente. Não há evidência de salto efetivo na engine atual.
managedGame guarda owner/chat/timer e mutex por partida. Não existe persistência/restauração de sessão; snapshots são apenas da engine. Renderer principal em internal/telegram/renderer.go.

## Arquivos analisados
- internal/uno/game.go, state.go, player.go, testes e regras
- internal/game/manager.go, service.go, views.go, errors.go e testes
- internal/telegram/renderer.go, commands.go, bot.go e testes
- AGENTS.md, .agent/context.md, docs/project-status.md

## Arquivos que poderão ser modificados
- internal/uno: testes de late join (comentário da inserção se necessário)
- internal/game: metadata/view, Service, erro e testes de lock
- internal/telegram: renderer, comandos/registro e testes afetados
- docs/project-status.md, README.md, docs/v2-application.md, docs/v2-telegram.md
- .agent: plano, contexto, memória e decisão

## Estratégia de implementação
Preservar algoritmo correto de join e provar ordem com ações reais. Mostrar ciclo a partir do atual no sentido vigente. Locked será metadata da sessão; SetLocked e Join usam entry.mu. Sem mudar revision da engine, tokens ou timeout. Renderer separa ação/efeito/resultado/próxima ação, oculta cor redundante e padroniza colocações.

## Passos detalhados
1. Registrar auditoria e autorização já concedida no pedido.
2. Adicionar regressões de late join normal/reverse/placements/múltiplas entradas e invariantes.
3. Implementar lock no Service, exposição pública e comandos owner-only idempotentes.
4. Compactar renderer preservando links, UNO, escolhas, efeitos e fim de partida.
5. Testar lifecycle, concorrência, handlers e mensagens; atualizar docs/status.
6. Validar normal/debugcards/race/vet/build/diff, revisar escopo, commit e push somente dev.

## Riscos
- Confundir representação física com próxima sequência de turnos.
- Resetar prazo ou invalidar ações inline numa alteração administrativa.
- Perder escaping/links, fases de escolha ou mensagens de término.

## Impactos esperados
Sala começa aberta; só owner decide entradas. Ordem exibida coerente; sem mudanças de cartas/regras/transporte.

## Compatibilidade
Linux/macOS/Windows: sem dependências novas. Docker/CI/CD: inalterados. Race local pode ser bloqueado por VMA ARM64; registrar e verificar CI.

## Como testar
### Build
```bash
go build ./...
go build -tags debugcards ./...
```
### Testes
```bash
go test ./...
go test -race ./...
go vet ./...
go test -tags debugcards ./...
go vet -tags debugcards ./...
git diff --check
```
### Execução
Homologação Telegram real será feita posteriormente pelo usuário; não iniciar bot nesta tarefa.

## Rollback
Reverter o commit da milestone na dev; nenhum merge/promoção ou comando destrutivo.

## Observações
Pedido atual autoriza expressamente auditar e depois implementar; não há necessidade de nova aprovação. Main de referência fd011ab deve permanecer inalterada.

## Resultado
Implementação e documentação concluídas somente na dev. Algoritmo da engine preservado porque regressões confirmam inserção correta; renderer corrigido para sequência lógica. Lock de sessão, comandos owner-only e mensagens compactas entregues.

Testes novos: TestLateJoinLogicalTail, TestLateJoinAfterReverseAndPlacement; TestRoomLockLifecycleAndAdmission, TestRoomLockConcurrentJoin, TestRoomLockTimeoutResetAndFinalization, TestLateJoinPreservesTurnDeadline, TestRoomLockNaturalCompletionAndOwnerTransfer; TestRoomLockCommands; quatro TestGameplay* de renderer. Expectativas de mensagens/lifecycle e registro de comandos atualizadas.

Passaram go test ./..., go vet ./..., go build ./... e variantes test/vet/build com debugcards; git diff --check aprovado. go test -race bloqueado por CGO=0; com CGO_ENABLED=1, ThreadSanitizer acusa VMA 39 em vez de 48. Não é resultado válido dos testes; acompanhar CI após push. Nenhum bot real iniciado. Homologação Telegram pendente.

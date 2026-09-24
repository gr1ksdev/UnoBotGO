# Plano: recuperacao-e-reset-por-grupo

## Pedido do usuário
Evitar no projeto Go a falha observada no bot Python em que, após alguma ação desconhecida, um grupo fica lento ou deixa de receber respostas. Adicionar um comando capaz de limpar todo o estado operacional daquele grupo e permitir que ele volte a funcionar normalmente sem reiniciar o bot inteiro nem afetar outros grupos.

## Objetivo
Entregar duas camadas complementares de proteção:

1. **Prevenção e contenção:** impedir que panic, chamada lenta ao Telegram, fila saturada ou tarefa defeituosa de um grupo derrube workers ou contamine o processamento de outros grupos.
2. **Recuperação operacional:** criar `/reset` como comando emergencial fora da fila normal do grupo, autorizado ao responsável da partida ou a administrador/dono do grupo, que cancele o trabalho corrente, descarte ações antigas e limpe partida, índices, histórico e tokens daquele chat.

Após um reset bem-sucedido, o grupo deverá poder executar `/novo` imediatamente. O comando não reiniciará o processo inteiro e não apagará partidas de outros chats.

## Contexto atual
- A branch ativa é `dev` e há alterações locais anteriores em engine, aplicação, Telegram, documentação, memória e simulador. Elas deverão ser preservadas.
- A V2 já evita vários problemas do legado: ações transacionais, revisão estrita, mutex por partida, estado isolado por chat, filas limitadas e timeout HTTP de 15 segundos.
- O `Dispatcher` atual possui oito filas particionadas por hash de `ChatID`. Grupos diferentes podem cair no mesmo worker; uma tarefa lenta atrasa todos os chats daquele shard.
- Os workers executam `task.fn` e o handler inline sem `recover`. Um panic não é convertido em diagnóstico e pode encerrar o processo Go.
- Quando a fila normal fica cheia, novas mensagens do grupo são recusadas. Um eventual `/reset` enviado por essa mesma fila também seria recusado ou aguardaria atrás do bloqueio, portanto não funcionaria como recuperação real.
- O `/cancelar` atual exige o responsável da partida, aplica uma ação normal da engine e invalida tokens. Ele não limpa tarefas já enfileiradas, não cancela uma chamada em andamento e não atende grupos sem partida ativa ou com índices inconsistentes.
- O dispatcher usa um único contexto global. Não há contexto cancelável nem geração por grupo para invalidar tarefas antigas depois de uma recuperação.
- O `TokenStore` já fornece `InvalidateGame`, que pode ser reutilizado para invalidar ações e cursores associados às partidas removidas.
- O cliente Telegram de produção já usa `http.Client{Timeout: 15s}`, limitando I/O de rede, mas os contexts das tarefas não podem hoje ser cancelados por grupo.
- O `BotAPI` ainda não expõe `GetChatMember`; será necessário para autorizar administradores e o dono do grupo quando o responsável atual não puder agir.

## Arquivos analisados
- `AGENTS.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- `cmd/bot/main.go`
- `internal/config/config.go`
- `internal/game/errors.go`
- `internal/game/manager.go`
- `internal/game/service.go`
- `internal/game/views.go`
- `internal/telegram/bot.go`
- `internal/telegram/client.go`
- `internal/telegram/commands.go`
- `internal/telegram/dispatch.go`
- `internal/telegram/inline.go`
- `internal/telegram/tokens.go`
- `internal/telegram/mock_test.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/dispatch_test.go`
- `internal/telegram/lifecycle_test.go`
- `internal/telegram/client_test.go`
- `docs/v2-application.md`
- `docs/v2-telegram.md`
- `README.md`

## Arquivos que poderão ser modificados
- `internal/game/errors.go`
- `internal/game/manager.go`
- `internal/game/manager_test.go`
- `internal/game/service.go`
- `internal/game/service_test.go`
- `internal/telegram/bot.go`
- `internal/telegram/bot_test.go`
- `internal/telegram/client.go`
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/dispatch.go`
- `internal/telegram/dispatch_test.go`
- `internal/telegram/lifecycle_test.go`
- `internal/telegram/mock_test.go`
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`
- `README.md`
- `docs/v2-application.md`
- `docs/v2-telegram.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- este plano, ao ser movido por `pending`, `approved` e `done`

Não há alteração planejada nas regras de UNO, no simulador, no banco, em Docker ou na estratégia de branches.

## Estratégia de implementação
O reset será tratado como operação administrativa de recuperação, não como uma jogada normal. A mensagem `/reset` será reconhecida antes da admissão na fila comum e enviada a uma pequena fila de recuperação independente e limitada. Assim, ela continuará disponível mesmo se a fila do grupo estiver cheia.

O dispatcher manterá uma geração e um contexto cancelável para cada chat. Cada tarefa normal será marcada com a geração vigente. Ao iniciar um reset:

1. a geração do chat será incrementada;
2. o contexto anterior será cancelado, interrompendo chamadas Telegram que respeitam context;
3. tarefas antigas ainda na fila serão ignoradas quando chegarem ao worker;
4. tarefas novas receberão contexto e geração novos.

Isso mantém a ordenação normal sem executar callbacks ou ações obsoletas depois da limpeza. A tarefa que já estiver executando não será encerrada à força pelo runtime, mas receberá cancelamento; o cliente HTTP possui timeout máximo de 15 segundos. Como a camada de aplicação não mantém I/O sob mutex de partida, o reset poderá obter o lock depois da ação corrente e remover o estado de forma consistente.

Na aplicação, `Service.ResetChat` fará uma remoção administrativa atômica. A autorização será:

- responsável atual da partida; ou
- dono/administrador real do grupo, confirmado por `getChatMember`;
- se não houver partida ativa, somente dono/administrador do grupo.

O reset removerá o jogo ativo, participações, índice por chat, resumos históricos retidos daquele chat e referências internas relacionadas, retornando os `GameID` removidos para que o adapter invalide seus tokens. O método será idempotente: ausência de estado será informada sem erro destrutivo.

Todos os workers de chat, inline e recuperação executarão cada callback dentro de uma barreira de `recover`, registrarão stack trace sanitizado e continuarão processando. Panic não será tratado como sucesso, e dados sensíveis não serão incluídos no log.

O comando responderá com uma confirmação clara, por exemplo: “♻️ Estado deste grupo resetado. Use /novo para iniciar uma nova partida.” Tentativas sem autorização serão rejeitadas. O reset não terá alias destrutivo implícito e não será executado em chat privado, fórum, canal ou administrador anônimo.

## Passos detalhados

### Milestone 1 — Cancelamento e geração por grupo

1. Adicionar ao dispatcher o registro concorrente de geração/contexto por `ChatID`.
2. Marcar tarefas de chat com sua geração e descartar tarefas obsoletas antes da execução.
3. Implementar `ResetChat` no dispatcher para cancelar o contexto corrente e abrir uma nova geração.
4. Adicionar fila limitada e workers exclusivos para comandos de recuperação.
5. Proteger callbacks de chat, inline e recuperação com `recover`, logging estruturado e stack trace.

**Critério de aceite:** reset continua admitido com a fila normal saturada; tarefa em andamento recebe cancelamento; tarefas antigas não executam; panic não mata o worker; outros chats continuam processando.

### Milestone 2 — Limpeza administrativa da aplicação

6. Criar resultado estruturado do reset contendo jogos removidos e indicação de estado ativo/histórico.
7. Implementar no manager a remoção consistente do jogo ativo, índices `byChat`/`byPlayer`/`byID`, histórico e runtime do chat.
8. Expor `Service.ResetChat` com autorização por responsável ou flag confiável de administrador do chat.
9. Garantir revalidação sob locks e que uma ação cancelada/concorrente não republique o estado removido.

**Critério de aceite:** após reset, `FindChatGame` não encontra estado, `FindPlayerGames` não contém a partida antiga e uma nova partida pode ser criada imediatamente no mesmo grupo.

### Milestone 3 — Comando `/reset` e autorização Telegram

10. Adicionar `GetChatMember` ao contrato `BotAPI` e aos mocks.
11. Detectar `/reset` e `/reset@nomedobot` antes da fila normal, preservando deduplicação de updates.
12. Verificar primeiro se o solicitante é o responsável atual; caso contrário, consultar se é `creator` ou `administrator` do grupo.
13. Cancelar/incrementar a geração do chat, executar `Service.ResetChat`, invalidar tokens de todos os jogos removidos e enviar confirmação.
14. Registrar o comando em `SetMyCommands`, ajuda e README.
15. Preservar os comandos existentes e verificar que `/estado` continua emitindo uma única resposta.

**Critério de aceite:** responsável e administradores conseguem recuperar o grupo; membro comum não; comando funciona com fila normal cheia e deixa `/novo` operacional.

### Milestone 4 — Testes de resiliência e documentação

16. Testar reset em lobby, partida ativa, estado ausente e histórico retido.
17. Testar autorização de responsável, dono, administrador, membro comum, canal/anônimo, chat privado e tópico.
18. Testar corrida entre ação e reset, invalidação de token, descarte de tarefas antigas e preservação de outros chats.
19. Testar panic em cada classe de worker e confirmar que a próxima tarefa é executada.
20. Testar polling e webhook para admissão do comando de recuperação e deduplicação.
21. Atualizar contratos, operação, memória e decisão arquitetural.
22. Rodar formatação, testes completos, race detector quando suportado, vet, build e whitespace.

**Critério de aceite:** todas as suites passam e os testes comprovam recuperação local sem reinício global ou impacto em outro grupo.

## Riscos
- **Reset usado por qualquer membro:** permitiria sabotagem de partidas. Mitigação: somente responsável atual ou administrador/dono verificado pelo Telegram.
- **Bot sem permissão para consultar membro:** a Bot API só garante `getChatMember` para terceiros quando o bot é administrador. Mitigação: o responsável atual continua autorizado localmente; para os demais, falha de verificação resulta em rejeição segura e mensagem explicativa.
- **Tarefa antiga publicar depois do reset:** poderia recriar ou anunciar estado obsoleto. Mitigação: contexto cancelado, geração invalidada, locks/revalidação na aplicação e tokens removidos.
- **Goroutine impossível de interromper:** Go não encerra goroutines à força. Mitigação: contexts por grupo, HTTP com timeout de 15 segundos, ausência de I/O sob lock e descarte das tarefas seguintes da geração antiga.
- **Reset concorrente com criação:** poderia apagar ou deixar escapar uma nova partida. Mitigação: geração é trocada antes da limpeza e o manager revalida o índice do chat sob lock.
- **Panic contendo dados sensíveis:** stack trace pode expor argumentos internos. Mitigação: logar classe do worker, chat/query e stack do runtime sem payloads, URLs ou token do bot.
- **Crescimento do mapa de contexts por chat:** muitos chats poderiam acumular entradas. Mitigação: remover estado de geração inativo com segurança ou manter somente chats observados com limpeza no reset/shutdown e teste de cardinalidade.
- **Reset não cura falha global de rede/processo:** um comando de grupo não pode recuperar processo encerrado ou indisponibilidade total do Telegram. Mitigação: documentar o limite; timeout e `recover` cobrem falhas locais, enquanto reinício do container/process manager permanece a recuperação global.
- **Alterações locais existentes:** arquivos alvo já possuem trabalho não commitado. Mitigação: editar apenas trechos necessários, não restaurar arquivos e revisar diff por escopo.

## Impactos esperados
- Um grupo travado poderá ser recuperado por `/reset` sem afetar partidas de outros chats.
- Tarefas e tokens antigos daquele grupo deixarão de ser aceitos após o reset.
- Panic de handler não encerrará o processo nem removerá permanentemente um worker.
- Chamadas lentas poderão ser canceladas por grupo e continuam limitadas pelo timeout HTTP.
- Ações comuns manterão ordenação e anti-cheat existentes.
- `/cancelar` continuará sendo o encerramento normal da partida; `/reset` será reservado para recuperação administrativa.

## Compatibilidade
- **Linux/macOS/Windows:** somente primitivas padrão de contexto, mutex, canais e atomics; sem dependência de sistema operacional.
- **Docker:** nenhuma alteração planejada na imagem; timeout e recuperação funcionam no mesmo processo.
- **Polling/Webhook:** ambos compartilham `submitUpdate/processUpdate` e deverão reconhecer a via de recuperação.
- **CI/CD:** nenhuma alteração de workflow; novos testes entram em `go test ./...`.
- **Telegram:** requer `getChatMember` para autorizar administradores que não sejam o responsável atual; o responsável usa autorização local existente.

## Como testar

### Build
```bash
gofmt -w internal/game internal/telegram
go build ./...
go vet ./...
```

### Testes
```bash
go test ./internal/game ./internal/telegram
go test ./...
go test -race ./...
git diff --check
```

Casos obrigatórios:

- fila normal cheia e `/reset` aceito pela fila de recuperação;
- tarefa bloqueada recebe `context.Canceled`;
- tarefas antigas do mesmo chat são descartadas;
- tarefa nova após reset executa normalmente;
- chat diferente, inclusive no mesmo shard antigo, permanece funcional;
- panic em chat/inline/recovery não mata worker;
- reset por responsável, creator e administrator;
- rejeição de membro comum, remetente anônimo, tópico e chat privado;
- remoção de lobby, partida ativa, índices de jogadores, histórico e tokens;
- `/novo` funciona imediatamente após reset;
- polling e webhook mantêm deduplicação e backpressure.

### Execução
```bash
go run ./cmd/bot
```

Homologação manual em grupo de teste:

1. criar e iniciar uma partida;
2. abrir algumas mãos para gerar tokens;
3. executar `/reset` como responsável e confirmar que botões antigos falham;
4. executar `/novo` e jogar normalmente;
5. repetir `/reset` como administrador diferente do responsável;
6. confirmar que membro comum não consegue resetar;
7. confirmar que outro grupo continua jogando durante todo o fluxo.

## Rollback
Reverter somente os trechos desta entrega no service/manager, dispatcher, comando, contrato de API, testes e documentação. Preservar todas as alterações locais anteriores. Não usar `git reset --hard`, `git clean` ou remoção ampla. Como o estado é somente em memória, retirar a funcionalidade e reiniciar o processo restaura o comportamento anterior; partidas já resetadas não são recuperáveis.

## Observações
- Aprovado pelo usuário em 2026-09-24 com: "pode sim".
- A causa exata da falha do bot Python não pode ser determinada sem código, logs e stack trace daquele processo. O plano cobre as classes de falha compatíveis com o sintoma e observáveis na arquitetura Go atual.
- `/reset` é deliberadamente mais forte que `/cancelar`: apaga estado operacional e histórico retido do chat, invalida interações pendentes e renova a geração da fila.
- O comando não deve prometer recuperar indisponibilidade global da Bot API, processo morto, falta de CPU/memória ou falha do host; esses casos exigem supervisão externa/restart.

## Resultado da implementação

- Milestones 1–4 concluídas em 2026-09-24 na branch `dev`.
- Dispatcher recebeu geração e contexto por chat, fila de recuperação independente, fila limpa pós-reset e barreiras de panic em chat, inline e recovery.
- `Service.ResetChat` remove sessão ativa, índices, histórico e runtime com tombstone, autorização e espera de mutex sensível a contexto.
- `/reset` foi integrado ao pipeline comum de polling/webhook, deduplicação, `GetChatMember`, registro de comandos, ajuda, tokens e mensagens operacionais.
- Documentação, contexto, memória e decisão arquitetural foram atualizados.

### Validação executada

```text
go test ./...                                      PASS
go test -count=20 ./internal/game ./internal/telegram PASS
go vet ./...                                       PASS
go build ./...                                     PASS
gofmt + git diff --check                           PASS
```

O race detector não pôde ser executado neste host Termux/Android. Sem CGO, o Go
recusou `-race`; com `CGO_ENABLED=1`, o toolchain falhou antes dos testes por
incompatibilidades de `getnameinfo`, `__errno_location` e `__android_log_vprint`.
Ele deve continuar sendo executado no CI Linux suportado.

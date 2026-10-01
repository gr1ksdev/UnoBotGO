# Plano: alias-join-ranking-atualizado

## Pedido do usuário
Adicionar /join@username como alias de /entrar@username e selecionar ranking Atualizado por padrão nos grupos novos.

## Objetivo
Reutilizar o handler de entrada e mudar somente o default de novas configurações, preservando grupos existentes.

## Contexto atual
Dev limpa e sincronizada após commit 3c7c455. Parser já aceita comandos com sufixo @bot e recusa destinatário diferente. Handler trata somente entrar. groups.Defaults usa Legacy; PostgreSQL cria grupos por INSERT que depende de DEFAULT legacy na migration 0001. Existem múltiplos caminhos de criação (configuração, instalação e observação do título), por isso mudar somente Go não resolve persistência.

## Arquivos analisados
- internal/telegram/commands.go
- internal/telegram/bot.go
- internal/telegram/commands_test.go
- internal/telegram/config_test.go
- internal/groups/groups.go e groups_test.go
- internal/game/group_config_test.go
- internal/storage/postgres/groups.go e groups_integration_test.go
- internal/storage/postgres/migrations/0001_groups.up.sql
- README.md, docs/v2-telegram.md e .agent/context.md

## Arquivos que poderão ser modificados
- internal/telegram/commands.go e testes
- internal/groups/groups.go e testes
- internal/game/group_config_test.go e testes afetados pelo default
- internal/storage/postgres/groups_integration_test.go e testes afetados
- internal/storage/postgres/migrations/0008_default_ranking_updated.up.sql (nova)
- README.md e docs/v2-telegram.md
- .agent/context.md, memory/memory.md e decisions.md

## Estratégia de implementação
Adicionar join ao mesmo case entrar, aproveitando parseBotCommand e validação de username, permissões e fluxo existente. Manter entrar como comando principal do menu; alias não precisa duplicar registro. Mudar groups.Defaults para Updated e criar migration nova com ALTER TABLE group_configs ALTER COLUMN ranking_system SET DEFAULT 'updated'. Não editar migrations anteriores nem executar UPDATE sobre linhas existentes. Todas as criações via DEFAULT passam a Updated e grupos existentes mantêm sistema/revisão/pontos. Inicialização já aplica migration pelo pipeline existente; não executar bot ou migrations de produção nesta tarefa.

## Passos detalhados
1. Após aprovação, mover plano a approved.
2. Adicionar alias no handler, sem duplicar lógica; documentar alias.
3. Alterar default Go e adicionar migration 0008 não destrutiva apenas do DEFAULT.
4. Ajustar testes que dependem do default mantendo cenários Legacy explícitos.
5. Testar /join, /join@bot, /entrar@bot e destinatário errado; verificar entrada efetiva e mesmas restrições.
6. Testar default Updated em memória e persistência; grupos existentes Legacy devem continuar Legacy depois de aplicar migration. Cobrir caminhos instalação/título/configuração e idempotência.
7. Atualizar documentação atual e memória, sem apagar planos históricos.
8. Rodar testes Go relevantes, go test ./..., go vet ./..., go build ./... e git diff --check; testes de integração com PostgreSQL somente se banco isolado disponível. Informar limites se indisponível.
9. Mover plano a done e deixar working tree para revisão; sem commit/push automático desta nova tarefa.

## Riscos
- Migration necessária para default SQL: deve alterar somente metadado de default, sem alterar grupos existentes.
- Testes antigos presumem Legacy; preservar cenários com configuração explícita sem mudar ranking/pontuação.
- Homologação Telegram do alias depende do bot correto e ambiente real.

## Impactos esperados
- /join funciona exatamente como /entrar.
- Apenas grupos sem configuração persistida passam a nascer Updated; grupos já configurados preservados.

## Compatibilidade
- Linux, macOS, Windows: Go existente.
- Docker, CI/CD: migration versionada e build atuais, sem alteração de infraestrutura.

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test ./internal/telegram ./internal/groups ./internal/game
go test ./...
go vet ./...
go test -tags integration ./internal/storage/postgres/...
git diff --check
```
Integração exige TEST_DATABASE_URL de teste, nunca produção.

### Execução
```bash
# Homologação manual posterior no Telegram:
/join@username_do_bot
```

## Rollback
Reverter mudanças desta tarefa; se migration já aplicada em ambiente autorizado, restaurar default SQL por migration corretiva, sem reescrever ledger nem converter sistemas existentes.

## Observações
Aguardar aprovação explícita segundo AGENTS.md. Nenhuma migration em produção, commit/push/deploy ou alteração de pontos nesta fase.

## Adição aprovada pelo usuário
Em grupos/supergrupos, aceitar somente comandos com @username deste bot, inclusive reset na recovery lane, antes de qualquer resposta. Privado mantém comandos sem sufixo. Usar helper compartilhado para handler e dispatcher e testes de rejeição de comandos bare e username errado. Autorização explícita: sim.

## Resultado
- Alias join usa handleEntrar; parseMessageCommand exige destinatário correto em grupos/supergrupos antes de respostas, e o dispatcher/reset reutilizam o filtro. Privado sem sufixo preservado.
- Default Go atualizado e migration 0008 altera apenas DEFAULT SQL. Testes de preservação Legacy, idempotência e criação via instalação/título/modo adicionados.
- Ajuda e documentação atualizadas; fixtures antigas endereçam bot explicitamente e cenários Legacy continuam explícitos.
- go test ./..., go vet ./..., go build ./... e git diff --check passaram. Telegram com tags debugcards passou. Testes SQL com tag integration compilaram com -run '^$', mas não executaram: TEST_DATABASE_URL não configurada. Sem executar migration em produção.
- Alterações no working tree da dev. Sem commit/push/deploy.

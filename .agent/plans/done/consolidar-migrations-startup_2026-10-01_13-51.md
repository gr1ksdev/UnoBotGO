# Plano: consolidar migrations no startup

## Pedido do usuário
Aplicar SQL versionado automaticamente antes de qualquer parte funcional, remover CLI separado e provar ledger/checksum/lock/timeout/fail-closed. Sem commit/push/main/deploy ou migrations em produção.

## Objetivo
Consolidar comportamento já existente, eliminar caminho operacional redundante e melhorar diagnóstico/validação/testes sem trocar migrator ou schema.

## Contexto atual
Dev, HEAD49400ba, working tree limpo. cmd/bot -> app.Run -> postgres.Open(Ping) -> Initialize(Migrate+VerifySchema), timeout interno2m, antes de game/service, Telegram, fotos/workers e HTTP. Mesmo Store/pool reutilizado. SQL0001..0007 embutido em internal/storage/postgres/migrations.go; sha256, nomes ordenados, ledger schema_migrations(version PK,checksum,applied_at), pg_advisory_xact_lock(71870101). TODAS pendentes são aplicadas em uma transação única com ledger, não uma transação por arquivo; rollback é atômico do lote inteiro. CLI cmd/migrate é redundante. README/docs ainda orientam execução manual. Docker/Compose/CI não têm wrapper/serviço/binário de migration e já iniciam só unobotgo.

Pontos de melhoria: operationError esconde causa SQL; checksum divergente retorna ErrSchema sem nome; validação de ledger é intercalada com execução pendente; não há teste explícito de parcial/ordem/tamanho ledger/timeout aguardando lock e startup real com Store.

## Arquivos analisados
- AGENTS.md, .agent/context.md
- cmd/bot/main.go, cmd/migrate/main.go
- internal/app/app.go, app_test.go
- internal/storage/postgres/migrations.go, migrations_integration_test.go, store.go
- internal/storage/postgres/migrations/0001..0007.up.sql (inventário; conteúdo permanece intacto)
- internal/config/defaults.go
- README.md, docs/m7-persistence.md, docs/miniapp.md, docs/build.md
- Makefile, Dockerfile.v2, docker-compose.yml, .github/workflows e scripts

## Arquivos que poderão ser modificados
- internal/storage/postgres/migrations.go
- novos testes unitários migrations_test.go e ampliação migrations_integration_test.go
- internal/app/app.go, app_test.go e novo teste startup PostgreSQL com tag integration
- cmd/migrate/main.go (remover)
- README.md, docs/m7-persistence.md, docs/miniapp.md, docs/build.md
- Makefile/.github/workflows/dev-ci.yml apenas para integrar novo teste de startup na suíte consolidada
- .agent/context.md, memory/memory.md, decisions.md e este plano

## Estratégia de implementação
Reutilizar Store.Migrate e SQL embed; sem novo framework/pacote paralelo. Separar pequenos helpers internos para descobrir/validar migrations e conferir ledger completo antes de executar SQL pendente. Validar convenção existente de nomes/versão e duplicação; ordenar deterministicamente. Acrescentar fs.FS como seam interno de testes para fixtures sem modificar SQL de produção. Manter API pública simples Migrate(ctx)/VerifySchema(ctx).

Preservar lote transacional inteiro e advisory lock xact com o mesmo identificador. Nenhuma migration registrada ou DDL do lote sobrevive falha. Primeiro validar todas versões/checksums já aplicadas e rejeitar desconhecidas/inconsistência de ordem, depois aplicar pendentes. Não atualizar checksums nem reconstruir ledger incompatível.

Erros de migrations com estágio, nome e causa original encadeada; apresentar mensagem PostgreSQL/SQLSTATE com cuidado para omitir detalhes/DSN/parâmetros de conexão e segredos. Não alterar operationError de persistência fora do migrator. Logs estruturados existentes: conexão confirmada sem DSN, checking, applying por arquivo, complete/up-to-date após commit, failures identificadas. Preservar cancelamento/deadline via errors.Is. Não logar SQL integral ou secrets.

Consolidar boundary de startup usando Initialize existente e política config.MigrationTimeout2m. API inicializa antes de qualquer callback funcional; evitar callback no contexto de migração cancelado após sucesso. Testar ordem/no-callback em erro/cancelamento e integração Store real + callback que verifica schema antes de simular inicialização HTTP/bot/workers. Não iniciar Telegram real. Manter listeners/readiness e pool únicos.

Remover cmd/migrate/main.go, diretório se vazio e instruções operacionais. Atualizar documentação concisa de auto-run, idempotência, checksum imutável, lock e fail-closed. Artefatos históricos .agent que citam CLI permanecem registros históricos, com contexto atual explícito.

## Passos detalhados
1. Completar inspeção dos sete SQLs e testes existentes; preservar conteúdo/checksums.
2. Refatorar helpers internos de descoberta/validação/ledger sem criar outro migrator.
3. Melhorar erros/logs e validar ledger inteiro antes do SQL pendente, mantendo transação/lock.
4. Consolidar/testar boundary startup com timeout2m, mesmo pool e inicialização funcional posterior.
5. Adicionar unitários de ordem/duplicação/checksum e integração de vazio/up-to-date/parcial/ledger/concorrrência/SQL inválido/timeout/lock/rollback.
6. Adicionar integração real do startup com Store e callback/mocks; integrar target/check/CI existente se necessário.
7. Remover CLI e referências da documentação ativa; preservar Docker/Compose sem alterações desnecessárias.
8. Executar gofmt nos alterados, go test ./..., go test -race ./..., go vet ./..., make check e make build/binário; git diff --check.
9. Confirmar por diff que SQL/schema/domínio/frontend/env/persistência de partidas não foram alterados; registrar decisões/memória, plano done e relatório.

## Riscos
- Mudança inadvertida de semântica transacional: manter uma transação única por lote, inclusive ledger, como implementação atual.
- Diagnóstico vazar detalhes: limitar texto exposto e preservar causa apenas via unwrap, não imprimir DSN/detalhes SQL; testes de sanitização.
- Timeout em advisory lock: testar contexto real e lock liberado após cancelamento sem mutação.
- Testes PostgreSQL: container efêmero separado, schemas isolados, sem usar banco aplicativo/produção.
- Erros de cleanup ou tests CI: não ignorar, investigar/repetir apenas após correção justificada.

## Impactos esperados
./unobotgo é único entrypoint operacional V2; bancos vazios/parciais são migrados pelo startup já existente com diagnóstico e cobertura melhorados. Nenhuma alteração nos SQLs publicados, regra de negócio ou UI.

## Compatibilidade
- Linux/macOS/Windows: Go/pgx padrão, SQL embutido independente de filesystem runtime.
- Docker: Node builder -> Go/embed -> distroless/nonroot, apenas unobotgo.
- CI/CD: checks existentes e teste startup integrados; sem publicação ou mudanças em main.

## Como testar

### Build
```bash
go build ./...
make build
```

### Testes
```bash
go test ./...
go test -race ./...
go vet ./...
TEST_DATABASE_URL=<banco-efemero> make check
TEST_DATABASE_URL=<banco-efemero> go test -race -tags integration ./internal/storage/postgres/... ./internal/app/...
git diff --check
```
Busca final cmd/migrate/migrate/main.go fora dos registros históricos .agent. Conferência de SQLs via git diff.

### Execução
```bash
./bin/unobotgo
```
Teste de inicialização com banco isolado e callback/mocks, sem conectar Telegram de produção.

## Rollback
Reverter somente alterações desta rodada após revisão; não executar downgrade/rollback SQL automático, não reset/clean, não descartar trabalho local.

## Observações
Auto-run já existe: esta tarefa não o reimplementa. Nenhuma razão concreta encontrada para conservar CLI separado. Plano aguarda aprovação conforme AGENTS.md; somente este arquivo criado na auditoria. Sem commit/push/main/deploy/schema novo/env novo.

## Execução concluída
- Aprovação explícita recebida para consolidar/endurecer o fluxo existente. Executado na dev, HEAD49400ba preservado e nenhum trabalho local descartado.
- Sete SQLs inspecionados e preservados byte a byte; runner permanece Store.Migrate/VerifySchema, embed e transação única do lote. CLI redundante e diretório removidos via remoção autorizada.
- Helpers readMigrations/pendingMigrations/readMigrationLedger para validação completa, descoberta determinística, versões inválidas/duplicadas, prefixo e checksums; nenhum reparo automático. FS seam interno apenas para testes; produção usa embed.
- MigrationError(stage/migration/Err) preserva unwrap/causa/ctx, mensagem SQLSTATE/Message sem detalhes de linha/DSN. Slog connected/checking/applying/complete/up-to-date/failed, sem framework ou parâmetros secretos.
- Initialize concentra timeout2m sobre migrations+verify e callback funcional recebe contexto original, nunca o deadline de migrations já cancelado. Mesmo pool e ordem fail-closed preservados.
- Unitários e integração adicionados para ordering/checksum/invalid names, completo/parcial/up-to-date/ledger intacto, inválidoSQL/rollback lote, checksum/unknown/gap antes de qualquer SQL (sequence não transacional como prova), lock timeout/recuperação, cancelamento sincronizado no PostgreSQL, exatamente uma execução via dois pools e startup Store real + mocks/no componentes em falha.
- Make check e CI PostgreSQL agora incluem internal/app com tagintegration. Docker/Compose não precisaram mudança: já iniciavam apenas unobotgo. Docs README/m7/build/miniapp atualizadas; busca final sem referências operacionais ao CLI fora de históricos .agent.
- Todos comandos previstos APROVADOS: gofmt nos alterados; go test ./...; go test -race ./...; go vet ./...; go build ./...; make check integral; make build; go test -race -count=1 -tags integration ./internal/storage/postgres/... ./internal/app/...; git diff --check. Frontend45testes/lint/typecheck/build aprovados, fonte frontend intacta.
- PostgreSQL17 em container efêmero unobotgo-migrations-audit-20261001 com porta15433 e schemas isolados; encerrado ao final sem alterar banco aplicativo. Não iniciado Telegram real nem aplicado SQL em produção. npm informou2vulnerabilidades moderadas preexistentes; dependências preservadas.
- Riscos/pendências: nenhuma falha bloqueante de validação; homologação/deploy real não executados. Nenhum commit/push/main/schema/env/frontend/domínio/migrations SQL alterado.

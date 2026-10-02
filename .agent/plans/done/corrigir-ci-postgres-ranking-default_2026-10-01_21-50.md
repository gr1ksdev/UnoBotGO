# Plano: corrigir falha do CI PostgreSQL apos migration 0008

## Pedido do usuario
Corrigir a falha no CI do GitHub Actions na branch `dev` no job PostgreSQL (`go test -race -tags integration ./internal/storage/postgres/... ./internal/app/...`), identificar a causa raiz exata e a assertion que falhou, garantir que a migration 0008 altere somente o default de novos grupos sem afetar grupos legados ou atualizados existentes, e reforçar os testes de integracao para cobrir os tres cenarios (novo grupo, grupo legacy existente, grupo updated existente).

## Objetivo
1. Identificar o teste e a linha exata da falha de integracao no PostgreSQL.
2. Corrigir o teste que falhou de forma cirurgica, sem alterar regras de negocio ou migrator.
3. Blindar a migration 0008 com testes de integracao cobrindo os tres cenarios requeridos:
   - Caso A: novo grupo sem configuracao anterior assume `Updated` por padrao;
   - Caso B: grupo existente `Legacy` permanece `Legacy` e com mesma revisao apos aplicar a migration 0008;
   - Caso C: grupo existente `Updated` permanece `Updated` e com mesma revisao apos aplicar a migration 0008.
4. Assegurar que `Legacy` e `Updated` permanecem 100% suportados e isolados.
5. Garantir aprovacao de todos os gates (`go test`, `go test -race`, `go vet`, `make check`, `git diff --check`).

## Contexto atual
- Commit `79315bd` introduziu a migration `0008_default_ranking_updated.up.sql` (`ALTER TABLE group_configs ALTER COLUMN ranking_system SET DEFAULT 'updated';`) e alterou `groups.Defaults` para `Updated`.
- A execucao do teste de integracao com banco PostgreSQL (`go test -race -count=1 -v -tags integration ./internal/storage/postgres/... ./internal/app/...`) reproduziu exatamente 1 falha:
  - Teste: `TestListGroupRankingAccumulationIsolationAndHistory`
  - Arquivo: `internal/storage/postgres/ranking_integration_test.go:55`
  - Erro: `ranking: scoring policy requires product decision`
- Causa raiz:
  - O teste valida isolamento de ranking entre grupos distintos (grupo 42 com ranking Updated e grupo 43 com ranking Legacy).
  - Na preparacao do grupo 43, o teste executava:
    ```go
    if _, err := s.GetOrCreateGroupConfig(ctx, 43); err != nil { ... }
    other := eligibleResult(t, groups.Legacy, 2, 0, "completed")
    other.GameID = "other-chat"
    other.ChatID = 43
    if _, err := s.RecordCompletedGame(ctx, other); err != nil { ... }
    ```
  - Antes da migration 0008, `GetOrCreateGroupConfig` criava o grupo com default `Legacy`. Com a migration 0008, o grupo novo e criado com default `Updated`.
  - Como o resultado `other` utilizava `groups.Legacy`, a chamada a `RecordCompletedGame` detectou incompatibilidade entre o sistema do grupo (`Updated`) e o resultado do jogo (`Legacy`), disparando `ranking.ErrNeedsProductDecision` na linha 55.
  - Ao mesmo tempo, na linha 77 o teste faz a assercao explícita: `isolated.System != groups.Legacy`, demonstrando que o teste pretendia que o grupo 43 fosse do sistema `Legacy`.
  - A correcao consiste em configurar explicitamente `s.SetRankingSystem(ctx, 43, groups.Legacy)` logo apos a criacao do grupo 43, respeitando o principio de nao assumir default quando se quer testar comportamento Legacy.

## Arquivos analisados
- `.github/workflows/dev-ci.yml`
- `internal/storage/postgres/migrations/0008_default_ranking_updated.up.sql`
- `internal/storage/postgres/groups.go`
- `internal/storage/postgres/results.go`
- `internal/storage/postgres/ranking.go`
- `internal/storage/postgres/ranking_integration_test.go`
- `internal/storage/postgres/groups_integration_test.go`
- `internal/storage/postgres/results_integration_test.go`
- `internal/storage/postgres/eligibility_integration_test.go`
- `internal/storage/postgres/migrations_integration_test.go`
- `internal/groups/groups.go`

## Arquivos que poderao ser modificados
- `internal/storage/postgres/ranking_integration_test.go`
- `internal/storage/postgres/groups_integration_test.go`

## Estrategia de implementacao
1. Em `internal/storage/postgres/ranking_integration_test.go`:
   - Configurar explicitamente o grupo 43 como `groups.Legacy` usando `s.SetRankingSystem(ctx, 43, groups.Legacy)` imediatamente apos `s.GetOrCreateGroupConfig(ctx, 43)`.
2. Em `internal/storage/postgres/groups_integration_test.go`:
   - Enriquecer `TestUpdatedDefaultMigrationPreservesExistingGroups` (ou adicionar teste dedicado) para validar explicitamente os tres cenarios solicitados:
     - **Caso A (grupo novo)**: novo grupo criado apos migracao 0008 tem `ranking_system == groups.Updated`.
     - **Caso B (grupo Legacy existente)**: grupo criado antes da 0008 com `Legacy` permanece `Legacy` com a mesma `config_revision` apos aplicacao da 0008.
     - **Caso C (grupo Updated existente)**: grupo configurado antes da 0008 com `Updated` (via `SetRankingSystem`) permanece `Updated` com a mesma `config_revision` apos aplicacao da 0008.
     - Reaplicacao idempotente: chamar `s.Migrate(ctx)` novamente e atestar estabilidade dos dados.
3. Validar:
   - Rodar o comando exato do CI com `-race` e `-tags integration`.
   - Rodar suite completa Go (`go test ./...`, `go test -race ./...`, `go vet ./...`).
   - Rodar `make check` e `git diff --check`.

## Passos detalhados
1. Aplicar a configuracao explicita `s.SetRankingSystem(ctx, 43, groups.Legacy)` no teste `TestListGroupRankingAccumulationIsolationAndHistory`.
2. Adicionar o cenario C (grupo Updated existente antes da 0008) no teste de integracao de migracao em `groups_integration_test.go`, garantindo cobertura formal dos tres casos (A, B e C).
3. Executar `go test -race -count=1 -v -tags integration ./internal/storage/postgres/... ./internal/app/...` e confirmar que 100% dos testes passam.
4. Executar `go test ./...`, `go test -race ./...`, `go vet ./...`.
5. Executar `make check` e `git diff --check`.
6. Atualizar a memoria persistente em `.agent/memory/memory.md` e registrar decisao em `.agent/decisions.md`.
7. Mover o plano para `.agent/plans/done/` e apresentar o relatorio final detalhado.

## Riscos
- **Risco**: Quebrar testes existentes ao alterar configuracoes.
  - **Mitigacao**: A alteracao e restrita ao grupo 43 de `TestListGroupRankingAccumulationIsolationAndHistory`, que ja esperava `isolated.System == groups.Legacy` nas assercoes posteriores.
- **Risco**: Mudar acidentalmente dados em producao ou migracao.
  - **Mitigacao**: Nenhuma migration SQL e modificada. Apenas testes de integracao sao corrigidos/estendidos.

## Impactos esperados
- O CI do GitHub Actions no job PostgreSQL passara com sucesso.
- O contrato de negocio (novos grupos Updated por padrao, grupos existentes inalterados) ficara blindado por testes automatizados.
- Zero impacto em regras de negocio, Mini App, frontend ou engine de jogo.

## Compatibilidade
- Linux: Sim
- macOS: Sim
- Windows: Sim
- Docker: Sim
- CI/CD: Sim (GitHub Actions `postgres` job)

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
TEST_DATABASE_URL="postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable" go test -race -count=1 -tags integration ./internal/storage/postgres/... ./internal/app/...
go test ./...
go test -race ./...
go vet ./...
make check
```

### Rollback
Reverter as alteracoes nos arquivos de teste com `git checkout -- internal/storage/postgres/ranking_integration_test.go internal/storage/postgres/groups_integration_test.go`.

## Observacoes
- A migration `0008_default_ranking_updated.up.sql` esta estritamente correta: executa apenas `ALTER TABLE group_configs ALTER COLUMN ranking_system SET DEFAULT 'updated';` sem mutar dados existentes.
- O alias `/join` nao possui qualquer relacao com a falha.

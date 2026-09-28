# Plano: corrigir observação de usuário no config

## Pedido do usuário
Confirmar se player_group_stats acumula scores entre GameIDs e investigar falha de ObserveGroupUser durante /config. Investigação concluída; correção abaixo ainda não autorizada.

## Objetivo
Garantir que a configuração do grupo exista antes de observar usuários no fluxo de configuração, preservando autorização, defaults e rankings.

## Contexto atual
Base dev@5bcc365; árvore limpa antes da auditoria. Score é acumulado por (chat_id,user_id), com idempotência por GameID. TestCompletedGameIdempotencyAndAccumulation (Legacy/Updated) e TestKnownUsersMutableNamesAndDuplicates passaram em PostgreSQL 18.6 temporário isolado.
/config observa usuário antes de CanConfigureUser, que cria group_configs. known_group_users possui FK obrigatória para group_configs. Reprodução com SQL real retornou 23503 antes da criação; mesma operação passou após criar o grupo. Callbacks cfg possuem ordem semelhante. /novo já cria configuração primeiro. operationError descarta a causa SQL, mantendo apenas operação/contexto. Mocks não reproduzem a FK.
Sem logs/banco de produção disponíveis: defeito confirmado, mas associação ao incidente específico ainda depende de evidência operacional.

## Arquivos analisados
- internal/storage/postgres/results.go e results_integration_test.go
- internal/storage/postgres/users.go e users_integration_test.go
- internal/storage/postgres/store.go, groups.go e migrations 0001/0002/0003
- internal/telegram/commands.go, callbacks.go e config_test.go
- internal/groups/groups.go
- AGENTS.md

## Arquivos que poderão ser modificados
- internal/telegram/commands.go e callbacks.go
- internal/telegram/config_test.go
- internal/storage/postgres/users_integration_test.go
- Teste de integração Telegram/PostgreSQL se necessário para cobrir a sequência real
- Registros de memória/decisões internos

## Estratégia de implementação
Garantir criação/leitura de configuração antes da observação em /config e callbacks relacionados, aproveitando os serviços existentes. Não remover a FK, não alterar GroupConfig nem tornar observação um mecanismo de autorização. Preservar comportamento fail-closed de permissões. Manter SQL de acumulação intacto. Melhoria geral de logging fica fora desta correção.

## Passos detalhados
1. Acrescentar regressão que falhe quando a observação anteceder a existência do grupo; usar mock que imponha a dependência e teste com PostgreSQL isolado.
2. Corrigir ordenação em /config e callbacks de configuração, tratando indisponibilidade de configuração sem tentar inserir usuário órfão.
3. Preservar observação de usuários conforme comportamento atual, sem autorizar configuração por observar usuário; validar admin/instalador/usuário comum.
4. Revalidar repetição, grupo novo/existente, defaults, falhas de storage e acumulação/idempotência entre partidas.
5. Rodar testes/checks e atualizar memória com resultados; sem push ou alterações na main.

## Riscos
- Corrigir observação e acidentalmente relaxar autorização.
- Observar apenas administradores por mudança involuntária de ordem.
- Confundir defeito reproduzido com causa comprovada do incidente em produção.

## Impactos esperados
- Primeiro /config em grupo ainda ausente deixa de tentar inserir usuário órfão.
- Score/ranking, gameplay, banco/schema e defaults permanecem iguais.

## Compatibilidade
Linux, macOS, Windows, Docker e CI/CD: sem dependências, migrations ou mudanças de configuração. PostgreSQL local de auditoria 18.6; CI usa PostgreSQL 17.

## Como testar
### Build
```bash
go build ./...
go vet ./...
```
### Testes
```bash
go test -count=1 ./...
go test -count=1 -tags debugcards ./...
go test -count=1 -race ./...
# Usar exclusivamente banco temporário de testes:
TEST_DATABASE_URL='<banco-temporario>' go test -count=1 -tags integration ./internal/storage/postgres/...
git diff --check
```
Race pode ser bloqueado pelo VMA 39/48 local; registrar como limitação, não aprovado.
### Execução
Homologar primeiro /config em grupo novo, repetir e testar usuário sem permissão. Não acessar/modificar produção para reproduzir.

## Rollback
Reverter somente futura correção por commit, sem reset destrutivo; não há migration nem mudança de dados de produção.

## Observações
Aguardando aprovação explícita conforme AGENTS.md. Nesta etapa, apenas investigação, testes existentes e este plano; nenhum código do produto alterado, nenhum commit/push.

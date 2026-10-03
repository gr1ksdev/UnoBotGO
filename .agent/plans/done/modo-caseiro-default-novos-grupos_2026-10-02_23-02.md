# Plano: modo-caseiro-default-novos-grupos

## Pedido do usuário
"adicione tbm que o modo caseiro seja o modo default para novos canais no inicio da configuracao"

## Objetivo
Configurar o modo `Caseiro` (`groups.Caseiro`) como o modo padrão de jogo para novos canais/grupos no início da configuração do UnoBotGO, mantendo a retrocompatibilidade com grupos existentes que já possuam configuração gravada no PostgreSQL, adicionando a migration correspondente `0010_default_mode_caseiro.up.sql` e atualizando os helpers e testes unitários e de integração pertinentes.

## Contexto atual
- Atualmente, a função `groups.Defaults(chatID int64) Config` em `internal/groups/groups.go:34-36` inicializa `DefaultGameMode: Classic` e `RankingSystem: Updated`.
- No banco de dados PostgreSQL, a tabela `group_configs` (criada em `0001_groups.up.sql`) define:
  `default_game_mode text NOT NULL DEFAULT 'classic' CHECK (default_game_mode IN ('classic','caseiro'))`.
- A mensagem de boas-vindas ao ser adicionado a um grupo (`RenderGroupWelcome` em `internal/telegram/renderer.go`) informa textualmente o modo padrão.
- Quando um novo grupo executa `/novo` sem argumentos antes de configurar o bot, a partida utiliza o modo padrão do grupo (`handleNovoObserved` em `internal/telegram/commands.go:448-469`).

## Arquivos analisados
- `internal/groups/groups.go`
- `internal/groups/groups_test.go`
- `internal/storage/postgres/groups.go`
- `internal/storage/postgres/groups_integration_test.go`
- `internal/storage/postgres/migrations/0001_groups.up.sql`
- `internal/storage/postgres/migrations/0008_default_ranking_updated.up.sql`
- `internal/storage/postgres/migrations_integration_test.go`
- `internal/telegram/commands.go`
- `internal/telegram/renderer.go`
- `internal/telegram/config_test.go`
- `internal/telegram/group_config_test.go`

## Arquivos que poderão ser modificados
- `internal/groups/groups.go`
- `internal/groups/groups_test.go`
- `internal/storage/postgres/migrations/0010_default_mode_caseiro.up.sql` (novo arquivo)
- `internal/storage/postgres/groups_integration_test.go`
- `internal/storage/postgres/migrations_integration_test.go`
- `internal/telegram/renderer.go`
- `internal/telegram/config_test.go`

## Estratégia de implementação
1. **Definição em Go (`internal/groups/groups.go`)**:
   - Alterar `groups.Defaults(chatID int64)` para:
     ```go
     func Defaults(chatID int64) Config {
         return Config{ChatID: chatID, DefaultGameMode: Caseiro, RankingSystem: Updated, Revision: 1}
     }
     ```
2. **Migration PostgreSQL (`internal/storage/postgres/migrations/0010_default_mode_caseiro.up.sql`)**:
   - Criar migration alterando apenas o DEFAULT da coluna para novas linhas criadas:
     ```sql
     -- Change only the default for newly created group configurations.
     -- Existing groups keep their configured mode, revision and scores.
     ALTER TABLE group_configs ALTER COLUMN default_game_mode SET DEFAULT 'caseiro';
     ```
   - O schema de migrações automáticas (`migrations.go`) embeda e aplica a migration incrementalmente sem impactar grupos preexistentes.
3. **Mensagem de Boas-Vindas (`internal/telegram/renderer.go`)**:
   - Atualizar `RenderGroupWelcome(config groups.Config)` para apresentar dinamicamente o modo e o ranking padrão a partir da `config` recebida (ou refletir Caseiro e Atualizado).
4. **Ajustes de Testes**:
   - `internal/groups/groups_test.go`: ajustar verificação em `TestDefaults` para `c.DefaultGameMode != Caseiro`.
   - `internal/storage/postgres/groups_integration_test.go`: ajustar o assert de default em `GetOrCreateGroupConfig` para esperar `groups.Caseiro` e testar a transição para `groups.Classic`.
   - `internal/storage/postgres/migrations_integration_test.go`: ajustar o assert de default em `TestMigrateAndVerify` para `mode == "caseiro"`.
   - `internal/telegram/config_test.go`:
     - Em `TestConfig_DefaultsAndNeverBlocked`: validar que `/novo` sem config cria jogo com `view.Rules.AllowSwapHands == true` (regras Caseiro).
     - Em `TestConfig_NonAdminCannotModify`: usuário não-admin tenta mudar para Classic (`cfg_mode_classic_%d`) e asserta que a config permaneceu `groups.Caseiro`.
     - Em `TestConfig_FirstConfigInGroupWithoutExistingConfig_Admin_ObservesUserAndCreatesConfig`: assertar `cfg.DefaultGameMode == groups.Caseiro`.
     - Em `TestConfig_FirstConfigInGroupWithoutExistingConfig_NonAdmin_ObservesUserAndPreservesAuth`: assertar `cfg.DefaultGameMode == groups.Caseiro`.
     - Em `TestConfig_AdminCanOpenAndModify`, `TestConfig_InstallerStillMemberCanConfigure`, `TestConfig_OrthogonalChanges`, `TestConfig_CallbackInNewlyCreatedGroup_ObservesUser`: adaptar callbacks e asserções para iniciarem com Caseiro e alternarem para Classic.

## Passos detalhados
1. Obter aprovação explícita do usuário.
2. Mover o plano de `.agent/plans/pending/` para `.agent/plans/approved/`.
3. Criar a migration `internal/storage/postgres/migrations/0010_default_mode_caseiro.up.sql`.
4. Alterar `groups.Defaults` em `internal/groups/groups.go`.
5. Atualizar `RenderGroupWelcome` em `internal/telegram/renderer.go`.
6. Ajustar asserções dos testes em `internal/groups/groups_test.go`, `internal/storage/postgres/groups_integration_test.go`, `internal/storage/postgres/migrations_integration_test.go` e `internal/telegram/config_test.go`.
7. Rodar a suíte completa de testes:
   - `go test -count=1 -v ./internal/groups/...`
   - `go test -count=1 -v ./internal/telegram/...`
   - `go test ./...`
   - `go test -race ./...`
   - `go test -tags debugcards ./...`
   - `go vet ./...`
   - `TEST_DATABASE_URL="postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable" go test -count=1 -tags integration ./internal/storage/postgres/... ./internal/app/...`
   - `git diff --check`
8. Atualizar `.agent/memory/memory.md` e `.agent/decisions.md`.
9. Mover o plano para `.agent/plans/done/`.
10. Apresentar relatório de conclusão ao usuário.

## Riscos
- **Impacto em grupos existentes**: Zero. A migration usa `ALTER COLUMN default_game_mode SET DEFAULT 'caseiro'`, o que altera exclusivamente a cláusula DEFAULT para novas inserções no banco; linhas existentes permanecem inalteradas.
- **Sobrescrita manual de partidas**: Nenhuma. O comando `/novo classico` continua funcionando normalmente para forçar o modo Clássico em uma partida específica quando desejado.

## Impactos esperados
- Grupos novos onde o bot é adicionado iniciarão automaticamente com o modo Caseiro ativado por padrão.
- Administradores continuam podendo alterar o modo a qualquer momento pelo menu `/config`.

## Compatibilidade
- Linux: Sim
- macOS: Sim
- Windows: Sim
- Docker: Sim
- CI/CD: Sim

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -count=1 -v ./internal/groups/...
go test -count=1 -v -run "TestConfig" ./internal/telegram/...
go test ./...
go test -race ./...
TEST_DATABASE_URL="postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable" go test -count=1 -tags integration ./internal/storage/postgres/...
git diff --check
```

### Execução
N/A (bot de Telegram com persistência).

## Rollback
Restaurar os arquivos para o estado do commit `0ae5b37` via `git checkout`.

## Observações
- A alteração segue o mesmo padrão arquitetural e de isolamento adotado na migration `0008_default_ranking_updated.up.sql`.

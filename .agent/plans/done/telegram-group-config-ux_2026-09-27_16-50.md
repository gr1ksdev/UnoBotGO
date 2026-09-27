# Plano: telegram-group-config-ux

## Pedido do usuário
Implementar a UX Telegram para configuração de grupo na Milestone M7 (persistência, configuração e ranking) na branch `dev`.
- Cada grupo possui uma configuração ativa: modo padrão (`classic` / `caseiro`) e sistema de ranking (`legacy` / `updated`).
- Defaults: `Classic` + `Legacy`. O setup nunca bloqueia o uso do bot ou de `/novo`.
- Configuração pertence ao ChatID/grupo. Mudanças futuras afetam somente partidas criadas depois; partidas já criadas preservam seus snapshots.
- `/novo` usa o default do grupo; `/novo classico` e `/novo caseiro` forçam o modo apenas naquela partida sem alterar `GroupConfig`.
- Interface baseada no comando único `/config` com botões inline para alternar modo e ranking. Callbacks validam autorização no backend e editam a mensagem de forma limpa.
- Permissão para configurar: administrador atual do grupo OU usuário que adicionou o bot (se ainda for membro atual). Falha fecha para alteração.
- Observar `my_chat_member` para detectar transição de instalação real (bot não membro -> membro), registrar `installed_by_user_id` e enviar mensagem curta opcional com botão `[ ⚙️ Configurar ]`.
- Troca `Legacy ↔ Updated`: persistir a alteração no grupo para partidas futuras sem alterar historicos/scores existentes e sem inventar conversão/migração.
- Cobertura completa de testes automatizados e preservação da branch `main` e regras existentes.

## Objetivo
Criar a interface e o fluxo completo de configuração de grupos no adapter Telegram, integrando `groups.Service`, repositórios PostgreSQL, monitoramento de `my_chat_member` e autorização estrita, acompanhado de testes unitários e de integração abrangentes.

## Contexto atual & HEAD inicial
- HEAD inicial da branch `dev`: `7756b774adaf1c38ab93bd3ad9b672d54eaff5f7` (`docs(plans): archive completed plan for telegram ranking integration`).
- O schema PostgreSQL em `internal/storage/postgres/migrations/0001_groups.up.sql` já possui a tabela `group_configs` com as colunas: `chat_id`, `default_game_mode`, `ranking_system`, `installed_by_user_id`, `installed_at`, `installation_update_id`, `config_revision`, `created_at` e `updated_at`.
- Em `internal/groups/groups.go`, `Config`, `Mode`, `RankingSystem`, `Snapshot` e `CanConfigure` já estão definidos, mas a interface `Repository` e o `Service` só possuem `SetDefaultGameMode`. Faltam `SetRankingSystem` e `SetInstalledBy`.
- Em `internal/storage/postgres/groups.go`, `Store` implementa `GetOrCreateGroupConfig` e `SetDefaultGameMode`. Falta implementar `SetRankingSystem` e `SetInstalledBy`.
- `internal/game/service.go` já aceita `CreateRequest.GroupConfig` e preserva o snapshot da sessão (`RankingSystem` e `ConfigRevision`), enquanto `handleNovoObserved` em `commands.go` já carrega a config e repassa o snapshot.
- `allowedUpdates` em `internal/telegram/bot.go` ainda não inclui `"my_chat_member"` e `processUpdate` ainda não despacha `MyChatMember`.
- `internal/telegram/commands.go` ainda não trata `/config`, e `callbacks.go` ainda não trata callbacks de configuração (`cfg_`).

## Auditoria da implementação atual

1. **GroupConfig e Modelos (`internal/groups`)**:
   - `Mode` suporta `Classic` e `Caseiro`. `RankingSystem` suporta `Legacy` e `Updated`.
   - `CanConfigure(config Config, userID int64, role Membership)` já implementa a regra exata: `isCurrentAdmin || (installedByUserID == userID && isCurrentMember)`.
   - `groups.Service` precisa ser estendido com `SetRankingSystem` e `CanConfigure` (para checagem antes de abrir a UI).

2. **Repositórios PostgreSQL (`internal/storage/postgres`)**:
   - A tabela `group_configs` suporta todas as propriedades necessárias.
   - `SetRankingSystem` atualizará `ranking_system`, incrementando `config_revision` caso o valor mude, com cláusula atômica `ON CONFLICT DO UPDATE`.
   - `SetInstalledBy` atualizará `installed_by_user_id` e `installed_at` quando uma nova instalação observável for identificada.

3. **Snapshots de Configuração por Partida**:
   - Já garantidos por `game.CreateRequest.GroupConfig` e `entry.groupConfig`. Uma partida iniciada nunca sofre mutação quando a configuração do grupo é alterada posteriormente.

4. **Comandos Telegram e Overrides de `/novo`**:
   - `/novo` já lê `config.DefaultGameMode` e respeita overrides `"classico"` e `"caseiro"`.
   - Adicionaremos `/config` no `CommandHandler` (restrito a grupos, com verificação de autorização e mensagem formatada).

5. **Callbacks e Botões**:
   - Prefixo dedicado `cfg_` para evitar colisão com `mode_` de lobby de jogo.
   - `cfg_mode_classic_<chatID>`, `cfg_mode_caseiro_<chatID>`, `cfg_rank_legacy_<chatID>`, `cfg_rank_updated_<chatID>`, `cfg_open_<chatID>`.
   - Validação de chat, ator e autorização revalidada a cada clique. Edição in-place da mensagem no Telegram sem envio de mensagens duplicadas.

6. **Tratamento de `my_chat_member`**:
   - Adicionar `"my_chat_member"` em `allowedUpdates` no `bot.go`.
   - Em `processUpdate`, despachar `update.MyChatMember` para a fila particionada do chat via `dispatcher.EnqueueChat`.
   - `HandleMyChatMember`:
     - Detectar transição de instalação real: `old_chat_member.status IN (left, kicked)` -> `new_chat_member.status IN (member, administrator)`.
     - Se `From` for usuário válido: registrar `installed_by_user_id` no repositório e atualizar `KnownGroupUser`.
     - Enviar mensagem curta de boas-vindas com botão `[ ⚙️ Configurar ]`.
     - Descartar transições irrelevantes (promoção de membro a admin, alteração de permissões ou saída do bot) sem enviar mensagens.

7. **Autorização / Admin Checks**:
   - Implementar `LookupMembership` usando `bot.GetChatMember`, mapeando status `creator`/`administrator` para `Admin=true, Member=true`, `member` para `Member=true`, `restricted` com checagem de `is_member`, e `left`/`kicked` para `false`.
   - Em falha de API ou erro de rede, falhar fechado (`ErrForbidden`).

8. **Troca de Sistema Legacy ↔ Updated**:
   - A alteração persiste em `group_configs.ranking_system` para partidas futuras.
   - O schema e repositório de resultados já protegem contra contaminação de dados históricos (retornando `ranking.ErrNeedsProductDecision` se houver stats incompatíveis na hora de pontuar). Nenhuma conversão mágica ou alteração retroativa é feita.

## Arquivos analisados
- `AGENTS.md`
- `README.md`
- `docs/project-status.md`
- `docs/m7-persistence.md`
- `docs/branching.md`
- `internal/groups/groups.go`
- `internal/groups/groups_test.go`
- `internal/groups/users.go`
- `internal/storage/postgres/groups.go`
- `internal/storage/postgres/groups_integration_test.go`
- `internal/storage/postgres/migrations/0001_groups.up.sql`
- `internal/storage/postgres/results.go`
- `internal/game/service.go`
- `internal/game/manager.go`
- `internal/telegram/bot.go`
- `internal/telegram/commands.go`
- `internal/telegram/callbacks.go`
- `internal/telegram/renderer.go`

## Arquivos que poderão ser modificados
- `internal/groups/groups.go`
- `internal/groups/groups_test.go`
- `internal/storage/postgres/groups.go`
- `internal/storage/postgres/groups_integration_test.go`
- `internal/telegram/bot.go`
- `internal/telegram/commands.go`
- `internal/telegram/callbacks.go`
- `internal/telegram/renderer.go`
- `internal/telegram/config_test.go` (novos testes para cobrir os 23 cenários)
- `docs/m7-persistence.md`
- `docs/project-status.md`
- `README.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`

## Estratégia de implementação

1. **Camada de Domínio (`internal/groups`)**:
   - Expandir a interface `Repository` com `SetRankingSystem` e `SetInstalledBy`.
   - No `groups.Service`, adicionar `SetRankingSystem`, `SetInstalledBy` e `CanConfigure`.
   - Adicionar testes unitários em `groups_test.go`.

2. **Camada de Armazenamento (`internal/storage/postgres`)**:
   - Implementar `SetRankingSystem` e `SetInstalledBy` em `internal/storage/postgres/groups.go`.
   - Adicionar testes de integração em `groups_integration_test.go` cobrindo a persistência atômica, idempotência e incremento de revisão.

3. **Camada de Apresentação e Botões (`internal/telegram`)**:
   - No `Renderer`, implementar:
     - `RenderGroupConfig(config groups.Config) string`
     - `RenderGroupWelcome(config groups.Config) string`
   - Implementar `makeGroupConfigButtons(config groups.Config) *telego.InlineKeyboardMarkup` e `makeGroupWelcomeButtons(chatID int64) *telego.InlineKeyboardMarkup`.
   - Em `bot.go`:
     - Adicionar `"my_chat_member"` em `allowedUpdates`.
     - Implementar `lookupMembership(ctx, chatID, userID) (groups.Membership, error)`.
     - Instanciar e conectar `groups.Service` no `CommandHandler` e `CallbackHandler`.
     - Registrar `/config` no menu de comandos de grupos.
   - Em `commands.go`:
     - Tratar `/config` em grupos (verificar permissão, exibir configuração atual com botões).
     - Implementar `HandleMyChatMember(ctx, update *telego.ChatMemberUpdated)`:
       - Filtrar transição real de instalação (`old: left/kicked -> new: member/admin`).
       - Registrar instalador em `GroupConfig` e `KnownGroupUser`.
       - Enviar mensagem de boas-vindas/setup com botão `[ ⚙️ Configurar ]`.
   - Em `callbacks.go`:
     - Tratar prefixos `cfg_mode_`, `cfg_rank_` e `cfg_open_`.
     - Revalidar autorização a cada clique via `groups.Service.CanConfigure`.
     - Persistir a opção selecionada, atualizar `KnownGroupUser` e editar a mensagem com `EditMessageText`.
     - Responder ao callback com alerta explicativo em caso de recusa.

4. **Testes Obrigatórios (`internal/telegram/config_test.go`)**:
   - Cobrir rigorosamente os 23 cenários listados no pedido:
     1. Grupo sem config explícita -> Classic + Legacy.
     2. `/novo` sem argumento -> usa default do grupo.
     3. `/novo classico` -> override daquela partida.
     4. `/novo caseiro` -> override daquela partida.
     5. Mudar default durante partida ativa -> partida mantém snapshot.
     6. Admin pode abrir `/config`.
     7. Admin pode alterar modo.
     8. Admin pode alterar ranking.
     9. Usuário comum não pode alterar.
     10. Installer conhecido e ainda membro pode alterar.
     11. Installer conhecido que saiu não pode alterar.
     12. Installer desconhecido -> admins continuam funcionando.
     13. Callback refaz autorização.
     14. Callback com valor inválido é recusado.
     15. Trocar Classic ↔ Caseiro não altera RankingSystem.
     16. Trocar Legacy ↔ Updated não altera DefaultGameMode.
     17. Update `my_chat_member` de instalação real registra installer quando há ator.
     18. Update irrelevante de `my_chat_member` não reenvia setup.
     19. Bot removido/re-adicionado substitui installer anterior.
     20. Setup ignorado -> `/novo` funciona normalmente com defaults.
     21. Configuração persistida sobrevive a reinício.
     22. Alteração de config não altera partida já criada.
     23. `KnownGroupUser` atualizado nos novos pontos de interação.

5. **Documentação & Rastreabilidade**:
   - Atualizar `docs/m7-persistence.md`, `docs/project-status.md`, `README.md`, `.agent/context.md`, `.agent/memory/memory.md` e `.agent/decisions.md`.

## Passos detalhados
1. Mover este plano para `.agent/plans/approved/` após aprovação explícita.
2. Implementar extensões em `internal/groups/groups.go` e seus testes unitários em `groups_test.go`.
3. Implementar métodos em `internal/storage/postgres/groups.go` e testes de integração em `groups_integration_test.go`.
4. Implementar renderização e botões em `internal/telegram/renderer.go` e `commands.go`.
5. Implementar `HandleMyChatMember` e `/config` em `internal/telegram/commands.go` e `bot.go`.
6. Implementar callbacks `cfg_` em `internal/telegram/callbacks.go`.
7. Criar a suíte completa de testes em `internal/telegram/config_test.go` cobrindo os 23 cenários.
8. Executar validações:
   - `go test ./...`
   - `go test -race ./...`
   - `go test -tags debugcards ./...`
   - `go vet ./...`
   - `go build ./...`
   - `git diff --check`
9. Atualizar documentação e registrar memória persistente.
10. Organizar commits pequenos e descritivos na branch `dev` (sem push).
11. Mover plano para `.agent/plans/done/` e apresentar o relatório final.

## Riscos
- **Spam por updates repetidos de `my_chat_member`:** mitigado por verificação estrita de status prévio (`left`/`kicked`) para status novo (`member`/`administrator`). Updates de permissões existentes não disparam boas-vindas.
- **Vazamento de autorização em callbacks:** mitigado por revalidação obrigatória de associação via `GetChatMember` a cada clique no callback handler, falhando fechado.
- **Conflito de sistema de ranking com partidas em andamento:** mitigado por manter o snapshot da partida congelado na criação; a partida em andamento finaliza usando seu próprio snapshot.

## Impactos esperados
- Experiência completa e intuitiva de configuração de grupo diretamente no chat do Telegram com `/config`.
- Boas-vindas claras e objetivas quando o bot é adicionado a um novo grupo.
- Rastreamento transparente de quem instalou o bot no grupo.
- Manutenção dos defaults Classic + Legacy sem qualquer bloqueio para quem preferir não configurar.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD.
- Nenhuma dependência externa nova adicionada ao `go.mod`.

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
go test -tags debugcards ./...
go vet ./...
git diff --check
```

## Rollback
Em caso de problemas, reverter os commits gerados com `git revert` na branch `dev`.

## Observações
- Não altera `main`.
- Não faz push sem autorização explícita.
- Não inventa migração de scores antigos Legacy ↔ Updated.

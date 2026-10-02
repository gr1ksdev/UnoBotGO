# Plano: promover-publicar-main

## Pedido do usuário
"faca o commit limpo para a main"

## Objetivo
Promover o estado aprovado, testado e corrigido da branch `dev` (incluindo o cancelamento por criador/administradores, modo anônimo no ranking/perfil, layout do mini app e correção do teste de lifecycle) para o histórico limpo e independente da branch `main`, em conformidade estrita com `docs/branching.md` e o workflow de segurança `public-tree.yml`, e publicar a nova versão gerando o build multi-arquitetura OCI no GHCR via `main-container.yml`.

## Contexto atual
- A branch `dev` está no commit `bbb1542`, com todas as validações de backend (Go 100% test, race, vet, integration) aprovadas.
- A branch `main` possui histórico independente e limpo (último release em `6feef1d`).
- Conforme a arquitetura definida em `docs/branching.md`:
  - `dev` é a branch de desenvolvimento contendo o histórico detalhado;
  - `main` é a publicação pública limpa do UnoBotGO V2;
  - **Nunca deve ser feito merge direto de `dev` em `main`**.
  - A árvore pública na `main` é auditada rigorosamente pelo workflow `.github/workflows/public-tree.yml`, que proíbe artefatos internos (`.agent/`, `AGENTS.md`, `.reports/`, `.opencode/`, `codemaps/`), arquivos V1 legados na raiz (`*.go` na raiz), `docker-compose.yml` e `Dockerfile` legados, e fotos brutas.
- Novidades a serem promovidas para `main`:
  1. `/cancelar` e `/kill` permitidos para criador original, líder atual e administradores do grupo;
  2. Modo anônimo no ranking global, menu de privacidade `/config` e aba Perfil com avatar/nome e conversão dinâmica;
  3. Correção de layout/footer do Mini App e navegação de header no Telegram;
  4. Migration `0009_ranking_privacy.up.sql` e métodos de persistência de privacidade;
  5. Correção de flakiness no helper `readyToFinish` para cartas `SwapHands` em regras `caseiro`;
  6. Script de inicialização unificado e configs sincronizadas.

## Arquivos analisados
- `docs/branching.md`
- `.github/workflows/public-tree.yml` (na `main`)
- `.github/workflows/main-container.yml`
- `.agent/plans/done/promover-publicar-main_2026-10-02_03-15.md`
- Histórico de `origin/main` (`6feef1d`) e `origin/dev` (`bbb1542`)

## Arquivos que poderão ser modificados
- **Na branch `main`**:
  - `cmd/`
  - `internal/`
  - `web/`
  - `docs/`
  - `assets/`
  - `Makefile`
  - `Dockerfile.v2`
  - `README.md`
  - `.dockerignore`
  - `.env.example`
  - `.gitignore`
  - `go.mod`
  - `go.sum`
  - `.github/workflows/main-container.yml`
  - `.github/workflows/public-tree.yml`
- **Na branch `dev`**:
  - `.agent/memory/memory.md`
  - `.agent/decisions.md`
  - Este plano, movido para `approved/` e depois `done/`.

## Estratégia de implementação
1. Utilizar um worktree git isolado temporário (`.worktrees/main`) apontando para `origin/main`.
2. Sincronizar os componentes públicos V2 a partir da branch `dev` via allowlist estrita:
   - `cmd/`, `internal/`, `web/`, `docs/`, `assets/`, `Makefile`, `Dockerfile.v2`, `README.md`, `.dockerignore`, `.env.example`, `.gitignore`, `go.mod`, `go.sum`, `.github/workflows/main-container.yml`, `.github/workflows/public-tree.yml`.
   - Garantir exclusão estrita de `web/node_modules/` e `web/dist/*` (preservando `web/dist/.keep`).
3. Executar auditoria estrita contra a regex de proibição do `public-tree.yml`:
   ```bash
   forbidden='^(\.agent/|\.opencode/|\.reports/|AGENTS\.md$|codemaps/|docker-compose\.yml$|Dockerfile$|docs/v2-audit\.md$|main\.go$|game\.go$|gamemanager\.go$|ranking\.go$|match\.go$|inline\.go$|commands\.go$|actions\.go$|player\.go$|deck\.go$|card\.go$|results\.go$|errors\.go$|cmd_)'
   ```
4. Executar bateria de testes e validação na árvore de `main`:
   - `go test -count=1 ./...`
   - `go test -race ./...`
   - `go vet ./...`
   - `go build ./...`
   - `git diff --check`
5. Realizar commit atômico no histórico independente da `main`:
   `feat(v2): release group admin cancel, privacy mode, and stability updates`
6. Realizar o push para `origin main`, disparando a validação de `public-tree` e o build multi-arquitetura OCI via `main-container.yml`.
7. Remover o worktree temporário `.worktrees/main`.
8. Documentar a promoção em `.agent/memory/memory.md` e `.agent/decisions.md` na branch `dev` e arquivar o plano em `done/`.

## Passos detalhados
1. Obter aprovação explícita do usuário.
2. Mover plano para `.agent/plans/approved/`.
3. Criar worktree temporário: `git worktree add .worktrees/main origin/main`.
4. Copiar arquivos permitidos de `dev` para `.worktrees/main`.
5. Executar verificação da regex de `public-tree.yml` para assegurar conformidade de 100%.
6. Executar testes Go, race, vet e build dentro de `.worktrees/main`.
7. Executar `git add -A` e verificar `git status` no worktree.
8. Criar o commit limpo na `main`.
9. Executar `git push origin main`.
10. Remover o worktree: `git worktree remove --force .worktrees/main`.
11. Atualizar `.agent/memory/memory.md` e `.agent/decisions.md` na `dev`.
12. Mover o plano para `.agent/plans/done/`.
13. Reportar os detalhes da publicação e o link do commit na `main`.

## Riscos
- **Vazamento de arquivos internos para a `main`**:
  - Mitigação: Uso de allowlist estrita e verificação local com a exata regex do `public-tree.yml` antes do commit.
- **Falha de compilação ou testes na `main`**:
  - Mitigação: Execução prévia de `go test ./...`, `go test -race ./...`, `go vet ./...` e `go build ./...` dentro do worktree.

## Impactos esperados
- A branch `main` receberá as novas funcionalidades e correções de estabilidade em um commit limpo.
- A imagem de container multi-arquitetura (`ghcr.io/gr1ksdev/unobotgo:latest`) será construída e publicada automaticamente pelo GitHub Actions.

## Compatibilidade
- Linux AMD64 e ARM64: Sim
- GitHub Actions CI/CD: Sim

## Como testar

### Build
```bash
go build ./...
go build -o bin/unobot ./cmd/bot
```

### Testes
```bash
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

### Rollback
Em `main`, reverter o commit de release com um commit de reversão via `git revert` e push. Nunca fazer force push em branches públicas remotas.

## Observações
- A publicação na `main` não realiza merge com histórico sujo da `dev`; ela preserva a linearidade do changelog público do V2.

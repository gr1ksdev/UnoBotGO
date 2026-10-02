# Plano: promover e publicar versao aprovada na branch main

## Pedido do usuario
Fazer o commit e push para a branch `main`.

## Objetivo
Promover o estado aprovado e testado da branch `dev` para o historico independente da branch `main`, mantendo rigorosamente a arvore publica limpa (sem artefatos de agentes, sem relatorios, sem codemaps, sem mockups/fotos, sem Docker legado e sem arquivos V1 da raiz), preservando a compatibilidade com o pipeline `public-tree.yml` e disparando a construcao e publicacao da imagem multi-arquitetura (`linux/amd64` e `linux/arm64`) no GitHub Packages (GHCR) via `main-container.yml`.

## Contexto atual
- A branch `dev` esta sincronizada com `origin/dev` no commit `44dc5f1` (CI PostgreSQL 100% verde).
- A branch `main` possui historico independente em `6eea6c1` (`origin/main`).
- Conforme definido em `docs/branching.md` e na decisao arquitetural do projeto:
  - `dev` contem o historico completo de desenvolvimento;
  - `main` e uma publicacao limpa e independente contendo exclusivamente o V2 publico e documentacao humana;
  - Nunca deve ser feito merge direto de `dev` em `main`.
- A arvore publica na `main` nao pode conter:
  - `.agent/`, `AGENTS.md`, `.reports/`, `.opencode/`, `codemaps/`;
  - Fontes legados V1 na raiz (`*.go` na raiz);
  - `docker-compose.yml`, `Dockerfile` legado;
  - Imagens/mockups em formato bruto na raiz (`Ranking do Grupo em Estilo UNO.png`);
  - `docs/v2-audit.md`.
- Componentes V2 novos a serem promovidos para a `main`:
  - Mini App Telegram (`web/` e `internal/httpapi`);
  - Proxy/cache de midia (`internal/media`);
  - Inicializador unificado com migracao automatica (`internal/app`);
  - Migrations 0006, 0007 e 0008 (`internal/storage/postgres/migrations/`);
  - Ranking mensal e desempate no Telegram e Mini App;
  - Alias `/join` e comandos enderecados `@bot` em grupos;
  - CLI `cmd/devseed` e pacote `internal/devseed`;
  - Workflow `main-container.yml` atualizado para validar web e Go;
  - Documentacao publica atualizada (`docs/miniapp.md`, etc.).

## Arquivos analisados
- `docs/branching.md`
- `.github/workflows/public-tree.yml` (na `main`)
- `.github/workflows/main-container.yml`
- `.github/workflows/dev-ci.yml`
- `Dockerfile.v2`
- `README.md`
- `.agent/plans/done/publicar-main-multiarch_2026-09-24_16-18.md`
- Historico e arvore de `origin/main` (`6eea6c1`) e `origin/dev` (`44dc5f1`)

## Arquivos que poderao ser modificados
- **Na branch `main`**:
  - `cmd/bot/`
  - `cmd/simulator/`
  - `cmd/devseed/`
  - `cmd/migrate/` (removido, consolidado em `internal/app`)
  - `internal/` (todos os pacotes publicos V2)
  - `web/` (codigo-fonte do Mini App e configs, mantendo `web/dist/.keep` e excluindo `node_modules` e `dist/*`)
  - `docs/` (documentacoes publicas V2)
  - `.github/workflows/main-container.yml` e `.github/workflows/public-tree.yml`
  - `Dockerfile.v2`, `README.md`, `.gitignore`, `.dockerignore`, `.env.example`, `go.mod`, `go.sum`
- **Na branch `dev`**:
  - `.agent/memory/memory.md`
  - `.agent/decisions.md`
  - Este plano, movido para `approved` e `done`.

## Estrategia de implementacao
1. Criar um worktree temporario isolado apontando para `origin/main` (`git worktree add ... origin/main`).
2. Sincronizar a arvore publica de `main` a partir de `dev` utilizando allowlist estrita dos componentes V2:
   - Copiar `cmd/`, `internal/`, `web/`, `docs/`, `assets/`, `Dockerfile.v2`, `README.md`, `.dockerignore`, `.env.example`, `.gitignore`, `go.mod`, `go.sum`, `.github/workflows/main-container.yml`.
   - Remover `cmd/migrate` (consolidado em `internal/app`).
   - Garantir que `web/node_modules/` e `web/dist/` (exceto `.keep`) nao sejam copiados.
3. Auditar a arvore do worktree contra o verificador `public-tree`:
   - Nenhum arquivo proibido por `public-tree.yml` (`.agent`, `.reports`, `AGENTS.md`, `*.go` na raiz, `*.png` na raiz, `docker-compose.yml`, `Dockerfile` legado, etc.).
4. Executar bateria de validacao completa dentro do worktree de `main`:
   - `npm --prefix web ci && npm --prefix web run lint && npm --prefix web run typecheck && npm --prefix web run test && npm --prefix web run build`
   - `go test ./...`
   - `go test -race ./...`
   - `go vet ./...`
   - `go build ./...`
   - `git diff --check`
   - Validacao estrita com regex de `public-tree.yml`.
5. Criar commit descritivo no historico de `main`:
   - Mensagem: `feat(v2): release Mini App, monthly ranking, unified app and V2 updates`
6. Fazer push de `main` para `origin main`:
   - Dispara a validacao do GitHub Actions (`public-tree.yml` e `main-container.yml`) e a publicacao no GHCR (`ghcr.io/gr1ksdev/unobotgo:latest`).
7. Remover o worktree temporario (`git worktree remove`).
8. Atualizar memoria e registrar decisao na branch `dev`.

## Passos detalhados
1. Mover este plano para `.agent/plans/approved/` apos aprovacao do usuario.
2. Criar worktree temporario `.worktrees/main` a partir de `origin/main`.
3. Atualizar arquivos publicos de V2 a partir de `dev`.
4. Remover `cmd/migrate` em `main`.
5. Verificar e limpar qualquer arquivo proibido ou nao versionavel.
6. Rodar os testes e builds no worktree de `main`.
7. Executar `git add -A` e verificar `git status` no worktree de `main`.
8. Executar `git commit` com mensagem padronizada no historico proprio de `main`.
9. Executar `git push origin main`.
10. Remover `.worktrees/main`.
11. Atualizar `.agent/memory/memory.md` e `.agent/decisions.md` em `dev`, e mover plano para `done`.

## Riscos
- **Vazamento de artefatos internos para a `main`**:
  - Mitigacao: Verificacao estrita com o regex de `public-tree.yml` antes do commit e do push.
- **Divergencia entre `dev` e `main`**:
  - Mitigacao: `dev` permanece como fonte de verdade do codigo V2. `main` recebe a arvore publica espelhada.
- **Falha no CI do GitHub Actions em `main`**:
  - Mitigacao: Execucao local previa de todos os passos do workflow (`npm ci`, `npm test`, `npm build`, `go test -race`, `go vet`, `go build`, `git diff --check`).

## Impactos esperados
- A branch `main` contera a versao mais recente e estavel do UnoBotGO V2 com Mini App Telegram.
- O GitHub Actions construira e publicara a imagem multi-arquitetura OCI atualizada no GHCR (`ghcr.io/gr1ksdev/unobotgo:latest` e `sha-<commit>`).
- O historico da `main` permanecera 100% limpo de arquivos internos.

## Compatibilidade
- Linux AMD64 e ARM64: Sim (via Dockerfile.v2 e GHCR).
- GitHub Actions CI/CD: Sim (`public-tree` e `main-container`).

## Como testar

### Build
```bash
npm --prefix web run build
go build ./...
```

### Testes
```bash
npm --prefix web run test
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

### Rollback
Em `main`, reverter o commit de release com um novo commit e fazer push para restabelecer a imagem anterior no GHCR. Nunca utilizar `git reset --hard` ou force push em branches publicas remotas.

## Observacoes
- A publicacao de containers no GHCR e automatica assim que o push em `main` e aceito pelo GitHub.
- Nenhuma chave de producao ou arquivo confidencial existe no repositorio.

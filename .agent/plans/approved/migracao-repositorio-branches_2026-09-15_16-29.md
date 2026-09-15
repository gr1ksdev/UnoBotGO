# Plano: migracao-repositorio-branches

## Pedido do usuário
Migrar o projeto UnoBotGO V2 para o repositório remoto `git@github.com:gr1ksdev/UnoBotGO.git` organizando duas branches distintas:
1. `dev`: preserva o histórico integral atual (commits das Milestones 1, 2 e 3) e mantém os arquivos e diretórios de fluxo de IA (`.agent/`, `AGENTS.md`, planos, memórias e decisões).
2. `main`: branch pública, limpa e profissional do V2, contendo apenas os arquivos de produção do V2 (código, documentação do produto, configurações necessárias, Dockerfile e testes), sem arquivos de workflow de IA e sem arquivos legados mortos do V1 em seu histórico ou working tree.
3. Estabelecer estratégia segura para evitar poluição acidental da `main` no futuro e fornecer os comandos exatos de push para o novo remote.

## Objetivo
Configurar a coexistência entre `dev` e `main` via Git:
- Criar a branch `dev` a partir do HEAD atual para manter 100% do histórico e os artefatos de desenvolvimento.
- Criar a branch `main` limpa e sem resíduos de IA nem código morto do V1, via branch órfã com commit inicial de release V2.
- Adicionar `.agent/` e `AGENTS.md` ao `.gitignore` da `main`.
- Atualizar/adicionar o remote `origin` apontando para `git@github.com:gr1ksdev/UnoBotGO.git` (mantendo o antigo preservado como `upstream` ou `old-origin`).
- Fornecer os comandos exatos para publicação no GitHub sem comandos destrutivos.

## Contexto atual
- A Milestone 3 foi concluída com sucesso com todos os testes passando (`go test -count=1 -race ./...`).
- O branch local atual `main` possui 7 commits acima de `origin/main` (commits `d7af8f8` a `376b551`).
- O repositório local atualmente rastreia arquivos legados do V1 na raiz (`actions.go`, `card.go`, `game.go`, `player.go`, `main.go`, `gamemanager.go`, `results.go`, `match.go`, etc.) e arquivos de IA (`.agent/`, `AGENTS.md`, `.opencode/`, `.reports/`).
- O remote atual está configurado para `https://github.com/leirbagxis/UnoBotGO.git`.

## Arquivos analisados
- `.gitignore`
- `.dockerignore`
- `Dockerfile` e `Dockerfile.v2`
- `Makefile`
- `README.md`
- `go.mod` e `go.sum`
- `cmd/bot/main.go`
- `internal/config/*`, `internal/game/*`, `internal/telegram/*`, `internal/uno/*`
- `docs/*`
- `AGENTS.md`

## Arquivos que poderão ser modificados / organizados
- Branch `dev`:
  - Preserva todos os arquivos existentes e o histórico completo até o commit `376b551`.
- Branch `main` (branch órfã limpa):
  - Inclusões/Manutenções:
    - `cmd/bot/main.go`
    - `internal/config/*`
    - `internal/game/*`
    - `internal/telegram/*`
    - `internal/uno/*`
    - `docs/v2-rules.md`, `docs/v2-application.md`, `docs/v2-telegram.md`, `docs/v2-audit.md`
    - `README.md` (ajustado para focar no UnoBotGO V2)
    - `Dockerfile` (substituído pelo `Dockerfile.v2` otimizado)
    - `Makefile` (atualizado com comandos V2: run, test, vet, build)
    - `.dockerignore`
    - `.env.example`
    - `.gitignore` (incluindo `.agent/` e `AGENTS.md` para blindar a `main`)
    - `go.mod` e `go.sum`
  - Exclusões completas na `main`:
    - `.agent/`, `AGENTS.md`, `.opencode/`, `.reports/`
    - Código legado V1 da raiz (`actions.go`, `card.go`, `deck.go`, `game.go`, `gamemanager.go`, `inline.go`, `main.go`, `player.go`, `results.go`, `match.go`, `ranking.go`, `cmd_game.go`, `cmd_challenge.go`, `cmd_ranking.go`, `config.go`, `errors.go`, `docker-compose.yml`, `codemaps/`)

## Estratégia de implementação

1. **Preservação de `dev`**:
   - Criar e mudar para a branch `dev` apontando para o HEAD atual:
     ```bash
     git checkout -b dev
     ```
   - A branch `dev` mantém todo o histórico de M1, M2 e M3, histórico do V1, e todo o diretório `.agent/` e `AGENTS.md`.

2. **Criação da `main` limpa via branch órfã**:
   - Criar uma branch órfã temporária ou sobrescrever a `main` local:
     ```bash
     git checkout --orphan main-clean
     ```
   - Limpar o stage de arquivos legados e de IA.
   - Manter no stage apenas os arquivos V2.
   - Atualizar o `.gitignore` na `main` para ignorar explicitamente `.agent/` e `AGENTS.md`.
   - Renomear `Dockerfile.v2` para `Dockerfile` na `main`.
   - Adaptar `Makefile` na `main` para os alvos V2 (`run`, `test`, `build`, `vet`).
   - Criar o commit inicial limpo: `feat(v2): initial release of UnoBotGO V2`.
   - Substituir o ponteiro local da `main` para apontar para esse commit limpo.

3. **Blindagem contra poluição futura entre `dev` e `main`**:
   - No `.gitignore` da `main`, incluir explicitamente `.agent/` e `AGENTS.md`.
   - Fluxo de promoção de `dev` para `main`:
     Para lançar novas versões ou milestones na `main`, não utilizar `git merge dev` diretamente (pois traria os commits históricos com `.agent/`).
     O fluxo recomendado de sincronização será:
     `git checkout main`
     `git checkout dev -- cmd/ internal/ docs/ go.mod go.sum ...`
     Ou utilizar squash/cherry-pick de código produtivo, garantindo que `.agent/` nunca seja commitado na `main`.

4. **Configuração de Remotes**:
   - Renomear o remote `origin` atual para `upstream` (ou `old-origin`) para preservar sem perda de referências:
     ```bash
     git remote rename origin old-origin
     ```
   - Adicionar o novo remote `origin`:
     ```bash
     git remote add origin git@github.com:gr1ksdev/UnoBotGO.git
     ```
   - Fornecer os comandos para o usuário publicar ambas as branches.

## Passos detalhados

1. Criar branch `dev` a partir do HEAD atual (`git checkout -b dev`).
2. Configurar remote `old-origin` e novo `origin` (`git@github.com:gr1ksdev/UnoBotGO.git`).
3. Criar branch órfã `main-clean` (`git checkout --orphan main-clean`).
4. Remover do index os arquivos V1 legados e os arquivos de IA.
5. Ajustar `.gitignore`, `Dockerfile`, `Makefile` e `README.md` na `main-clean`.
6. Validar compilação e testes na `main-clean` (`go test -count=1 -race ./...`).
7. Fazer o commit inicial na `main-clean`: `feat(v2): initial release of UnoBotGO V2`.
8. Atualizar a branch local `main` para apontar para `main-clean` e deletar a branch temporária.
9. Voltar para a branch de trabalho desejada (`dev`) para manter o ambiente do agente funcional.
10. Validar status e diff de ambas as branches.

## Riscos
- Risco: perda acidental de histórico de M1..M3 na `dev`.
  - Mitigação: a branch `dev` é criada diretamente do HEAD atual antes de qualquer manipulação de branch órfã.
- Risco: `.agent/` desaparecer do disco ao alternar para a branch `main`.
  - Mitigação: na `main` o `.agent/` não é rastreado. Ao alternar para `dev`, o Git restaura os arquivos rastreados da `dev`. Como precaução, a branch padrão de desenvolvimento local do agente continuará sendo a `dev`.
- Risco: comando de push para o novo remote falhar por falta de permissão SSH no terminal do agente.
  - Mitigação: os comandos de push serão fornecidos de forma pronta para o usuário executar no seu terminal caso as chaves SSH estejam no keyring local do usuário.

## Impactos esperados
- A branch `main` terá histórico 100% limpo, sem rastros de IA, sem referências a agentes e sem dívidas técnicas do V1.
- A branch `dev` manterá o histórico contínuo completo.
- O repositório remoto apontará para `git@github.com:gr1ksdev/UnoBotGO.git`.

## Compatibilidade
- Linux: Sim
- macOS: Sim
- Windows: Sim
- Docker: Sim
- CI/CD: Sim

## Como testar

### Build
```bash
go build -o unobot ./cmd/bot
```

### Testes
```bash
go test -count=1 -race ./...
```

### Execução
```bash
go run ./cmd/bot
```

## Rollback
- Caso necessário desfazer a reestruturação local, o Git reflog mantém todas as referências do HEAD anterior (`376b551`), bastando executar `git checkout -B main 376b551` e restaurar o remote com `git remote rename old-origin origin`.

## Observações
- Nenhuma alteração da Milestone 4 será iniciada neste procedimento.

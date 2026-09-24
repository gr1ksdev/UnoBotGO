# Plano: publicar-main-multiarch

## Pedido do usuário
Publicar na branch `main` uma versão limpa do estado aprovado em `dev` e ampliar o build de containers para produzir imagens `linux/amd64` e `linux/arm64`, sem levar relatórios, arquivos de agente, código legado ou outros artefatos internos para a branch pública.

## Objetivo
Atualizar e validar o pipeline multi-arquitetura em `dev`, promover somente a árvore pública V2 para o histórico independente de `main` e disparar a publicação no GHCR de um manifesto único com variantes AMD64 e ARM64 nas tags `latest` e `sha-<commit>`.

## Contexto atual
- `dev` está limpa e sincronizada com `origin/dev` no commit `fcbccfd`.
- `main` está em `bd6af6e` e mantém histórico próprio. Ela não deve receber merge direto de `dev`.
- A árvore pública atual possui apenas o bot V2, testes e documentação humana; `.agent`, `.reports`, `AGENTS.md`, V1 e arquivos de desenvolvimento são proibidos por `.github/workflows/public-tree.yml`.
- Os workflows de `dev` e `main` constroem apenas `linux/amd64`.
- `Dockerfile.v2` já usa `TARGETOS` e `TARGETARCH`, mas o estágio builder ainda não está fixado em `$BUILDPLATFORM`.
- A imagem final `gcr.io/distroless/static-debian12:nonroot` oferece suporte às arquiteturas pretendidas e não executa comandos no estágio final.
- O ambiente local atual não possui Docker nem GitHub CLI. A validação local de código e cross-build Go é possível; o build multiarch real e a publicação serão realizados pelo GitHub Actions.

## Arquivos analisados
- `AGENTS.md`
- `.github/workflows/dev-ci.yml`
- `.github/workflows/main-container.yml`
- `.github/workflows/public-tree.yml` na `main`
- `Dockerfile.v2`
- `README.md`
- `docs/branching.md`
- `docs/build.md` na `main`
- `.dockerignore`
- `.gitignore`
- `.env.example`
- árvore versionada de `origin/main`
- diferenças entre `origin/main` e `dev`

## Arquivos que poderão ser modificados

### Em `dev`
- `Dockerfile.v2`
- `.github/workflows/dev-ci.yml`
- `.github/workflows/main-container.yml`
- `README.md`
- `docs/branching.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- este plano, movido entre `pending`, `approved` e `done`

### Na árvore pública de `main`
- `Dockerfile.v2`
- `.github/workflows/main-container.yml`
- `README.md`
- `docs/branching.md`
- `docs/build.md`
- arquivos públicos V2 alterados desde a última promoção em `cmd/bot`, `internal/config`, `internal/game`, `internal/telegram` e `internal/uno`
- `cmd/simulator` e `internal/simulation`, pois são componentes V2 documentados e testados, sem credenciais nem relatórios gerados

`main` não receberá `.agent`, `.opencode`, `.reports`, `AGENTS.md`, `Dockerfile` legado, `docker-compose.yml`, fontes V1 da raiz, `docs/v2-audit.md`, codemaps ou o workflow exclusivo de `dev`.

## Estratégia de implementação
O `Dockerfile.v2` usará o builder na arquitetura nativa do runner (`FROM --platform=$BUILDPLATFORM`) e continuará compilando o binário estático com `GOOS=$TARGETOS` e `GOARCH=$TARGETARCH`. Isso evita executar o compilador Go ARM64 por emulação e mantém a imagem final específica de cada plataforma.

Os dois workflows Buildx passarão a validar/publicar `linux/amd64,linux/arm64`. A `main` publicará um manifest list por tag, permitindo que Docker selecione automaticamente a variante correta. O CI de `dev` validará as duas plataformas sem publicação.

A promoção será feita em um worktree temporário baseado em `origin/main`. A árvore pública será atualizada por allowlist a partir de `dev`, preservando os controles exclusivos de `main`. Antes do commit, a lista completa de arquivos será comparada com o padrão de caminhos proibidos e com o conteúdo ignorado. Não haverá merge entre os históricos.

## Passos detalhados

1. Mover este plano para `approved` após autorização.
2. Alterar `Dockerfile.v2` para usar `$BUILDPLATFORM` no estágio builder.
3. Alterar `dev-ci.yml` e `main-container.yml` para `linux/amd64,linux/arm64`.
4. Atualizar README e documentação de build/branching para registrar o manifesto multi-arquitetura.
5. Atualizar memória e decisão arquitetural em `dev`.
6. Rodar formatação, `git diff --check`, testes, vet, build normal e cross-build `GOOS=linux GOARCH=arm64 CGO_ENABLED=0`.
7. Commitar e enviar a alteração de build para `origin/dev`.
8. Criar worktree temporário a partir de `origin/main`, atualizar exclusivamente a allowlist pública com o conteúdo aprovado de `dev` e incluir simulador sem relatórios.
9. Executar na árvore pública testes, vet, build AMD64/ARM64, whitespace e verificação dos caminhos proibidos.
10. Revisar a lista integral do commit público, commitá-lo em `main` e enviar para `origin/main`.
11. Confirmar que `origin/main` aponta para o commit publicado. Como `gh` e Docker não existem neste host, registrar que a conclusão do workflow e o manifesto GHCR deverão ser acompanhados pela interface do GitHub/GHCR ou por consulta HTTP disponível.
12. Remover o worktree temporário, finalizar o plano em `done` na `dev` e publicar esse registro final em `dev` se a promoção tiver sido concluída.

## Riscos
- **Arquivos internos vazarem para `main`:** mitigado por allowlist, workflow `public-tree`, revisão de `git ls-files` e padrão de caminhos proibidos antes do commit.
- **Relatórios de simulação entrarem no release:** `.reports/` não será copiado; `.reports/simulations/` já está ignorado em `dev` e é proibido em `main`.
- **Imagem ARM64 falhar durante comandos de build:** builder executará em `$BUILDPLATFORM` e fará cross-compile estático; o estágio final não possui `RUN`.
- **CI remoto falhar depois do push:** validar localmente ambos os binários e os workflows; sem `gh`/Docker local, a publicação final depende do GitHub Actions e será reportada com essa limitação se não houver meio de consultar o status.
- **Atualização remota concorrente:** buscar `dev` e `main` imediatamente antes dos pushes e interromper a promoção se a referência remota divergir.
- **Históricos independentes serem unidos por engano:** criar commit sobre `origin/main` em worktree próprio, sem merge, cherry-pick ou rebase de `dev`.

## Impactos esperados
- `dev` passa a validar containers AMD64 e ARM64.
- Cada push em `main` publica as tags `latest` e `sha-<commit>` como manifestos multi-arquitetura.
- A `main` recebe as correções atuais, o `/reset`, as regras restauradas e o simulador V2, mantendo a árvore pública limpa.
- Deploys em servidores ARM64 poderão usar a mesma referência de imagem GHCR usada por AMD64.

## Compatibilidade
- Linux AMD64: preservada e validada.
- Linux ARM64: adicionada ao container e ao cross-build.
- macOS/Windows: código Go continua compilável; a imagem publicada usa Linux.
- Docker/OCI: uma tag aponta para manifest list com duas plataformas.
- CI/CD: Buildx executa a matriz multiarch; publicação continua exclusiva de push na `main`.

## Como testar

### Build
```bash
go build ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/unobot-amd64 ./cmd/bot
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/unobot-arm64 ./cmd/bot
```

### Testes
```bash
go test ./...
go vet ./...
git diff --check
```

Na árvore de `main` também será executado o mesmo regex de caminhos proibidos usado em `.github/workflows/public-tree.yml`.

### Execução
```bash
docker pull ghcr.io/gr1ksdev/unobotgo:latest
docker buildx imagetools inspect ghcr.io/gr1ksdev/unobotgo:latest
```

Os comandos Docker acima serão responsabilidade do CI ou de outro host, pois Docker não está instalado neste ambiente.

## Rollback
- Em `dev`, reverter o commit do pipeline multiarch com um novo commit.
- Em `main`, reverter o commit de promoção no histórico próprio e fazer push; isso republicará `latest` com a árvore anterior.
- A tag imutável `sha-<commit>` anterior continuará disponível para fixar uma versão conhecida.
- Não usar `git reset --hard`, force push ou apagar tags/imagens durante o rollback.

## Observações
- A solicitação de publicar `main` e fazer push já foi expressa pelo usuário; a nova aprovação exigida abaixo existe por causa do fluxo obrigatório de `AGENTS.md` antes de qualquer alteração.
- A publicação no GHCR é disparada pelo push na `main` e só ocorre após o job remoto de validação.
- A árvore pública incluirá o código-fonte do simulador, mas nunca seus relatórios gerados.
- Aprovado pelo usuário em 2026-09-24 com: "sim".

## Resultado da implementação

- Pipeline multi-arquitetura enviado para `dev` no commit `c052195`.
- CI de `dev` concluído com sucesso: `https://github.com/gr1ksdev/UnoBotGO/actions/runs/36033808406`.
- Árvore pública promovida sobre o histórico independente de `main` no commit `d7ecdb5`.
- Verificador `public-tree` concluído com sucesso: `https://github.com/gr1ksdev/UnoBotGO/actions/runs/36034149271`.
- Validação e publicação `main-container` concluídas com sucesso: `https://github.com/gr1ksdev/UnoBotGO/actions/runs/36034149342`.
- O manifesto `ghcr.io/gr1ksdev/unobotgo:latest` foi consultado diretamente pela API OCI do GHCR e contém:
  - `linux/amd64`: `sha256:e41a25933168d9b2ccc22f67274031814b21a5c6fc6ebf6cac1ed51c3ffbcd37`
  - `linux/arm64`: `sha256:d2cf33a2b2bbd59e5bd8d0c3ef8f2d0bd31923af77212d00aa41d926ce02ab11`
- A árvore pública final passou em testes, vet, build, cross-build estático AMD64/ARM64, whitespace e verificação de caminhos proibidos.
- `.agent`, `.reports`, relatórios de simulação, fontes V1, Docker legado e arquivos locais não foram publicados na `main`.

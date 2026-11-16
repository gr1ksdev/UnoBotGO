# Branches públicas e desenvolvimento

`dev` contém o histórico completo de desenvolvimento do v2. A branch `main` é
uma publicação limpa do v2 e tem histórico próprio, sem artefatos de agentes ou
arquivos exclusivos do V1.

## Branches

| Branch | Conteúdo |
|---|---|
| `main` | Publicação estável do v2, com Mini App. |
| `dev` | Linha de desenvolvimento do v2. |
| `v3` | Bot de Telegram puro: sem Mini App, sem API REST, sem WebSocket. |

A `v3` nasce de `main` mais o delta real de código do `dev` (a substituição de
lobby vazio no `/novo`). Não recebe os artefatos internos — `.agent/`,
`.opencode/`, `.reports/` — nem o legado do V1.

## Fluxo

1. Desenvolva e faça commits na branch da linha correspondente (`dev` para o
   v2, `v3` para o novo produto).
2. O CI da branch executa testes, race detector, vet, build e validação Docker.
3. Gere uma árvore pública a partir do estado aprovado, mantendo somente
   arquivos do produto e documentação técnica humana.
4. Atualize `main` com essa árvore pública e deixe o CI validar e publicar a
   imagem no GHCR.

Não faça merge direto de `dev` em `main`: os históricos são independentes e a
árvore pública não deve receber artefatos internos. A publicação usa as tags
`latest` e `sha-<commit>` em `ghcr.io/gr1ksdev/unobotgo`. Cada tag é um manifesto
OCI com variantes `linux/amd64` e `linux/arm64`.

O workflow `public-tree` rejeita na `main` os caminhos privados e do V1:
`.agent/`, `.opencode/`, `.reports/`, `AGENTS.md`, `codemaps/`,
`docker-compose.yml`, `Dockerfile`, `docs/v2-audit.md`, `main.go` e os demais
arquivos do legado.

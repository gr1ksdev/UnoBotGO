# Branches públicas e desenvolvimento

`dev` contém o histórico completo de desenvolvimento. A branch `main` é uma
publicação limpa do V2 e tem histórico próprio, sem artefatos de agentes ou
arquivos exclusivos do V1.

## Fluxo

1. Desenvolva e faça commits em `dev`.
2. O CI de `dev` executa testes, race detector, vet, build e validação Docker.
3. Gere uma árvore pública a partir do estado aprovado, mantendo somente
   arquivos do V2 e documentação técnica humana.
4. Atualize `main` com essa árvore pública e deixe o CI validar e publicar a
   imagem no GHCR.

Não faça merge direto de `dev` em `main`: os históricos são independentes e a
árvore pública não deve receber artefatos internos. A publicação usa as tags
`latest` e `sha-<commit>` em `ghcr.io/gr1ksdev/unobotgo`.

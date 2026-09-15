# Branches públicas e desenvolvimento

`dev` contém o histórico completo de desenvolvimento. A branch `main` é uma
publicação limpa do V2 e tem histórico próprio, sem artefatos de agentes ou
arquivos exclusivos do V1.

## Fluxo

1. Desenvolva e faça commits em `dev`.
2. Execute `go test ./...`, `go test -race ./...`, `go vet ./...` e `go build ./...`.
3. Gere uma árvore pública a partir do estado aprovado, mantendo somente arquivos
   do V2 e documentação técnica humana.
4. Atualize `main` com essa árvore pública e abra uma revisão antes de publicar.

Não faça merge direto de `dev` em `main`: isso reintroduziria `.agent/`, planos,
prompts ou histórico privado. A branch `main` deve continuar sendo publicada como
um histórico independente. O `.gitignore` de `main` bloqueia os nomes de artefatos
locais mais comuns; a verificação de CI também rejeita esses caminhos se forem
adicionados explicitamente.

O remoto legado não faz parte desse fluxo e não deve receber force push.

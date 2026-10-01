# Plano: remover-periodo-hero

## Pedido do usuário
Remover "Total em Outubro" do detalhe do grupo, pois o mês já aparece no título do ranking interno.

## Objetivo
Eliminar legenda redundante sem alterar pontuação ou período do ranking.

## Contexto atual
Dev com alterações anteriores de fullscreen, que serão preservadas. Legenda renderizada em RankingsPage; teste de navegação espera esse texto; CSS exclusivo hero-caption.

## Arquivos analisados
- web/src/pages/Rankings.tsx
- web/src/App.test.tsx
- web/src/styles.css
- .agent/context.md

## Arquivos que poderão ser modificados
- web/src/pages/Rankings.tsx
- web/src/App.test.tsx
- web/src/styles.css
- .agent/memory/memory.md

## Estratégia de implementação
Remover o parágrafo e CSS sem uso; atualizar a asserção existente para o mês no título interno e ausência da legenda.

## Passos detalhados
1. Registrar plano autorizado pelo pedido direto.
2. Remover legenda e CSS exclusivo.
3. Atualizar teste existente e executar validações.
4. Registrar conclusão sem commit/push.

## Riscos
- Nenhum impacto em dados; preservar alterações anteriores.

## Impactos esperados
- Hero sem mês duplicado; mês preservado no ranking interno.

## Compatibilidade
- Linux, macOS, Windows, Docker e CI/CD: mesma estrutura de build.

## Como testar

### Build
```bash
npm --prefix web run build
go build ./...
```

### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
go test -count=1 ./...
git diff --check
```

### Execução
```bash
npm --prefix web run dev
```

## Rollback
Restaurar somente parágrafo, CSS e asserção removidos nesta tarefa.

## Observações
Pedido autoriza remoção diretamente; sem nova confirmação. Sem commit/push/deploy/main/migration.

## Resultado
Removidos parágrafo redundante e CSS exclusivo. Teste existente confirma mês no título interno e ausência de `Total em`. Lint, typecheck, 45 testes frontend, build frontend, `go test -count=1 ./...`, `go build ./...` e `git diff --check` passaram. Node local 26.10.0. Alterações anteriores preservadas na dev; sem commit/push/deploy.

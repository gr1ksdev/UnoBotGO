# Plano: status do projeto

## Pedido do usuário
Auditar maturidade real, documentar dev/main, promover apenas documentação e tentar mudar a default branch para main. Sem gameplay.

## Objetivo
Separar implementação, testes, homologação real, publicação e design.

## Contexto atual
Auditoria concluída: dev ea22a1e; main 4a63591; históricos independentes. Default dev. Main já possui simulador/reset/correção de penalidades; novidades posteriores permanecem na dev. Codemaps existentes descrevem V1. gh e autenticação administrativa indisponíveis.

## Arquivos analisados
- README.md, docs/*, AGENTS.md, codemaps/*
- .agent/context.md, .agent/memory/memory.md, .agent/decisions.md e planos relevantes
- cmd/*, internal/*, Dockerfiles, workflows de ambas as branches
- Histórico, árvores remotas e API pública GitHub

## Arquivos que poderão ser modificados
- docs/project-status.md e README.md em cada branch
- Este plano e registros locais de contexto/memória/decisão na dev

## Estratégia de implementação
Documento público seguro de aproximadamente 100–200 linhas com matriz e evidências. README mínimo, específico de cada branch. Worktree independente da main, sem merge.

## Passos detalhados
1. Registrar auditoria e autorização.
2. Criar status, ajustar README e validar dev.
3. Commit/push origin/dev.
4. Copiar seletivamente apenas documentação pública; adaptar identificação da main.
5. Validar main, incluindo script public-tree; commit/push origin/main.
6. Confirmar default e permissões; mudar somente se houver autenticação suficiente.
7. Relatar SHAs, checks, diferenças e limitações.

## Riscos
- Confundir teste automático com aceite Telegram.
- Promover features inadvertidamente ou expor artefatos internos.
- Seções históricas e codemaps não representam a V2 atual.

## Impactos esperados
Documentação operacional confiável; nenhum comportamento alterado.

## Compatibilidade
- Linux: validação local.
- macOS e Windows: sem alteração de executáveis/configuração.
- Docker e CI/CD: workflows preservados, sem novo build local obrigatório.

## Como testar
### Build
```bash
go build ./...
```
### Testes
```bash
go test ./...
go test -race ./...
go vet ./...
git diff --check
```
### Execução
Somente checks locais e script public-tree existente; nenhum bot real iniciado.

## Rollback
Reverter os commits documentais de cada branch separadamente; sem reset destrutivo ou merge.

## Observações
Autorização já fornecida no pedido: “Faça primeiro a auditoria, depois aplique as mudanças”, incluindo commits e pushes. Não requer nova confirmação. API pública informa default dev, branches sem proteção e rulesets vazios; detalhes administrativos exigem autenticação.

## Resultado da implementação
Documentos preparados e revisados nas duas branches, com 182 linhas no status. Testes normais, vet, build e diff check aprovados; debugcards aprovado na dev; public-tree aprovado na main. Race local impedido pelo VMA do host, registrado sem ocultar a falha. Commits/pushes documentais seguem na entrega. Mudança da default indisponível por ausência de autenticação API; proteção/rulesets consultados sem alterações.

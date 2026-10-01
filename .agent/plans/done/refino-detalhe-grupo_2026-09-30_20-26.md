# Plano: refino-detalhe-grupo

## Pedido do usuário
Refinar visualmente o detalhe do grupo na dev, sem commit/push/deploy, com auditoria e implementação autorizadas na mesma tarefa.

## Objetivo
Integrar dados ao hero vermelho compacto, reduzir decoração e densidade das linhas, preservar funcionalidade.

## Contexto atual
RankingsPage atende /groups/:groupRef. group-hero cria card com border/background/shadow; avatar 104px; cartas até 166px; rows 80px. Mockup local examinado contém card, mas instrução atual exige removê-lo. Anexos novos não disponíveis.

## Arquivos analisados
- web/src/pages/Rankings.tsx
- web/src/components/Ranking.tsx
- web/src/components/Ranking.test.tsx
- web/src/styles.css
- web/src/api/client.ts
- web/embed.go
- internal/telegram/user_metadata.go
- internal/telegram/ranking_test.go
- internal/storage/postgres/results.go
- internal/storage/postgres/global_rankings.go
- .agent/context.md
- mockup_de_rankings_uno_em_iphones.png

## Arquivos que poderão ser modificados
- web/src/pages/Rankings.tsx
- web/src/components/Ranking.tsx
- web/src/components/Ranking.test.tsx
- web/src/styles.css
- .agent/memory/memory.md
- .agent/decisions.md
- .agent/context.md

## Estratégia de implementação
Estilos compactos restritos ao detalhe; resumo sem caixa interna, duas cartas contidas em coluna própria. Fallback visual de jogador apenas para vazio/pontuação sem conteúdo, mantendo nomes Unicode/emoji e dados originais.

## Passos detalhados
1. Registrar plano e autorização explícita da tarefa; mover para approved.
2. Remover aparência de card e compactar hero/decoração.
3. Compactar título, rows, avatares e medalhas; tratar nomes e pontuações extensas.
4. Aplicar e testar fallback de apresentação sem alterar backend.
5. Executar validações frontend, diff check e Go devido a embed; inspecionar mobile se navegador disponível.
6. Registrar resultados/memória e mover plano para done.

## Riscos
- Pontuações extensas podem comprimir nomes; usar quebra controlada e ellipsis.
- Novos anexos ausentes limitam comparação com referência atual.

## Impactos esperados
- Mais jogadores na viewport e hero integrado.
- Sem alteração de ordenação, API, auth ou persistência.

## Compatibilidade
- Linux, macOS, Windows: frontend padrão.
- Docker e CI/CD: build e embed preservados.

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
git diff --check
go test -count=1 ./...
```

### Execução
```bash
npm --prefix web run dev
```
Usar API simulada somente na validação visual local, sem iniciar backend/migrations.

## Rollback
Reverter apenas os hunks desta tarefa, preservando outros arquivos e histórico dos planos.

## Observações
Autorização explícita: usuário solicita auditoria seguida de implementação sem nova pergunta. Nenhum commit/push/deploy/migration.


## Resultado
Implementado conforme estratégia; detalhes registrados em memory/context/decisions.

- lint, typecheck, test (31 testes) e build frontend aprovados com Node 24.21.0.
- go test -count=1 ./... e go build ./... aprovados após geração dos assets embed.
- git diff --check aprovado.
- Inspeção visual Brave/Playwright temporário com dados simulados em 280/320/360/390/480px: sem overflow horizontal, inclusive int64 máximo e nomes longos. Em 390x844: hero 192px e nove rows de 62px visíveis, sem insets Telegram. Back navega para ranking global.
- Capturas: /tmp/unobot-visual-validation/detail-390.png e detail-320-stress.png.
- Anexos novos ausentes; usado mockup local e critérios textuais atuais. Homologação no celular pendente com usuário.
- Sem alterações Go, API, auth, group_ref, ordenação, desempate, mês, paginação ou persistência. Build regenerou dist ignorado pelo Git.
- Investigação '.' feita pelo código e teste existente, sem consultar dados de produção; correção de apresentação separada da rodada visual.
- Nenhum commit/push/deploy/migration; somente dev.

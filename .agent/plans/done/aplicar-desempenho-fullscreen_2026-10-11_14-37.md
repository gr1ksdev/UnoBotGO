# Plano: aplicar desempenho e fullscreen

## Pedido do usuário
Aplicar somente patch incremental aprovado, preservar trabalho e recursos atuais, validar redução de trabalho e fullscreen exclusivo Android/iOS compatíveis. Sem commit/push/deploy ou atualização de dependências.

## Objetivo
Eliminar renderização/layouts/tweens redundantes e atraso do arraste sem encurtar animações ou alterar regras.

## Contexto atual
HEAD dev 7b85f69510ad5be1d2bd46b9a7e7dab16a798084 com pacote completo aplicado no working tree. Hashes dos arquivos de código iguais à base do manifesto. ZIP íntegro e patch externo idêntico ao interno. Check completo falha somente nas três memórias .agent que receberam registros locais.

## Arquivos analisados
- AGENTS.md (instruções vigentes), .agent/context.md
- LEIA-ME.md, ARQUIVOS.json, docs/desempenho-fullscreen.md e patch do ZIP
- TableScene.ts, useTableMotion.ts, telegram.ts e performance-check.mjs fornecido
- Diffs Game.tsx, testes Telegram/Game e motion-check.mjs

## Arquivos que poderão ser modificados
- Treze caminhos do manifesto incremental: renderer, hook, Telegram, Game, testes/harnesses, documentação e registros .agent
- Correções pontuais nesses caminhos caso testes revelem regressões
- docs/validacao-desempenho-local.md, .reports/desempenho-local/ e este plano

## Estratégia de implementação
Aplicar primeiro apenas o novo harness para coletar baseline na implementação atual. Validar patch restante excluindo conflitos .agent e harness já aplicado. Aplicar código via hunks Git; acrescentar somente linhas novas das três memórias, mantendo registros locais. Não copiar arquivos inteiros sobre trabalho existente. Conferir hashes dos assets, fontes, áudios e docs/validacao-experiencia-local.md antes/depois.

## Passos detalhados
1. Registrar aprovação explícita e mover plano para approved.
2. Aplicar harness de performance isoladamente, iniciar Vite em porta livre e medir baseline com assertions da correção desabilitadas.
3. Aplicar demais hunks válidos e adaptar somente os três registros .agent por acréscimo.
4. Conferir ticker GSAP, cleanup, cache/reuso, zero tweens no arraste, fullscreen por plataforma/API e ACK imediato do coringa.
5. Executar lint, typecheck, testes e build. Executar performance após correção, movimentos, interação e 324 layouts.
6. Comparar métricas locais antes/depois e referências do ZIP; separar contadores de trabalho e intervalos headless de FPS físico não medido.
7. Revisar capturas, registrar limites, preservar recursos/relatório anterior, encerrar somente Vite desta tarefa e concluir plano/memória.

## Riscos
- Otimização pode omitir invalidação ou cancelamento: validar revision/recovery/pending/resize/scroll e desmontagem.
- Corrida ACK/ticker no seletor: teste específico sem avançar tick.
- Headless software não representa FPS ou latência física.

## Impactos esperados
- Menos layouts, alocações e tweens redundantes; posição do arraste atualizada imediatamente.
- Fullscreen automático só em Telegram android/ios com API8+; desktop/web preservam janela.

## Compatibilidade
- Linux, macOS, Windows: stack existente, execução local macOS.
- Docker e CI/CD: sem alterações; sem novo banco ou deploy.

## Como testar

### Build
```bash
npm --prefix web run build
```

### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web test
node web/scripts/performance-check.mjs
node web/scripts/motion-check.mjs
node web/scripts/experience-check.mjs
node web/scripts/visual-check.mjs
git diff --check
```

### Execução
```bash
npm --prefix web run dev -- --port 5174 --strictPort
```
Usar UNO_VISUAL_URL e saídas próprias .reports/desempenho-local para não sobrescrever evidências anteriores. Baseline usa UNO_PERFORMANCE_EXPECT_FIX=0.

## Rollback
Reverter somente hunks desta etapa após conferência, mantendo pacote anterior e histórico. Não usar reset/clean.

## Observações
Aprovação explícita inicial cobre planejamento, aplicação, adaptação e validação; não pedir novamente. Relatório anterior preservado byte a byte. Nenhum Telegram/aparelho físico disponível; não afirmar 60/120 FPS.


## Conclusão
Patch aplicado; adaptação apenas nos três registros .agent e no harness de performance (sincronização com desmontagem e pointermove efetivamente entregue). Código de produção coincide com pacote. Baselines/contadores comparados, 128 testes/lint/typecheck/build e performance/motion/interaction/324layouts aprovados. Todos os assets/docs preexistentes preservados. Documentação em docs/validacao-desempenho-local.md; limites físicos explícitos. Servidores temporários encerrados; sem commit/push/deploy.

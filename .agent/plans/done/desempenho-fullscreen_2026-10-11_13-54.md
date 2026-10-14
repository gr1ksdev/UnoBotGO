# Plano: desempenho e fullscreen móvel

## Pedido do usuário
Criar patch incremental para engasgos durante animações e impedir fullscreen automático no PC. Autorização explícita na mensagem que solicita o patch; não pedir novamente.

## Objetivo
Reduzir trabalho da apresentação por interação, sincronizar render com atualização GSAP e validar antes/depois sem prometer FPS físico não medido.

## Contexto atual
Dev em 7b85f695, pacote completo já aplicado e preservado; todos os seus arquivos registrados em árvore Git temporária sem commit e sem alterar índice real. Canvas autoStart:false agenda RAF separado. Arraste aloca dois tweens por evento. Layout remonta mesa/oponentes e seleção anima toda mão. API fullscreen não verifica plataforma.

## Arquivos analisados
- AGENTS.md, .agent/context.md, .agent/decisions.md, .agent/memory/memory.md
- web/src/components/scene/TableScene.ts, web/src/hooks/useTableMotion.ts
- web/src/lib/telegram.ts e telegram.test.tsx
- web/scripts/motion-check.mjs, experience-check.mjs, visual-check.mjs
- HAR e MatchoRoute (referência apenas); documentação oficial Pixi, GSAP e Telegram

## Arquivos que poderão ser modificados
- TableScene.ts, useTableMotion.ts, telegram.ts, Game.tsx e testes
- web/scripts/performance-check.mjs; docs/desempenho-fullscreen.md
- .agent/plans/, memória, decisões e contexto

## Estratégia de implementação
Registrar benchmark Chromium de seleção, arraste, rolagem e frames renderizados. Renderizar uma vez após atualização GSAP somente quando cena suja; remover assinaturas em repouso/desmontagem. Arraste atualiza propriedades diretamente. Evitar layout e tweens em estados inalterados; reutilizar mesa no scroll; cachear mapa DOM da mão por layout. Fullscreen automático apenas android/ios, PC/web/plataforma ausente conservam janela e opção manual do cliente. Preservar regras, comandos, duração dos efeitos e visuais.

## Passos detalhados
1. Medir baseline com 30 cartas e dez jogadores; registrar contadores e intervalos em fixture identificada.
2. Implementar sincronização, arraste direto e redução de alocações/leitura DOM sem mudar contrato.
3. Adicionar matriz de testes de plataforma e regressões Chromium de lifecycle, gestos, compra, reduced motion e repouso.
4. Rodar lint, typecheck, frontend testes/build e harnesses necessários; documentar métricas e limites físicos.
5. Atualizar registros, mover plano para done e gerar patch somente sobre o pacote completo, com instruções/checksums. Sem commit/push/deploy.

## Riscos
Agendamento e cleanup precisam desenhar frame final sem manter ticker ocioso. Reuso deve invalidar em resize/revisão e manter destino correto de compra. Chromium software não reproduz GPU/Telegram físico.

## Impactos esperados
Menos tweens, menos churn de sprites e leituras DOM, menor atraso de entrada. Desktop sem fullscreen forçado.

## Compatibilidade
Linux/macOS/Windows/CI portáveis; sem dependências novas, backend/migrations/Docker intactos.

## Como testar
### Build
npm --prefix web run build
### Testes
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
node web/scripts/performance-check.mjs
node web/scripts/motion-check.mjs
node web/scripts/experience-check.mjs
git diff --check
### Execução
Vite local com fixtures somente; sem credenciais reais.

## Rollback
Reverter somente este patch incremental; preservar pacote completo e demais mudanças do usuário. Não usar reset/clean.

## Observações
HAR não contém medição real de FPS. Resultado deve separar contadores locais e intervalos de frames de homologação física. Autorizado pelo pedido atual de criação do patch.

Durante os checks surgiu uma corrida real: ACK antes do primeiro tick não pode encerrar seletor usando isActive(). Validar progresso total, com regressão de confirmação imediata e duração preservada.

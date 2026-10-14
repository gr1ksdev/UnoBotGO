# Plano: diagnosticar fluidez versus Matcho

## Pedido do usuário
Investigar implementação local e HAR, medir com fixtures em dev/produção, produzir relatório fundamentado e sanitizado. Não editar código de produção, dependências ou fazer publicação.

## Objetivo
Identificar gargalos restantes com evidência separando custo local, composição e latência; comparar arquitetura sem executar bundles ou usar endpoints do HAR.

## Contexto atual
Workspace contém alterações autorizadas anteriores de experiência e desempenho/fullscreen. Há relatórios e harnesses Chromium. HAR recém-anexado na raiz. HEAD base histórico, alterações atuais em working tree precisam ser preservadas. Versões fixadas no package e instaladas via node_modules devem ser distinguidas.

## Arquivos analisados
- AGENTS.md, .agent/context.md
- docs/validacao-experiencia-local.md e docs/validacao-desempenho-local.md
- docs/desempenho-fullscreen.md e docs/experiencia-completa-har.md
- web/src/components/scene/TableScene.ts, hooks/useTableMotion.ts, pages/Game.tsx, hooks/useGame.ts e game-v2.css
- web/scripts/performance-check.mjs, motion-check.mjs, experience-check.mjs e visual-check.mjs
- package manifests, lockfile, Vite config e HAR

## Arquivos que poderão ser modificados
- Este plano em .agent/plans/done/
- docs/diagnostico-fluidez-vs-matcho.md
- .reports/diagnostico-fluidez/: extração sanitizada, inventário, métricas, traces, harness isolado e checksums
- Nenhum arquivo de produção ou dependência

## Estratégia de implementação
Inventariar HAR em streaming/sem imprimir URLs, query, cookies ou payload sensível. Extrair somente corpos de recursos para diretório isolado com nomes opacos/classificados e metadados sanitizados; analisar bundles estaticamente, sem executar nem reproduzir endereços. Coletar versões/config atuais. Criar harness local temporário identificável com perf APIs, tarefas longas, contadores de draw/ticker/RAF e cenários fixos; medir dev e build local em Chromium comparável. Registrar limites de GPU sem inferir apresentação física. Documentar achados priorizados com evidência, classificação, risco e critério de correção.

## Passos detalhados
1. Preservar estados/diffs e criar diretório de relatório/harness sem tocar produção.
2. Inventariar/analisar HAR, conteúdos presentes e referências sem corpo; sanitizar completamente.
3. Mapear fluxo React/Pixi/GSAP, estado WebSocket, assets e custos medíveis.
4. Inspecionar docs oficiais das versões instaladas para comportamento relevante.
5. Executar cenários em Vite dev e build preview com mesmo Chromium/viewport/DPR/software settings, repetidos e repouso; registrar timers, RAF/render, long tasks e memória quando disponível.
6. Comparar traces e contadores; revisar explicitamente esperas de servidor/rede versus frames.
7. Escrever relatório com achados classificados, tabela Matcho/UnoBotGO, plano priorizado e comandos. Verificar ausência de segredo e que produção/deps permanecem sem mudanças.
8. Mover plano para done; não propor implementação como se já estivesse aprovada.

## Riscos
- HAR pode conter credenciais, dados pessoais e endereços privados.
- Dev HMR e build mudam o perfil; comparação precisa distinguir ambos.
- Chromium/macOS headless software não mede display/GPU/Telegram físicos.
- Instrumentação adiciona custo; marcar e não usar como FPS benchmark.

## Impactos esperados
Relatório/evidências temporárias rastreáveis; nenhum comportamento do app alterado.

## Compatibilidade
- Linux/macOS/Windows: ferramentas Node/Playwright; execução disponível em macOS.
- Docker/CI/CD: sem alteração.

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
node web/scripts/motion-check.mjs
node web/scripts/experience-check.mjs
node web/scripts/visual-check.mjs
```

### Execução
Harness fixture via Vite dev e preview do build; offline/local apenas.

## Rollback
Remover somente diretório de análise novo se solicitado. Nunca tocar assets, relatórios e alterações existentes.

## Observações
Usuário autorizou expressamente análise, medições locais, harness temporário e relatório; isso não autoriza implementação. Não alterar regras, protocolo, assets ou dependências. Métricas headless não são FPS apresentado.

# Plano: corrigir-harness-diagnostico

## Pedido do usuário
Corrigir somente a delimitação das estatísticas RAF e o fixture de compra de três cartas no harness, repetir seis cenários em desenvolvimento e produção, preservar evidências antigas e atualizar o relatório.

## Objetivo
Produzir medições por etapa sem callbacks acumulados e validar compras com CardIDs, contagens e voos coerentes, sem modificar código de produção ou dependências.

## Contexto atual
O harness usa um cursor RAF Node não enviado ao browser e contém uma atribuição inalcançável após `return`. O fixture seguinte ao draw de uma carta recria `N+3` cartas a partir de `c0`, preservando apenas duas cartas novas em relação a `N+1`.

## Arquivos analisados
- `.reports/diagnostico-fluidez/diagnostic-check.mjs`
- `docs/diagnostico-fluidez-vs-matcho.md`
- `web/package.json`

## Arquivos que poderão ser modificados
- `.reports/diagnostico-fluidez/diagnostic-check.mjs`
- `.reports/diagnostico-fluidez/runs/` (novos diretórios versionados de evidência)
- `docs/diagnostico-fluidez-vs-matcho.md`
- Este plano, movido para `approved/` e depois `done/`

## Estratégia de implementação
A autorização explícita do usuário nesta solicitação cobre planejamento, correção e validação, sem necessidade de nova aprovação. Fazer teste determinístico puro do consumo de amostras RAF; ler cursor absoluto de callbacks no browser e atualizar o cursor Node após cada leitura. Usar mão anterior do snapshot para acrescentar exatamente três IDs únicos e comparar voos instrumentados/markers. Gravar resultados em diretórios novos sem sobrescrever os históricos.

## Passos detalhados
1. Registrar este plano autorizado e corrigir os dois defeitos no harness.
2. Validar isolamento de pausa pregressa e consumo sem reutilização; executar sintaxe/check determinístico.
3. Executar os seis cenários nos servidores dev e preview de produção, sem simultaneidade de medição.
4. Atualizar comparativos e conclusões do relatório, identificando métricas históricas afetadas e limitações de FPS.

## Riscos
- Variabilidade de Chromium headless/SwiftShader pode alterar contadores e tempos.
- Marcadores de voo podem ser transitórios; aguardar IDs esperados reduz risco de leitura tardia.
- Alterar relatório sem preservar claramente as tabelas antigas pode apagar rastreabilidade; manter resultados históricos e citar divergências.

## Impactos esperados
- Estatísticas RAF passam a refletir callbacks capturados entre duas marcações sucessivas.
- Compra de três cartas passa a conter três novos IDs e jogador/evento coerentes.
- Nenhum impacto em produção.

## Compatibilidade
- macOS local, Node 24 e Chromium via Playwright; fixture local Vite.

## Como testar

### Build
Não requerido: código de produção e dependências não mudam.

### Testes
`node --check .reports/diagnostico-fluidez/diagnostic-check.mjs` e assertions determinísticas; depois, o harness nos seis casos em dev e produção.

### Execução
Iniciar Vite dev em 5174 e preview do build existente em 4173; rodar o harness sequencialmente com saída em diretórios corrigidos novos.

## Rollback
Reverter somente o script, os novos diretórios de evidência e o relatório para as versões anteriores; não remover nem alterar as evidências históricas.

## Observações
Resultados RAF continuam sendo callbacks headless, não frames apresentados. Os resultados físicos do Telegram permanecem fora do alcance da medição local.

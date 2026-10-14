# Plano: aplicar experiencia completa

## Pedido do usuário
Aplicar o ZIP aprovado, preservar trabalho local, integrar binários e validar apresentação completa. Sem commit, push ou deploy.

## Objetivo
Integrar atlas, gestos, animações e feedback à partida existente, sem modificar regras ou backend.

## Contexto atual
Branch dev no commit 7b85f69510ad5be1d2bd46b9a7e7dab16a798084, exatamente a base indicada. Working tree limpo exceto ZIP não rastreado. Checksums do pacote válidos; git apply --check aprovado. Patch anterior ausente.

## Arquivos analisados
- AGENTS.md e .agent/context.md
- LEIA-ME.md e arquivos/docs/experiencia-completa-har.md no ZIP
- web/package.json, Makefile e web/scripts/motion-check.mjs
- Patch consolidado e alterações em TableScene.ts, Game.tsx, hooks, CSS, feedback e testes

## Arquivos que poderão ser modificados
- Os 39 caminhos listados no patch: .agent, docs, web/src, web/scripts e web/public/assets
- Harnesses e implementação desses recursos se as verificações identificarem defeitos
- .agent/plans, .agent/memory/memory.md, .agent/context.md e .agent/decisions.md
- docs/validacao-experiencia-local.md e evidências em .reports/experiencia-local/

## Estratégia de implementação
Aplicar patch validado na base exata, incluindo binários. Comparar os 39 arquivos com versões fornecidas. Manter registros históricos do pacote distinguindo-os dos testes locais atuais. Executar checks frontend, Go e navegadores disponíveis; corrigir falhas verificadas dentro do escopo.

## Passos detalhados
1. Registrar aprovação explícita já dada na mensagem do usuário e mover plano para approved.
2. Aplicar patch consolidado, verificar identidade de todos os arquivos fornecidos e assets.
3. Rodar lint, typecheck, testes e build frontend; testes, vet e build Go.
4. Iniciar Vite local isolado, executar harnesses motion, experience, visual e preview.
5. Conferir dois toques, retorno/arraste, distribuição/compra/troca, coringa, feedback/som, resultado/revanche e matriz de até oito cartas por fileira, margens e rolagem.
6. Inspecionar capturas, documentar resultados reais e limitações; parar somente processos iniciados nesta tarefa.
7. Atualizar memória/contexto/decisões e mover plano para done.

## Riscos
- Diferenças de GPU/tempo do navegador nos harnesses.
- Áudio e haptics físicos exigem aparelho/Telegram real.
- Relatórios do ZIP são históricos e não substituem validação local.

## Impactos esperados
- Interação e apresentação mais completas sem mudança de contrato servidor.
- Assets locais extras e persistência de preferência de som/vibração.

## Compatibilidade
- Linux, macOS e Windows: stack existente; validação local macOS.
- Docker: nenhum ajuste de infraestrutura.
- CI/CD: scripts existentes e build sem publicação.

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
npm --prefix web test
go test ./...
go vet ./...
node web/scripts/motion-check.mjs
node web/scripts/experience-check.mjs
node web/scripts/visual-check.mjs
node web/scripts/experience-preview.mjs
git diff --check
```

### Execução
```bash
npm --prefix web run dev -- --port 5174 --strictPort
```

## Rollback
Reverter apenas hunks introduzidos nesta tarefa após revisão; não apagar histórico nem usar reset/clean. ZIP original preservado.

## Observações
Usuário aprovou explicitamente aplicação e adaptações necessárias antes desta análise. Nenhuma nova aprovação necessária. Não executar commit, push, deploy ou reiniciar instâncias existentes. E2E com PostgreSQL depende de banco isolado disponível e será distinguido dos harnesses de apresentação.


## Conclusão
Aplicado integralmente após aprovação explícita inicial. 39 arquivos conferidos. Lint/typecheck/117 testes/build frontend, go test/vet/build, motion, interação, matriz visual324 e prévia aprovados. Atlas63/áudios14 validados. Evidências e limitações em docs/validacao-experiencia-local.md; sem E2E PostgreSQL/Telegram físico. Vite temporário encerrado, sem commit/push/deploy.

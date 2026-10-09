# Plano: partida PixiJS e fileiras

## Pedido do usuário
Correção previamente autorizada: aplicar skills locais PixiJS/GSAP à partida real, oito cartas por fileira, agrupamento estável e coreografia MOVIMENTOS.md. Preservar alterações locais; sem commit, push ou deploy.

## Objetivo
Cena PixiJS de cartas/camadas com movimentos GSAP subordinados aos snapshots autorizados. Fileiras acessíveis sem rolagem horizontal, seletor com feedback completo e evidências de navegador/serviço reais.

## Contexto atual
React/TypeScript/Vite, GSAP instalado, sem PixiJS. PNGs/fontes v2 presentes. Mão horizontal DOM e movimentos curtos. Serviço já implementa consenso de revanche, finalização transacional e transporte privado com revisão/deduplicação.

## Arquivos analisados
- AGENTS.md; .agent/context.md
- docs/design-reference/unobotgo-v2/{MOVIMENTOS.md,referencia.html,fonts.css,references/original-*}
- .agents/skills/{pixijs*,gsap*}/SKILL.md (roteador e especializações pertinentes)
- web/src/pages/Game.tsx; web/src/hooks/{useGame,useTableMotion}.ts
- web/src/game-v2.css; web/package.json; web/scripts/*check.mjs
- internal/game/rematch.go; internal/httpapi/browser_integration_test.go (integração existente)

## Arquivos que poderão ser modificados
- web/package.json; web/package-lock.json
- web/src/pages/Game.tsx; web/src/game-v2.css
- web/src/components/scene/*; web/src/lib/handLayout*; web/src/hooks/useTableMotion.ts
- web/src/pages/Game.test.tsx; web/scripts/{visual-check,motion-check,game-e2e}.mjs
- internal/httpapi/browser_integration_test.go (destino de evidências e bundle/CSP de produção no teste)
- .reports/partida-pixi/*; docs/miniapp-webapp.md; .agent/{context,decisions,memory/memory}.md

## Estratégia de implementação
PixiJS v8 renderiza sprites originais e camadas de mesa/mão/voos; DOM mantém nomes, avatares, comandos e botões acessíveis com áreas de toque não sobrepostas. Layout compartilhado calcula fileiras de no máximo oito, ordem cor/rank/ID, seleção estável e espaço reservado para elevação. Região vertical da mão é mascarada na cena. GSAP anima objetos Pixi e superfícies do seletor/resultado. Eventos aceitos acionam movimentos uma vez; reconexão apenas recompõe estado. Interação em curso conserva seu ID. Backend preservado e validado novamente.

## Passos detalhados
1. Ler skills integrais relevantes e comparar originais; registrar plano aprovado.
2. Adicionar PixiJS compatível e implementação de cena com lifecycle assíncrono, assets e cleanup.
3. Criar layout estável agrupado, fileiras centralizadas e rolagem vertical exclusiva da mão; testes dos limites.
4. Implementar distribuição alternada, voos/viradas, compras múltiplas e reorganização; revisão e disponibilidade imediatas.
5. Ajustar escolha de cor (450+700+350ms), rejeição/duplicação e resultado (550ms).
6. Adaptar checks visuais/movimento/duas contas reais; gerar capturas quatro viewports e vídeo curto.
7. Executar checks obrigatórios, conferir visualmente e registrar evidências/limites locais versus Telegram.

## Riscos
- Lifecycle async StrictMode e texturas compartilhadas: cancelamento/cleanup explícitos.
- Snapshots durante movimentos e resize: baseline canônico, limpeza e deduplicação.
- 320×568/30 cartas: tamanho adaptativo e rolagem vertical da mão, mesa separada.
- Não há acesso autorizado a sessões Telegram reais: declarar homologação pendente.

## Impactos esperados
- Cartas reais renderizadas em PixiJS, ordem previsível e todas acessíveis.
- Coreografia perceptível, sem atrasar engine/comandos.
- Contratos privados, consenso multiplayer, pontuação e políticas preservados.

## Compatibilidade
Linux, macOS, Windows, Docker e CI/CD mantêm build Vite/Go. WebGL nos navegadores da Mini App; erro explícito se renderizador falhar. Movimento reduzido respeitado.

## Como testar
### Build
```bash
cd web && npm run build
```
### Testes
```bash
TEST_DATABASE_URL=postgres://unobot:unobot@localhost:5432/unobot_v2_checks?sslmode=disable make check
cd web && npm run test:visual
TEST_DATABASE_URL=... UNO_BROWSER_E2E=1 CGO_ENABLED=1 go test -race -count=1 -tags integration ./internal/httpapi -run TestTwoBrowserWebAppGame -v
```
### Execução
```bash
cd web && npm run dev
```

## Rollback
Reverter somente os trechos desta correção mediante revisão do diff; preservar todas as alterações locais anteriores e artefatos históricos. Sem reset/clean.

## Observações
Aprovação explícita já concedida na solicitação “Estou aprovando a implementação desta correção”. O novo limite de oito/fileira substitui a instrução anterior de rolagem horizontal. Nenhuma publicação ou comunicação Telegram será realizada.

## Execução e entrega

Implementação concluída localmente em 2026-10-09, preservando mudanças anteriores e sem commit/push/deploy. PixiJS para cena/cartas/camadas e GSAP para coreografia; oito cartas por fileira e ordenação cliente estável; recursos/fontes v2; integração real e consenso de revanche preservados.

- `make check` passou: 108 testes frontend, lint/TypeScript/build, Go/vet/builds e PostgreSQL. Log `.reports/partida-pixi/check-final.log`.
- Matriz de 324 composições passou; 16 casos finais também. Quatro tamanhos, nove contagens de mão, dois a dez jogadores, toque estreito e rolagem vertical.
- Stress de movimentos/cleanup passou incluindo rejeição/retry, ACK rápido, turno/recovery, CardID, gestos, falha de inicialização e desmontagem.
- Race backend e integração real com dois navegadores passaram. PostgreSQL confirmou um resultado, dois retries sem duplicação, ranking e histórico; consenso abriu um único novo GameID e preservou voto em reconexão.
- 76 recursos v2 com hashes idênticos; comparações visuais com originais conferidas. Vídeo real local de 10,56 s mostra distribuição, compra/virada e cor, sem acelerar.

Relatório: `.reports/partida-pixi/README.md`. Capturas: `final-states/` e `comparisons/`. Vídeo: `production/video/movimentos-reais-11s.mp4`. Skills: todos os 34 SKILL.md locais PixiJS/GSAP lidos integralmente.

Limite explícito: autenticação/SDK usam identidades exclusivas de teste; não foi homologado no Telegram real nem em aparelho físico. Esses ambientes permanecem pendentes, sem publicação autorizada.

## Autorização posterior de versionamento

Usuário solicitou explicitamente “faca o commit e push pra dev”. A restrição anterior de commit/push foi substituída para esta correção; deploy continua fora do escopo.

Plano de versionamento: conferir origin/dev e diff; adicionar implementação e dependências locais necessárias (salas diretas, consenso, recursos v2), documentação e evidências finais compactas; preservar skills locais, relatórios históricos e vídeos completos sem adicioná-los; conferir índice, criar commit descritivo e fazer push normal para origin/dev; verificar igualdade dos hashes remoto/local. Checks locais já aprovados são reutilizados, sem novas mudanças de lógica.

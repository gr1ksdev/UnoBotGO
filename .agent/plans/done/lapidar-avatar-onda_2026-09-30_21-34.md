# Plano: lapidar-avatar-onda

## Pedido do usuário
Última rodada exclusivamente visual: avatar levemente maior/centralizado e transição branca com duas lombadas rasas. Preservar demais elementos aprovados. Auditoria seguida de implementação autorizada, somente dev, sem commit/push/main/deploy/backend/API.

## Objetivo
Ampliar avatar em 8px, deslocar composição alguns pixels à direita e adicionar onda responsiva sem aumentar o hero ou mudar a lista.

## Contexto atual
App → /groups/:groupRef → RankingsPage. Hero .group-hero usa Avatar/Score e HeroCards; avatar 108px (92px narrow), grid 108px/gap12/reserva64. Cartas WebP reais em web/src/assets/cards, rotacionadas e cortadas. Ranking-panel com overlap22, raio36/padding-top24; rows88 e estilos aprovados. Estilos concentrados em styles.css.

## Arquivos analisados
- web/src/App.tsx
- web/src/pages/Rankings.tsx
- web/src/components/Ranking.tsx
- web/src/components/HeroCards.tsx
- web/src/styles.css
- .agent/context.md
- Ranking do Grupo em Estilo UNO.png

## Arquivos que poderão ser modificados
- web/src/styles.css
- web/src/assets/hero-wave.svg (novo)
- .agent/context.md
- .agent/memory/memory.md
- .agent/decisions.md

## Estratégia de implementação
Avatar 116px (100px narrow), pequeno padding-left6 no grid e gap ajustado em 4px para preservar largura do texto. Reserva lateral reduzida 8px e cartas deslocadas 8px para direita para não invadir informações. Nenhuma mudança na arte/quantidade/tamanho das cartas. Onda em pseudo-elemento ::before da sheet, com máscara SVG alpha, viewBox responsivo e preserveAspectRatio none, altura24px, dois picos assimétricos de variação aproximada20px. Sobrepor a região do hero sem adicionar altura/layout. Compartilhar background horizontal de mesmos tons entre máscara e sheet para evitar emenda. O raio da borda superior será substituído pelo contorno suave da onda; padding do ranking e todos os estilos de rows preservados.

## Passos detalhados
1. Salvar plano e mover para approved conforme autorização explícita.
2. Ajustar somente avatar/grid/offset lateral das cartas em CSS e narrow breakpoint.
3. Adicionar SVG vetorial de duas curvas rasas e pseudo-elemento decorativo da sheet com mask-size100%100%, sem repetição e sem interação.
4. Inspecionar browser em 390/430, estreitas e desktop: altura hero, layout dos rows, cartões reais, ausência de overflow e de sobreposição do resumo.
5. Executar checks frontend/build/diff; confirmar navegação e sistemas com testes/fixtures existentes.
6. Registrar resultados e mover plano para done.

## Riscos
- Avatar maior pode comprimir score: compensar largura do grid e reserva, manter tipografia.
- Máscara precisa casar com fundo branco: gradient horizontal compartilhado; antes da máscara, sem linha de sombra reta.
- Parte decorativa pode sobrepor texto se não deslocada com a nova reserva; medir no browser.

## Impactos esperados
- Mudança moderada de foco/centralidade; wave com dois picos discretos.
- Hero e posições da lista mantidos; nenhuma alteração de API/backend/regra.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD: SVG/CSS mask padrão em navegadores atuais e Vite sem dependências.
- Mobile-first, desktop max-width480 preservado.

## Como testar

### Build
```bash
npm --prefix web run build
```

### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
git diff --check
```

### Execução
```bash
npm --prefix web run dev
```
Browser local com API/Telegram simulados; sem backend/migrations. Sem Go modificado; go build poderá validar assets embed gerados.

## Rollback
Reverter apenas os hunks CSS e o SVG desta rodada, preservando working tree anterior.

## Observações
Autorização explícita: usuário pede implementar diretamente sem perguntar novamente. Homologação visual final pertence ao usuário; nenhuma comparação com densidade como objetivo.


## Resultado
- Avatar +8px:116px em mobile390–430 e desktop;100px <=360. Deslocamento6px à direita; resumo10px. Cartas só deslocadas8px à direita, mesmas artes/quantidade/dimensões/rotações.
- Onda em CSS ::before/mask, SVG viewBox480x24, preserveAspectRatio none, duas lombadas assimétricas com máximo21px de variação, faixa24px. Sem adicionar altura ao hero e sem deslocar título/rows.
- Comparação automática com snapshot de CSS anterior em320/360/390/430/1280 confirmou lista/título idênticos em posição/tamanho/tipografia/cores/sombras/radius e gaps; hero usual304px antes/depois. Browser verificou assets reais, auth header, foto, Legacy/Updated e back com mesmos params, sem overflow nem sobreposição de resumo/cartas. Em280px cartas permanecem ocultas.
- Check visual das capturas390/430 com duas curvas suaves realizado; capturas em /tmp/unobot-visual-validation/final-wave-390.png, final-wave-430.png, final-wave-desktop.png e final-wave-with-avatar-390.png (dados de teste).
- Lint, typecheck,31 testes frontend,build e git diff --check aprovados. go build ./... aprovado para embed; nenhum Go alterado. CSS gerado inclui -webkit-mask; CSP existente permite data: para SVG inline, sem mudança de headers/backend.
- Nenhum commit/push/main/deploy/migration. Arquivos anteriores do working tree preservados. Homologação visual final com usuário.

# Fluidez das cartas PixiJS — 2026-10-11

Base: branch `dev`, commit `7b85f69510ad5be1d2bd46b9a7e7dab16a798084`.

## Comportamento

- Compra e distribuição usam `PerspectiveMesh`: os quatro cantos recebem projeção de profundidade, o verso troca para a frente no ponto de giro e o número mantém orientação legível. O ponto de 90 graus conserva largura mínima para evitar uma projeção singular.
- Cada voo tem sombra própria que acompanha elevação, dimensão e giro. Cartas da mão projetam sombra e inclinam levemente quando selecionadas. A aterrissagem tem um pequeno ajuste de escala.
- Um voo de compra acompanha o slot atual de seu CardID caso um novo snapshot reorganize a mão durante o movimento. Cartas de adversários mantêm a proporção de carta ao pousar na região de um avatar quadrado.
- Seletor entra com giro e pétalas escalonadas. A pétala escolhida começa o preenchimento; as quatro alcançam a cor selecionada em 450 ms. Mantém 700 ms de permanência e fecha em 350 ms: feedback total de 1,5 segundo, independentemente da confirmação rápida do servidor.
- Escolha nova, rejeição e desmontagem cancelam movimentos antigos. Toque durante a abertura transfere todos os alvos para a timeline de confirmação, evitando tweens concorrentes.
- O contorno do avatar local e o brilho de turno pulsam suavemente. `prefers-reduced-motion` desliga os pulsos; a cena recompõe o snapshot autorizado ao mudar a preferência durante um voo.

Assets, fontes, ordenação por cor/valor/ID, fileiras de até oito cartas, margens, alvos acessíveis, dois toques para jogar, consenso de revanche e transporte existentes foram reutilizados. Não houve mudança em autenticação, regras, pontuação, persistência ou contratos de rede.

## Reprodução das verificações

```bash
npm ci --prefix web
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
npm --prefix web run build
git diff --check
```

Com Vite local em execução e Chromium disponível no Playwright:

```bash
UNO_MOTION_OUTPUT=.reports/fluidez-cartas node web/scripts/motion-check.mjs
UNO_VISUAL_OUTPUT=../../.reports/fluidez-cartas/visual/ node web/scripts/visual-check.mjs
```

Os scripts aceitam `PLAYWRIGHT_EXECUTABLE_PATH` e `UNO_VISUAL_URL`. O harness de movimento verifica compras próprias e adversárias, distribuição, CardID, revisões, recuperação, rejeições, toque elevado, desmontagem, descarte e escolhas consecutivas. A matriz visual verifica margens, fileiras, fontes, ausência de overflow horizontal, alcance da última carta e alvos de toque nas quatro dimensões.

Todos os dados dos harnesses e da gravação são fixtures identificadas de teste. Estes checks verificam a apresentação com snapshots interceptados; não substituem uma partida real no backend, Telegram ou aparelho físico. O backend não foi modificado nesta tarefa.

## Aplicação das alterações

O patch fornecido é relativo à raiz do repositório e inclui somente arquivos desta tarefa. Conferir e aplicar sobre a base indicada, preservando alterações locais:

```bash
git apply --check unobotgo-fluidez-cartas.patch
git apply unobotgo-fluidez-cartas.patch
```

Se o check indicar conflito, adaptar os hunks em uma branch de trabalho; não usar reset ou clean. Nenhum commit, push ou deploy foi executado.

## Resultado da validação

- 112 testes frontend passaram (baseline: 109; três verificações novas da projeção).
- Lint, TypeScript e build Vite passaram; `git diff --check` passou. O build conserva o aviso de chunk Pixi acima de 500 kB.
- Harness de movimento passou, incluindo compra com perspectiva, giro na distribuição, troca de reduced-motion durante voo, recuperação, rejeição, desmontagem e apresentação da cor com ACK rápido.
- 324 combinações de quantidade de cartas/participantes/viewport passaram em 320x568, 360x800, 390x844 e 430x932; 37 seleções por toque no viewport estreito e 36 listas de resultado foram conferidas. Sem overflow horizontal; a última carta é alcançável por rolagem vertical, com oito cartas por fileira e margens de 16 px.
- Capturas e quadros da gravação de 13,84 s foram inspecionados. Evidências locais em `.reports/fluidez-cartas/`; gravação utiliza fixtures locais, sem conta real.
- Patch relativo à base indicada foi conferido com um índice Git temporário, sem alterar o índice do repositório.

# Correção incremental: desempenho da mesa e fullscreen móvel

Data: 2026-10-11. Autorização explícita do usuário para gerar patch de correção. Sem commit, push ou deploy.

## Base

Aplicar DEPOIS de unobotgo-experiencia-completa.zip. Base Git original: dev/7b85f69510ad5be1d2bd46b9a7e7dab16a798084 mais os 39 arquivos desse pacote. O patch foi gerado contra essa árvore, sem commit nem alteração do índice real; não contra HEAD puro. Preservar o relatório de validação local e quaisquer alterações posteriores do usuário.

## Problema encontrado

O relato é de engasgos/perda de continuidade, não pedido para encurtar os efeitos. O HAR registra rede e corpos JS, não FPS efetivo. MatchoRoute mostra uso de Pixi/GSAP e pausa do render quando não há animações; serve como referência, sem substituir engine/protocolo.

No renderer do pacote, sync idêntico ainda executava layout, remontava sprites da mesa e criava dois tweens para cada carta. O campo recovery permanecia verdadeiro no snapshot inicial e fazia a seleção ser reanimada em toda atualização. O scroll remontava monte/descarte. Cada pointermove criava dois tweens de 100 ms. Além disso, o desenho solicitado após a atualização GSAP entrava em outro requestAnimationFrame.

## Correção

- Desenhar a cena suja no listener do ticker GSAP após a atualização da timeline, uma vez por tick. Remover listener no repouso e ao desmontar; Pixi continua com autoStart:false. Não adicionar segundo loop ou configurar FPS artificialmente.
- Ignorar estados de apresentação idênticos. Antes dessa otimização, cancelar gesto indisponível por conexão/pending mesmo se snapshot/revisão não mudou. Chegadas privadas adiadas continuam exigindo layout.
- Na seleção, animar somente a carta anterior e a nova. Apenas tingir quando o tint realmente precisa mudar.
- Atualizar posição/inclinação/sombra diretamente no pointermove, sem atraso de tween e sem alocar timelines. Manter retorno elástico na soltura/cancelamento e mesmas guardas de comando por CardID.
- Cachear geometria/estado público da mesa, oponentes e máscara; reutilizar sprites no scroll/seleção e invalidar por tamanho, posição, carta, turno, direção e contagem. Restaurar efeitos transitórios no cancelamento sem reconstruir a mesa.
- Mapear botões da mão uma vez por layout e reutilizar origem medida e referências nos callbacks; evitar busca repetida da mão por cada carta/frame.
- Aplicar pressão/retorno do monte apenas nos gestos do monte.
- Fullscreen automático somente para Telegram platform android/ios e API compatível. tdesktop/macos/web/weba/webk/plataforma ausente ou desconhecida não recebem requestFullscreen. Manter ready/expand, safe areas e opção manual do próprio cliente; não forçar exitFullscreen de uma escolha manual. Não inferir celular por janela estreita.
- Corrigir corrida do feedback de cor: timeline recém-agendada ainda pode ter isActive()==false. Um ACK antes do primeiro tick deve respeitar totalProgress()<1, preservando os 1,5 s do feedback.

Não altera assets, fontes, limite de oito cartas por fileira, durações deliberadas dos efeitos, regras, ranking, transporte WebSocket, privacidade, revisões ou pontuação. Sem novas dependências.

## Medição reproduzível de trabalho

Fixture de desenvolvimento: 30 cartas próprias, dez jogadores, viewport 390x844, DPR 2; Chromium headless com renderização software. Instrumentação só no harness, não no jogo em produção.

| Exercício | Antes | Depois |
| --- | --- | --- |
| 20 syncs idênticos: layouts | 20 | 0 |
| 20 syncs idênticos: sprites novos | 180 | 0 |
| 20 syncs idênticos: tweens novos | 1.200 | 0 |
| Trocar carta selecionada: sprites novos | 27 | 0 |
| Trocar carta selecionada: tweens novos | 182 | 4 |
| 16 posições de scroll: sprites novos | 144 | 0 |
| Arraste físico CDP: tweens novos | 26 | 0 |
| Frames renderizados durante 600 ms de repouso | 0 | 0 |
| Fullscreen solicitado no cliente desktop simulado | 1 | 0 |

Os 20 syncs são uma carga controlada para demonstrar o custo de atualizações redundantes, não uma estimativa de chamadas por segundo em produção. Os intervalos brutos de renderer e RAF também são registrados em checks.json. Variam com SwiftShader, carga e composição da máquina; não são um benchmark físico pareado nem comprovam 60/120 FPS ou ganho de FPS no celular. A mudança reduz trabalho e atraso de entrada verificáveis; avaliar fluidez final no Telegram real.

## Verificação

128 testes frontend (incluindo matriz de plataformas e ACK imediato do seletor), lint, typecheck e build. Harnesses motion-check/experience-check validam comandos únicos, compra, distribuição, descartes, reconexão, cancelamento, reduced motion, áudios opcionais, Troca de Mãos e repouso. performance-check também cobre cancelamento ao perder disponibilidade com snapshot idêntico, descarte do renderer/tweens na desmontagem e zero comando decorrente das medições. visual-check aprovou 324 composições nos quatro tamanhos. O harness de redução de movimento aguarda o evento real MediaQueryList.change ao restaurar movimento, pois a resposta CDP pode preceder esse evento; assim a ação seguinte não disputa o cancelamento da preferência anterior.

O chunk Pixi continua acima do aviso informativo de 500 kB. Nenhuma atualização de dependências, execução Go/PostgreSQL, publicação ou homologação física nesta etapa exclusivamente frontend.

### Reproduzir

```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
npm --prefix web run build
npm --prefix web run dev -- --port 5174 --strictPort
```

Em outro terminal com Chromium Playwright disponível:

```bash
UNO_VISUAL_URL=http://127.0.0.1:5174 node web/scripts/performance-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 node web/scripts/motion-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 node web/scripts/experience-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 node web/scripts/visual-check.mjs
git diff --check
```

performance-check importa o módulo de desenvolvimento para instrumentar métodos; execute contra Vite dev, não produção. UNO_PERFORMANCE_OUTPUT seleciona saída; UNO_PERFORMANCE_EXPECT_FIX=0 permite coletar baseline sem assertions do comportamento corrigido. Fixtures não usam credenciais reais, resultados ganhos ou engine simulada em produção.

## Homologação física necessária

No mesmo telefone/cliente, testar antes/depois com 7/15/30 cartas: seleção, arraste, compra múltipla e distribuição; observar regularidade dos frames, demora do toque e consumo, mantendo tela/energia/temperatura semelhantes. Desktop deve abrir na janela normal; celular android/ios mantém fullscreen quando suportado. Não há garantia de taxa fixa de frames sem essa medição.

## Aplicação e reversão

```bash
git apply --check unobotgo-desempenho-fullscreen.patch
git apply unobotgo-desempenho-fullscreen.patch
```

Se houver mudanças após o pacote, adaptar somente os hunks correspondentes mantendo todas as alterações locais. Não reaplicar o ZIP completo, não usar reset/clean nem sobrescrever arquivos inteiros indiscriminadamente. Para reverter esta etapa, conferir primeiro git apply -R --check; reverter somente os hunks deste patch e preservar o pacote anterior e o trabalho local.

Referências: https://gsap.com/docs/v3/GSAP/gsap.ticker/ ; https://pixijs.com/8.x/guides/concepts/performance-tips ; https://core.telegram.org/bots/webapps .

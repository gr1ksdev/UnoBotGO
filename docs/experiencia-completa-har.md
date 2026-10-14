# Experiência completa da partida — 11/10/2026

Base do patch consolidado: branch `dev`, commit `7b85f69510ad5be1d2bd46b9a7e7dab16a798084`.
Inclui as alterações anteriores de `unobotgo-fluidez-cartas.patch`. Não aplicar ambos em sequência.

## Entrega

- Cartas e fontes originais preservadas. PixiJS carrega a folha de sprites com o JSON original de 63 frames, incluindo recortes girados. Os 63 recortes foram comparados pixel a pixel com os PNGs existentes; nenhuma carta foi redesenhada. Troca de mãos usa o PNG próprio já existente.
- Distribuição escalonada por jogador; compra com percurso curvo, verso/frente em perspectiva, sombra e aterrissagem; compra em outra fileira rola somente a mão até o destino. Carta do adversário vira durante o descarte apenas após o evento tornar sua face pública.
- Seleção eleva e aumenta a carta com movimento elástico; hover no mouse; carta selecionada acompanha o arraste com inclinação limitada e sombra variável. Soltar no descarte envia o mesmo comando físico validado do segundo toque. Soltar fora ou cancelar volta à mão. Toques e arraste nunca duplicam a jogada.
- Mão por cor/valor/ID, até oito por fileira, margens de 16 px e fileira final centralizada. As fileiras adicionais usam rolagem vertical. Primeiro toque continua selecionando; segundo toque continua jogando. Para arrastar no celular, selecionar primeiro; as outras cartas mantêm o gesto vertical de rolagem.
- Mudança de turno/indisponibilidade suaviza o escurecimento. Contorno e brilho azuis continuam indicando o jogador da vez. Monte responde à pressão e ao hover. Inversão, bloqueio, turno e cor têm efeitos por eventos novos confirmados. Troca de mãos anima a chegada da mão própria sem revelar mãos adversárias.
- Coringa abre com giro e pétalas escalonadas. A cor escolhida preenche todo o seletor em 450 ms, permanece 700 ms e fecha em 350 ms. A confirmação rápida do servidor não corta os 1,5 s de feedback; rejeição ou nova escolha exigida restauram o controle.
- Resultado entra com linhas escalonadas, coroa com faixa de luz e brilho, botão de revanche com entrada elástica e marca de aceitação. Consenso, pontuação pendente e pontos confirmados seguem o backend existente.
- 14 efeitos sonoros originais locais para seleção, hover, retorno, compra, distribuição, descarte, cor, inversão, bloqueio, seletor, vitória/derrota e revanche. Som começa desligado; ativar em **Opções da partida → Ativar som**. A preferência fica neste dispositivo. Vibração opcional usa a API Telegram quando suportada. Som tem limite de simultaneidade, descarte ao silenciar/sair e tratamento de falha de autoplay.
- Corrigidos cancelamento indevido de arraste ao renderizar novamente o snapshot inicial de recuperação, cancelamento por revisão/indisponibilidade e faixa de erro que bloqueava o menu.
- Canvas renderiza sob demanda, coalescendo atualizações em um frame e parando de desenhar em WebGL quando a mesa está imóvel. Timelines, layout e gestos pedem novos frames; cleanup cancela o RAF antes de destruir a cena.
- Reduced-motion recompõe o estado, desliga efeitos de movimento e mantém os comandos acessíveis.

## O que veio da captura e dos recursos do projeto

| Fonte | Uso |
| --- | --- |
| `Geometry-foVdG5LD.js` | Identificação da infraestrutura Pixi: projeções, matrizes e mesh. Usadas as APIs oficiais da dependência existente, sem copiar biblioteca compilada. |
| `browserAll-CKxMxBMn.js` | Registro de extensões do navegador; o build Pixi do próprio projeto faz essa função. |
| `MatchoRoute-DNDm1nZ2.js` | Referência de seleção, escala, inclinação, retorno elástico, sombras, compra, descarte e virada. Comportamentos adaptados à arquitetura e aos pedidos do usuário. |
| CSS da partida e imagens de efeitos | Referência visual; recursos locais reais para sombra, setas, destaque e vencedor. |
| JSONs dos atlas | Frames, rotação, tamanho e resolução. O atlas padrão do pacote v2 foi integrado; o último HAR também traz um atlas de variante colorblind, que não substitui automaticamente as cartas padrão pedidas. |
| `docs/design-reference/unobotgo-v2/assets/audio/` | Áudios efetivos. O último HAR lista referências, mas não inclui os corpos dos sons. |
| Fontes locais já presentes | Nunito variável e Lexend Deca Black; o último HAR não inclui seus bytes. |

O pacote não inclui HAR bruto, cookies, tokens, contas, protocolos de outro serviço ou JS compilado de terceiros. Não acrescenta outro backend. Usa o WebSocket autenticado, revisões, deduplicação, engine e finalização existentes.

## Aplicar

Extrair o ZIP em uma pasta de trabalho; não sobrescrever o repositório inteiro.
Na raiz do UnoBotGO:

```bash
git apply --check /caminho/unobotgo-experiencia-completa.patch
git apply /caminho/unobotgo-experiencia-completa.patch
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
npm --prefix web run build
```

Se o patch anterior já foi aplicado, ou a branch avançou, o check pode indicar conflitos. Comparar com `arquivos/` do ZIP e adaptar apenas as diferenças, preservando alterações locais. Não forçar, não executar reset/clean, não aplicar os dois patches completos juntos. Build do frontend gera o bundle que o Go incorpora; reinício/publicação de uma instância externa não faz parte desta entrega.

## Reproduzir apresentação

Com Vite em execução (`npm --prefix web run dev`) e Chromium instalado:

```bash
node web/scripts/motion-check.mjs
node web/scripts/experience-check.mjs
node web/scripts/visual-check.mjs
node web/scripts/experience-preview.mjs
```

`PLAYWRIGHT_EXECUTABLE_PATH` seleciona Chromium local; `PLAYWRIGHT_BROWSERS_PATH` seleciona o cache Playwright/FFmpeg quando necessário. `UNO_VISUAL_URL` seleciona o Vite. Scripts usam fixtures identificadas e interceptam API/WebSocket exclusivamente no harness; não são incluídos no app de produção.

A prévia demonstra distribuição, seleção/arraste/retorno, compra em fileiras, coringa, inversão e resultado/revanche. Seus pontos são fixtures, não ganhos reais. O vídeo não grava áudio; os arquivos de som integram o app e foram verificados separadamente.

## Validação concluída

- 117 testes frontend passaram; lint, TypeScript, build e `git diff --check` passaram.
- Harness de movimento passou: distribuição/compra/descarte, perspectiva, CardID, revisões, rejeição, recovery, escolha repetida, reduced-motion e desmontagem.
- Harness de interação passou com toque físico via Chromium: arraste com uma jogada, cancelamento sem jogada, perda de turno durante captura, virada pública adversária, efeitos sem replay, áudio opt-in/silenciar, haptics, troca com 16 cartas e WebGL sem draw calls durante o repouso.
- Matriz final: 324 combinações em 320x568, 360x800, 390x844 e 430x932, com 1/2/7/8/9/15/16/17/30 cartas e 2–10 participantes. 37 seleções estreitas, 36 listas de resultado, estados de cor/turno/reconexão e safe areas conferidos; sem overflow horizontal, última carta acessível.
- Os 63 frames do atlas foram comparados pixel a pixel e os 14 áudios têm streams válidos. Áudio no navegador usa respostas locais 206 (Range), normais para reprodução.
- Prévia gravada em fixtures, sem erros de página. Evidências locais: `.reports/experiencia-completa/`; relatório final de composição em `visual-final/checks.json`.

## Limites de homologação

Esta entrega não publica nem reinicia o bot. Nenhum commit, push ou deploy. API/engine/pontuação não foram modificadas. Go/PostgreSQL não estão disponíveis neste ambiente para uma nova execução do E2E real; a validação atual é frontend e apresentação local. Telegram real, som físico, vibração e GPU do aparelho precisam de homologação no ambiente do usuário. O build mantém o aviso preexistente de chunk Pixi maior que 500 kB.

Documentação de API consultada: https://pixijs.com/8.x/guides/components/scene-objects/mesh ; https://pixijs.download/v8.22.0/docs/scene.PerspectiveMesh.html ; https://gsap.com/docs/v3/GSAP/gsap.matchMedia()/ ; https://gsap.com/docs/v3/GSAP/Timeline/killTweensOf()/ .

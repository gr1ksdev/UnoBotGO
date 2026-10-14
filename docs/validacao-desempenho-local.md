# Aplicação e validação local: desempenho e fullscreen

Data: 2026-10-11. Autorização explícita do usuário para planejar/aplicar/adaptar/validar, sem nova confirmação, commit, push ou deploy.

## Base, aplicação e preservação

HEAD dev `7b85f69510ad5be1d2bd46b9a7e7dab16a798084`, mais pacote completo já aplicado. ZIP/checksums íntegros; patch externo idêntico ao interno. Todos os arquivos de código coincidiam com hashes `before_sha256` de ARQUIVOS.json.

`git apply --check` completo falhou somente em `.agent/context.md`, `.agent/decisions.md` e `.agent/memory/memory.md`, que tinham registros locais adicionais. Hunks restantes passaram no check e foram aplicados via Git. Nos três registros, acrescidas apenas linhas novas do patch; conteúdo local anterior preservado. Pacotes anteriores não reaplicados.

Código de produção idêntico ao resultado do patch. A única adaptação adicional foi no harness `web/scripts/performance-check.mjs`:

- Aguardar `.game-v2` sair do DOM antes de medir frames após desmontagem. O clique de navegação sozinho podia terminar antes da desmontagem e produzir falha espúria.
- Medir o erro da posição da carta após o handler da cena no mesmo evento `pointermove`, aguardando entrega da última coordenada. A confirmação CDP pode preceder a entrega do evento: a verificação original encontrou um evento de atraso (2,236 px), embora o handler atualizado já acompanhasse diretamente cada evento recebido. A nova assertion verifica todos os eventos recebidos, sem esperar um tween alcançar o destino.

Primeira medição pós-patch encontrou módulo diferente na instrumentação por causa do cache/HMR após editar a cena; reiniciar somente Vite temporário resolveu. Falhas de diagnóstico e alterações no método são registradas aqui, não atribuídas a FPS físico.

Assets/fontes/áudios (166 arquivos) e `docs/validacao-experiencia-local.md` conferidos por SHA-256 e preservados byte a byte. Nenhuma mudança de dependências, CSS, regras, pontuação, API/WebSocket, privacidade ou consenso de revanche.

## Trabalho antes/depois: medição local

Mesmo harness final, Chromium headless, 390×844 @2x, 30 cartas, dez jogadores. Baseline final executado em cópia temporária isolada, com hashes dos quatro módulos de runtime verificados contra o manifesto; projeto atual não foi revertido. Saídas: `.reports/desempenho-local/antes-final/checks.json`, `depois/checks.json` e `comparacao.json`. Primeira baseline e diagnóstico preservados separadamente.

| Exercício | Antes local | Depois local |
| --- | ---: | ---: |
| 20 syncs idênticos: layouts | 20 | 0 |
| 20 syncs idênticos: sprites | 180 | 0 |
| 20 syncs idênticos: tweens | 1200 | 0 |
| Seleção: sprites | 27 | 0 |
| Seleção: tweens | 182 | 4 |
| 16 posições de scroll: sprites | 144 | 0 |
| Arraste: tweens | 26 | 0 |
| Erro máximo de posição no mesmo pointermove (px) | 8.944 | 0.000 |
| Renders em 600 ms de repouso | 0 | 0 |
| Fullscreen solicitado em tdesktop simulado | 1 | 0 |

Contadores de trabalho coincidem com as evidências fornecidas no ZIP. O erro de arraste usa método mais rigoroso que o ZIP e não deve ser comparado com seu valor antigo, medido entre chamadas CDP.

O código instalado GSAP registra updateRoot primeiro e acrescenta listeners não prioritários ao fim: flushRender desenha a cena suja após atualizar a timeline, no mesmo ticker, sem RAF encadeado. Listener sai no repouso e antes de destruir Pixi. Cache preserva mesa/máscara em seleção/scroll; posições e referências DOM são reutilizadas; apenas cartas anterior/nova recebem tweens de seleção. Durações deliberadas de voo/retorno/coringa preservadas.

## Intervalos headless: não são FPS físico

| Diagnóstico (ms) | Antes | Depois |
| --- | ---: | ---: |
| Renderer: mediana | 82.30 | 67.30 |
| Renderer: p95 | 96.60 | 152.00 |
| RAF: mediana | 16.70 | 16.70 |
| RAF: p95 | 83.30 | 66.80 |

O p95 do renderer piorou nesta execução, mesmo com menor trabalho e mediana menor. Isso impede concluir ganho uniforme de fluidez a partir desses intervalos. Renderização sob demanda inclui pausas sem desenho; SwiftShader e carga da máquina afetam os resultados. Não houve medição de FPS físico, latência toque→display ou garantia de 60/120 FPS. HAR não fornece esses dados.

## Validações

- Lint, typecheck, 128 testes frontend em 12 arquivos e build aprovados; lint repetido após adaptar o harness.
- Plataforma/API: testes para android/ios compatíveis, tdesktop/macos/web/weba/webk/ausente/desconhecida, SDK antigo, recusa e fullscreen pré-existente. Sem critério de largura e sem forçar saída de fullscreen manual.
- Teste de ACK imediato do coringa antes do primeiro tick aprovado; feedback permanece aberto.
- Performance final com assertions habilitadas aprovada: zero alocações redundantes nos cenários, arraste síncrono, cancelamento por indisponibilidade sem revisão nova, repouso, cleanup e nenhum comando de jogo gerado pelas medições.
- Motion harness aprovado: dois toques, compra/distribuição, coringa com ACK imediato/feedback completo, rejeição/cancelamento, revisões/recovery, chegadas adiadas, preferência de movimento, descarte, privacidade e desmontagem.
- Interação aprovada: arraste/toque com comando único, retorno, perda de turno, compra adversária pública, efeitos sem replay, áudio opt-in e mute, API haptic simulada, troca de 16 cartas privadas, repouso WebGL.
- Matriz visual aprovada: 324 combinações em 320×568, 360×800, 390×844 e 430×932, 37 seleções estreitas, 36 listas de resultado, quatro safe areas e estados de reconexão/revanche. Até oito cartas/fileira, margens, centralização e rolagem vertical preservadas.
- Capturas revisadas: mão de 30 cartas/dez participantes em 320 px e resultado com voto de revanche em 390 px; aparência consistente com evidências da experiência anterior.
- Verificação final de integridade: todos os assets e documentos preexistentes preservados; relatório em `.reports/desempenho-local/integridade.json`. `git diff --check` aprovado.
- Servidores Vite temporários (5174/5175) encerrados; nenhum processo real do bot foi iniciado ou reiniciado.

## Reproduzir os harnesses na versão corrigida

Com `npm --prefix web run dev -- --port 5174 --strictPort` em execução:

```bash
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_PERFORMANCE_OUTPUT=.reports/desempenho-local/reteste node web/scripts/performance-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_MOTION_OUTPUT=.reports/desempenho-local/reteste-motion node web/scripts/motion-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_EXPERIENCE_OUTPUT=.reports/desempenho-local/reteste-interaction node web/scripts/experience-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_VISUAL_OUTPUT=../../.reports/desempenho-local/reteste-visual/ node web/scripts/visual-check.mjs
```

Reiniciar Vite após mudar a versão do renderer antes de instrumentar seu prototype evita divergência de URLs de módulos por HMR. Baseline usa `UNO_PERFORMANCE_EXPECT_FIX=0` sobre a versão anterior isolada; não executar sobre a versão corrigida e chamar o resultado de "antes".

## Limites

Telegram Android/iOS físicos e desktop/web reais não disponíveis nesta sessão. Homologar entrada na janela/fullscreen no cliente real, frames regulares, compra/distribuição/arraste com 7/15/30 cartas, áudio, vibração e consumo com condições equivalentes de temperatura/energia. Não rodado novo E2E Go/PostgreSQL nesta correção frontend. Build mantém aviso informativo do chunk Pixi (939,82 kB / 272,03 kB gzip). Sem atualização de dependências e sem commit/push/deploy.

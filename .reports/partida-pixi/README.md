# Partida: PixiJS, fileiras e GSAP

Correção autorizada implementada sobre a integração existente. Alterações locais anteriores preservadas. Sem commit, push, deploy, reinício do bot real ou mensagens Telegram.

## Implementação

- PixiJS 8.22 renderiza cartas originais, versos, pilhas, setas, mão mascarada e voos. GSAP 3.15/PixiPlugin executa os movimentos; as duas bibliotecas são usadas de fato.
- Os 34 SKILL.md locais de PixiJS/GSAP foram lidos integralmente. Referência prioritária: `docs/design-reference/unobotgo-v2`, capturas `original-*` e `MOVIMENTOS.md`.
- Mão: até oito cartas por fileira; vermelho, amarelo, verde, azul e coringas; rank e CardID como desempate estável. Margens de 16 px, última fileira centralizada, tamanho adaptativo, rolagem vertical apenas da mão. O array da engine não é reordenado.
- Alvos de toque não se sobrepõem. O gesto conserva o layout e o ID; eventos atualizam imediatamente turno/jogabilidade. Entradas durante o gesto aguardam sua liberação para acomodação visual.
- Cartas apagadas usam tint multiplicativa, preservando cor. Turno, brilho e disponibilidade vêm da projeção autorizada. Canvas não intercepta comandos; controles e nomes acessíveis ficam no DOM.
- Distribuição alternada; compras/viradas de 780/500 ms, intervalos de 130 ms em múltiplas compras, jogadas de 560 ms, seleção de 380 ms e reorganização de 580 ms. Compra em nova fileira rola suavemente a mão até o destino. Versos são usados para adversários.
- Cor: intenção enviada imediatamente; pétalas mudam por 450 ms, mantêm 700 ms e saem em 350 ms. ACK rápido não encurta feedback; rejeição restaura cores e permite tentar novamente. O descarte usa a variante da cor confirmada, sem alterar o tipo da engine.
- Resultado sobre a mesa, participantes/posição/pontos reais, estado de persistência, vencedor dourado e coroa com luz. Revanche usa consenso no servidor; uma aceitação não reinicia. Saída/desconexão não exclui ninguém do grupo exigido; retirada explícita revoga apenas o próprio voto.
- Novo GameID publicado de forma atômica e idempotente; votos antigos não criam outra partida. Resultado pendente bloqueia revanche até ACK do COMMIT. Resultado anterior permanece no PostgreSQL.
- Limpeza de tweens antes de destruir sprites, máscara/listeners/renderer removidos na desmontagem. Recuperação não repete histórico; resize real e movimento reduzido recompõem o estado. A primeira notificação do ResizeObserver, sem mudança de tamanho, não cancela a distribuição.
- Autenticação, engine, timers, WebSocket, revisões, deduplicação, projeção privada, finalização e ranking mensal Legacy/Updated preservados. Criação/convite/entrada/início ficam disponíveis na própria Mini App.

## Recursos e fidelidade

`assets-check.json` confirma os 76 arquivos do pacote v2 idênticos por SHA256 aos originais. Cartas v2 e verso são 256×344, com transparência; `card_overlay.png` não é carregado como carta. Nunito variável local (base 550) e Lexend Deca Black local (900 no resultado) são verificados com fontes carregadas.

O pacote v2 não contém a carta de troca de mãos (rank 15). Ela mantém o PNG original do bot, de 342×512, acomodado ao quadro 256:344. A virada considera suas dimensões próprias; nenhuma carta foi redesenhada. Esse caso está em `swap-original-in-flight-320.png`.

Comparações lado a lado: `comparisons/` (original fornecido à esquerda, implementação com fixture identificada à direita). Foram conferidas cartas, verso, fontes, escurecimento, seletor, cor confirmada, composição e resultado; a entrega não se baseia somente no build.

## Checks locais

| Check | Resultado / evidência |
| --- | --- |
| `make check` com banco exclusivo de testes | Passou; `check-final.log`: lint, TypeScript, 108 testes frontend, build, Go, vet, builds normais/debugcards, integração PostgreSQL/app e diff |
| `go test -race -count=1 ./internal/game ./internal/httpapi` | Passou; `race.log`, incluindo revanche concorrente, votos duplicados/retirados, dez participantes, falha/retry e resultado pendente |
| Composição em Chromium, quatro dimensões | Passou: 324 combinações; ajuste visual final também em 16 casos de `final-states/checks.json`; `visual.log` e `checks.json` |
| Movimentos/lifecycle no bundle estático | Passou; `motion.log` / `motion-checks.json` |
| Dois navegadores, engine/WS/PostgreSQL e bundle Go reais | Passou; 39 jogadas, 23 compras, cinco cores, consenso e transação confirmados; `production-e2e.log`, `production/browser.json`, `production/backend.json` |
| Recursos originais | Passou; `assets-check.json` |

A composição usa 320×568, 360×800, 390×844 e 430×932, DPR 2, nomes longos e avatares ausentes. Mãos 1, 2, 7, 8, 9, 15, 16, 17 e 30, jogadores 2–10. Inclui minha/outra vez, seleção, cor antes/depois, resultado, voto único/todos e reconexão. Verifica margens, centralização, oito/fileira, ausência de rolagem horizontal, sete sem rolagem, última carta alcançável, toque nas 30 cartas e gesto vertical estreito. Safe areas e lista de dez participantes também são exercitadas.

O check de movimentos testa rejeição e nova tentativa de cor, ACK rápido, duplicação por revisão, evento atrasado, turno durante compra, entrada durante gesto, compra em nova fileira, distribuição visível após inicialização, asset de troca, desmontagem e troca de GameID. Também confirma CardID durante voo, versos anônimos de adversários, recovery atrasado na mesma revisão, nova escolha exigida durante feedback anterior e cleanup após falha de recursos. As animações não enviam comandos.

## Partida real local e vídeo

`production/` usa o bundle embutido servido pelo Go, com CSP de produção intacta, dois Chromium isolados, HTTP/WebSocket/engine/finalizador/PostgreSQL reais. Autenticação HMAC usa identidades de teste assinadas com token fictício. SDK e validação Telegram de membros são exclusivos do teste; nenhuma mão alheia chega a outro cliente.

O percurso cria/compartilha/entra/inicia pela UI, joga até conclusão normal, reconecta, repete o último comando, valida ranking de jogadores/grupo e um histórico por conta. Vota, reconecta com voto mantido e abre uma única nova partida por consenso. O backend confirma um resultado, uma notificação, dois participantes no ranking e retries de finalização sem duplicação.

Vídeo curto (10,56 s), em velocidade normal: `production/video/movimentos-reais-11s.mp4`. Contém trechos reais de distribuição, compra/virada e escolha de cor; `segments.json` registra os cortes dos vídeos completos `account-1.webm` / `account-2.webm`. `contato-movimentos.png` permite localizar frames; `compra-conta-1-frames.png` detalha a saída do verso, virada e chegada da carta verde autorizada, mas o vídeo é a evidência de movimento. `generate-evidence.py` reproduz comparações e cortes com Pillow/FFmpeg.

## Limites e confirmação Telegram

Confirmado localmente: navegador com toque emulado/CDP, bundle de produção, CSP, transporte, engine, persistência, consenso, privacidade e ranking reais. Dados de composição e seus pontos são fixtures explícitas; os resultados em `production/` vêm do serviço real, não da referência ilustrativa.

**Não confirmado no Telegram real ou em aparelho físico.** Não houve uso de contas reais, BotFather, URL publicada, WebView Telegram físico ou acesso ao ambiente de produção. Homologação nesses ambientes permanece pendente.

Salas, receipts e votos continuam em memória, conforme o runtime existente; reinício do servidor não restaura esses votos. Resultados já transacionados permanecem no PostgreSQL. Não foi introduzida outra fonte de pontuação ou alteração de regras.

Logs/capturas de tentativas anteriores em `integration/`, `e2e.log`, `visual-focused.log` e `.reports/partida-v2/` foram preservados. A revisão do vídeo revelou o cancelamento indevido pela notificação inicial de ResizeObserver; esse caso ganhou check específico e foi corrigido. A evidência final de integração é `production/`.

## Reexecutar

```bash
TEST_DATABASE_URL='postgres://unobot:unobot@localhost:5432/unobot_v2_checks?sslmode=disable' make check
# Em web/: node node_modules/vite/bin/vite.js preview --host 127.0.0.1 --port 5176 --strictPort
UNO_VISUAL_URL=http://127.0.0.1:5176 PLAYWRIGHT_EXECUTABLE_PATH=/caminho/chrome npm --prefix web run test:visual
UNO_VISUAL_URL=http://127.0.0.1:5176 PLAYWRIGHT_EXECUTABLE_PATH=/caminho/chrome node web/scripts/motion-check.mjs
TEST_DATABASE_URL='postgres://unobot:unobot@localhost:5432/unobot_v2_checks?sslmode=disable' UNO_BROWSER_E2E=1 UNO_E2E_PRODUCTION=1 PLAYWRIGHT_EXECUTABLE_PATH=/caminho/chrome CGO_ENABLED=1 go test -race -count=1 -tags integration ./internal/httpapi -run TestTwoBrowserWebAppGame -v
python3 .reports/partida-pixi/generate-evidence.py
```

## Conteúdo versionado

Após autorização posterior de commit/push para dev, são versionados código, pacote v2, relatório/checks/logs finais, capturas finais da partida nos quatro tamanhos, comparações e vídeo curto. Matriz completa de PNGs, relatórios anteriores, skills baixadas e vídeos completos permanecem locais. O script de evidências requer esses vídeos completos para recortar novamente; `segments.json` registra as fontes e cortes do vídeo curto já versionado. Nenhum deploy solicitado.

# Aplicação e validação local da experiência completa

Data: 2026-10-11. Aprovação: mensagem explícita do usuário para aplicar o pacote, adaptar diferenças e validar, sem commit/push/deploy.

## Base e preservação

- Branch `dev`, HEAD `7b85f69510ad5be1d2bd46b9a7e7dab16a798084`, exatamente a base declarada no ZIP.
- Antes da aplicação, único arquivo não rastreado: `unobotgo-experiencia-completa.zip`; nenhuma alteração em arquivos rastreados.
- SHA-256 do patch e vídeo conferido contra `SHA256SUMS.txt`.
- `git apply --check` aprovado; aplicado somente o patch consolidado.
- Os 39 arquivos aplicados foram comparados byte a byte com `arquivos/`: todos idênticos, incluindo binários. Nenhuma adaptação de código necessária.
- Registros `.agent` do pacote preservados como histórico de sua preparação; esta seção documenta a execução local posterior.

## Verificações locais

- Dependências instaladas com `npm --prefix web ci`, sem alteração de package.json/lockfile.
- Lint, typecheck, 117 testes em 12 arquivos e build frontend: aprovados.
- `go test ./...`, `go vet ./...`, `go build ./...`: aprovados, com bundle frontend atual incorporado.
- Harness `motion-check.mjs`: aprovado; dois toques/CardID, distribuição, perspectiva/compra, descarte anterior, recovery, rejeição, escolha repetida, retorno de movimento interrompido, reduced motion e desmontagem.
- Harness `experience-check.mjs`: aprovado; toque CDP, arraste com comando único, cancelamento e perda de turno, revelação pública adversária, efeitos sem replay, áudio opt-in/persistência/silenciar, API haptic simulada, troca de 16 cartas privadas e ausência de draw calls WebGL em repouso.
- 63 frames do atlas: comparação pixel a pixel via Canvas com PNGs originais aprovada, incluindo frames girados conforme dimensões do formato Pixi.
- 14 áudios: idênticos aos assets originais do projeto e decodificados com sucesso pelo AudioContext do Chromium.
- Harness `visual-check.mjs`: 324 combinações em 320×568, 360×800, 390×844 e 430×932 aprovadas; até oito cartas por fileira, margens de 16 px, última fileira centralizada, rolagem vertical por toque e última carta acessível, sem overflow horizontal. Também passaram 37 seleções estreitas, 36 listas de resultado, quatro estados de derrota, 20 estados de ranking e quatro safe areas.
- Capturas inspecionadas: descarte em 320 px, mão de 30 cartas/dez jogadores e sua última fileira em 320 px, resultado de dez jogadores em 360 px.
- Harness `experience-preview.mjs`: vídeo WebM gravado e captura final revisada; distribuição, seleção/retorno, compra de três cartas, coringa, inversão, vitória e aceitação da revanche. Nenhum erro de página.
- `git diff --check`: aprovado. Servidor Vite temporário da porta 5174 encerrado ao concluir.

## Evidências e reprodução

Evidências locais em `.reports/experiencia-local/`: `motion/motion-checks.json`, `interaction/checks.json`, `assets.json`, capturas, relatório visual e prévia. Fixtures não contêm contas reais ou pontos ganhos.

```bash
npm --prefix web run dev -- --port 5174 --strictPort
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_MOTION_OUTPUT=.reports/experiencia-local/motion node web/scripts/motion-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_EXPERIENCE_OUTPUT=.reports/experiencia-local/interaction node web/scripts/experience-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_VISUAL_OUTPUT=../../.reports/experiencia-local/visual/ node web/scripts/visual-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_PREVIEW_OUTPUT=.reports/experiencia-local/preview node web/scripts/experience-preview.mjs
```

## Limites e avisos

- macOS, Node 24.21.0, Go 1.27.2 e Chromium Playwright. Renderização headless com SwiftShader; não equivale à GPU física do telefone.
- Docker daemon indisponível e PostgreSQL não encontrado no PATH. Não executados `make check` (exige TEST_DATABASE_URL), testes de integração PostgreSQL ou E2E real de duas contas com backend/banco. Nenhuma instância real foi iniciada ou reiniciada.
- Telegram real, som audível e vibração física não homologados. Vídeo de prévia não grava áudio.
- Aviso de bundle Pixi: chunk TableScene 938,90 kB (271,69 kB gzip), acima do limite informativo de 500 kB.
- `npm audit` registrou quatro vulnerabilidades nas dependências já fixadas pelo projeto: @vitest/mocker (moderada), source-map-js (alta), tinypool e vitest (críticas). Relatório completo: `.reports/experiencia-local/npm-audit.json`. Nenhuma atualização de dependências fora do escopo foi aplicada.
- Sem commit, push ou deploy. O ZIP original permanece preservado.

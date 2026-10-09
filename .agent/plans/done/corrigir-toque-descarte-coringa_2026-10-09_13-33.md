# Plano: corrigir toque, descarte e coringa

## Pedido do usuário
Primeiro toque na carta jogável eleva/seleciona; segundo toque na mesma carta joga diretamente, removendo botão de confirmação. Descarte deve conservar aparência de monte, sem sumir a carta anterior durante o voo. Seletor do coringa não deve herdar feedback de escolha anterior.

## Objetivo
Corrigir interação e lifecycle visual em React/PixiJS/GSAP, preservando CardID, estado autorizado, revisões e deduplicação existentes.

## Contexto atual
Branch dev, último commit ffdcd34 enviado. GameTable seleciona pelo ID e confirma por botão separado. TableScene recria camada da mesa a cada layout e oculta somente a nova carta do descarte durante o voo, sem camada anterior. Choice limpa chosen/sent ao terminar feedback, mas conserva cores inline das pétalas; reset atual depende de sent ainda estar ativo, permitindo reapresentar cores antigas.

## Arquivos analisados
- AGENTS.md
- .agent/context.md
- web/src/pages/Game.tsx
- web/src/pages/Game.test.tsx
- web/src/components/scene/TableScene.ts
- web/src/api/client.ts
- web/scripts/game-e2e.mjs
- web/scripts/motion-check.mjs

## Arquivos que poderão ser modificados
- web/src/pages/Game.tsx
- web/src/pages/Game.test.tsx
- web/src/components/scene/TableScene.ts
- web/src/game-v2.css
- web/scripts/game-e2e.mjs
- web/scripts/motion-check.mjs
- web/scripts/visual-check.mjs
- .reports/correcao-toque-descarte-coringa/
- .agent/memory/memory.md
- .agent/decisions.md
- .agent/context.md

## Estratégia de implementação
Reutilizar skills PixiJS/GSAP locais já lidas. Primeiro toque seleciona por CardID; segundo toque nesse ID envia play imediatamente. Carta diferente apenas muda seleção. Remover confirmação separada e manter informação acessível da interação, compra/passar/desafio e bloqueio de comandos pendentes.

Descarte usa pequena pilha visual de cartas públicas confirmadas, com offsets e rotações discretos. A antiga face permanece sob a carta em voo; a nova ocupa o topo ao chegar. Limitar histórico visual, deduplicar por ID/revisão e recompor após cancelamento/recovery/resize. Histórico incompleto na reconexão usa somente informação pública disponível; não inventar cartas nem expor mão alheia.

Resetar as quatro cores e estado local ao começar nova escolha, identificando sala, carta e fase. Matar feedback/saídas antigos que poderiam afetar novo seletor. Conservar feedback 450+700+350 ms para a escolha vigente, ACK rápido e retry por rejeição. Conferir também variante do descarte para coringa novo versus cor confirmada, sem alterar a engine.

## Passos detalhados
1. Após aprovação, mover plano para approved.
2. Atualizar gesto em GameTable com guardas de disponibilidade e comando duplicado; remover botão separado.
3. Atualizar testes e automação para segundo toque na mesma carta, incluindo troca de seleção e turno/rejeição.
4. Implementar pilha pública limitada no PixiJS e conservação da face anterior durante jogada.
5. Corrigir reset de cores/feedback por nova escolha e verificar coringas consecutivos de jogadores diferentes.
6. Executar checks obrigatórios, browser stress e integração real de dois clientes.
7. Registrar screenshots/vídeo nas quatro dimensões e revisar ausência de vazio no descarte e cores herdadas.
8. Atualizar memória/decisões/contexto e mover plano para done. Não realizar novo commit/push/deploy sem solicitação para esta alteração.

## Riscos
- Segundo toque/teclado repetido pode duplicar intenção antes de pending chegar: guarda síncrona e receipts existentes.
- Jogadas consecutivas durante voo: manter topo e camadas alinhados às revisões aceitas.
- Callback antigo do seletor fechar escolha nova: cancelar e vincular à identidade atual.
- Resize/recovery não podem restaurar pilha histórica incorreta nem repetir comandos.

## Impactos esperados
- Jogada por dois toques na carta, sem botão separado.
- Monte de descarte contínuo com cartas reais, sem vazio durante animação.
- Cada nova escolha abre com quatro cores próprias, sem herança da escolha anterior.

## Compatibilidade
- Linux, macOS e Windows: stack Go/Vite existente.
- Docker e CI/CD: mesmos builds/checks, sem novas dependências.
- Toque, mouse, teclado e movimento reduzido preservados.

## Como testar

### Build
```bash
npm --prefix web run build
```

### Testes
```bash
TEST_DATABASE_URL='postgres://unobot:unobot@localhost:5432/unobot_v2_checks?sslmode=disable' make check
UNO_VISUAL_URL=http://127.0.0.1:5176 node web/scripts/motion-check.mjs
TEST_DATABASE_URL=... UNO_BROWSER_E2E=1 UNO_E2E_PRODUCTION=1 CGO_ENABLED=1 go test -race -count=1 -tags integration ./internal/httpapi -run TestTwoBrowserWebAppGame -v
```
Usar Chromium disponível e banco isolado de testes. Testar 320×568, 360×800, 390×844 e 430×932; dois toques, seleção diferente, repetição rápida, rejeição, outra vez, comandos pendentes, pilha durante todo voo, coringas consecutivos, ACK rápido, reconexão, resize e desmontagem. Separar testes locais de homologação Telegram real.

### Execução
```bash
npm --prefix web run dev
```

## Rollback
Reverter apenas trechos desta correção após revisão, preservando arquivos locais e histórico. Sem reset/clean.

## Observações
Esta é nova alteração de interação e correção posterior ao commit enviado; aguarda aprovação explícita do plano conforme AGENTS.md. Nenhuma alteração de implementação foi feita nesta análise.

## Execução
Aprovado pelo usuário (“sim”) e implementado. make check passou (109 testes frontend), build final passou, movimentos passaram, 16 composições e estados nos quatro tamanhos passaram, E2E real local com race/Go/WS/PostgreSQL passou. Capturas do descarte durante voo e reset de cor conferidas; vídeos reais locais de 10,56 s e 4 s conferidos por frames. Docs atuais, memória e decisões atualizados. Relatório .reports/correcao-toque-descarte-coringa/README.md. Telegram real/aparelho físico pendentes; sem novo commit/push/deploy.

## Autorização posterior de commit/push
Usuário solicitou explicitamente commit e push para dev. Conferir diff e sincronização; versionar implementação, documentação e evidências compactas; preservar demais arquivos locais e vídeos completos; criar commit e fazer push normal para origin/dev; verificar hashes. Autorização substitui a restrição anterior de commit/push desta correção, mantendo deploy fora do escopo.

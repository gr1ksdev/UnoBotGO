# Plano: completar acesso à partida WebApp

## Pedido do usuário
Continuar escopo aprovado: acesso interno claro, sala após /novo e /entrar, dois clientes WebSocket até commit e ranking, evidências e instruções. Sem commit/push/deploy.

## Objetivo
Conectar descoberta/retomada/deep link à partida real e validar fluxo ponta a ponta.

## Contexto atual
Rotas de salas, snapshots, socket, comandos, engine e finalizador existem. Home oferece redirecionamento externo primário em lista vazia, omite lista para uma sala e não atualiza em activated. Bot não oferece atalho de partida e root não interpreta start_param. Testes prévios são parciais; faltam dois navegadores com banco real até resultado.

## Arquivos analisados
- AGENTS.md, memória/contexto e plano anterior
- web/src/pages/Home.tsx, Game.tsx, App.tsx, hooks/useGame.ts, lib/telegram.ts
- internal/httpapi/live.go, live_test.go, internal/game/service.go, manager.go, finalization.go
- internal/telegram/commands.go, ranking.go, bot.go e testes
- internal/storage/postgres/store.go, migrations_integration_test.go, webapp_integration_test.go
- docs/design-reference/unobotgo-v1/docs/INTEGRACAO.md

## Arquivos que poderão ser modificados
- Home, App, lib Telegram, estilos e testes frontend
- commands/ranking e testes Telegram
- testes de integração HTTP e script browser E2E
- configuração proxy Vite, docs/miniapp-webapp.md, relatórios e memória

## Estratégia de implementação
Ação principal interna; lista de salas visível inclusive única sala; atualização explícita e ao retomar Telegram/foco/visibilidade. Deep links startapp apontam sala, nunca autorizam usuário. Testar com dois contextos Playwright isolados, initData assinado apenas para bot fictício de teste, API/socket/engine/PostgreSQL reais. Nenhum bypass ou mock em produção.

## Passos detalhados
1. Corrigir Home e estados; adicionar atualização ao retomar sem polling de partida.
2. Bot oferece Abrir na Mini App com startapp da sala; root encaminha parâmetro para rota autorizada.
3. Testes regressivos de descoberta/links/foco e bot.
4. Integração real de dois navegadores: sala, início, mãos, compra/jogada/turnos, reconexão, término, commit, histórico e rankings idempotentes.
5. Comparar visual Home nos quatro tamanhos, executar checks, documentar evidências e teste com duas contas.

## Riscos
- Parâmetro de lançamento é apenas endereço; backend deve preservar autorização.
- Partidas ainda em memória; ambas contas precisam mesmo processo/bot.
- Telegram real exige URL HTTPS acessível e Mini App configurada; sem publicar automaticamente.

## Impactos esperados
Acesso de sala deixa de depender de redirecionamento externo. Evidência reprodutível do percurso completo.

## Compatibilidade
Linux/macOS/Windows, Docker/CI; E2E requer Chromium e PostgreSQL isolado.

## Como testar
### Build
```bash
npm --prefix web run build
go build ./...
```
### Testes
```bash
make check
go test -race ./...
# E2E opt-in documentado após implementação
```
### Execução
```bash
make dev
```

## Rollback
Reverter somente alterações desta continuação; preservar .env e arquivos locais. Sem operações destrutivas.

## Observações
Aprovado explicitamente pela mensagem atual dentro do escopo existente; não solicitar nova aprovação. Referências oficiais Telegram Mini Apps e coder/websocket consultadas.


## Conclusão
Implementados acesso interno/lista/retomada, deep link gerado pelo bot e encaminhamento da sala a partir de qualquer URL inicial. Testes dos comandos /novo e /entrar confirmam sala descobrível e link correto, mantendo outsider sem acesso. E2E de dois navegadores com banco real passou com -race, incluindo encerramento normal e ranking/histórico/score idempotentes. make check completo, race normal e 64 testes frontend passaram. Capturas nas quatro dimensões revisadas. Evidências/instruções/limites documentados. Sem commit/push/deploy e sem alterações adicionais no .env.

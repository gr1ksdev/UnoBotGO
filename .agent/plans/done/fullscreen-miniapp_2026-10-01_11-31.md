# Plano: fullscreen-miniapp

## Pedido do usuário
Usar o cabeçalho próprio do Mini App em vez do cabeçalho Telegram, implementando fullscreen nativo quando possível. Usuário autorizou explicitamente: implemente pra mim; continue.

## Objetivo
Solicitar fullscreen uma vez na abertura, preservar modo expandido como fallback e manter conteúdo fora dos controles nativos.

## Contexto atual
Branch dev limpa. useTelegram é usado em App e RankingsPage, chama ready/expand e sincroniza safeAreaInset + contentSafeAreaInset. Headers próprios e botão voltar já existem. API oficial 8.0 oferece requestFullscreen/isFullscreen/fullscreenChanged/fullscreenFailed. Controles nativos não podem ser removidos pelo app.

## Arquivos analisados
- AGENTS.md (instruções da sessão)
- .agent/context.md
- web/src/lib/telegram.ts
- web/src/App.tsx
- web/src/App.test.tsx
- web/src/pages/Rankings.tsx
- web/src/styles.css
- web/src/test/setup.ts
- web/package.json
- https://core.telegram.org/bots/webapps

## Arquivos que poderão ser modificados
- web/src/lib/telegram.ts
- web/src/lib/telegram.test.tsx
- web/src/App.tsx
- web/src/pages/Rankings.tsx
- .agent/memory/memory.md
- .agent/decisions.md

## Estratégia de implementação
Inicialização explícita no App, uma vez por instância WebApp; verificar suporte 8.0 e estado antes de solicitar fullscreen. Expand permanece fallback. Eventos fullscreen atualizam insets/viewport sem solicitar novamente. Header color acompanha página azul/vermelha para contraste dos controles Telegram. Não remover BackButton nem alterar autenticação, API ou ranking.

## Passos detalhados
1. Registrar plano e autorização e mover a approved.
2. Separar inicialização do hook de navegação e adicionar métodos opcionais Telegram.
3. Solicitar fullscreen com detecção de suporte, proteção contra chamadas duplicadas e exceções, e listeners registrados antes da solicitação.
4. Manter insets existentes; atualizar em fullscreenChanged/fullscreenFailed; definir cor nativa conforme página.
5. Testar clientes antigos, já fullscreen, erro síncrono/assíncrono, insets, cleanup, StrictMode e navegação.
6. Executar lint/typecheck/test/build, Go test/build por embed e diff check.
7. Documentar resultado e mover plano a done.

## Riscos
- Telegram pode recusar fullscreen por plataforma: fallback expandido.
- Controles obrigatórios Telegram permanecem; homologação real no celular necessária.

## Impactos esperados
Cabeçalho próprio ocupa a apresentação principal em fullscreen; navegação e ranking preservados.

## Compatibilidade
- Linux, macOS, Windows: fallback em clientes sem suporte.
- Docker, CI/CD: processo de build/embed existente.

## Como testar

### Build
```bash
npm --prefix web run build
go build ./...
```

### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
go test -count=1 ./...
git diff --check
```

### Execução
```bash
npm --prefix web run dev
```
Homologar abertura global/detalhe no Telegram móvel e área segura/retorno.

## Rollback
Reverter somente hunks desta tarefa e reconstruir frontend; nenhum dado persistido alterado.

## Observações
Sem commit, push, main, deploy ou migration. Node disponível localmente v26 embora engines declare 24: reportar limitação. Pedido atual autoriza implementação sem nova confirmação, mantendo fluxo documental.

## Resultado
- Implementados fullscreen uma vez na raiz, detecção de API 8.0/estado, fallback expandido, eventos de fullscreen e contraste nativo por página.
- Testes frontend: 45 passaram, incluindo 8 novos casos do hook e navegação existente. Lint, typecheck e build frontend passaram.
- `go test -count=1 ./...`, `go build ./...` e `git diff --check`: passaram após build frontend, validando embed.
- Validação local usou Node 26.10.0; execução sob Node 24 não realizada. Testes PostgreSQL condicionais não representam homologação em produção.
- Nenhum commit/push/deploy/migration. Homologação real no celular pendente.

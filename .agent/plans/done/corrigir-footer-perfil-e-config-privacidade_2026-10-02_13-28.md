# Plano: corrigir-footer-perfil-e-config-privacidade

## Pedido do usuário
1. Corrigir o posicionamento do footer (BottomNavigation) na aba do perfil, que está subindo para o cabeçalho ao acessar a página.
2. Adicionar a seção e botões de privacidade no menu `/config` do grupo no Telegram bot.

## Objetivo
1. Garantir que a barra de navegação inferior (`.bottom-navigation`) permaneça perfeitamente fixada na parte inferior da tela na aba `/profile` (assim como no Ranking Global), com fallbacks CSS seguros contra variáveis indefinidas e aplicação correta das classes de layout e variáveis `--bottom-nav-*`.
2. Integrar a configuração de privacidade do grupo no comando `/config` do Telegram:
   - Exibir a seção de privacidade no texto formatado por `RenderGroupConfig` (`Público` / `Anônimo` com blockquote explicativo e separador).
   - Adicionar uma 3ª linha de botões inline em `makeGroupConfigButtons`: `[ Público ]  [ Anônimo ]` com indicador `✅` no estado ativo.
   - Tratar os callbacks `cfg_privacy_public_<chatID>` e `cfg_privacy_anon_<chatID>` em `handleConfigCallback`, invocando `svc.SetRankingPrivate` e respeitando a autorização de administradores/instalador (`groups.CanConfigureUser`).

## Contexto atual
- Na aba do perfil (`web/src/pages/Profile.tsx`), o elemento principal foi definido com `<main className="app-shell profile-view">`.
- As variáveis CSS `--bottom-nav-gap`, `--bottom-nav-safe` e `--bottom-nav-height` estavam declaradas exclusivamente sob a classe `.global-view`.
- Como `.profile-view` não declarava essas variáveis nem herdava fallbacks em `:root`, a expressão CSS `bottom: calc(var(--bottom-nav-gap) + var(--bottom-nav-safe))` tornava-se inválida no browser (especialmente no WebKit/Safari/Telegram iOS), fazendo com que `bottom` fosse descartado para `auto`. Com `top: auto` e `bottom: auto`, o elemento `position: fixed` subia para o topo do contêiner/cabeçalho.
- No Telegram, o comando `/config` exibe e alterna apenas `Modo padrão` e `Sistema de ranking`, sem permitir aos administradores visualizar ou alternar a privacidade do grupo por botões inline (atualmente só possível por comando textual `/privacidade`).

## Arquivos analisados
- `web/src/styles.css`
- `web/src/pages/Profile.tsx`
- `web/src/pages/Rankings.tsx`
- `web/src/components/BottomNavigation.tsx`
- `internal/telegram/commands.go`
- `internal/telegram/callbacks.go`
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`
- `internal/telegram/config_test.go`
- `internal/groups/groups.go`

## Arquivos que poderão ser modificados
- `web/src/styles.css`
- `web/src/pages/Profile.tsx`
- `internal/telegram/commands.go`
- `internal/telegram/callbacks.go`
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`
- `internal/telegram/config_test.go`

## Estratégia de implementação

### 1. Frontend (Fix do Footer no Perfil)
- Em `web/src/styles.css`:
  - Declarar fallbacks padrão para `--bottom-nav-height: 60px;`, `--bottom-nav-gap: 12px;`, `--bottom-nav-safe: max(env(safe-area-inset-bottom, 0px), var(--telegram-bottom, 0px));`, `--bottom-nav-frame: 10px;` no `:root`.
  - Atualizar o seletor `.global-view` para `.global-view, .profile-view`, garantindo que a página de perfil herde as variáveis de safe inset e o `padding-bottom` necessário para não sobrepor o conteúdo.
  - Blindar `.bottom-navigation` com fallbacks embutidos: `bottom: calc(var(--bottom-nav-gap, 12px) + var(--bottom-nav-safe, 0px));`.
  - Blindar `.bottom-nav-item` com fallback: `height: var(--bottom-nav-height, 60px);`.
- Em `web/src/pages/Profile.tsx`:
  - Adicionar a classe `global-view` ao elemento raiz: `<main className="app-shell global-view profile-view">`.
  - Adicionar `window.scrollTo(0, 0)` no mount da página para evitar herança de scroll da listagem de ranking.

### 2. Telegram Bot (/config com Sessão de Privacidade)
- Em `internal/telegram/renderer.go`:
  - Adicionar `groupPrivacySummary(private bool) (string, string)` retornando título e descrição (`🔒 Anônimo` / `🌐 Público`).
  - Atualizar `RenderGroupConfig(config groups.Config)` para incluir:
    - Linha de cabeçalho: `<b>Privacidade no ranking:</b> %s` (`Público` ou `Anônimo`).
    - Terceiro bloco de citação `<blockquote>...</blockquote>` separado pelo divisor `────────────`.
- Em `internal/telegram/commands.go`:
  - Atualizar `makeGroupConfigButtons(config groups.Config)`:
    - Adicionar 3ª linha no teclado inline com dois botões:
      - `[ ✅ Público ]` ou `[ Público ]` com callback `cfg_privacy_public_<chatID>`.
      - `[ ✅ Anônimo ]` ou `[ Anônimo ]` com callback `cfg_privacy_anon_<chatID>`.
- Em `internal/telegram/callbacks.go`:
  - Expandir o parsing de `parts` em `handleConfigCallback`: aceitar `"privacy"` além de `"mode"` e `"rank"`.
  - Adicionar `case "privacy":` no switch de ação:
    - Mapear `actionArg`: `"public"` -> `false`, `"anon"` -> `true`.
    - Chamar `svc.SetRankingPrivate(ctx, targetChatID, actorID, targetPrivate)`.
    - Tratar erros `ErrForbidden` ("⚠️ Somente administradores ou quem adicionou o bot pode alterar esta configuração.") e genéricos.
    - Atualizar a mensagem via `EditMessageText` com o novo texto de `RenderGroupConfig` e teclado de `makeGroupConfigButtons`.
    - Responder o callback com `AnswerCallbackQuery` informando "Privacidade do grupo alterada para Público/Anônimo.".

### 3. Testes Automatizados
- Em `internal/telegram/renderer_test.go`:
  - Atualizar os casos existentes de `TestRenderer_RenderGroupConfig` com as novas asserções de cabeçalho e blockquote de privacidade.
  - Adicionar cenários explícitos para `RankingPrivate: true` e `RankingPrivate: false`.
- Em `internal/telegram/config_test.go`:
  - Adicionar teste para os botões de privacidade gerados por `makeGroupConfigButtons`.
  - Adicionar teste para o callback `cfg_privacy_anon_<chatID>` e `cfg_privacy_public_<chatID>`, validando autorização, persistência e atualização da mensagem.
  - Testar recusa de alteração de privacidade por usuário sem permissão no grupo.
- No Frontend:
  - Validar testes existentes do frontend (`web/src/pages/Profile.test.tsx` e `web/src/App.test.tsx`).

## Passos detalhados
1. Editar `web/src/styles.css` adicionando fallbacks no `:root`, atualizando `.global-view, .profile-view` e usando fallbacks em `.bottom-navigation`.
2. Editar `web/src/pages/Profile.tsx` adicionando `global-view` e scroll ao topo.
3. Editar `internal/telegram/renderer.go` adicionando `groupPrivacySummary` e atualizando `RenderGroupConfig`.
4. Editar `internal/telegram/commands.go` atualizando `makeGroupConfigButtons` com a 3ª linha de botões de privacidade.
5. Editar `internal/telegram/callbacks.go` adicionando tratamento de `cfg_privacy_...` em `handleConfigCallback`.
6. Atualizar testes unitários em `internal/telegram/renderer_test.go` e adicionar testes em `internal/telegram/config_test.go`.
7. Executar a suíte de testes do frontend (`npm test`), compilar (`npm run build`).
8. Executar os testes unitários do backend (`go test -race ./...`) e integração PostgreSQL.
9. Executar `make check` e `git diff --check`.
10. Atualizar documentação e memória (`memory.md`, `decisions.md`).

## Riscos
- **Risco**: Quebra de layout em telas mobile menores na exibição de 3 blockquotes em `/config`.
  - **Mitigação**: O texto do blockquote de privacidade é conciso (2 linhas) e o Telegram suporta mensagens de até 4096 caracteres.
- **Risco**: Quebra de testes existentes que realizam `assert` no texto de `RenderGroupConfig`.
  - **Mitigação**: Atualizar a tabela de testes em `renderer_test.go` cobrindo todas as combinações de modo, ranking e privacidade.

## Impactos esperados
- A aba de perfil no Mini App manterá a barra de navegação no rodapé de forma estável e consistente em qualquer cliente Telegram (iOS, Android, Desktop e Web).
- O menu `/config` nos grupos permitirá gerenciar a privacidade do grupo visualmente via botões inline com 1 clique, além do comando `/privacidade`.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- Telegram iOS / Android / Desktop / Web

## Como testar

### Build
```bash
export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web run build
go build ./...
```

### Testes
```bash
export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web test
go test -race ./internal/telegram/...
TEST_DATABASE_URL="postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable" make check
```

## Rollback
Reverter as alterações no working tree através do `git checkout` dos arquivos modificados.

## Observações
Trabalho realizado exclusivamente na branch `dev` e mantido no working tree, sem commit ou push.

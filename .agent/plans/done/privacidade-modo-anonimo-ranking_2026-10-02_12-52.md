# Plano: privacidade-modo-anonimo-ranking

## Pedido do usuário
Implementar funcionalidade de privacidade que permita a usuários e grupos aparecerem anonimamente nas superfícies públicas do Ranking Global (bot do Telegram e Mini App).
A feature NÃO remove dados do banco, NÃO altera identidade interna, NÃO interfere no gameplay e NÃO altera cálculos/desempates de ranking. É exclusivamente uma camada de privacidade/apresentação.

## Objetivo
1. Criar persistência para privacidade de grupos (`group_configs.ranking_private`) e usuários globais (`user_privacy_settings.ranking_private`).
2. Adicionar comando `/privacidade` no Telegram:
   - Em chat privado: alterna a privacidade global do usuário remetente (qualquer usuário pode alternar seu próprio perfil).
   - Em grupo/supergrupo: alterna a privacidade daquele grupo específico, respeitando estritamente a autorização de `/config` (apenas administradores ou quem adicionou o bot) e suporte a comandos endereçados (`/privacidade@bot`).
3. Camada de projeção e anonimização no backend (`internal/storage/postgres` e `internal/httpapi`):
   - Projeção no PostgreSQL com joins sem impacto em ordenação ou empates;
   - Regras de composição:
     - Global Grupos: se grupo privado → `Grupo anônimo`, sem avatar, sem masked_id;
     - Global Players: se usuário privado → `Anônimo`, sem avatar, sem masked_id;
     - Detalhe do Grupo: se grupo privado → cabeçalho e TODOS os jogadores do grupo anonimizados; se grupo público → cada jogador respeita sua privacidade pessoal.
   - Proteção de proxy de avatar: não servir avatar de entidades anonimizadas mesmo sob tentativa de adivinhação.
4. Mini App:
   - Adicionar 3ª aba no rodapé deslizante (liquid glass): `Grupos | Players | Perfil`;
   - Nova tela de `Perfil` (`/profile`): obter e alternar privacidade pessoal via endpoints autenticados `GET /api/v1/me/privacy` e `PUT /api/v1/me/privacy` (autenticação HMAC via `initData`);
   - Renderização segura no frontend: avatars neutros/silhueta, sem iniciais de nomes reais, sem masked ID visível para anônimos.

## Contexto atual
- `group_configs` armazena configurações de grupos (`turn_timeout_seconds`, `draw_penalty`, `allow_stacking_plus_two`, `allow_custom_draw_four_stacking`, `allow_seven_swap`, `allow_zero_pass`, `allow_interception`, `ranking_system`).
- Usuários de grupos são salvos em `known_group_users` com chave composta `(chat_id, user_id)` para histórico local de grupo, mas não há tabela para preferências globais de usuário.
- O endpoint de ranking global (`/api/v1/rankings/...`) gera itens via `makeItem`, com referências opacas seladas para avatar e detalhe de grupo.
- O rodapé do Mini App (`BottomNavigation`) possui atualmente 2 abas (`Grupos` e `Players`) com seleção deslizante estilizada (`glass-selection`).
- Testes de integração utilizam PostgreSQL isolado via `-tags integration`.

## Arquivos analisados
- `AGENTS.md`
- `internal/groups/groups.go`
- `internal/storage/postgres/groups.go`
- `internal/storage/postgres/migrations/`
- `internal/ranking/global.go`
- `internal/storage/postgres/global_rankings.go`
- `internal/telegram/commands.go`
- `internal/telegram/render.go`
- `internal/httpapi/server.go`
- `internal/httpapi/auth.go`
- `web/src/components/BottomNavigation.tsx`
- `web/src/components/Ranking.tsx`
- `web/src/pages/Rankings.tsx`
- `web/src/api/client.ts`
- `web/src/styles.css`
- `web/src/App.tsx`

## Arquivos que poderão ser modificados
- `internal/storage/postgres/migrations/0009_ranking_privacy.up.sql` (novo)
- `internal/groups/groups.go`
- `internal/storage/postgres/groups.go`
- `internal/storage/postgres/privacy.go` (novo)
- `internal/storage/postgres/store.go`
- `internal/ranking/global.go`
- `internal/storage/postgres/global_rankings.go`
- `internal/telegram/commands.go`
- `internal/telegram/render.go`
- `internal/httpapi/server.go`
- `internal/httpapi/auth.go`
- `internal/httpapi/server_test.go`
- `internal/storage/postgres/global_rankings_test.go`
- `internal/storage/postgres/privacy_test.go` (novo)
- `internal/telegram/commands_test.go`
- `web/src/api/client.ts`
- `web/src/components/BottomNavigation.tsx`
- `web/src/components/Ranking.tsx`
- `web/src/pages/Profile.tsx` (novo)
- `web/src/App.tsx`
- `web/src/styles.css`
- `web/src/components/BottomNavigation.test.tsx`
- `web/src/pages/Profile.test.tsx` (novo)

## Estratégia de implementação

1. **Migração de Banco de Dados (`0009_ranking_privacy.up.sql`)**:
   - `ALTER TABLE group_configs ADD COLUMN ranking_private boolean NOT NULL DEFAULT false;`
   - `CREATE TABLE IF NOT EXISTS user_privacy_settings (user_id bigint PRIMARY KEY, ranking_private boolean NOT NULL DEFAULT false, updated_at timestamptz NOT NULL DEFAULT now());`
   - O default `false` garante compatibilidade regressiva total para dados pré-existentes.

2. **Repositórios e Serviços (Go)**:
   - Em `internal/groups`:
     - Adicionar `RankingPrivate bool` em `GroupConfig`;
     - Adicionar `SetRankingPrivate(ctx, chatID, private)` e `ToggleRankingPrivate(ctx, chatID) (bool, error)`;
     - Usar a verificação existente `svc.CanConfigureUser(ctx, chatID, actorID)` para garantir política idêntica a `/config`.
   - Em `internal/storage/postgres`:
     - Implementar `GetUserRankingPrivacy`, `SetUserRankingPrivacy` e `ToggleUserRankingPrivacy`;
     - Atualizar leituras e escritas de `group_configs` para incluir `ranking_private`.

3. **Leitura do Ranking Global (`internal/storage/postgres/global_rankings.go`)**:
   - Modificar CTEs da query:
     - Grupos: `LEFT JOIN group_configs c` com projeção `COALESCE(c.ranking_private, false) AS anonymous`;
     - Jogadores: `LEFT JOIN user_privacy_settings u ON u.user_id = s.user_id` com projeção `COALESCE(u.ranking_private, false) AS anonymous`;
     - Detalhe de grupo: `(COALESCE(gc.ranking_private, false) OR COALESCE(u.ranking_private, false)) AS anonymous` para as linhas de jogadores; e `COALESCE(c.ranking_private, false) AS anonymous` para o cabeçalho do grupo;
     - Ordenação de tie-break (`score_units DESC, last_placement ASC, last_completed_game_at DESC, name COLLATE "C" ASC, id ASC`) permanece intacta sobre os nomes/critérios reais.

4. **API HTTP do Mini App (`internal/httpapi`)**:
   - Contexto de autenticação: injetar `user_id` decodificado do `initData` no request context;
   - Novos endpoints:
     - `GET /api/v1/me/privacy`: retorna `{"anonymous": boolean}`;
     - `PUT /api/v1/me/privacy`: payload `{"anonymous": boolean}`, atualiza e retorna estado salvo;
   - Projeção em `makeItem`:
     - Se `row.Anonymous`:
       - `Name`: `"Grupo anônimo"` ou `"Anônimo"`;
       - `MaskedID`: `""` (omitido);
       - `Avatar`: `""` (sem URL gerada);
       - `Anonymous`: `true`;
       - `GroupRef`: preservado para grupos para permitir navegação.
   - Blindagem do endpoint de mídia `/api/v1/media/{ref}`:
     - Se o ref apontar para entidade anônima, rejeita imediatamente (204/404) sem consultar upstream nem servir cache.

5. **Telegram Bot (`internal/telegram`)**:
   - Registrar `/privacidade`:
     - Chat privado: alterna a privacidade do usuário (`user_id = actorID`) com resposta visual amigável (🔒 ativada / 🔓 desativada);
     - Grupo: exige `@bot` quando aplicável, valida se `svc.CanConfigureUser` é permitido. Se sim, alterna a privacidade do grupo (`chat_id`); se não, responde com negação de permissão clara.

6. **Frontend Mini App (`web/`)**:
   - Atualizar `BottomNavigation`: 3 abas (`Grupos`, `Players`, `Perfil`), layout responsivo (mínimo 320px), cálculo do slider (`calc((100% - 8px) / 3)` e posições 0%, 100%, 200%);
   - Implementar tela `ProfilePage` (`web/src/pages/Profile.tsx`):
     - Toggle switch para `Aparecer como Anônimo`;
     - Texto explicativo dos efeitos da anonimização;
     - Feedback de salvamento ("Salvando...", "Salvo com sucesso");
   - Ajustar `RankingCard` e `Avatar`:
     - Quando `anonymous: true`, exibir avatar neutro (ícone SVG minimalista) e suprimir masked ID;
     - Não tentar carregar imagem nem gerar iniciais de nome real.

## Passos detalhados

1. Criar migração SQL `0009_ranking_privacy.up.sql`.
2. Implementar métodos de persistência em `internal/storage/postgres/` e estender `internal/groups/`.
3. Atualizar `ReadGlobalRanking` em `internal/storage/postgres/global_rankings.go` para ler os status de privacidade e preencher `GlobalRow.Anonymous`.
4. Implementar endpoints `/api/v1/me/privacy` e mascaramento seguro de `makeItem` e `/api/v1/media/{ref}` em `internal/httpapi/`.
5. Implementar dispatch do comando `/privacidade` em `internal/telegram/commands.go` para chat privado e de grupo.
6. Atualizar `web/src/api/client.ts` com tipos e chamadas de privacidade.
7. Atualizar `web/src/components/BottomNavigation.tsx` e `web/src/styles.css` para acomodar a terceira aba `Perfil`.
8. Criar componente `web/src/pages/Profile.tsx` e registrar rota `/profile` em `web/src/App.tsx`.
9. Atualizar renderização de `RankingCard` e `Avatar` para entidades anônimas em `web/src/components/Ranking.tsx`.
10. Executar baterias completas de testes unitários e de integração (`go test` e `npm test`).
11. Validar `make check` e ausência de regressões.

## Riscos
- **Risco**: Impacto acidental na ordenação ou empates de ranking ao mascarar nomes.
  - **Mitigação**: O banco de dados ordena pelo nome real no CTE `base` e apenas projeta a flag booleana `anonymous`. A substituição do texto para `"Anônimo"` ou `"Grupo anônimo"` ocorre exclusivamente no nível do DTO HTTP (`makeItem`).
- **Risco**: Quebra do layout do rodapé em telas muito estreitas (320px).
  - **Mitigação**: Ajustar padding, tamanhos de fonte e larguras para garantir que as 3 abas caibam com folga e sem quebra de linha.
- **Risco**: Vazar avatar real via referência direta na API de mídia.
  - **Mitigação**: Checagem de privacidade ativa no handler de mídia do `httpapi`.

## Impactos esperados
- Nenhuma alteração nas regras de cartas, motores de jogo, contagem de pontos ou encerramento de partidas.
- Preservação integral do histórico e dados no banco de dados.
- Total transparência e controle de privacidade para usuários e administradores de grupos.

## Compatibilidade
- Linux (x86_64, aarch64)
- macOS
- Windows
- Docker (imagens existentes continuam compatíveis com migrações automáticas)
- CI/CD (GitHub Actions)

## Como testar

### Build
```bash
go build ./...
cd web && npm run build && cd ..
```

### Testes
```bash
go test -race ./...
go test -race -tags integration ./internal/storage/postgres/... ./internal/app/...
cd web && npm test && cd ..
```

### Execução / Verificação de qualidade
```bash
make check
```

## Rollback
Caso necessário reverter:
- Remover as referências nos arquivos de código Go e TypeScript via Git checkout.
- A migração adiciona uma coluna com default e uma nova tabela isolada; se necessário, executar `ALTER TABLE group_configs DROP COLUMN ranking_private; DROP TABLE user_privacy_settings;`.

## Observações
- Não fazer commit ou push sem aprovação do usuário.
- Manter o working tree limpo e validar com `git diff --check`.

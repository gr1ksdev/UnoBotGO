# UnoBotGO — Estado do projeto

Revisão: **2026-11-16**, branch `v3`.

---

## 1. O que é a v3

Um **bot de Telegram puro**. A v3 remove a Mini App, a API REST e o WebSocket
do produto v2. Não há frontend, não há endpoints, não há segunda interface.

| Antes (v2) | Agora (v3) |
|---|---|
| Mini App React/PixiJS (`web/`, 4.221 linhas TS) | removido |
| API REST + WebSocket (`internal/httpapi`, 1.432 linhas) | removido |
| Cache de avatares (`internal/media`) | removido |
| Import de ranking legado (`internal/rankingimport`) | removido |
| Ranking global, perfil e salas livres | removidos |
| Dois modos (Clássico e Caseiro) | **um modo: Caseiro** |
| `/play`, `/privacidade`, `/dar` | removidos |
| `g_<GameID>_<revision>` visível no campo de texto | **nada visível** |
| Estado detalhado em toda mensagem | só a ação e `Vez de: Fulano` |

Ganhos mensuráveis: **~5.000 linhas Go + 4.221 linhas TS + 2 MB de assets**
removidos, e a imagem Docker deixou de ter runtime Node.

---

## 2. Matriz de maturidade

| Área | Status | Testado | Homologado | Observação |
|---|---|---|---|---|
| Engine (`internal/uno`) | IMPLEMENTED + HOMOLOGATED | regras, invariantes, lifecycle | interno e Telegram real | copy-on-write, revisão estrita |
| Aplicação (`internal/game`) | IMPLEMENTED + HOMOLOGATED | autorização, views, concorrência | interno e Telegram real | isolamento por partida |
| Adapter Telegram | IMPLEMENTED + HOMOLOGATED | handlers, comandos, callbacks | Telegram real | tokens de uso único |
| Modo Caseiro | IMPLEMENTED **BUT NOT HOMOLOGATED** | engine, serviço, inline, renderer | **pendente** | único modo agora |
| Ranking Vitória/Colocação | IMPLEMENTED; N=1 PENDENTE DE HOMOLOGAÇÃO | cálculo, elegibilidade, monthly | Telegram real (N≥2) | política `completed-placements-v1`; N=1 agora pontua |
| `/pular` / `/skip` | IMPLEMENTED **NOT HOMOLOGATED** | engine, serviço, comando | pendente | novo na v3 |
| `/expulsar` / `/kick` | IMPLEMENTED **NOT HOMOLOGATED** | engine, serviço, comando | pendente | novo na v3 |
| `/stats` | IMPLEMENTED **NOT HOMOLOGATED** | read, renderer, comando | pendente | novo na v3 |
| Idioma de comandos por grupo | IMPLEMENTED **NOT HOMOLOGATED** | tabelas disjuntas, cache, menu por chat com `BotCommandScopeChat`, migration 0014 | pendente | padrão PT; admin troca no `/config` |
| Webhook | IMPLEMENTED; PRODUCTION VALIDATION PENDING | HTTP, secret, retry, status, dedupe | Coolify real pendente | recomendado em produção |
| Polling | IMPLEMENTED + HOMOLOGATED | transporte com mocks | Telegram real | `timeout=30` |
| Docker V2 | IMPLEMENTED BUT NOT HOMOLOGATED | build no CI | operação não certificada | multi-arch sem Node |
| GHCR | IMPLEMENTED + HOMOLOGATED | workflow aprovado | publicação confirmada | `latest` e `sha-<commit>` |
| Recuperação / reset | IMPLEMENTED + HOMOLOGATED | autorização, isolamento, filas | Telegram real | fila dedicada |
| Simulador | IMPLEMENTED + HOMOLOGATED | runner, estratégia, relatório | interno | direto sobre a engine |
| Privacidade da mão | IMPLEMENTED + HOMOLOGATED | views, tokens | Telegram real | `PlayerView` só do solicitante |
| Política de menções | IMPLEMENTED | renderer | aceitação visual pendente | BotID por padrão |

---

## 3. Garantias que a v3 preserva

1. **Anti-cheat**: token de 128 bits, consumo atômico, revisão na engine.
2. **Privacidade das mãos**: `PlayerView` devolve só a mão do solicitante.
3. **Política de menções**: BotID por padrão; UserID real só para quem tem a
   decisão em mãos.
4. **Atomicidade**: cópia-on-write + `Validate()` + commit tardio.
5. **Idempotência**: `payload_hash` no Postgres + recibos em RAM.
6. **Migrations**: advisory lock transacional, checksum, fail-closed.
7. **Webhook**: segredo derivado, `drop_pending_updates=false`, 503 no
   backpressure, `/healthz` e `/readyz`.
8. **Concorrência**: ordem de lock `entry.mu → indexMu`; zero I/O sob lock.

---

## 4. Decisões de produto da v3

### Modo único

O Caseiro é o único modo. O grupo não guarda mais o modo padrão e `/novo`
ignora seus argumentos. A engine continua genérica — `ClassicRules()` e
`BotRules()` existem para os testes e para o simulador.

### Idioma dos comandos

O padrão é português. Um administrador troca o grupo para inglês em
`/config`. Não existem aliases entre idiomas: o parser aceita somente a
grafia configurada. O menu e a sugestão do Telegram acompanham — o Grupo EN
recebe uma lista por chat via `SetMyCommands(BotCommandScopeChat)`, que tem
prioridade sobre a lista global; voltar para PT chama `DeleteMyCommands` no
mesmo escopo. O idioma fica em `group_configs.command_language` (migration
0014) e é lido em cache pelo adapter.

### Sem identificador no campo de texto

O botão **🃏 Suas cartas** abre o menu inline com uma query vazia. O bot
resolve a partida pelo **jogo atual do jogador**, rastreado quando ele age em
um grupo ou joga uma carta. É o mesmo modelo do bot upstream, que mantém um
único jogo corrente por jogador.

Consequência: o seletor de partidas e o prefixo `g_` sumiram. O token de
ação continua amarrando cada jogada ao usuário, à partida, ao chat, à ação e
à revisão.

### Mensagens

O fluxo anuncia a ação e o próximo turno. O estado detalhado — topo, cor,
classificação e ordem — fica no `/estado`.

### Privacidade e ranking global

`/privacidade` e a coluna `ranking_private` saíram: só afetavam o ranking
global da Mini App. O ranking do Telegram mostra os nomes normalmente.

---

## 5. Riscos conhecidos

| # | Risco | Mitigação |
|---|---|---|
| 1 | **O modo Caseiro nunca foi homologado no Telegram real** e agora é o único | roteiro de aceite antes de publicar |
| 2 | AGPL-3.0 sem decisão explícita de licença | `LICENSE` adicionado; revisar antes do publish |
| 3 | Estado das partidas em memória | documentado; resultados e ranking no Postgres |
| 4 | Webhook exige instância única | nome consistente no Coolify |
| 5 | Multi-grupo: o menu inline mostra o jogo corrente | `/entrar` e jogar uma carta atualizam o jogo corrente |
| 6 | Carga em muitos grupos nunca foi caracterizada | limitação conhecida |

---

## 6. Base de validação

```
go build ./...
go vet ./...
go test ./...
go test -race ./...
go vet -tags integration ./...
git diff --check
```

A integração com PostgreSQL exige `-tags integration` e `TEST_DATABASE_URL`;
a migration `0013_v3_cleanup` remove `ranking_private`,
`user_privacy_settings` e `default_game_mode`.

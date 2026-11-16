# UnoBotGO

Bot de UNO em Go para o Telegram, em modo inline, com stickers para jogar as
cartas.

---

## O que é a v3

A v3 é um **bot de Telegram puro**. Sem Mini App, sem API REST, sem WebSocket:
tudo acontece dentro do Telegram, com a mão privada no menu inline e o estado
público no chat do grupo.

- **Um único modo de jogo — Caseiro.** A carta 🔀 Trocar cartas faz parte do
  baralho (109 cartas). Não há escolha de modo.
- **Ranking que conta colocação e pontos.** Duas políticas de pontuação
  (`Legado` e `Atualizado`) e um ranking mensal por grupo, com desempate por
  melhor colocação, conclusão mais recente e UserID.
- **Webhook como transporte de produção.** Long polling é o padrão local.
- **Mensagens enxutas.** O fluxo anuncia a jogada e o próximo turno
  (`Vez de: Fulano`). O estado detalhado fica no `/estado`.

O projeto descends do bot [@unobot](https://github.com/jh0ker/mau_mau_bot)
(AGPL-3.0), cujo baralho de stickers é preservado aqui.

---

## Executando

```bash
cp .env.example .env
# Configure TOKEN, DATABASE_URL e MINIAPP_SECRET no .env
go run ./cmd/bot
```

Ou via Docker:

```bash
docker build -f Dockerfile.v2 -t unobotgo:v3 .
docker run --rm -p 8080:8080 --env-file .env unobotgo:v3
```

As migrações do PostgreSQL são verificadas e aplicadas automaticamente na
inicialização, com advisory lock transacional fail-closed.

### Variáveis de ambiente

| Variável | Finalidade / padrão |
|---|---|
| `TOKEN` | Token obrigatório do bot Telegram |
| `DATABASE_URL` | Conexão PostgreSQL obrigatória |
| `TELEGRAM_MODE` | `polling` (padrão) ou `webhook` |
| `TURN_TIMEOUT` | Duração positiva; padrão `2m` |
| `WEB_ADDR` | Endereço HTTP das sondas e do webhook; padrão `:8080` |
| `MINIAPP_SECRET` | Exatamente 32 bytes em Base64; gere com `openssl rand -base64 32` |
| `WEBHOOK_URL` | Somente no modo webhook: URL pública HTTPS com caminho dedicado |

Políticas internas fixas: logs `info`, histórico de 100 partidas encerradas,
tokens inline com TTL `2m`, limite global de 20.000 e por usuário de 512,
timeout das migrations de `2m` e descarte de updates pendentes desativado.

---

## Comandos

| Comando | Descrição |
|---|---|
| `/start` | Apresentação do bot (privado). |
| `/help` (ou `/ajuda`) | Lista de comandos e instruções. |
| `/novo` | Cria uma partida no grupo (você é o responsável). |
| `/entrar` (ou `/join`) | Entra na partida aberta ou em andamento. |
| `/trancar` · `/destrancar` | Controla novas entradas (responsável). |
| `/iniciar` | Inicia a partida com pelo menos dois jogadores. |
| `/estado` | Mostra o lobby ou o estado detalhado da partida. |
| `/ranking` | Ranking mensal do grupo; no privado, seus rankings. |
| `/stats` | Suas estatísticas: partidas, vitórias, pontos, colocação. |
| `/sair` | Sai da partida em andamento. |
| `/pular` | Pula o turno do jogador atual (responsável ou admin). |
| `/expulsar` | Expulsa um jogador; responda à mensagem dele. |
| `/cancelar` (ou `/kill`) | Cancela a partida (responsável, criador ou admin). |
| `/reset` | Recupera o grupo e limpa partida e histórico (responsável ou admin). |

Em grupos e supergrupos, todo comando precisa mencionar o bot:
`/novo@SeuBot`. No privado, o sufixo é opcional.

### Como jogar as cartas

O jogo acontece no **menu inline**: toque em **🃏 Suas cartas** no chat ou
digite `@SeuBot`. Sua mão privada aparece com as cartas jogáveis coloridas e
as indisponíveis em cinza. Toque numa carta colorida para jogá-la — não há
nenhum identificador no campo de texto.

A confirmação oficial e o estado da partida são enviados no grupo.

---

## Ranking e pontuação

| Política | Fórmula | Exemplo com 3 jogadores |
|---|---|---|
| `Legado` | +1 ponto para todo elegível, menos o último | 1º e 2º: `+1 pt` |
| `Atualizado` | `1000 × (N − posição) / (N − 1)`, arredondado | 1º: `+10,00`, 2º: `+5,00`, 3º: `+0,00` |

A pontuação é inteira em centésimos — nunca ponto flutuante.

**Elegibilidade:** só pontuam quem concluiu a partida com colocação válida.
Quem abandona recebe zero pontos e fica fora do ranking, exibido como
`Nome · fora do ranking`. Partidas com menos de dois elegíveis são gravadas
para auditoria, sem alterar os acumulados.

O ranking mensal usa o calendário de `America/Sao_Paulo` e desempata por
melhor colocação na última partida elegível do mês, depois pela conclusão mais
recente e, por estabilidade, pelo UserID.

`/config` (administradores ou quem adicionou o bot) alterna a política do
grupo. A troca é recusada quando o grupo já tem pontuações no sistema
anterior.

---

## Operação

### Webhook

O webhook elimina o ciclo de `getUpdates`, não sofre `429` e permite que o
Telegram reenvie updates quando o endpoint responde `5xx`. Configure
`TELEGRAM_MODE=webhook` e `WEBHOOK_URL=https://bot.exemplo.com/telegram` e
publique esse caminho pelo proxy até o servidor (`WEB_ADDR`, padrão `:8080`).

O segredo é derivado internamente por HMAC-SHA256 do `MINIAPP_SECRET` com o
contexto `unobotgo/v2/telegram-webhook-secret/v1`; o segredo mestre nunca é
enviado ao Telegram. `drop_pending_updates` é sempre `false`.

As sondas `GET /healthz` e `GET /readyz` respondem na mesma porta.

### Instância única

O estado das partidas vive em memória: rode **uma única instância**. No
Coolify, configure o container com nome consistente ou sem rolling update,
para que o container antigo seja parado antes do novo. Em polling, duas
instâncias recebem `409 Conflict`; em webhook, o proxy pode dividir os
updates entre elas.

Qualquer reinício perde as partidas em andamento. Os resultados e o ranking
permanecem no PostgreSQL.

### Migrations

Versionadas e embutidas no binário. O ledger e os checksums SHA-256 são
verificados antes de qualquer SQL; versões desconhecidas ou checksums
divergentes impedem o startup, sem corrigir o ledger nem executar downgrade.
Não há comando manual de migration.

---

## Desenvolvimento

```bash
go build ./...
go vet ./...
go test ./...
go test -race ./...
```

Testes de integração com PostgreSQL real usam uma base e schemas isolados:

```bash
TEST_DATABASE_URL='postgres://postgres:senha@localhost:5432/unobot_test?sslmode=disable' \
  go test -race -tags integration ./internal/storage/postgres/...
```

O simulador executa bots diretamente sobre a engine, sem Telegram nem banco:

```bash
go run ./cmd/simulator --players 4 --mode caseiro --seed 20260924
```

Ele grava um relatório Markdown em `.reports/simulations/` com colocações,
estatísticas e a linha do tempo completa.

---

## Arquitetura

```
cmd/bot → internal/app (migrations + bot)
              │
     internal/telegram (adapter: filas, tokens, inline, renderer, webhook)
              │
        internal/game (autorização, views, locking, lifecycle)
              │
         internal/uno (engine: regras, atomicidade, revisão)
              │
     internal/storage/postgres ← internal/ranking, internal/groups
```

- A **engine** (`internal/uno`) não conhece Telegram, SQL nem rede. Toda
  ação exige a revisão esperada, é atômica e falha sem alterar o estado.
- A **camada de aplicação** (`internal/game`) autoriza os atores, isola as
  partidas por chat e separa a visão pública da privada: `PlayerView` nunca
  devolve a mão de outro jogador, nem ao responsável.
- O **adapter** (`internal/telegram`) serializa as jogadas por chat, protege o
  inline com tokens de uso único de 128 bits e aplica a política de menções
  (só quem tem a decisão em mãos é citado diretamente).

Detalhes em [docs/](docs/).

---

## Documentação técnica

- [Estado do projeto](docs/project-status.md)
- [Regras da engine](docs/v2-rules.md)
- [Camada de aplicação](docs/v2-application.md)
- [Adapter Telegram e roteiro de aceite](docs/v2-telegram.md)
- [Persistência, configuração e ranking](docs/m7-persistence.md)
- [Branches e publicação](docs/branching.md)
- [Build e distribuição](docs/build.md)

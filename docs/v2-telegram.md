# UnoBotGO V2 — Telegram Adapter (Milestone 3)

Este documento descreve a arquitetura, o fluxo de execução, os detalhes de segurança e o roteiro de homologação do adapter Telegram do UnoBotGO V2.

---

## 1. Visão Geral e Arquitetura

O UnoBotGO V2 é executado via `./cmd/bot` e consome diretamente a camada de aplicação desacoplada (`internal/game.Service`).

```text
                               +----------------------------------+
                               | Telegram Bot API (Long Polling)  |
                               +-----------------+----------------+
                                                 |
                                                 v
                                    +------------+-----------+
                                    |       Dispatcher       |
                                    +------------+-----------+
                                                 |
                       +-------------------------+-------------------------+
                       |                                                   |
                       v (8 filas particionadas por ChatID)                v (Fila com 4 workers)
            +----------+----------+                               +--------+---------+
            |    Chat Workers     |                               |  Inline Workers  |
            +----------+----------+                               +--------+---------+
                       |                                                   |
       +---------------+---------------+                                   |
       |               |               |                                   |
       v               v               v                                   v
  [Commands]     [Callbacks]     [ChosenResult]                      [InlineQuery]
       |               |               |                                   |
       +---------------+---------------+                                   |
                       |                                                   |
                       v                                                   v
           +-----------+-----------+                           +-----------+-----------+
           |  internal/game.Service|                           |      TokenStore       |
           +-----------+-----------+                           +-----------------------+
                       |                                                   |
                       v                                                   v
           +-----------+-----------+                           +-----------+-----------+
           |   internal/uno Engine |                           | Tokens de Ação/Cursor |
           +-----------------------+                           +-----------------------+
```

### Princípios de Design
1. **Separação estrita**: A engine (`internal/uno`) e o serviço (`internal/game`) não importam pacotes do Telegram nem dependem de detalhes da API externa.
2. **Autor confiável**: O Telegram adapter obtém o ID do usuário de `update.From.ID` e passa para o serviço como `game.Actor`. O usuário nunca pode falsificar sua identidade.
3. **Mão privada via Inline Mode**: Apenas o jogador autenticado vê sua mão através de inline query privada (`@bot`). Nenhuma carta da mão de outros jogadores é enviada ao grupo ou a outros clientes.
4. **Confirmação oficial**: A seleção via inline publica imediatamente o sticker selecionado e dispara o evento `ChosenInlineResult`. O bot consome o token atomicamente, aplica a ação no `Service` e envia a confirmação oficial no grupo cadastrado.

---

## 2. Transporte e Concorrência

- **Biblioteca**: `telego v1.10.0` com long polling e `net/http`.
- **Startup seguro**:
  - `GetMe`: valida nome de usuário e flag `SupportsInlineQueries`. Se o modo inline estiver desabilitado, o bot falha no startup orientando o uso do `/setinline` no @BotFather.
  - `GetWebhookInfo`: se houver webhook ativo, falha no startup instruindo o usuário a deletar o webhook manualmente para evitar conflito com long polling.
  - `SetMyCommands`: registra apenas os comandos implementados no Telegram.
- **Particionamento por ChatID**:
  - 8 workers com canais de capacidade 32 dedicados às mensagens, comandos e confirmações de ações agrupados pelo `ChatID`.
  - Garante ordem estrita de execução para a mesma partida, eliminando condições de corrida entre comandos e jogadas.
- **Inline Workers**:
  - 4 workers dedicados a responder consultas inline através de uma fila com capacidade 64.
  - Não bloqueiam nem são bloqueados por requisições de rede no chat de grupo.
- **Backpressure**:
  - Filas limitadas rejeitam trabalho de forma segura sob sobrecarga sem corromper o estado do jogo.
- **Shutdown gracioso**:
  - Intercepta `SIGINT`/`SIGTERM`, interrompe o polling, drena os trabalhos admitidos nas filas em até 10 segundos e encerra as conexões.

---

## 3. Token Store e Anti-Cheat

Para prevenir ataques de repetição, falsificação de jogadas e cache indevido do cliente Telegram:

1. **Tokens de 128 bits**:
   - Gerados a partir de `crypto/rand` e codificados em base64 URL-safe (22 caracteres).
   - Cabem com folga no limite de 64 bytes do `id` de resultados inline do Telegram.
2. **Dois tipos de tokens**:
   - `ActionToken`: vinculado a `(UserID, GameID, ChatID, Action, Revision, TTL)`.
   - `CursorToken`: vinculado a paginação de mãos ou lista de partidas `(UserID, Offset, Revision, TTL)`. Nunca pode ser consumido como ação.
3. **Consumo atômico**:
   - `ConsumeAction(token, actorID)` é uma operação sob mutex. Apenas o primeiro concorrente obtém sucesso (`ConsumeOK`).
   - Tentativas subsequentes recebem `ConsumeAlreadyConsumed` e não reexecutam a ação.
4. **Invalidação**:
   - Cancelamento, encerramento ou saída de jogador invalidam imediatamente todos os tokens pendentes daquela partida/usuário.
5. **Limites e Evicção FIFO**:
   - Limite global configurável (`INLINE_TOKEN_LIMIT`, padrão 20.000).
   - Limite por usuário (`INLINE_TOKEN_USER_LIMIT`, padrão 512).
   - Limpeza oportunista na inserção sem necessidade de timers em background.

---

## 4. Bypassing do omitempty da Telego

Na biblioteca `telego v1.10.0`, o tipo `AnswerInlineQueryParams` possui tags `omitempty` nos campos:
- `cache_time`
- `is_personal`
- `next_offset`

Quando `cache_time` é 0, o encoder padrão omite o campo, fazendo o Telegram adotar o cache default de 300 segundos. Para contornar isso de forma nativa e sem HTTP paralelo:
- Foi criado o `InlineRequestConstructor` implementando `telegoapi.RequestConstructor`.
- Intercepta chamadas de `AnswerInlineQueryParams` e serializa uma struct explícita sem `omitempty`.
- Garante o envio estrito de:
  ```json
  {
    "inline_query_id": "...",
    "results": [...],
    "cache_time": 0,
    "is_personal": true,
    "next_offset": ""
  }
  ```

---

## 5. Roteiro para Aceite Manual no Telegram

> [!IMPORTANT]
> **Critério de conclusão**: Este aceite deve ser executado manualmente em ambiente real com Telegram antes de declarar a homologação final.

### Pré-requisitos
1. Obter um bot token com o [@BotFather](https://t.me/BotFather).
2. Habilitar o modo inline: `/setinline` -> escolher o bot -> definir placeholder (ex: `Suas cartas de UNO`).
3. Habilitar o feedback inline em 100%: `/setinlinefeedback` -> escolher o bot -> selecionar `Enabled (100%)`.
4. Criar dois grupos no Telegram (Grupo A e Grupo B) e adicionar o bot como administrador/membro com permissão para enviar mensagens.

### Passos do Teste

1. **Inicialização do Bot**:
   ```bash
   cp .env.example .env
   # Preencher TOKEN=.env com o token real
   go run ./cmd/bot
   ```
   Verificar no log: `connected to telegram bot`, `started long polling updates`.

2. **Lobby e Inscrição (Grupo A)**:
   - Jogador 1 (Owner) digita: `/novo`
     - Verificar: O bot responde com a mensagem de lobby, indicando Jogador 1 como responsável e 0 jogadores inscritos.
   - Jogador 2 digita: `/entrar`
     - Verificar: O bot atualiza o lobby com 1/10 jogadores inscritos.
   - Jogador 3 digita: `/entrar`
     - Verificar: O bot atualiza o lobby com 2/10 jogadores inscritos.
   - Jogador 2 tenta `/iniciar`
     - Verificar: O bot recusa informando que apenas o responsável (Jogador 1) pode iniciar.
   - Jogador 1 digita: `/iniciar`
     - Verificar: O bot anuncia o início da partida, exibe a carta do topo, o jogador da vez e anexa os botões `🃏 Suas cartas` e `🔄 Atualizar estado`.

3. **Fluxo de Jogada via Inline**:
   - O jogador da vez clica no botão `🃏 Suas cartas` (ou digita `@usernamebot` no chat):
     - Verificar: Abre o menu inline com:
       1. Artigo de cabeçalho com informações da mesa.
       2. Botão de comprar (se ainda não comprou) ou passar (se já comprou).
       3. Cartas da mão: cartas jogáveis com stickers normais e coloridos; cartas não jogáveis com stickers cinzas.
   - O jogador clica em uma carta jogável:
     - O sticker é enviado no grupo com o teclado inline `⏳ Aguardando validação`.
     - Quase instantaneamente, o teclado do sticker é atualizado para `✅ Confirmado`.
     - O bot envia a confirmação oficial no grupo: quem jogou a carta, a nova carta do topo e quem é o próximo jogador.

4. **Cartas Especiais e Escolha de Cor**:
   - Jogar um Wild (Coringa) ou +4:
     - O bot entra na fase `ChoosingColor` e anuncia no grupo que o jogador deve escolher a cor.
     - O jogador abre `@usernamebot`: o menu inline exibe os 4 botões de cores (Vermelho, Azul, Verde, Amarelo).
     - O jogador seleciona uma cor.
     - O bot confirma a cor escolhida e passa a vez aplicando as penalidades devidas (+4 faz o próximo comprar 4 e perder a vez).

5. **Teste de Rejeição de Stale (Anti-Cheat)**:
   - Jogador abre o menu `@usernamebot`.
   - Outro jogador realiza uma jogada ou entra na partida, alterando a `Revision`.
   - O jogador tenta clicar no resultado que estava aberto anteriormente:
     - Verificar: O bot recusa a jogada informando "Seleção antiga: a partida mudou. Abra Suas cartas novamente.", sem alterar o estado do jogo.

6. **Multi-grupo**:
   - No Grupo B, criar outra partida com `/novo`.
   - Um participante do Grupo A que também está no Grupo B digita apenas `@usernamebot` (sem `g_<id>`):
     - Verificar: O bot lista os dois grupos disponíveis para o jogador escolher qual mão abrir.

7. **Encerramento da Partida**:
   - Continuar a partida até um jogador bater (zero cartas).
   - Sob `BotRules`, o jogador obtém a 1ª colocação e a partida continua para os demais decidirem as próximas colocações até o encerramento total (`GameFinished`).

## Transporte Webhook

O transporte padrão é o long polling (`TELEGRAM_MODE=polling`). Para receber updates por webhook, configure `TELEGRAM_MODE=webhook`, uma `WEBHOOK_URL` pública HTTPS, `WEBHOOK_SECRET` e, se necessário, `WEBHOOK_LISTEN_ADDR` (padrão `:8080`). A terminação TLS fica no reverse proxy ou plataforma externa; o processo atende HTTP internamente. O segredo é validado no header `X-Telegram-Bot-Api-Secret-Token` e nunca é registrado.

No modo webhook o bot aplica `setWebhook` em todo startup, inclusive quando a URL não mudou, para garantir que alterações do segredo sejam efetivadas. `WEBHOOK_DROP_PENDING_UPDATES` é `false` por padrão. Ao voltar para polling, um webhook existente é removido com `drop_pending_updates=false`, preservando updates pendentes. O shutdown normal não remove o webhook remoto.

`GET /healthz` retorna apenas `200 OK` para liveness. O endpoint de webhook aceita somente `POST` JSON no caminho configurado, com corpo limitado a 1 MiB. Updates repetidos são ignorados por uma deduplicação em memória; após reinício essa proteção é perdida.

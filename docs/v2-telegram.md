# UnoBotGO V2 — Telegram Adapter (Milestone 3)

## Trocar cartas — exclusivo do Caseiro (2026-09-25)

O sticker `CAACAgEAAxkBAAER8VtqteJsR8-zG10NFeLTIZyxuZYsBQACBwkAAkkSsEU562tb90Ja3D0E`
representa a carta **🔀 Trocar cartas**. Depois de jogá-la, abrir **Suas cartas**
mostra um menu como o do coringa, com nomes e contagens de cartas dos demais
participantes ativos. Selecionar um nome confirma a troca integral das mãos
restantes, mantém a cor da mesa e passa a vez. O menu não pede cor.

Somente quem jogou a carta recebe tokens de escolha; os demais veem a mensagem de
espera e o resumo privado da própria mão. Tokens são pessoais, opacos, de uso
único e vinculados à revisão. Um alvo que sair deixa de ser elegível, e menus
anteriores precisam ser reabertos. O responsável pela escolha recebe a menção
real; os demais nomes seguem a política de links ao bot.

O sticker cinza
`CAACAgEAAxkBAAER8aRqtlf6ZtRKfAj02K5AnlVcRz_W_AACVAcAAkaGsEXgXGCANqlQKz0E`
representa a carta indisponível. Ela segue o mesmo fluxo visual das demais
cartas cinza, com resultado `grey_` e sem token de jogada. UNO é comunicado para
ambos os envolvidos quando aplicável.

Homologação manual pendente em cliente Telegram: selecionar Caseiro no lobby,
confirmar a renderização dos stickers colorido e cinza, jogar a carta, abrir o
menu, escolher outro participante e verificar novas mãos, cor, contagens e
próximo turno. O sticker cinza foi aceito pela Bot API como arquivo estático de
342×512 pixels; testes locais validam o payload e a ausência de ação no resultado
indisponível.


> Atualização de 2026-09-23: as regras abaixo descrevem a milestone corretiva.
> O roteiro histórico da M3 mais adiante contém comportamentos já substituídos.

## Recuperação isolada por grupo

O comando `/reset` existe para recuperar um grupo quando uma chamada anterior
ficou lenta, bloqueada ou deixou estado inconsistente. Ele percorre uma fila de
recuperação própria, com dois workers e capacidade limitada, portanto não depende
de espaço na fila normal particionada por `ChatID`.

Antes de alterar o estado, o adapter autentica o remetente. O responsável pela
partida ativa pode executar o comando diretamente. Outros usuários precisam ser
confirmados pela API do Telegram como criador ou administrador do grupo. Mensagens
privadas, tópicos de fórum e remetentes anônimos são recusados explicitamente.

Após a autorização, o dispatcher cancela o contexto da geração anterior daquele
chat e incrementa sua geração. Tarefas antigas que ainda estejam nas filas são
descartadas antes da execução. Uma fila dedicada e limpa passa a atender o grupo,
permitindo `/novo` imediatamente mesmo se o shard antigo continuar ocupado. O
serviço remove a partida ativa, os índices de participantes e todo histórico do
chat; o TokenStore invalida os tokens de cada partida removida. Outros chats não
são interrompidos.

Cada worker possui uma barreira de recuperação de panic com log do tipo do worker,
chat e stack trace. Payloads, mãos, tokens e segredos não são incluídos nesse log.
Cancelamento cooperativo não encerra à força código externo que ignore `context`,
mas isola esse trabalho da nova geração e impede que ele volte a alterar o estado
removido pelo serviço.

## Encerramento, contexto inline e menções — milestone corretiva

### Encerramento

A engine já produzia `GameFinished` sem `TurnChanged` ao restar um jogador. O
problema relatado como "partida não disponível ou você não participa dela" vinha
do botão `Suas cartas` anexado à confirmação final. Agora os teclados recebem a
view pública e não oferecem ações em `Closed`/`Finished`. Refresh de uma mensagem
encerrada envia teclado vazio explicitamente, removendo botões anteriores.

Botões em mensagens históricas não são editados em massa. Ao abrir um deles,
`ErrGameClosed` produz "Partida encerrada", sem mão ou convite de continuação;
após descarte do resumo, permanece a resposta genérica de indisponibilidade.
Erros de ação consultam a view atual e só oferecem `Suas cartas` se o usuário
continua participante de uma partida aberta. Encerramento natural, cancelamento e
saída terminal invalidam os tokens existentes daquele jogo; ações já consumidas
ou consultas concorrentes continuam sujeitas à validação definitiva do serviço.

O scheduler descobre candidatos sem mutação e executa AutoSkip + mensagem dentro
da fila do chat. Isso impede a publicação tardia de um turno obsoleto após vitória.
O mesmo código é usado por polling e webhook.

### Por que o contexto permanece visível

`Suas cartas` e o seletor de partidas usam `switch_inline_query_current_chat` com
`g_<GameID>` (34 caracteres: prefixo + 128 bits em hexadecimal). Esse identificador
seleciona a partida; não é o token de ação de uso único. A mão exige participação
validada por `PlayerView`. Tokens de ação permanecem vinculados a usuário, jogo,
chat, ação, carta/cor, revision e TTL; `ChosenInlineResult` usa o registro do token,
não a query recebida, para aplicar a ação e escolher o grupo de confirmação.

O botão insere a query no campo de texto. Não existe payload oculto equivalente
nesse mecanismo. `InlineQuery` e `ChosenInlineResult` não fornecem `chat_id`, e
`inline_message_id` não permite recuperar o destino por uma API documentada.
Fontes: [botões](https://core.telegram.org/bots/api#inlinekeyboardbutton),
[InlineQuery](https://core.telegram.org/bots/api#inlinequery),
[ChosenInlineResult](https://core.telegram.org/bots/api#choseninlineresult).

Query vazia continua abrindo a mão quando há uma única partida ou o seletor quando
há várias. Ela não identifica o grupo de origem. Manter o contexto evita seleção
ambígua para quem joga em vários grupos. Uma representação base64url dos mesmos
128 bits reduziria o texto, mas continuaria visível e exigiria compatibilidade
com botões anteriores; não foi introduzida nesta milestone. Não há "última
partida" global por usuário, mudança de GameID, token ou regra de autorização.

### Destino dos nomes

Após `GetMe`, o BotID é disponibilizado ao renderer antes da entrada de updates.
`PlayerLink` separa nome exibido de destino e usa a view resultante da ação:

| Estado | Destino |
|---|---|
| Lobby, encerrado, cancelado ou sem contexto | BotID para todos |
| TakingTurn | UserID real só para CurrentTurn; demais apontam ao BotID |
| ChoosingColor | UserID real só para ColorChooserID; demais apontam ao BotID |
| ChoosingPlayer | UserID real só para PlayerChooserID; demais apontam ao BotID |

A regra inclui responsável, colocações, entrada/saída, confirmações, UNO, erros e
timeout. Nomes e usernames exibidos são preservados com escape HTML. Renderer
isolado sem BotID válido produz texto escapado, sem fabricar link com ID zero.
Mensagens históricas não têm seus targets reescritos a cada mudança de turno.

A API suporta `tg://user?id=...` em links HTML e seu objeto `User` inclui bots.
O uso do BotID é fundamentado nesse contrato, com o bot presente no grupo. A
apresentação e abertura do perfil precisam de homologação nos clientes; não há
garantia de comportamento visual idêntico em todos eles. Fontes:
[formatação](https://core.telegram.org/bots/api#formatting-options),
[User](https://core.telegram.org/bots/api#user).

### Homologação manual pendente

Executar com um bot de teste, repetindo em polling e webhook:

1. Dois jogadores: finalizar com carta numérica, Reverse, Skip, +2, Wild e +4,
   nos modos Clássico e Caseiro. Conferir efeitos homologados, escolha de cor,
   colocações e ausência de botão/turno posterior. Com stacking terminal não
   exigir compra extra nem permitir rebater.
2. Com TURN_TIMEOUT habilitado, concluir próximo ao vencimento e aguardar:
   nenhuma mensagem de timeout ou "Vez de" pode surgir após a confirmação final.
3. Conferir links em lobby, troca de turno, escolha de cor, UNO e encerramento.
   Testar abertura dos targets em Android, iOS e Desktop; só o responsável atual
   deve apontar ao jogador real nas novas mensagens.
4. Participar de dois grupos, usar query vazia e os botões contextuais; confirmar
   mão e destino corretos. Abrir um botão antigo de jogo encerrado.

Testes automatizados validam payloads, estado, concorrência e ambos os ingressos
com API mockada; não substituem esta homologação visual real.


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
  - `SetMyCommands`: registra menus por escopo. O privado recebe `/start` e
    `/help`; grupos recebem comandos de partida e `/help`; o escopo padrão mantém
    `/help` como fallback.
- **Apresentação privada**:
  - `/start` envia boas-vindas, uma descrição curta e um botão para adicionar o
    bot a grupos usando `https://t.me/<username>?startgroup=true`.
  - O username vem de `GetMe`; não há nome de bot fixo no texto nem no link.
  - O payload de grupo `/start@bot true` confirma a adição e orienta `/novo` e
    `/help`, sem tentar iniciar uma partida inexistente. `/start` sem payload
    continua como alias compatível de `/iniciar` nos grupos.
  - `/help` lista comandos em blockquote, mantém `/ajuda` como alias e registra a
    origem brasileira baseada no `@unopybot`.
- **Particionamento por ChatID**:
  - 8 workers com canais de capacidade 32 dedicados às mensagens, comandos e confirmações de ações agrupados pelo `ChatID`.
  - Garante ordem estrita de execução para a mesma partida, eliminando condições de corrida entre comandos e jogadas.
- **Recuperação por ChatID**:
  - `/reset` entra por uma fila independente com 2 workers e capacidade 16.
  - Cada reset troca a geração e o contexto do chat; trabalhos antigos são descartados e comandos novos seguem por uma fila limpa dedicada àquele grupo.
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

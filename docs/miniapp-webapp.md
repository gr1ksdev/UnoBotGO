# Mini App mobile e partidas WebApp

Implementação em `dev`, sem publicação ou homologação no Telegram real. A interface de produção consulta a API autenticada; fixtures existem exclusivamente nos testes e no script de verificação visual.

## Telas e fluxo

- `/` e `/ranking`: ranking mensal de Jogadores/Grupos, pódio 2/1/3, paginação, posição pessoal e estados de carregamento/vazio/erro. Mês/ano e política são explícitos. Legacy e Updated continuam separados; Inline/WebApp não são categorias.
- `/home`: estatísticas reais, posição mensal Atualizado, prévia do ranking e acesso às salas em que a identidade autenticada já participa. O atalho usa o username obtido por `getMe` do bot.
- `/profile`: identidade do initData validado, avatar pela API existente, totais por política, contagens combinadas de partidas elegíveis/vitórias, privacidade e histórico com modalidade/grupo. Histórico usa cursor keyset selado, vinculado ao usuário.
- `/game/:gameID`: lobby, mesa e resultado confirmado. Dois jogadores mantêm oponente no topo e usuário na base. Três distribuem dois oponentes no topo; quatro a seis usam topo/laterais; sete a dez usam badges compactos no topo. Mão mantém cartas de 80 × 112 e scroll horizontal, sem zoom progressivo.

Para começar: `/novo` e `/entrar` no grupo pelo bot; abrir a Mini App e escolher a sala em Jogar. O responsável pode iniciar pelo WebApp. O fluxo reutiliza criação/admissão do bot, em vez de aceitar um chat ID informado pelo navegador. Não foi criada pontuação para salas privadas: a persistência atual exige grupo configurado e validado pelo serviço.

## Contrato de transporte

Consultas HTTP usam `Authorization: tma <initData>`:

| Rota | Função |
| --- | --- |
| `GET /api/v1/config` | Username real do bot; indisponível enquanto inicializa |
| `GET /api/v1/rooms` | Salas de participação do usuário autenticado |
| `GET /api/v1/rooms/{id}` | Snapshot personalizado para recuperação |
| `GET /api/v1/me?cursor=...` | Perfil e histórico paginado |
| `GET /api/v1/me/position?system=updated\|legacy` | Posição pessoal no mês corrente |
| Rotas existentes de rankings/media/privacy | Mantidas |

A partida usa **WebSocket**, sem polling periódico: `GET /api/v1/live`, mesma origem. O primeiro frame, limitado a cinco segundos, contém:

```json
{"init_data":"<initData original>","game_id":"<identidade canônica da sala>"}
```

O servidor valida HMAC/idade/identidade e participação na sala antes de enviar dados. Credenciais não vão na URL. Há limite de três conexões por usuário, limite de tamanho de mensagem, heartbeat e expiração da sessão conforme a validade original do initData. Shutdown cancela as conexões.

Exemplo de comando:

```json
{"game_id":"<sala>","request_id":"<UUID da ação>","expected_revision":20,"action":"play","card_id":"<ID físico da carta>"}
```

Ações: `start`, `play`, `draw`, `pass`, `color`, `target`, `keep`, `bluff`, `leave`, `cancel`. Identidade, chat e privilégios são definidos no servidor; campos extras de ator/grupo são rejeitados. Destinatários de Troca de Mãos usam chaves opacas de assento, convertidas pelo servidor dentro da sala autorizada.

Frames `snapshot`, `accepted` e `rejected` carregam uma projeção integral autoritativa. Apenas a mão do solicitante é enviada; demais jogadores têm nome/quantidade/estado público. IDs Telegram não são expostos nas projeções. Mudanças feitas pelo Inline e pelo timer também invalidam as projeções via o mesmo serviço de partidas.

Snapshots com revision antiga são ignorados. Saltos de revision são seguros porque o payload já é um snapshot completo, não um delta. Receipts são comparados sob o lock da partida, por jogador e request ID; retry reutiliza o comando original, inclusive revision. A reconexão recupera snapshot por HTTP/WebSocket e reenvia somente a ação ainda pendente. Duplicatas não reaplicam engine, resultado ou animações.

O timer vem de `TurnStarted + TurnTimeout`; o relógio do navegador usa o horário do servidor para apresentação. A engine e o scheduler existentes continuam decidindo compra, jogada, efeitos, turno e encerramento. Se a leitura do score final falhar após um commit, o heartbeat recupera somente esse status terminal e o envia pelo socket; turnos ativos continuam exclusivamente orientados a mudanças confirmadas.

## Finalização e ranking

`game.Finalizer` é a operação compartilhada pelos adapters Telegram e WebApp: `ranking.Prepare` → `RecordCompletedGame` → confirmação de commit → `AcknowledgeResult` → notificação de commit novo. A transação/idempotência por GameID/hash existente permanece a fonte de verdade para pontos e estatísticas mensais/acumuladas.

O processo tenta novamente resultados pendentes com backoff de 1 a 30 segundos. Erros e commits indeterminados mantêm o resultado imutável pendente; retries confirmados como já persistidos não repetem score, histórico ou notificações. O resultado da UI mostra espera enquanto o banco não confirma, incluindo ao encerrar com dois jogadores. Ganhos de cada colocado vêm exclusivamente de dados persistidos.

A migration aditiva `0011_game_origin.up.sql` identifica modalidade, preservando histórico anterior como Inline. A modalidade descreve o transporte que **iniciou** a partida; alternar de cliente durante o jogo não cria outra partida. Campo vazio de origem permanece omitido no hash de resultados antigos, preservando compatibilidade. Não há contadores ou políticas exclusivos do WebApp.

## Verificação

Executados: `make check` com PostgreSQL 17 isolado, `go test -race ./...`, integração PostgreSQL com `-race`, e checks direcionados após ajustes finais. O frontend possui 60 testes passando. Cobertura adicional inclui WebSocket real com atualização do Inline/reconexão, initData alterado/expirado, ator/grupo forjados, revisions antigas, receipts duplicados, encerramento normal de dois jogadores, recuperação automática após falha de persistência, paridade Legacy/Updated, origem, histórico keyset e privacidade. A engine é exercitada também com dez jogadores.

Verificação em Chromium via Playwright:

```bash
npm --prefix web ci
npm --prefix web exec -- playwright install chromium
npm --prefix web run dev -- --port 5173
# Em outro terminal:
npm --prefix web run test:visual
```

É possível usar `PLAYWRIGHT_EXECUTABLE_PATH` para um Chromium já instalado. Artefatos locais ficam em `.reports/redesign-mobile/`: seis telas nos viewports 320×568, 360×800, 390×844 e 430×932; 120 combinações de 1/2/7/15/30 cartas com 2/3/4/5/6/10 jogadores; estados de ranking e casos com áreas seguras. Capturas usam DPR 2, igual às previews do pacote. Fixtures do navegador são explicitamente dados de teste, nunca fontes da aplicação de produção.

A comparação visual corrigiu links azuis/sublinhados, deslocamento do pódio causado pelo seletor de política, cores dos avatares, sobreposição de badges laterais com as pilhas e espaço dos controles quando há áreas seguras. Os números e textos ilustrativos do pacote não foram preservados como resultados reais.

## Limitações operacionais

- Partidas ativas, receipts e resultados ainda pendentes usam a memória do processo, como a arquitetura existente. Retry recupera falhas enquanto o processo permanece ativo; reinício não é recuperação durável dessas partidas/resultados. Banco confirmado continua persistente.
- Não foram executadas partidas com clientes Telegram reais nem publicação/deploy. O WebSocket foi verificado localmente com servidor Go real; as capturas usam API/transporte controlados de teste.
- Criação/entrada inicial permanece no grupo pelo bot. Não há uma política nova de score para partidas privadas.
- Notificações preservam a política existente: envio após commit novo, sem outbox ou reenvio de mensagens cuja entrega falhou/ficou indeterminada. Falha Telegram não estorna pontos.

Referências oficiais consultadas: [Telegram Mini Apps](https://core.telegram.org/bots/webapps), [coder/websocket](https://pkg.go.dev/github.com/coder/websocket) e [Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch). A identidade visual vem de `docs/design-reference/unobotgo-v1/`, preservado sem alterações.

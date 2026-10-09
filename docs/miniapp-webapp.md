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
- Criação/entrada pela Mini App exige grupo real verificado; não há uma política nova de score para partidas privadas.
- Notificações preservam a política existente: envio após commit novo, sem outbox ou reenvio de mensagens cuja entrega falhou/ficou indeterminada. Falha Telegram não estorna pontos.

Referências oficiais consultadas: [Telegram Mini Apps](https://core.telegram.org/bots/webapps), [coder/websocket](https://pkg.go.dev/github.com/coder/websocket) e [Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch). A identidade visual vem de `docs/design-reference/unobotgo-v1/`, preservado sem alterações.

## Continuação: acesso à partida e evidência ponta a ponta

O problema da entrega anterior estava no acesso: a Home oferecia o bot como ação principal quando a lista estava vazia, escondia a lista para uma única sala e não atualizava ao retomar a Mini App. O bot não fornecia link da sala e a aplicação não interpretava o parâmetro de lançamento. As rotas e a engine já estavam conectadas, mas isso não comprovava o percurso do usuário até a partida.

Agora:

- A ação principal de uma sala disponível é **Abrir partida**, com navegação interna para `/game/:gameID`. A seção **Suas salas** aparece inclusive com uma única sala; cada item oferece **Abrir na Mini App** e informa se aguarda início ou está em andamento.
- A lista atualiza ao retomar o Telegram (`activated`), ao recuperar foco/visibilidade e pelo botão **Atualizar salas**, sem polling de turnos. Sem salas, a ação principal continua na Mini App e explica a criação/admissão; somente **Jogar pelo Telegram** abre o bot.
- Respostas de `/novo` e `/entrar` oferecem **Abrir partida na Mini App**, usando o username/short name já configurados e `startapp=game_<gameID>`. O lançamento funciona mesmo se a URL da Mini App estiver configurada para `/ranking`. O parâmetro só seleciona uma rota; HMAC e participação continuam obrigatórios no backend.
- Pelo bot, `/novo` mantém o responsável observador e exige `/entrar`. Pela Mini App, **Criar sala** executa criação e admissão da identidade autenticada, sem exigir comandos.

### Evidência reproduzível com dois navegadores

O teste opt-in `TestTwoBrowserWebAppGame` inicia a API Go real, engine e finalizador com um schema PostgreSQL isolado. Dois contextos Chromium recebem identidades fictícias assinadas com um token fictício. Não há mock de API/WebSocket ou bypass de autenticação em produção. A sala é criada e o convidado é admitido pelos botões da aplicação e endpoints reais; somente a consulta de membros Telegram usa uma fixture. Testes do handler Telegram verificam os comandos e links separadamente.

O teste percorre: Home vazia → criação/admissão → retomada e descoberta da sala → abrir lobby → iniciar pelo botão → mãos privadas diferentes → reconexão/deep link da segunda conta → comprar/selecionar/confirmar/jogar e alternar turnos → encerramento **normal**, sem desistência → resultado persistido → ranking mensal de jogadores/grupo → perfil/histórico. O embaralhamento determinístico exclusivo do build de teste garante um coringa: a escolha de cor passa pela UI. Repetir a ação final mantém a revision; duas novas chamadas ao finalizador canônico mantêm pontos e histórico e não repetem o callback de notificação.

As evidências desta continuação ficam em `.reports/miniapp-immersive/`; `.reports/webapp-flow/` preserva a entrega anterior. `flow.png` apresenta o percurso; `browser.json` registra comandos e etapas; `backend.json` confirma uma partida, dois jogadores, um grupo, um histórico por conta e uma chamada de notificação após commit. Essa contagem verifica o callback, não entrega de mensagem no Telegram real. Capturas de salas nas quatro dimensões verificam acesso e ausência de overflow horizontal.

```bash
# PostgreSQL de teste: testes criam e removem exclusivamente um schema isolado.
export TEST_DATABASE_URL='postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable'
npm --prefix web ci
npm --prefix web exec -- playwright install chromium
UNO_BROWSER_E2E=1 go test -race -count=1 -tags integration \
  -run TestTwoBrowserWebAppGame -v ./internal/httpapi
```

Se Chromium já estiver instalado, usar `PLAYWRIGHT_EXECUTABLE_PATH`. O teste inicia/encerra seu próprio Vite em `127.0.0.1:5184`; essa porta deve estar livre. `VITE_API_PROXY_TARGET` seleciona o servidor Go de teste, mantendo `127.0.0.1:8080` como padrão de desenvolvimento. Não use credenciais Telegram reais nesse teste. Os 64 testes frontend e o `make check` completo continuam sendo executados separadamente.

### Teste manual exato com duas contas Telegram

1. No `.env` local, mantenha `TOKEN` do bot de teste com Inline habilitado, `MINIAPP_SECRET` válido, `TELEGRAM_MODE=polling` e `DATABASE_URL` do PostgreSQL local. Use uma única instância do backend para as duas contas. Não reinicie durante a partida, pois as salas vivem em memória.
2. Execute `docker compose up -d db` e `make dev`. O frontend fica em `http://localhost:5173`, com HTTP/WebSocket encaminhados ao backend `:8080`. O backend deve responder `200` em `/healthz` e `/readyz`.
3. Para abrir em **contas Telegram reais**, a Mini App do bot precisa estar registrada com short name **ranking**, e sua URL HTTPS de desenvolvimento precisa apontar a esse frontend e ser acessível pelos dois clientes. O proxy deve permitir upgrade WebSocket em `/api/v1/live`. Uma URL de produção ainda servindo o bundle antigo não testa estas alterações locais. Configurar essa URL/conectividade é preparação do ambiente, não foi publicada automaticamente nesta tarefa. Abrir `localhost` no navegador comum não gera initData Telegram.
4. Adicione o bot ao grupo que contém as duas contas. Na Mini App da conta A, toque **Criar sala**, selecione o grupo verificado e o modo, depois **Criar e entrar**. Se o grupo público ainda não aparecer, informe seu @username e toque **Verificar grupo**. Grupos privados dependem do cadastro de instalação/participantes conhecido pelo bot; o servidor revalida a associação atual.
5. No lobby da conta A, toque **Compartilhar convite** e envie o link à conta B. B abre o link, confere o lobby e toca **Entrar na sala**. A sala também aparece na Home para retomada; a ação principal abre a partida internamente.
6. Um jogador inscrito toca em **Começar partida** na WebApp, conforme a permissão já existente do serviço. As duas contas devem ver suas próprias sete cartas e apenas a quantidade/verso das cartas alheias.
7. Na conta indicada pelo turno, toque na pilha **Comprar carta** ou selecione uma carta habilitada e confirme em **Jogar carta**. Após compra, use **Passar turno** quando disponível. Coringa abre escolha de cor; no Caseiro, Troca de Mãos abre escolha de destinatário. A outra conta deve acompanhar as mudanças pelo socket.
8. Feche/reabra a tela de uma conta e confirme recuperação da mesma sala/mão. Continue jogando até alguém esvaziar a mão. Ambas devem receber resultado e perder controles de turno; pontos só aparecem quando confirmados pelo banco.
9. Toque em **Ver ranking**, usando a política da sala e o mês mostrado; confira Jogadores e Grupos. Em **Perfil**, cada conta deve ter uma única entrada **Mini App** com o grupo e seu ganho. Reabrir o resultado ou reconectar não pode aumentar os pontos nem criar outra entrada.

Ainda não foi executada homologação manual com duas contas Telegram reais. A evidência entregue é de dois navegadores locais com transporte, engine e persistência reais. Não houve commit, push, deploy, alteração no BotFather ou publicação de URL nesta continuação.

## Correções: salas diretas, mão e movimento confirmado

A API adiciona criação, catálogo/validação de grupos e convites selados. O cliente nunca escolhe um chat numérico ou uma identidade para pontuar: a configuração vem do grupo validado pelo servidor. Convites expiram em sete dias; admissão respeita revisão, lock e limite de dez participantes. Retry de criação/entrada usa IDs estáveis. Ranking permanece mensal, combinado Inline/WebApp, com Legacy/Updated e privacidade existentes.

A mão atual usa fileiras de até oito cartas, centralizadas com margens de 16 px. O tamanho se adapta à altura disponível; mãos grandes rolam verticalmente apenas dentro da mão. A seleção acompanha CardID. PixiJS renderiza as cartas originais e GSAP anima eventos públicos confirmados; snapshots iniciais/recuperados e duplicatas não repetem movimentos. Versos representam mãos alheias. Há distribuição alternada, compra com virada, reflow, seleção, descarte, seletor em losango e resultado confirmado. Componentes limpam timelines/sprites e respeitam movimento reduzido.

Evidências: `browser.json`, `backend.json`, screenshots das salas e seletor nas quatro dimensões, `layouts/checks.json` com 120 combinações e casos de safe area, e vídeos `video/account-1.webm`/`account-2.webm`. O teste usa engine, socket e PostgreSQL reais, mas não duas contas Telegram reais. A homologação Telegram exige URL HTTPS de desenvolvimento registrada e acesso de ambas as contas; nada foi publicado nesta tarefa.


### Partida v2 e revanche (2026-10-08)

A apresentação da partida usa PNGs originais, verso e efeitos de `docs/design-reference/unobotgo-v2/`. Nunito variável (base 550) e Lexend Deca Black (900 no resultado) são locais. O tipo 15 usa o PNG original já existente do bot, porque o pacote v2 não contém carta de troca; não há novo desenho nem uso de `card_overlay` como carta.

O descarte coringa mostra `*_wild` / `*_wild_draw4` conforme `active_color` confirmado, mantendo a cor neutra e o CardID da engine. A organização no cliente agrupa vermelho, amarelo, verde, azul e coringas, com rank e CardID como desempate; o array da engine não é alterado. Cada carta tem área própria de toque; o canvas não intercepta o toque das vizinhas. A disposição permanece congelada durante um gesto, enquanto turno e jogabilidade autorizados mudam imediatamente. Animações GSAP não enviam comandos e são interrompidas ao recuperar, redimensionar ou desmontar.

`rematch` e `rematch_leave` usam o WebSocket autenticado existente, `game_id`, `request_id` e `expected_revision` da partida final. A projeção `rematch` expõe `revision` (versão dos votos), `required`, `accepted` (chaves opacas), `ready` e `next_game_id`. A versão da engine não é incrementada por votos; o cliente ignora versões de voto atrasadas. Receipts vinculados a participante/request impedem que uma aceitação retransmitida volte a valer após retirada.

Todos os participantes registrados ao final são exigidos, inclusive quem saiu. Desconexão não remove participante nem voto; uma aceitação continua registrada na reconexão. “Retirar minha aceitação” revoga apenas o voto próprio. Quem saiu pode retornar à tela do resultado e aceitar. Se alguém não retornar, a revanche não começa; os demais podem voltar ao início e criar outra sala pelo fluxo normal. Cancelamento não oferece revanche. Se outra partida ocupa o grupo, a revanche é rejeitada e a sala ativa é preservada.

A finalização anterior precisa sair de `pendingResults` por ACK do finalizador após COMMIT antes de qualquer voto. Sob o lock da partida anterior, o serviço prepara uma engine privada com novo ID, regras/política/lock e todos os participantes originais, distribui sete cartas e inicia. A publicação dos índices de grupo/jogadores e da ponte `next_game_id` é atômica, após revalidar conflito e pending. Votos no ID anterior ficam sem efeito depois da publicação. A mesa anterior continua encerrada e sua pontuação/histórico não muda. Nenhuma migração, nova fonte de pontuação ou mudança de mês/política foi introduzida.

As salas e votos continuam em memória, como o runtime existente; reconexão recupera no mesmo processo, reinício do servidor não restaura esses votos. O resultado transacional permanece no PostgreSQL. Evidências e limites em `.reports/partida-v2/README.md`; nenhuma confirmação com contas reais do Telegram nesta correção.

### Cena PixiJS e movimentos (2026-10-09)

`TableScene` usa PixiJS v8 e GSAP/PixiPlugin. Sprites usam PNGs originais, proporção 256:344 e transparência. Camadas separam oponentes, mesa, mão mascarada e voos. DOM conserva comandos, acessibilidade, avatares e placar. O loader usa o caminho CSP sem geração dinâmica de funções e decodifica texturas sem blob workers; o cabeçalho de segurança permanece intacto.

Seleção dura 380 ms, reorganização 580 ms, voo de compra 780 ms com virada de 500 ms iniciada aos 200 ms; compras múltiplas têm intervalos de 130 ms. Distribuição alterna jogadores e mostra frentes somente da própria mão. Jogadas aceitas viajam ao descarte. Compras em novas fileiras rolam suavemente a mão até o destino. A troca consensual para nova partida distribui uma vez; reabrir/reconectar apenas recompõe o estado. Nenhum movimento envia comandos ou governa regras.

O seletor envia a intenção no toque, mantém a transição de cor por 450 ms, segura 700 ms e sai em 350 ms. Confirmação rápida não encurta o feedback; rejeição restaura as cores e libera nova tentativa. Recursos e fontes são locais. O pacote não oferece arte de troca de mãos; continua sendo usado o PNG original do projeto para rank 15.

Checks e evidências desta revisão ficam em `.reports/partida-pixi/README.md`. O teste opt-in pode usar o bundle embutido no Go e CSP de produção com `UNO_E2E_PRODUCTION=1`; requer build atualizado, PostgreSQL de testes e Chromium. SDK/identidades Telegram são fixtures locais; homologação com contas/aparelhos reais continua pendente. Não houve commit, push ou deploy.

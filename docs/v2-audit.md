# Auditoria V1 e direção V2

Data: 2026-09-14. Base: checkout local anterior à Milestone 1.

## Estrutura e funcionamento

Todo o V1 está em `package main`. `main.go` carrega .env, exige TOKEN e PostgreSQL,
registra comandos e recebe updates via long polling com telego v1.10.0. O módulo
atual é `github.com/malbs/UnoGoBot`, Go 1.26.3; não foi renomeado nesta milestone.

`game.go`, `player.go` e `deck.go` usam referências compartilhadas: jogadores em
anel Next/Prev, mãos/pilhas de ponteiros e cartas sem identidade física. O manager
indexa jogos por chat e jogadores por usuário, com ponteiro de jogo atual.
`actions.go` mistura mutação, Telegram, resultado da rodada e ranking.

Modos: Classic; Fast/Sanic com temporizador; Wild com outro deck; Caseiro com
empilhamento cruzado; Test com mão fabricada; Text anunciado, mas apresentação
continua em stickers. Partidas comuns continuam após cada jogador zerar. Match
MD1/MD3/MD5 já é um conceito separado, mas suas transições também enviam mensagens
Telegram e escrevem no ranking.

PostgreSQL guarda wins, challenge_wins, challenge_headtohead e group_settings.
Rankings diário, semanal e mensal usam janelas diferentes: diário desde meia-noite
em America/Sao_Paulo, semanal últimos sete dias, mensal um mês móvel. Há consulta
privada da posição do usuário entre grupos. O estado das partidas continua em RAM.

Docker já é multi-stage, mas usa tags flutuantes e usuário root. Compose publica
PostgreSQL com credenciais de desenvolvimento; seu healthcheck pg_isready é real.
Não havia README. Memória/contexto antigos descreviam apenas RAM e um anti-cheat
que não está mais no código; não são fontes confiáveis para o comportamento atual.

## Inline atual

1. O usuário digita @bot; query.From.ID seleciona UserIDCurrent.
2. Esse ponteiro é alterado por mensagens em grupos e transições de turno.
3. O handler gera stickers, versões cinza, comprar/passar/blefe e seletor de cor.
4. Cartas são ordenadas por cor/rank, sem priorizar todas as jogáveis primeiro.
5. IDs como r_7:3 identificam aparência e posição na resposta.
6. ChosenInlineResult consulta novamente UserIDCurrent e ignora o sufixo.
7. A ação é executada sem verificar revision, turno ou validade da jogada.

Query textual e offset não são interpretados. O V1 envia cache_time zero
explicitamente por HTTP próprio, com is_personal=true. Telego v1.10.0 usa
omitempty em CacheTime; definir zero na struct padrão não envia esse campo.

## KEEP / REWORK / REMOVE

**KEEP:** mão privada inline, representação gráfica, comandos em português,
Suas cartas, UNO automático, dinâmica de colocação e entrada tardia, conceitos de
ranking por grupo, desafios e modos úteis.

**REWORK:** engine e actions, revision, IDs físicos, snapshots, autorização,
multigrupo, concorrência, temporizadores, ranking idempotente, Match, configuração,
logs e encerramento.

**REMOVE da V2:** singletons; locks públicos nos handlers; IDs por índice/aparência;
mutação durante leitura; SQL/Telegram nas regras; modo Test em produção; flags
textuais de modo espalhadas. Nenhuma remoção de código V1 nesta entrega.

## Riscos constatados no código

- Player.Play chama Game.PlayCard mesmo quando não encontra a carta na mão.
- handleChosenInlineResult não revalida turno/posse/fase/cor nem vincula partida.
- firstCard atribui LastCard e chama PlayCard com ela; a mesma referência entra
  no descarte reciclável, embora ainda seja topo, permitindo duplicação posterior.
- PlayableCards altera Bluffing; consultas influenciam contestação. doCallBluff
  usa Prev mesmo quando a direção está invertida.
- O temporizador libera lock antes de doSkip; handlers leem e escrevem estado
  compartilhado sem protocolo único. Getters retornam slices/ponteiros mutáveis.
- NewGame pode acrescentar lobby quando já existe jogo iniciado naquele grupo.
- Configuração/cancelamento de desafios não possuem autorização consistente nem
  identificação de sessão nos callbacks. CancelMatch remove apenas a referência
  do match, podendo deixar a rodada corrente e seus índices ativos.
- isAdmin sempre retorna false; callbacks assumem query.Message presente e
  mensagens assumem From presente, embora esses campos possam faltar.
- SQL agrupa vitórias por ID e nome, fragmentando usuários que mudam nome.
- endMatchGame passa Wins1/Wins2 como placar vencedor/perdedor mesmo quando o
  vencedor é o segundo jogador, invertendo estatísticas nesse cenário.
- Ranking usa gravações separadas, sem transação/idempotência; limpeza ao remover
  o bot apaga histórico. Erros de persistência frequentemente ficam só no log.
- /notificar grava interesse, mas não há fluxo que efetivamente consuma os lembretes.
- Sem graceful shutdown. Erros HTTP podem incluir a URL com token nos logs.
- HTML recebe nomes sem escape em alguns caminhos; displayLink é mais seguro,
  mas sua utilização não é uniforme.

Os testes V1 passam, inclusive com race detector e vet. Eles cobrem principalmente
construção, textos, ordenação e classificação de callbacks. Não exercitam as
condições de concorrência e autorização acima; aprovação do detector não prova
segurança dos caminhos não executados.

## API Telegram verificada

Fontes: [Bot API](https://core.telegram.org/bots/api#inlinequery) e
[Inline Bots](https://core.telegram.org/bots/inline#collecting-feedback).
InlineQuery e ChosenInlineResult não fornecem chat_id. inline_message_id é opcional,
depende de teclado inline e não deve ser decodificado por formato não documentado.
A mensagem já é enviada quando a seleção chega. Rejeição protege estado, não
impede publicação anterior. Até 50 resultados por resposta; paginação será necessária.

V2: uma partida ativa por chat, várias por usuário, seleção explícita quando
ambígua, contexto no botão Suas cartas e token opaco por resultado ligado no
servidor a usuário/partida/ação/carta/revision. Conferir cache_time=0 explícito e
is_personal=true. Feedback deve estar habilitado em 100%. A ação pertence à partida
selecionada; não há comprovação do chat de destino pelo feedback. Confirmação oficial
vai ao grupo cadastrado. Testar UX/cache/stickers em bot de teste na M3.

## Arquitetura e limites desta entrega

internal/uno é puro e não importa Telegram. Futuramente internal/game orquestrará
instâncias e views, internal/storage implementará repositório, internal/telegram
adaptará updates e apresentação, internal/config validará ambiente, cmd/bot fará
composição e shutdown. Criar esses packages apenas quando implementados.

A Milestone 1 entrega regras Classic com as políticas explícitas documentadas em
[v2-rules.md](v2-rules.md). V1 continua sendo o executável atual. Não houve alteração
de token, PostgreSQL, dados, Docker ou handlers. Migração funcional e retirada do V1
exigem milestones posteriores e validação do fluxo inline.

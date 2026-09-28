# UnoBotGO V2 — Estado do projeto

Última revisão da troca opcional: **2026-09-28** (base `dev@bce47d0`, implementação `9afc944`). Esta versão acompanha a **dev**.
Base da auditoria geral anterior (colunas Main e diferenças abaixo são históricas): `dev@badf81c` antes de Gameplay UX Polish e `main@fd011ab`.
A revisão geral anterior incluiu Gameplay UX Polish e M7 na dev. A atualização atual
é restrita à troca opcional, sem promoção à main.
A M7 foi auditada sobre `e72cd66`; implementação e limitações em [M7](m7-persistence.md).

Este documento descreve **maturidade, validação e publicação**, não arquitetura.
Para funcionamento interno, consulte a [documentação técnica](../README.md#documentação-técnica).
Seções antigas dos documentos técnicos registram contratos de milestones;
as regras atuais devem ser conferidas no código da branch indicada.

## Como ler os status

- **IMPLEMENTED + HOMOLOGATED**: Implementado com testes automatizados e homologado em uso real no Telegram.
- **IMPLEMENTED BUT NOT HOMOLOGATED**: Implementado com testes automatizados completos, mas com homologação operacional real pendente.
- **DEFERRED**: Fundação técnica ou proposta adiada formalmente para uma milestone futura (ex: importação de ranking legado).
- **OUT OF SCOPE**: Fora do escopo do projeto (ex: XP, moedas, shop, badges, Mini App).

## Matriz principal

A coluna Implementado considera a `dev`; Main indica a presença na base pública auditada.

| Área | Status | Testado automaticamente | Homologado | Main | Situação |
|---|---|---|---|---|---|
| Engine V2 | IMPLEMENTED + HOMOLOGATED | Regras, invariantes, lifecycle | Interno e Telegram real | Sim, base | Operacional; diferenças de Caseiro abaixo |
| Application Layer | IMPLEMENTED + HOMOLOGATED | Autorização, views, concorrência | Interno e Telegram real | Sim, base | Estado em memória, isolamento por partida |
| Telegram adapter | IMPLEMENTED + HOMOLOGATED | Handlers, comandos, callbacks | Telegram real | Sim, base | Produto utilizável em grupos |
| Inline Mode | IMPLEMENTED + HOMOLOGATED | Contexto, tokens, revisão, cache | Telegram real | Sim | Menu privado com cartas e stickers |
| Clássico | IMPLEMENTED + HOMOLOGATED | Regras e partidas determinísticas | Telegram real | Sim | Usa BotRules, não ClassicRules estrito |
| Caseiro | IMPLEMENTED + HOMOLOGATED | Stacking, blefe e troca de mãos | Telegram real | Parcial | Regras diferentes por branch |
| TURN_TIMEOUT | IMPLEMENTED + HOMOLOGATED | Revalidação e corrida com término | Telegram real | Sim | Turno pulado automaticamente por timeout |
| Polling | IMPLEMENTED + HOMOLOGATED | Transporte/pipeline com mocks | Telegram real | Sim | Padrão e recomendado |
| Webhook | IMPLEMENTED BUT NOT HOMOLOGATED | HTTP, secret, dedupe, lifecycle | Não aprovado no teste real | Sim | Experimental; não recomendado atualmente |
| Docker V2 | IMPLEMENTED BUT NOT HOMOLOGATED | Build no CI | Build confirmado; operação não certificada | Sim | linux/amd64 e linux/arm64 |
| GHCR | IMPLEMENTED + HOMOLOGATED | Workflow de publicação aprovado | Publicação pelo CI confirmada | Sim | latest e sha-commit |
| CI | IMPLEMENTED + HOMOLOGATED | Execuções aprovadas | N/A | Sim | dev valida com Postgres; main valida/publica |
| Simulador | IMPLEMENTED + HOMOLOGATED | Runner, estratégia e relatório | Interno: sim; Telegram: N/A | Sim, base | Local, diretamente sobre a engine |
| Recovery / reset | IMPLEMENTED + HOMOLOGATED | Autorização, isolamento, filas/panic | Telegram real | Sim | /reset por grupo na fila de recuperação |
| GameFinished / lifecycle | IMPLEMENTED + HOMOLOGATED | Dois jogadores, botões e timeout | Telegram real | Sim | Encerramento limpo sem novo turno |
| Menções / links | IMPLEMENTED + HOMOLOGATED | Destinos e estados do renderer | Telegram real | Sim, base | UserID apenas do responsável atual |
| Troca opcional + cor | IMPLEMENTED BUT NOT HOMOLOGATED | Engine, serviço, inline, renderer e simulador; race local bloqueado por VMA | Pendente | Não (fluxo novo) | Última carta encerra sem escolhas |
| Comandos privados | IMPLEMENTED + HOMOLOGATED | Boas-vindas, ajuda, escopos | Telegram real | Não | /start, /help e botão adicionar ao grupo |
| Correção de falso tópico | IMPLEMENTED + HOMOLOGATED | Threads comuns e tópicos reais | Telegram real | Não | Apenas IsTopicMessage identifica tópico |
| Gameplay UX Polish | IMPLEMENTED + HOMOLOGATED | Ordem, lock, renderer e regressões | Telegram real | Não | Ordem a partir do atual e /trancar /destrancar |
| Blefe em +4 sobre +2 | IMPLEMENTED + HOMOLOGATED | Counter legal não desafiável | Telegram real | Não | Caseiro: +4 sobre +2 não é blefe |
| Reentrada e colocação | IMPLEMENTED + HOMOLOGATED | Late join após saída vs finalizados | Telegram real | Não | Reentrada de quem saiu; colocado bloqueado |
| M7 Persistência e Snapshots | IMPLEMENTED + HOMOLOGATED | PostgreSQL real, race, migrations, snapshots | Telegram real | Não | Defaults Classic+Legacy; snapshot imutável por jogo |
| M7 Ranking Legacy e Updated | IMPLEMENTED + HOMOLOGATED | Cálculo determinístico, half-up, elegibilidade | Telegram real | Não | Concessão e anúncio pós-commit homologados N=2 e N=3 |
| M7 UX de Configuração (/config) | IMPLEMENTED + HOMOLOGATED | /config, botões inline, my_chat_member, 23 cenários | Telegram real | Não | Admin/installer, boas-vindas e bloqueio de conflito |
| M7 Import de ranking antigo | DEFERRED | Parser, reconciliação e staging | N/A | Não | Adiado para milestone futura; sem comando ou aplicação |
| /dar | IMPLEMENTED BUT NOT HOMOLOGATED | Testes debugcards e exclusão normal | Uso de desenvolvimento | Não | Fora do produto/build padrão (com tag) |


## Transportes e evidência real

**Polling** permanece o default de `internal/config/config.go` e a recomendação atual.
Há relato de uso em partidas reais, em escala pequena. Isso não homologa toda combinação
de regra, cliente, concorrência ou indisponibilidade da API.

**Webhook** está implementado nas duas branches e tem testes automatizados.
O relato operacional fornecido para esta revisão registra ausência de resposta,
atrasos grandes e comportamento não confiável no teste real. A revisão anterior
não identificou causa óbvia; isso **não atribui a falha ao código nem declara o
transporte quebrado**. Não há evidência posterior de aceite que substitua esse relato.
Permanece **experimental, não homologado e não recomendado**; use polling.
Esta milestone não investigou nem alterou o transporte.

## Modos e cartas

O modo exibido como **Clássico** usa `BotRules()`: colocações, entrada tardia,
primeira carta numérica, empilhamentos de +2 e de +4, blefe e restrições de coringas.
Não deve ser confundido com `ClassicRules()`, a configuração estrita da engine.
Skip/bloqueio, Reverse, coringa, +2 e +4 estão presentes em ambos os modos.

**Caseiro** possui regras próprias de resposta às penalidades:
`+2 → +4` acumula **6**, e `+4 → +2` exige a cor escolhida.
A correção que preserva o total acumulado já está nas duas branches.
Caseiro recusa `+4 → +4` na dev e na main atual (`62fc344`).
O Clássico mantém `+4 → +4` em ambas. Esta publicação documental não muda essas regras.

O baralho Clássico tem 108 cartas. Caseiro tem 109 em ambas as branches atuais,
com uma única **Trocar cartas**, sem reduzir as demais especiais.
Poucas aparições em algumas partidas não demonstram distribuição incorreta.

**Trocar cartas — fluxo opcional na dev:** descarta a carta, oferece outro jogador
ativo ou **Manter minha mão**, e exige cor nos dois caminhos. Somente a cor aplica
a troca/manutenção e avança turno. Última carta termina imediatamente, sem troca
ou escolhas, preservando o fluxo normal de colocações/ranking. Cada ação aceita
avança uma revisão; escolhas pendentes continuam fora de TURN_TIMEOUT.

Implementado e testado automaticamente; **homologação Telegram real pendente**.
O aceite da carta anterior não homologa este fluxo novo. Esta correção não foi
publicada na main (`62fc344`, que já contém a versão anterior da carta); a matriz
histórica de publicação acima não representa uma nova auditoria completa de branches.
Nenhuma promoção ou push faz parte desta entrega.

## Lifecycle, contexto e decisões mantidas

- **GameFinished:** ao restar um jogador, não há novo turno nem botão para continuar.
  Encerramento, mensagens antigas e timeout concorrente têm regressões automatizadas
  em `internal/uno/lifecycle_test.go` e `internal/telegram/lifecycle_test.go`.
  A correção está publicada; o roteiro de aceite real continua em
  [Telegram](v2-telegram.md).
- **Menções:** em turno/escolha, só o responsável atual usa seu UserID real;
  demais links usam BotID. Lobby, encerrado e estados sem responsável usam BotID
  para todos. A escolha de jogador integra essa regra apenas na dev.
  Testes validam HTML/destinos; abertura visual em Android/iOS/Desktop segue pendente.
- **Contexto inline:** `g_<GameID>_<revision>` seleciona a partida correta.
  Não é o token de ação. InlineQuery/ChosenInlineResult não trazem chat_id suficiente
  para resolver todos os casos; contexto e tokens pessoais preservam multigrupo,
  autorização e roteamento. O texto visível é uma decisão conhecida, não um bug a remover.
- **Reset:** responsável/admin autenticado pode limpar o estado daquele grupo.
  Gerações e fila de recuperação isolam trabalho antigo. Código externo que ignora
  cancelamento não é encerrado à força; isso não representa recuperação universal.
- **Persistência:** partidas, tokens e histórico limitado vivem em memória.
  Snapshots da engine não equivalem a banco durável nem recuperação automática após reinício.

## Simulação, debug e limites

`cmd/simulator` já está na main: 2–10 jogadores, seleção de modo, seed reproduzível,
estatísticas, diagnósticos, explicações de especiais, histórico completo e duração.
Executa a engine local, sem application/infraestrutura Telegram de ponta a ponta.
A dev acrescenta estratégia e relatório para Trocar cartas. Relatórios gerados não
fazem parte dos arquivos versionados/publicados.

`/dar` é ferramenta de desenvolvimento da dev, protegida pela build tag `debugcards`
e autorização específica. Não faz parte da ajuda ou do produto normal; o build padrão,
Docker e releases atuais não incluem sua implementação. Não deve entrar no release público padrão.

Carga em muitos grupos reais simultâneos **não foi caracterizada formalmente**.
Testes de concorrência e simulações locais não comprovam capacidade para centenas
de grupos ou milhares de jogadores. Isso é uma limitação conhecida, não um bloqueio desta entrega.

## Milestones e maturidade

| Marco registrado | Estado atual |
|---|---|
| M1 — Engine | Implementada, validada internamente e publicada; regras evoluíram desde o contrato inicial |
| M2 — Application | Implementada, validada internamente e publicada; sem persistência durável |
| M3 — Telegram MVP | Publicado e usado em partidas; aceite completo de UX não documentado |
| M5 — Build/container | CI e publicação GHCR entregues; extensão AMD64/ARM64 publicada |
| M6 — Webhook transport | Implementado e publicado; homologação real não aprovada, experimental |
| Milestone corretiva V2 | Lifecycle/timeout/menções publicados e testados; aceite visual pendente |
| Simulador e recuperação | Implementados, testados e publicados; não são homologação de carga Telegram |
| Gameplay UX Polish | Ordem lógica validada/exibida, room lock e mensagens compactas; testes concluídos, aceite real pendente; somente dev |
| Evoluções Caseiro/UX/debug | Implementadas e testadas na dev; promoção pública pendente |

A numeração acima usa os marcos efetivamente registrados; não reaproveita propostas
antigas de roadmap como se fossem entregas. Não há marco global M4 concluído identificado nesta auditoria.

## Desenvolvimento à frente da main

Diferenças de produto confirmadas pelas árvores Git, sem promoção nesta milestone:

- Gameplay UX Polish: ordem exibida a partir do atual no sentido vigente,
  `/trancar` e `/destrancar` exclusivos do owner e renderer compacto. A inserção
  de late join já respeitava a cauda lógica na engine auditada; novos testes
  garantem essa regra. Lock pertence à sessão e não altera turno/revisão da engine.
  Implementado e testado na dev; homologação Telegram real pendente; main: não.

- Troca opcional + cor via Inline Mode e exceção de última carta (novo fluxo somente dev).
- Recusa de `+4` sobre `+4` no Caseiro.
- Remoção da linha textual redundante de direção do estado público.
- Boas-vindas privadas, `/help` por contexto, escopos de comandos e botão de grupo
  com username obtido automaticamente do bot.
- Correção que evita classificar threads comuns como tópicos de fórum.
- Blefe em +4 como counter de +2 no Caseiro: o +4 jogado sob `StackWildDrawFourOnTwo`
  não é sujeito ao desafio de blefe nem acusa infração por cor anterior. Opção/sticker
  de blefe omitida e chamadas forçadas rejeitadas com segurança.
- Reentrada e colocações: jogador que usou `/sair` sem colocação pode reentrar via
  `/entrar` com sala aberta (recebendo nova mão e cauda lógica); se trancada, recebe
  aviso de sala trancada. Jogadores já colocados (`WentOut` / presente em `Placements`)
  são definitivamente bloqueados (`ErrAlreadyFinished`). Precedência no Join: colocado ->
  ativo -> trancado. Unicidade de colocações garantida.
- Ferramenta de desenvolvimento `/dar`, somente em build explícito com tag.

Não foi encontrada funcionalidade de jogo exclusiva da main. Seu check `public-tree`
é específico da publicação; artefatos de desenvolvimento não contam como features.
As branches têm históricos independentes; a [promoção é seletiva](branching.md), sem merge.

## Design e temas adiados

**Campeonato — Design:** ID de campeonato, múltiplas rodadas, pontuação por colocação,
cartas/jogadas, bônus contextuais, stacks/combos, score ledger, extrato por jogador,
consulta de rodada e comando conceitual `/split`. Não implementado na V2;
valores, fórmulas e regras ainda não definidos.

**Ranking global/persistente — Futuro, não implementado na V2.** Não se confunde
com colocações de uma partida ou campeonato. Persistência, temporadas, rating,
farming, volume de partidas e balanceamento entre modos permanecem em discussão.

## Base de validação e atualização

A revisão confrontou código, testes, documentação, registros de aceite e ambas as
árvores remotas. Testes Go, vet, build e `git diff --check` passaram nas duas branches;
o check público passou na main. Race foi tentado em ambas: CGO estava desativado;
com `CGO_ENABLED=1`, ThreadSanitizer recusou o VMA local (39 bits, requer 48).
Não houve resultado local válido de race; essa verificação depende do CI Linux suportado.
Os testes com `debugcards` foram verificados separadamente na dev.
CI da base main: [build/publicação](https://github.com/gr1ksdev/UnoBotGO/actions/runs/36080638236)
e [árvore pública](https://github.com/gr1ksdev/UnoBotGO/actions/runs/36080638192), ambos aprovados.

Ao promover uma feature ou registrar novo aceite real, atualizar a matriz, a diferença
entre branches e a referência auditada. Commit ou teste verde isolado não comprova homologação Telegram.

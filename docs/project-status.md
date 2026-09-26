# UnoBotGO V2 — Estado do projeto

Última revisão: **2026-09-26**. Esta versão acompanha a **dev**.
Base desta revisão: `dev@badf81c` antes de Gameplay UX Polish e `main@fd011ab`.
Esta revisão inclui Gameplay UX Polish na dev; nenhum código foi promovido à main.

Este documento descreve **maturidade, validação e publicação**, não arquitetura.
Para funcionamento interno, consulte a [documentação técnica](../README.md#documentação-técnica).
Seções antigas dos documentos técnicos registram contratos de milestones;
as regras atuais devem ser conferidas no código da branch indicada.

## Como ler os status

- **Implementado**: existe código executável; não implica aceite operacional.
- **Testado**: há testes automatizados relevantes; mocks não equivalem ao Telegram real.
- **Homologado**: aceite no escopo indicado. “Pendente” significa ausência de evidência suficiente.
- **Publicado / Main**: presente no código público; não comprova qual versão está em execução no servidor.
- **Experimental**: implementação disponível, sem recomendação de uso operacional.
- **Design / futuro**: sem implementação V2. “N/A” indica que teste Telegram não se aplica.

A homologação interna pode se apoiar em testes; UX e transportes exigem evidência real.
“Uso relatado” registra partidas pequenas, sem certificar todos os casos ou clientes.

## Matriz principal

A coluna Implementado considera a dev; Main indica a presença na base pública auditada.

| Área | Implementado | Testado automaticamente | Homologado | Main | Situação |
|---|---|---|---|---|---|
| Engine V2 | Sim | Regras, invariantes, lifecycle | Interno: sim | Sim, base | Operacional; diferenças de Caseiro abaixo |
| Application Layer | Sim | Autorização, views, concorrência | Interno: sim | Sim, base | Estado em memória, isolamento por partida |
| Telegram adapter | Sim | Handlers/API simulada | Uso relatado; aceite completo pendente | Sim, base | Produto utilizável em grupos |
| Inline Mode | Sim | Contexto, tokens, revisão, cache | Uso relatado; matriz de clientes pendente | Sim | Contexto multigrupo mantido |
| Clássico | Sim | Regras e partidas determinísticas | Uso relatado; regressões específicas pendentes | Sim | Usa BotRules, não ClassicRules estrito |
| Caseiro | Sim | Stacking, blefe e cartas especiais | Uso relatado; novidades pendentes | Parcial | Regras diferentes por branch |
| TURN_TIMEOUT | Sim | Revalidação e corrida com término | Telegram: pendente | Sim | Não deve publicar turno após encerramento |
| Polling | Sim | Transporte/pipeline com mocks | Uso real relatado | Sim | Padrão e recomendado |
| Webhook | Sim | HTTP, secret, dedupe, lifecycle | Não aprovado no teste real | Sim | Experimental; não recomendado atualmente |
| Docker V2 | Sim | Build no CI | Build confirmado; operação não certificada | Sim | linux/amd64 e linux/arm64 |
| GHCR | Sim | Workflow de publicação aprovado | Publicação pelo CI confirmada | Sim | latest e sha-commit; não implica deploy |
| CI | Sim | Execuções aprovadas | N/A | Sim | dev valida; main valida/publica e verifica árvore |
| Simulador | Sim | Runner, estratégia e relatório | Interno: sim; Telegram: N/A | Sim, base | Local, diretamente sobre a engine |
| Recovery / reset | Sim | Autorização, isolamento, filas/panic | Recuperação real: pendente | Sim | /reset por grupo; não garante cura de toda falha |
| GameFinished / lifecycle | Sim | Dois jogadores, botões e timeout | Regressão Telegram: pendente | Sim | Correção concluída no código e nos testes |
| Menções / links | Sim | Destinos e estados do renderer | Visual nos clientes: pendente | Sim, base | UserID apenas do responsável atual |
| Trocar cartas | Sim | Engine, serviço, inline e renderer | Parcial: sticker cinza confirmado | Não | Exclusiva do Caseiro na dev |
| Comandos privados reorganizados | Sim | Boas-vindas, ajuda, escopos | Telegram: pendente | Não | /start, /help e botão adicionar ao grupo |
| Correção de falso tópico | Sim | Threads comuns e tópicos reais | Grupo afetado: pendente | Não | Apenas IsTopicMessage identifica tópico |
| Gameplay UX Polish | Sim, dev | Ordem, lock, renderer e regressões | Telegram: pendente | Não | Pronta para homologação manual |
| Blefe em +4 sobre +2 | Sim, dev | Counter legal não desafiável | Telegram: pendente | Não | Caseiro: +4 sobre +2 não é blefe |
| Reentrada e colocação | Sim, dev | Late join após saída vs finalizados | Telegram: pendente | Não | Reentrada de quem saiu; colocado bloqueado |
| /dar | Sim, com tag | Testes debugcards e exclusão normal | Não certificada; uso de desenvolvimento | Não | Fora do produto/build padrão |

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
Na **dev**, Caseiro recusa `+4 → +4`; na **main auditada**, ainda permite essa resposta.
O Clássico mantém `+4 → +4` em ambas. Esta publicação documental não muda essas regras.

O baralho Clássico tem 108 cartas. Caseiro tem 108 na main e 109 na dev,
onde foi acrescentada uma única **Trocar cartas**, sem reduzir as demais especiais.
Poucas aparições em algumas partidas não demonstram distribuição incorreta.

**Trocar cartas — apenas dev:** descarta a carta, abre `ChoosingPlayer` e permite
escolher outro jogador ativo para trocar integralmente as mãos restantes.
Preserva cor, ordem e direção; o renderer/inline apresenta seleção de jogador,
contagens e stickers colorido/cinza. Há validação de autorização e revisão.
Não pode ser última carta, responder a penalidade ou ser jogada sobre coringa.
A renderização do sticker cinza foi confirmada em Telegram real; esse aceite
é específico e não substitui a homologação completa da troca e de seus casos extremos.

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

- Trocar cartas, seleção de jogador, stickers e suporte no simulador.
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

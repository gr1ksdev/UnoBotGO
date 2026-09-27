# UnoBotGO V2 — Estado do projeto

Última revisão: **2026-09-27**. Esta versão acompanha a **main pública**.
Código promovido seletivamente de `dev@e72cd66`, sobre `main@fd011ab`.
Publicado significa disponível nesta árvore; homologação real é uma dimensão separada.

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

A coluna Implementado considera o código promovido; Main indica presença nesta publicação.

| Área | Implementado | Testado automaticamente | Homologado | Main | Situação |
|---|---|---|---|---|---|
| Engine V2 | Sim | Regras, invariantes, lifecycle | Interno: sim | Sim | Operacional; diferenças de Caseiro abaixo |
| Application Layer | Sim | Autorização, views, concorrência | Interno: sim | Sim | Estado em memória, isolamento por partida |
| Telegram adapter | Sim | Handlers/API simulada | Uso relatado; aceite completo pendente | Sim | Produto utilizável em grupos |
| Inline Mode | Sim | Contexto, tokens, revisão, cache | Uso relatado; matriz de clientes pendente | Sim | Contexto multigrupo mantido |
| Clássico | Sim | Regras e partidas determinísticas | Uso relatado; regressões específicas pendentes | Sim | Usa BotRules, não ClassicRules estrito |
| Caseiro | Sim | Stacking, blefe e cartas especiais | Uso relatado; novidades pendentes | Sim | Regras próprias do Caseiro |
| TURN_TIMEOUT | Sim | Revalidação e corrida com término | Telegram: pendente | Sim | Não deve publicar turno após encerramento |
| Polling | Sim | Transporte/pipeline com mocks | Uso real relatado | Sim | Padrão e recomendado |
| Webhook | Sim | HTTP, secret, dedupe, lifecycle | Não aprovado no teste real | Sim | Experimental; não recomendado atualmente |
| Docker V2 | Sim | Build no CI | Build confirmado; operação não certificada | Sim | linux/amd64 e linux/arm64 |
| GHCR | Sim | Workflow de publicação aprovado | Publicação pelo CI confirmada | Sim | latest e sha-commit; não implica deploy |
| CI | Sim | Execuções aprovadas | N/A | Sim | dev valida; main valida/publica e verifica árvore |
| Simulador | Sim | Runner, estratégia e relatório | Interno: sim; Telegram: N/A | Sim | Local, diretamente sobre a engine |
| Recovery / reset | Sim | Autorização, isolamento, filas/panic | Recuperação real: pendente | Sim | /reset por grupo; não garante cura de toda falha |
| GameFinished / lifecycle | Sim | Dois jogadores, botões e timeout | Regressão Telegram: pendente | Sim | Correção concluída no código e nos testes |
| Menções / links | Sim | Destinos e estados do renderer | Visual nos clientes: pendente | Sim | UserID apenas do responsável atual |
| Trocar cartas | Sim | Engine, serviço, inline e renderer | Parcial: sticker cinza confirmado | Sim | Exclusiva do Caseiro |
| Comandos privados reorganizados | Sim | Boas-vindas, ajuda, escopos | Telegram: pendente | Sim | /start, /help e botão adicionar ao grupo |
| Correção de falso tópico | Sim | Threads comuns e tópicos reais | Grupo afetado: pendente | Sim | Apenas IsTopicMessage identifica tópico |
| Gameplay UX Polish | Sim | Ordem, lock, renderer e regressões | Telegram: pendente | Sim | Pronta para homologação manual |
| Blefe em +4 sobre +2 | Sim | Counter legal não desafiável | Telegram: pendente | Sim | Caseiro: +4 sobre +2 não é blefe |
| Reentrada e colocação | Sim | Late join após saída vs finalizados | Telegram: pendente | Sim | Reentrada de quem saiu; colocado bloqueado |
| /dar (somente dev) | Sim, com tag na dev | Testes debugcards e exclusão normal | Não certificada; uso de desenvolvimento | Não | Fora do produto/build padrão |

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
Caseiro recusa `+4 → +4` nas duas branches; Clássico mantém essa resposta.
No Caseiro, +4 sobre +2 é um counter legal sem desafio de blefe; o total é preservado.

O baralho Clássico tem 108 cartas e o Caseiro tem 109 em ambas as branches,
com uma única **Trocar cartas**, sem reduzir as demais especiais.
Poucas aparições em algumas partidas não demonstram distribuição incorreta.

**Trocar cartas — publicada:** descarta a carta, abre `ChoosingPlayer` e permite
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
  para todos. A escolha de jogador também integra essa regra na main.
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
Estratégia e relatório para Trocar cartas também estão publicados. Relatórios gerados não
fazem parte dos arquivos versionados/publicados.

`/dar` é ferramenta de desenvolvimento da dev, protegida pela build tag `debugcards`
e autorização específica. Não faz parte da ajuda ou do produto normal; o build padrão,
Docker e releases atuais não incluem sua implementação. Seus arquivos e hook
não foram promovidos para a main, nem mesmo como código opcional.

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
| Gameplay UX Polish | Ordem lógica validada/exibida, room lock e mensagens compactas; testes concluídos, aceite real pendente; publicado na main |
| Evoluções Caseiro/UX/debug | Caseiro/UX publicados; debug permanece apenas na dev |

A numeração acima usa os marcos efetivamente registrados; não reaproveita propostas
antigas de roadmap como se fossem entregas. Não há marco global M4 concluído identificado nesta auditoria.

## Dev e main após esta publicação

As funcionalidades públicas de `dev@e72cd66` foram promovidas seletivamente:
Trocar cartas e simulador correspondente, regras Caseiro, UX/room lock, comandos
privados, correção de falso tópico, blefe de +4 sobre +2 e reentrada sem colocação.
Quem já terminou permanece bloqueado; o lock recusa novas entradas/reentradas.

Não há feature pública dessa base pendente de promoção. `/dar` permanece somente
na dev como ferramenta de desenvolvimento; foi excluído da árvore pública.
Os históricos continuam independentes, conforme a [política de branches](branching.md).
A publicação não substitui os aceites Telegram pendentes listados na matriz.

## Design e temas adiados

**Campeonato — Design:** ID de campeonato, múltiplas rodadas, pontuação por colocação,
cartas/jogadas, bônus contextuais, stacks/combos, score ledger, extrato por jogador,
consulta de rodada e comando conceitual `/split`. Não implementado na V2;
valores, fórmulas e regras ainda não definidos.

**Ranking global/persistente — Futuro, não implementado na V2.** Não se confunde
com colocações de uma partida ou campeonato. Persistência, temporadas, rating,
farming, volume de partidas e balanceamento entre modos permanecem em discussão.

## Base de validação e atualização

A promoção usa o código/testes da dev, com exclusão da ferramenta de debug.
A árvore pública é validada com testes Go, vet, build, `git diff --check` e o
workflow `public-tree`. Race local é limitado pelo VMA do host (39 bits, requer 48);
a validação efetiva de race ocorre no CI Linux suportado.
Os workflows preservados publicam imagens AMD64/ARM64 após validar a main.

Ao promover uma feature ou registrar novo aceite real, atualizar a matriz, a diferença
entre branches e a referência auditada. Commit ou teste verde isolado não comprova homologação Telegram.

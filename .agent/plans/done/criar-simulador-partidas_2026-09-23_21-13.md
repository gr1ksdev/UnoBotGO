# Plano: criar-simulador-partidas

## Pedido do usuário
Criar, na branch `dev`, um ambiente local de testes em que o operador inicia uma simulação, escolhe a quantidade de jogadores (mínimo de dois) e o modo de jogo, deixa os jogadores automatizados concluírem a partida e recebe ao final um relatório com estatísticas, possíveis erros e explicações das jogadas com cartas especiais, como bloqueio, reversão, coringas, +2, +4, empilhamentos e penalidades.

## Objetivo
Adicionar um simulador de partidas separado do bot do Telegram, executável pelo terminal e baseado diretamente na engine V2 real. O simulador deverá:

- aceitar de 2 a 10 jogadores, respeitando o limite atual da engine;
- permitir selecionar `classico` (`uno.BotRules`) ou `caseiro` (`uno.CaseiroRules`);
- conduzir todos os jogadores por uma estratégia automática, sem depender de Telegram, token, banco de dados ou rede;
- usar semente informada ou gerada e sempre registrá-la, permitindo reproduzir exatamente uma partida;
- validar o estado após cada ação e impedir execução infinita por um limite configurável de jogadas;
- mostrar o andamento de forma legível no terminal;
- produzir ao final um relatório detalhado em Markdown e um resumo no terminal;
- registrar estatísticas gerais e por jogador, erros encontrados e a explicação contextual de cada jogada especial que realmente ocorreu.

## Contexto atual
- A branch ativa já é `dev` e acompanha `origin/dev`.
- O repositório possui alterações locais não commitadas em `internal/uno`, `internal/game`, `internal/telegram`, testes e arquivos de memória. Elas pertencem ao trabalho atual e deverão ser preservadas.
- `internal/uno` é uma engine independente de Telegram e fornece `NewGame`, `WithShuffler`, `Apply`, `CanPlay`, `Snapshot` e `State.Validate`, o que permite executar partidas reais e determinísticas sem duplicar as regras.
- `internal/uno/lifecycle_test.go` já contém um loop simples de 40 partidas completas com sementes fixas. Ele comprova que a engine pode ser simulada, mas não oferece interface de uso, seleção de modo, rastreamento detalhado, explicação de efeitos nem relatório persistido.
- A engine aceita no máximo 10 participantes, inicia com pelo menos 2 e usa revisões estritas em todas as ações.
- Os modos expostos atualmente ao usuário são:
  - `classico`: `uno.BotRules()`, com colocações, empilhamento do mesmo tipo, regras restauradas da V1 e entrada tardia habilitada;
  - `caseiro`: `uno.CaseiroRules()`, que acrescenta empilhamento cruzado entre +2 e +4.
- Os eventos de domínio já descrevem cartas jogadas, compras, bloqueios, inversões, escolha de cor, UNO, blefe, colocações e encerramento. O simulador poderá correlacionar ação, snapshot anterior, eventos e snapshot posterior para explicar cada efeito sem alterar a engine.
- A suíte atual está saudável: `go test ./...` foi executado durante a análise e passou em todos os pacotes.

## Arquivos analisados
- `AGENTS.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- `README.md`
- `Makefile`
- `.gitignore`
- `go.mod`
- `codemaps/architecture.md`
- `docs/v2-application.md`
- `docs/v2-audit.md`
- `docs/v2-rules.md`
- `docs/build.md`
- `docs/branching.md`
- `cmd/bot/main.go`
- `internal/uno/action.go`
- `internal/uno/card.go`
- `internal/uno/deck.go`
- `internal/uno/errors.go`
- `internal/uno/event.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/uno/lifecycle_test.go`
- `internal/uno/rules.go`
- `internal/uno/state.go`
- `internal/game/manager.go`
- `internal/game/service.go`
- `internal/game/views.go`
- planos recentes de regras, blefe e seleção de modo em `.agent/plans/done/`

## Arquivos que poderão ser modificados
- `cmd/simulator/main.go` (novo)
- `internal/simulation/config.go` (novo)
- `internal/simulation/runner.go` (novo)
- `internal/simulation/strategy.go` (novo)
- `internal/simulation/report.go` (novo)
- `internal/simulation/runner_test.go` (novo)
- `internal/simulation/report_test.go` (novo)
- `internal/simulation/config_test.go` (novo)
- `cmd/simulator/main_test.go` (novo)
- `README.md`
- `Makefile`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`, somente se a implementação confirmar uma decisão arquitetural duradoura
- este plano, ao ser movido entre `.agent/plans/pending/`, `.agent/plans/approved/` e `.agent/plans/done/`

Não há alteração planejada em `internal/uno`, `internal/game`, `internal/telegram`, Docker, CI/CD, V1 ou regras de produção. Se a implementação revelar que uma mudança nesses componentes é indispensável, o trabalho deverá parar e o plano deverá ser revisado antes de ampliar o escopo.

## Estratégia de implementação
Criar o pacote `internal/simulation` como orquestrador da engine e o executável `cmd/simulator` como interface de terminal. Essa separação permitirá testar o motor do simulador sem subprocessos nem entrada interativa e manterá apresentação, estratégia automática, coleta de métricas e regras do UNO em responsabilidades distintas.

Quando `--players` ou `--mode` não forem informados, o executável perguntará esses valores no terminal. Também aceitará flags para automação: `--players`, `--mode`, `--seed`, `--max-actions`, `--output` e, opcionalmente, `--quiet`. O padrão será uma partida, semente gerada e exibida, limite defensivo de ações e relatório salvo sob `.reports/simulations/`. Entradas inválidas deverão retornar mensagem clara e código de saída diferente de zero sem iniciar uma partida.

O runner criará a partida com a engine real, adicionará jogadores numerados, escolherá um dealer de forma determinística pela semente e iniciará o jogo. Em cada ciclo ele obterá um snapshot, decidirá uma única ação válida, aplicará a revisão atual, coletará os eventos e validará o snapshot resultante. A simulação encerrará somente em `uno.Finished` ou com um erro diagnóstico.

A estratégia automática será simples, explícita e reproduzível:

- escolher entre cartas jogáveis por uma ordenação estável, usando a semente apenas quando houver empate que deva variar;
- responder a penalidades com uma carta de empilhamento válida quando houver uma; caso contrário, comprar a penalidade;
- escolher a cor mais frequente na própria mão, com desempate estável;
- decidir o desafio de blefe por uma política documentada e determinística, registrando sucesso ou falha;
- após compra voluntária, jogar uma carta válida quando possível ou passar;
- nunca acessar ou alterar campos privados da engine por atalhos fora de sua API pública.

O coletor manterá um registro por ação contendo número sequencial, revisão, jogador, ação tentada, estado relevante antes/depois, eventos emitidos e erro. Para cartas especiais, o texto explicará o efeito efetivo naquela configuração e naquela mesa, incluindo particularidades como Reverse equivaler a bloqueio com dois jogadores, cor escolhida por Wild, acúmulo de `DrawCounter`, resposta cruzada exclusiva do Caseiro, alvo da penalidade e resultado do blefe do +4.

O relatório Markdown deverá conter configuração e semente, resultado/colocações, duração, total de ações, compras e cartas compradas, maior mão, contagem de UNO, jogadas por tipo de carta, bloqueios, reversões, coringas, +2, +4, empilhamentos, penalidades e blefes. Haverá uma tabela por jogador e uma linha do tempo somente das jogadas especiais. A seção de diagnóstico listará ações rejeitadas, falhas de invariantes, estouro do limite, término anormal e seus contextos; se nada ocorrer, registrará explicitamente que nenhum erro foi detectado. O relatório não deverá incluir segredos ou depender do `.env`.

## Passos detalhados

### Milestone 1 — Contratos e núcleo determinístico

1. Mover este plano para `.agent/plans/approved/` após aprovação explícita e registrar o estado inicial sem tocar nas alterações locais existentes.
2. Criar `internal/simulation/config.go` com configuração validada: 2–10 jogadores, modos `classico`/`caseiro`, semente, limite positivo de ações, destino do relatório e nível de saída.
3. Criar `internal/simulation/runner.go` para montar a partida, aplicar ações com a revisão correta, validar o estado após cada passo, capturar falhas com contexto e finalizar com um resultado estruturado.

**Critério de aceite:** uma partida configurada pelo pacote inicia com a engine real, registra cada transição e termina de forma determinística ou retorna um diagnóstico estruturado.

### Milestone 2 — Jogadores automáticos e cobertura das regras

4. Criar `internal/simulation/strategy.go` com seleção determinística de carta, compra/passe, escolha de cor e política de desafio de blefe, sempre usando `Snapshot` e `CanPlay` da engine.
5. Criar testes determinísticos do runner cobrindo os dois modos, limites de jogadores, término normal, mesma semente produzindo o mesmo resultado e limite máximo de ações.
6. Criar cenários controlados para Skip, Reverse com dois e três ou mais jogadores, Wild, +2, +4, empilhamento, penalidade e blefe.

**Critério de aceite:** bots concluem partidas Clássicas e Caseiras com sementes fixas, sem ações inválidas, e os cenários especiais produzem os eventos esperados.

### Milestone 3 — Estatísticas, explicações e relatório

7. Criar `internal/simulation/report.go` para transformar ações, eventos e snapshots em estatísticas gerais e por jogador, explicações de cartas especiais e relatório Markdown.
8. Testar entradas inválidas e falhas diagnósticas, garantindo relatório parcial quando a partida já tiver começado e erro de configuração antes dela.
9. Testar o relatório por campos e trechos essenciais, evitando comparações frágeis do documento inteiro.

**Critério de aceite:** cada execução gera um resumo e um Markdown reproduzível com configuração, resultado, métricas, jogadas especiais e seção explícita de erros.

### Milestone 4 — Interface de terminal e documentação

10. Criar `cmd/simulator/main.go` com flags e perguntas interativas em português. Validar entrada antes de iniciar e imprimir o caminho final do relatório.
11. Adicionar um alvo conveniente no `Makefile` e documentar no `README.md` exemplos interativo e não interativo, reprodução por semente, limites aceitos e localização dos relatórios.

**Critério de aceite:** o operador consegue iniciar pelo terminal, escolher quantidade e modo, acompanhar a execução e localizar o relatório sem configurar Telegram ou `.env`.

### Milestone 5 — Validação e rastreabilidade

12. Executar formatação, testes, race detector, vet, build e verificação de whitespace. Rodar manualmente ao menos uma simulação Clássica e uma Caseira com sementes fixas e inspecionar seus relatórios.
13. Atualizar `README.md`, `.agent/context.md`, `.agent/memory/memory.md` e, se aplicável, `.agent/decisions.md`; registrar no plano os resultados da implementação e movê-lo para `.agent/plans/done/`.

**Critério de aceite:** todas as verificações passam, os dois relatórios manuais são válidos e o histórico técnico registra a entrega e suas limitações.

## Riscos
- **Loop muito longo ou estratégia sem progresso:** certas sequências de compra e reciclagem podem prolongar uma partida. Mitigação: `--max-actions`, progresso monitorado e relatório parcial com semente/snapshot quando o limite for atingido.
- **Simulador mascarar defeitos da engine:** escolher somente caminhos convenientes reduziria o valor do teste. Mitigação: aplicar decisões pela API pública, registrar toda rejeição inesperada e validar invariantes após cada ação.
- **Explicação divergir do efeito real:** textos inferidos apenas pelo nome da carta podem errar em mesas de dois jogadores ou durante empilhamento. Mitigação: produzir explicações a partir da combinação de ação, eventos e snapshots anterior/posterior.
- **Não exercitar todas as cartas especiais em uma partida aleatória:** uma rodada pode terminar sem determinado efeito. Mitigação: relatório declara apenas eventos ocorridos; testes com baralhos controlados cobrem todos os efeitos obrigatórios.
- **Reprodutibilidade incompleta:** usar fontes aleatórias diferentes no embaralhamento e na estratégia quebraria a repetição. Mitigação: derivar ambas de uma semente registrada e testar equivalência de execuções.
- **Conflito com alterações locais atuais:** vários arquivos de engine e Telegram estão modificados. Mitigação: concentrar a implementação em novos arquivos, nunca restaurar alterações existentes e revisar o diff por arquivo antes de finalizar.
- **Confusão entre `ClassicRules` da engine e o modo Clássico do Telegram:** são contratos diferentes. Mitigação: mapear explicitamente `classico` para `uno.BotRules()` e `caseiro` para `uno.CaseiroRules()`.
- **Exposição indevida de mãos:** o ambiente é local e de teste, mas o relatório pode revelar cartas. Mitigação: documentar que o relatório é técnico/local e, por padrão, registrar apenas contagens e cartas efetivamente jogadas; snapshots completos ficam restritos a diagnósticos de erro.

## Impactos esperados
- Um comando local permitirá simular partidas completas sem iniciar o bot nem configurar credenciais.
- Bugs de regra, ações rejeitadas e violações de estado ganharão um artefato reproduzível por semente.
- Será possível comparar Clássico e Caseiro com métricas equivalentes.
- Jogadas especiais terão explicações auditáveis ligadas aos eventos reais da engine.
- A engine e o fluxo Telegram continuarão com o mesmo comportamento.
- Os relatórios gerados em execução ficarão em `.reports/simulations/`; deverá ser verificado se o padrão atual de `.gitignore` já os mantém fora do versionamento e ajustado apenas se necessário.

## Compatibilidade
- **Linux:** execução nativa via `go run ./cmd/simulator`; ambiente principal de validação.
- **macOS:** compatível por usar somente biblioteca padrão e a engine Go existente.
- **Windows:** compatível em terminal; caminhos serão tratados com `filepath`, sem comandos shell internos.
- **Docker:** o simulador não será incluído nem iniciado na imagem de produção nesta etapa; continuará compilável por `go build ./...`.
- **CI/CD:** nenhuma mudança de workflow planejada; a nova suíte será descoberta por `go test ./...` e o binário por `go build ./...`.

## Como testar

### Build
```bash
gofmt -w cmd/simulator internal/simulation
go build ./...
go vet ./...
```

### Testes
```bash
go test ./...
go test -race ./...
git diff --check
```

Testes focados previstos:

```bash
go test ./internal/simulation -run 'TestRunner|TestReport|TestConfig'
```

### Execução
Fluxo interativo:

```bash
go run ./cmd/simulator
```

Fluxos reproduzíveis:

```bash
go run ./cmd/simulator --players 2 --mode classico --seed 20260923
go run ./cmd/simulator --players 4 --mode caseiro --seed 20260924
```

Critérios manuais: ambas as partidas devem terminar ou emitir diagnóstico explícito; o terminal deve informar colocação, semente e caminho do relatório; o Markdown deve conter estatísticas, seção de erros e explicações de todas as jogadas especiais ocorridas.

## Rollback
Como a implementação será concentrada em arquivos novos, remover apenas `cmd/simulator/` e `internal/simulation/`, e reverter exclusivamente as linhas adicionadas ao `README.md`, `Makefile` e arquivos `.agent` por esta tarefa. Preservar todas as alterações locais que já existiam antes da implementação. Se houver commit próprio, preferir um revert desse commit após confirmar o alvo; não usar `git reset --hard`, `git clean` ou restauração ampla da árvore.

## Observações
- Implementação concluída em 2026-09-24, organizada nas cinco milestones deste plano.
- Aprovado pelo usuário em 2026-09-23 com: "sim, faca em formato de milestones, pfvr".
- A interpretação adotada é que os participantes serão bots locais autônomos e que o operador escolhe quantidade e modo antes da partida. Não haverá controle manual carta a carta nesta primeira versão.
- O escopo é validar a lógica da engine e gerar diagnóstico. Ele não simulará Bot API, callbacks, cache inline, filas do Telegram, latência de rede ou interface de stickers.
- A primeira versão executará uma partida por invocação. Lotes de centenas de partidas e relatórios agregados podem ser adicionados depois, usando o mesmo runner, caso sejam necessários.
- O modo chamado `classico` no simulador deve refletir exatamente o modo Clássico atual do bot (`uno.BotRules`), não `uno.ClassicRules`/`FirstWinner` usado por testes estritos da engine.
- Nenhum commit, push, merge, início do bot real ou alteração de branch faz parte desta etapa.

## Resultado da implementação

### Milestone 1 — Contratos e núcleo determinístico
- Criados `Config`, validação de 2–10 jogadores, modos Clássico/Caseiro, seed e limite de ações.
- Runner aplica exclusivamente ações públicas da engine, registra antes/depois/eventos e chama `State.Validate()` após cada ação aceita.
- Diagnósticos estruturados cobrem ação rejeitada, estado inválido, limite e cancelamento de contexto.

### Milestone 2 — Jogadores automáticos e cobertura das regras
- Estratégia automática escolhe cartas por RNG reproduzível, responde a penalidades, escolhe a cor predominante, compra/passa e desafia blefe quando não pode empilhar.
- Mesma seed reproduz dealer, baralho, decisões, eventos e estado final.
- Testes cobrem ambos os modos, 2, 4 e 10 jogadores, matriz adicional de 20 combinações seed/modo, limite de ações e cancelamento.

### Milestone 3 — Estatísticas, explicações e relatório
- Relatório Markdown contém configuração, colocações, duração, métricas gerais/por jogador, especiais explicados e diagnóstico.
- Explicações testadas para bloqueio, Reverse com dois e três ou mais jogadores, Coringa, +2, +4, escolha de cor, empilhamento, penalidade e blefe certo/errado.
- Relatório parcial é preservado quando a execução termina com diagnóstico.

### Milestone 4 — Interface e documentação
- `cmd/simulator` suporta fluxo interativo e flags `--players`, `--mode`, `--seed`, `--max-actions`, `--output` e `--quiet`.
- `make simulator` e instruções foram adicionados ao `README.md`.
- Saída padrão usa `.reports/simulations/partida_<data>_<milissegundos>_seed-<seed>.md`; diretório ignorado pelo Git.

### Milestone 5 — Validação e rastreabilidade
- `go test ./...`: aprovado.
- `go vet ./...`: aprovado.
- `go build ./...`: aprovado.
- `gofmt` e whitespace dos arquivos da entrega: aprovados.
- Simulações manuais aprovadas:
  - Clássico, 2 jogadores, seed `20260923`: 25 ações, conclusão normal.
  - Caseiro, 4 jogadores, seed `20260924`: 37 ações, conclusão normal; relatório incluiu +4, blefe detectado, bloqueios, reversão, +2, empilhamento e penalidades.
- `go test -race ./...` não pôde ser executado neste ambiente: sem CGO, o Go rejeita `-race`; com `CGO_ENABLED=1`, o linker Termux/Android falha em símbolos do runtime (`__errno_location` e `__android_log_vprint`). A falha ocorre antes da execução dos testes e não indica falha da implementação.
- `git diff --check` global continua apontando linhas em branco finais em três arquivos previamente modificados (`internal/telegram/inline.go`, `internal/telegram/inline_test.go`, `internal/uno/game_test.go`). Eles foram preservados. O check restrito aos arquivos desta entrega passou.
- Memória, contexto e decisão arquitetural atualizados. Nenhum commit, push ou merge realizado.

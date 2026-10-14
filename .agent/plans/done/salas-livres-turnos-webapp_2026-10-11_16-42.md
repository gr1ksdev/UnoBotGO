# Plano: salas-livres-turnos-webapp

## Pedido do usuário
Implementar salas livres no WebApp com convites Telegram, simplificar os controles de turno com compra manual e passagem autoritativa após compra sem jogada legal, e pontuar resultados livres individualmente sem grupo.

## Objetivo
Manter explicitamente salas `group` e `free`, proteger entrada/capacidade no servidor, reutilizar a engine e finalização transacional, preservar fluxo Telegram `/novo` e respectivas opções de desafio, e mostrar jogos livres no perfil/histórico.

## Contexto atual
- A Home já abre um modal de criação que exige grupo verificado; convite opaco via Telegram `startapp=join_...` exige participação no grupo.
- O manager indexa cada jogo por ChatID não-zero e finalização exige `chat_id` ligado a `group_configs`.
- `ApplyWeb` já possui recibos de ações sob lock, enquanto o engine `DrawCard` atual permite jogo após compra conforme `Rules.FreePlayAfterDraw`; não há passagem automática de WebApp quando nenhuma carta é legal.
- A UI mostra “Passar turno” após compra e “Desafiar +4” quando a engine informa desafio. O deck já resolve a compra de penalidades (+2/+4) no caminho engine.
- Migration 0011 adiciona `origin=inline|webapp`; origem do transporte não é o tipo da sala.
- Perfil soma estatísticas de grupo e lista histórico unindo `completed_games.chat_id` a grupos.
- Alterações locais de desempenho, experiência, assets, áudio, fullscreen e diagnóstico estão presentes e devem ser preservadas.

## Arquivos analisados
- `AGENTS.md`, `.agent/context.md`, `.agent/decisions.md`
- `internal/uno/game.go`, `internal/uno/rules.go`, eventos e testes
- `internal/game/service.go`, `manager.go`, `views.go`, `results.go`, `rematch.go`, finalização e testes
- `internal/httpapi/rooms.go`, `live.go`, `server.go`, `rooms_test.go`
- `internal/ranking/ranking.go`, `profile.go`, elegibilidade e testes
- `internal/storage/postgres/results.go`, `profile.go`, `ranking.go`, migrations e integration tests
- `web/src/pages/Home.tsx`, `Invite.tsx`, `Game.tsx`, `Profile.tsx`, `CreateRoom.tsx`, `ShareRoom.tsx`, `api/client.ts` e testes
- `Makefile`, `internal/app/app.go`

## Arquivos que poderão ser modificados
- `internal/uno/` engine e testes
- `internal/game/` serviços, projeções, resultados, rematch e testes
- `internal/httpapi/` rotas/projeções/testes
- `internal/ranking/` DTO, validação e testes
- `internal/storage/postgres/` finalização, perfil, migration 0012 e testes
- `web/src/` Home, criação de sala, convite, lobby/turnos, perfil e testes
- `.agent/plans/` e, se necessário, memória/decisões internas

## Estratégia de implementação
1. Manter o `Apply` usado por partidas Telegram intacto; criar caminho engine de compra WebApp que, no mesmo clone/commit/revision, verifica cada carta por `CanPlay` e passa o turno somente se nenhuma for legal. A compra manual no início permanece necessária. Recibos `ApplyWeb` continuam evitando repetição; o deck continua aceitando/resolvendo penalidades pendentes, inclusive +4. Não mudar a decisão de desafio nas partidas Telegram.
2. Introduzir `room_type=group|free` como domínio explícito. Salas livres usam ChatID zero apenas como ausência, nunca criam grupo/configuração fictícios; proprietário autenticado inicia o lobby e a engine mantém limite e admissão.
3. Generalizar criação e convite: Home oferece criar diretamente sala livre e, opcionalmente, escolher uma sala vinculada a grupo; convite mantém token AEAD/Telegram `startapp`, valida usuário autenticado e estado/capacidade em toda entrada; para `group` continua exigindo membership verificado.
4. Adicionar migration aditiva: permitir `chat_id NULL` apenas em resultados `free`, armazenar o tipo, e criar acumulados individuais por usuário/política de pontuação. No finalizador, resultados livres entram no mesmo ledger/hash/idempotência/transaction, atualizam apenas estatísticas individuais; grupo mantém bloqueio, verificação de sistema e tabelas de grupo sem mudanças.
5. Projetar tipo e nome “Sala livre” no WebApp sem inventar grupo. Mostrar pontos livres separadamente no perfil, preservar scores/histórico de grupo e suportar rematch/reconexão com o mesmo tipo.
6. Cobrir turnos, compras/duplicatas, +4 e Telegram, create/join/autorização/capacidade, reconexão/rematch, perfis e PostgreSQL concorrente; executar checks do projeto.

## Passos detalhados
1. Mover plano autorizado para `approved/`; implementar tipo explícito da sala nos contratos de jogo e projeções.
2. Implementar compra/passagem atômica no caminho WebApp e testes de legalidade/penalidades/idempotência, mantendo `Apply` de grupo.
3. Adaptar convites e criação para livres e grupos com verificação de estado, capacidade, owner e identidade autenticada.
4. Adicionar migration e finalização individual transacional/idempotente para Legacy/Updated; ajustar Profile/history.
5. Atualizar Home, CreateRoom, lobby, invite e Profile; remover botões WebApp de blefe/passar turno e preservar o monte manual.
6. Executar testes Go/Vitest, integração PostgreSQL quando disponível, lint/typecheck/build, checks exigidos e `git diff --check`; documentar limitações reais.
7. Atualizar contexto/decisão/memória só se a nova arquitetura demandar e mover plano para `done/`.

## Riscos
- Misturar tipo da sala com origem do transporte poderia creditar grupo a jogo livre; manter campos distintos e constraints.
- Duas mutações para compra+passagem criariam revisões/eventos intermediários incoerentes; resolver em uma única ação engine WebApp.
- Retry concorrente do mesmo resultado livre poderia duplicar pontos sem lock por GameID; serializar a chave idempotente na transação e testar.
- Regressão na admissão/convite de grupos; preservar validação existente por membership em todo acesso.
- O +4 desafiável continua resolvível no WebApp ao comprar a penalidade pelo monte; o desafio segue disponível no Telegram. Não alterar a regra de desafio.

## Impactos esperados
- Usuários autenticados Telegram criam e compartilham lobby livre sem selecionar grupo.
- Uma compra sem jogada legal conclui com passagem de turno e um snapshot/revision coerente; compra com jogada legal conserva a vez.
- Resultados livres aparecem no perfil/histórico e somam em acumulado individual da política selecionada, sem alterar placares de grupo.

## Compatibilidade
- macOS local, Linux e CI; PostgreSQL existente via migration aditiva; Docker/binário embed compilados sem alteração de dependências.

## Como testar

### Build
`make check` e `make build` (ou targets disponíveis no Makefile).

### Testes
`go test ./...`, testes `-tags integration` com `TEST_DATABASE_URL`, suites `npm --prefix web run test`, além de lint/typecheck.

### Execução
Criar sala livre na Home, copiar/abrir convite em outro Telegram autenticado, atingir capacidade de 10, iniciar como responsável, jogar/reconectar/fazer revanche; finalizar e conferir pontos individuais sem linha de grupo.

## Rollback
Reverter migration e código somente antes de aplicar a migration em ambiente persistente. Para qualquer banco com a migration aplicada, manter a migration append-only e fazer correção aditiva; não usar rollback destrutivo automático.

## Observações
A mensagem do usuário aprova explicitamente as regras e a implementação; não pedir nova aprovação. A decisão de +4 não precisa mudar: o WebApp resolve a penalidade existente por compra no monte, enquanto o desafio Telegram permanece.

## Resultado da execução
- Aplicados sala `free` tipada, convite Telegram por referência selada, admissão/autorização no servidor, scores individuais idempotentes e passagem após compra no único revision WebApp.
- Passaram `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, verificações completas com `debugcards`, 129 testes Vitest, lint, typecheck, build frontend e `git diff --check`.
- O pacote de integração PostgreSQL compilou com `go test -tags integration -run '^$' ./internal/storage/postgres`, mas sua execução depende de `TEST_DATABASE_URL`, ausente nesta máquina. Nenhum commit, push ou deploy.

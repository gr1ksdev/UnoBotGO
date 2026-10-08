# Plano: redesign mobile e integracao WebApp

## Pedido do usuário
Implementar as seis telas mobile da Mini App com as referencias de docs/design-reference/unobotgo-v1, integrando partidas reais ao UNO existente e unificando resultados Inline/WebApp sem alterar regras ou pontuacao.

## Objetivo
Converter a referencia para React/TypeScript, adicionar acesso autenticado a partidas do servico Go existente e reutilizar uma unica finalizacao transacional, com privacidade, revisoes e idempotencia.

## Contexto atual
Branch dev. Unica alteracao local observada: docs/design-reference/ nao rastreado; preservar integralmente. Frontend React 19/TypeScript/Vite/TanStack Query/React Router, npm package-lock. API autenticada por initData HMAC; ranking mensal em America/Sao_Paulo, cursores opacos e score_units string formatado com BigInt. Perfil atual somente identidade e privacidade, sem estatisticas/historico. Nao existem rotas HTTP de partida. internal/game controla a engine, revisions, projecoes privadas, encerramento e timers; suporta ate dez jogadores. Resultados pendentes vivem em memoria. Telegram prepara/persiste resultados por RecordCompletedGame e notifica apos commit. PostgreSQL exige grupo/configuracao existente; sistemas Legacy/Updated permanecem separados, sem converter ou somar politicas incompatíveis. Origem da partida ainda nao e persistida.

## Arquivos analisados
- AGENTS.md
- .agent/context.md, .agent/memory/memory.md, .agent/decisions.md
- docs/branching.md, Makefile, go.mod, web/package.json, .github/workflows/dev-ci.yml
- docs/design-reference/unobotgo-v1/index.html, ui.css, ui.js, tokens.css
- docs/design-reference/unobotgo-v1/docs/DESIGN-SYSTEM.md, INTEGRACAO.md, REFERENCIA.md
- docs/design-reference/unobotgo-v1/assets/cards/, assets/icons/, previews/overview.png
- web/src/App.tsx, api/client.ts, pages/Rankings.tsx, pages/Profile.tsx, lib/telegram.ts
- internal/httpapi/server.go, auth.go
- internal/game/service.go, manager.go, views.go, results.go
- internal/uno/action.go, rules.go
- internal/ranking/global.go
- internal/storage/postgres/results.go, global_rankings.go
- internal/telegram/results.go, bot.go
- internal/app/app.go

## Arquivos que poderão ser modificados
- web/src/pages/, components/, hooks/, api/client.ts, lib/telegram.ts, styles.css, App.tsx e testes relacionados
- web/public/assets/ ou web/src/assets/ para cartas e icones derivados do pacote
- internal/httpapi/ e testes de autenticacao, DTOs, salas, comandos e snapshots
- internal/game/ para projecoes de timer/metadados e deduplicacao atomica de comandos
- internal/ranking/ para finalizador compartilhado e leituras de perfil/posicao
- internal/storage/postgres/ e nova migration aditiva para modalidade, quando necessaria
- internal/telegram/ para convites de sala, integracao com finalizador e notificacoes
- internal/app/app.go para compartilhar servico, finalizador e configuracao do bot
- docs/ para contrato efetivo e verificacao visual; .agent/ para memoria e decisoes
- web/package.json e package-lock.json somente se necessario para verificacao em navegador

## Estratégia de implementação
Executar ranking, perfil/home e partida/resultado nessa ordem. Reutilizar DOM/React, tokens e SVGs proprios. Manter apenas categorias Jogadores/Grupos; preservar distincao das politicas existentes sem criar categoria de transporte. Adicionar consultas reais para minha posicao, estatisticas e historico paginado, respeitando anonimato e elegibilidade. Adaptar finalizacao Telegram para operacao compartilhada que prepara resultado, persiste, reconhece commit e dispara notificacao somente apos commit novo.

Fluxo minimo de jogo parte de sala real em grupo: bot gera referencia opaca e servidor valida contexto e participacao/admissao antes de servir snapshot ou aceitar comandos. Home permite abrir salas acessiveis e orienta criacao/entrada pelo fluxo real do bot; nao inventar IDs de grupo nem pontuacao privada. Reutilizar engine e autorizações, derivando identidade exclusivamente de initData validado. Usar HTTP autenticado com polling de snapshot versionado como transporte inicial simples, com comando contendo gameId, requestId e expectedRevision; deduplicacao e validacao sob lock. Snapshots contem apenas mao do solicitante e dados publicos. Ao reconectar recuperar snapshot; descartar revisions antigas e duplicatas. Timer exibido deriva do deadline oficial. Modalidade e metadado da sessao/resultados, nunca fonte separada de score.

## Passos detalhados
1. Completar leitura pontual dos componentes, testes, assets e screenshots individuais; registrar baseline de checks e dependencias locais.
2. Implementar tokens, icones, Avatar, RankingTabs, Podium, RankingRow, MyPosition, BottomNav e estados; lista depois do podio com paginacao existente.
3. Expor posicao real no ranking e leituras de perfil/historico no backend com score exato, mes/sistema e privacidade existentes.
4. Integrar perfil/home com dados reais e atalho usando username obtido da configuracao/runtime do bot; manter controle de privacidade.
5. Extrair finalizador canonico compartilhado, preservar hash/idempotencia, elegibilidade e notificacao apos commit; persistir origem com migration aditiva se requerida.
6. Implementar referencias de sala autorizadas, rotas de snapshot/comando e deduplicacao atomica; conectar o mesmo servico em HTTP e Telegram. Rejeitar grupo/identidade forjados e comando obsoleto.
7. Implementar lobby/partida/resultados reais, selecao por CardID, confirmar jogada, comprar/passar/desistir conforme engine, escolha de cor/destinatario e Troca de Maos com SVG proprio.
8. Adaptar mesa a 2, 3, 4, 5, 6 e ate dez jogadores suportados, mao rolavel e alvos tocaveis; timer oficial, reconexao e encerramento sem controles de turno.
9. Testar integracao PostgreSQL, duplicatas, falsificacoes, revisions e encerramento de dois jogadores, paridade de score dos transportes e privacidade. Falha de commit apresenta resultado pendente sem ganho inventado.
10. Capturar telas nos quatro tamanhos solicitados, comparar referencias e corrigir diferencas materiais; exercitar 1/2/7/15/30 cartas, nomes longos e dez jogadores. Dados controlados de teste somente em harness identificado, nunca como resultado de producao.
11. Executar checks obrigatorios, documentar contrato/limitacoes e atualizar memoria/decisoes; mover plano para done ao concluir. Sem commit, push ou deploy.

## Riscos
- Finalizacao atualmente pertence ao adapter Telegram; extracao precisa preservar notificacoes e retries inclusive em timeout.
- Partidas ativas sao em memoria e nao sobrevivem a reinicio; nao prometer recuperacao persistente inexistente.
- Pontuacao privada nao possui politica implementada; fluxo inicial usa grupos validados, sem criar uma politica nova.
- Politicas Legacy/Updated nao podem ser misturadas para reproduzir numeros ficticios do pacote.
- Dez jogadores e viewport 320x568 exigem composicao compacta adicional as variantes do pacote.
- Cliente Telegram real e credenciais podem limitar homologacao ponta a ponta; separar isso de testes locais verificaveis.

## Impactos esperados
- Mini App com identidade propria, dados reais e navegacao mobile em tres destinos.
- Inline/WebApp compartilham identidade, engine, armazenamento e ranking por politica vigente.
- Novos endpoints autenticados e possivel migration aditiva para origem, preservando historico.
- Maior cobertura de autorizacao, sincronizacao, finalizacao e responsividade.

## Compatibilidade
- Linux: ambiente principal e testes locais.
- macOS: manter comandos e dependencias portaveis.
- Windows: frontend e Go sem novos caminhos especificos de sistema.
- Docker: preservar build embutido V2 e startup fail-closed com migrations.
- CI/CD: manter gates atuais; nao publicar producao.

## Como testar

### Build
```bash
npm --prefix web run build
go build ./...
go build -tags debugcards ./...
```

### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
go test -count=1 ./...
go test -race ./...
go vet ./...
go test -tags debugcards ./...
go vet -tags debugcards ./...
go test -count=1 -tags integration ./internal/storage/postgres/... ./internal/app/...
git diff --check
# Com TEST_DATABASE_URL de banco isolado local:
make check
```
Verificacao em navegador: screenshots 320x568, 360x800, 390x844 e 430x932; estados vazios/erro/loading, poucas entradas, paginacao, anonimato, nomes longos, avatares ausentes, mãos de 1/2/7/15/30 cartas e maximo de jogadores. Testes novos devem verificar contratos e falhas materiais, nao espelhar implementacao.

### Execução
```bash
make dev
# Requer configuracao local valida e PostgreSQL; nao imprimir segredos.
```

## Rollback
Reverter exclusivamente hunks e arquivos criados nesta tarefa, preservando referencias e alteracoes locais do usuario. Migration deve ser aditiva/retrocompativel; nao apagar historico ou pontuacao. Nenhum reset/clean destrutivo. Rollback de banco/ambiente externo exige autorizacao propria.

## Observações
Aguardar aprovacao explicita antes da implementacao, conforme AGENTS.md. A criacao deste plano e a unica escrita preparatoria. Documentacao oficial consultada: https://core.telegram.org/bots/webapps e https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch. Nao presumir endpoints do pacote nem copiar fixtures/innerHTML. Nao executar commit/push/publicacao por conclusao do redesign.

## Ajustes aprovados em 2026-10-08
- WebSocket autenticado e autorizado como transporte principal; HTTP somente consultas/recuperacao. Sem polling periodico da partida.
- Mes mensal explicitamente identificado nas telas de ranking.
- Retry automatico idempotente de resultados retidos, com backoff e confirmacao exclusivamente apos commit.


## Conclusão — 2026-10-08
Escopo implementado e verificado. Relatório técnico e limitações em docs/miniapp-webapp.md; screenshots e comparações em .reports/redesign-mobile/. make check final passou (frontend lint/typecheck/60 testes/build, Go normal/debugcards test/vet/build, integração PostgreSQL e diff check); race normal e integração passaram. Verificação visual: quatro dimensões, seis telas, 120 combinações de mão/jogadores, 20 estados de ranking e quatro safe areas. Referências locais preservadas. Nenhum commit/push/deploy. Homologação com clientes Telegram reais não executada; pendências/partidas em memória e notificações sem outbox estão documentadas.

- Autorização posterior em 2026-10-08: usuário solicitou commit e push exclusivos para dev. Não autoriza deploy ou promoção para main.

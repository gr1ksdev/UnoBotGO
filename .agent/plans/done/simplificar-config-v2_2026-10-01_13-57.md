# Plano: simplificar-config-v2

## Pedido do usuário
Reduzir configuração externa V2 às seis opções normais, eliminar parsers e campos desnecessários, gerar Direct Link automaticamente e preservar webhook seguro. Sem commit/push/deploy ou alterações de gameplay, dados, schema e frontend.

## Objetivo
Config loader único e pequeno para startup V2; políticas fixas internas; webhook funcional com somente URL pública adicional.

## Contexto atual
Branch dev, HEAD d17162c; Git status/diff/branch/log auditados. Há alterações locais anteriores em web/src/App.tsx, App.test.tsx, lib/telegram.ts, pages/Rankings.tsx, styles.css e arquivos .agent, além de testes/planos novos: preservar integralmente. Config V2 está dividida em Config e Web, com políticas internas indevidamente externalizadas. Bot.Run já consulta getMe uma vez e guarda username. Webhook compartilhado registra setWebhook em todo startup, usa secret token e valida header em tempo constante; remove webhook ao voltar a polling sem descartar updates. WEB_ADDR é bind interno e não determina domínio HTTPS público. Config legado V1 independente está fora do escopo de gameplay.

## Arquivos analisados
- internal/config/config.go e config_test.go
- internal/config/web.go
- cmd/bot/main.go
- cmd/migrate/main.go
- cmd/devseed/main.go e internal/devseed/seed.go
- internal/app/app.go e referências em testes
- internal/telegram/bot.go, shared_http.go, transport.go, ranking.go, commands.go
- internal/telegram/transport_test.go, bot_test.go e ranking_test.go
- config.go e main.go (legado V1)
- .env.example, README.md, docs/miniapp.md, docs/v2-telegram.md
- Dockerfile, Dockerfile.v2, docker-compose.yml
- scripts/dev.mjs, Makefile, .github/workflows
- .agent/context.md, memória e handoff

## Arquivos que poderão ser modificados
- internal/config/config.go, web.go e testes (remover arquivo se inteiramente substituído)
- internal/config/defaults.go e testes novos, se necessário
- cmd/bot/main.go
- internal/app/app.go e testes de integração de startup
- internal/telegram/bot.go, ranking.go e testes correspondentes
- .env.example e docker-compose.yml
- README.md, docs/miniapp.md, docs/v2-telegram.md e outros documentos ativos com instruções obsoletas
- .agent/context.md, memory/memory.md, decisions.md e handoff atual
- cmd/migrate/main.go somente se necessário para centralizar acesso DATABASE_URL, preservando timeout próprio existente

## Estratégia de implementação
Unificar configuração de startup em estrutura com TOKEN, TELEGRAM_MODE, DATABASE_URL, TURN_TIMEOUT, WEB_ADDR, MINIAPP_SECRET e WEBHOOK_URL condicional. Políticas internas constantes: log info, histórico 100, TTL 2m, limites 20000/512, max age 1h, timeout de migrations do startup 2m e drop false. Remover alias WEBHOOK_LISTEN_ADDR, LaunchURL e campos de políticas da configuração externa. Preservar formato Base64 de exatamente 32 bytes e criptografia existente de refs/cursores.

Para eliminar WEBHOOK_SECRET, derivar segredo via HMAC-SHA256 da chave mestre MINIAPP_SECRET com mensagem de domínio estável exclusiva, por exemplo `UnoBotGO:webhook-secret:v1`, codificado Base64 URL sem padding. Nunca enviar chave mestre diretamente; segredo derivado compatível com caracteres aceitos pelo Telegram. Não alterar derivação AES/refs atual. Registrar justificativa e rotação: ao alterar MINIAPP_SECRET, startup reaplica setWebhook com novo segredo, como já ocorre. URL pública completa HTTPS e path dedicado continuam necessários e validados. Mover validação de conflito de rotas para boundary de config quando adequado, preservando rejeição existente.

Após getMe, montar `https://t.me/<username>/ranking`, removendo @ inicial, sem nova consulta API e sem depender de URL externa BotFather. Remover setter externo de launch URL onde morto e adaptar testes para geração por identidade real.

Documentos ativos devem ensinar apenas configurações atuais; planos históricos não serão apagados/sobrescritos, e referências históricas terão seu caráter histórico distinguido de instruções vigentes. Não ler/imprimir segredos do .env pessoal nem sobrescrever esse arquivo automaticamente; fornecer orientação de remoção das chaves antigas, que deixarão de ser lidas. Ferramentas devseed, migrations e testes continuam com variáveis técnicas próprias, fora do .env normal e sem exigir token/segredo do bot.

## Passos detalhados
1. Após aprovação, mover plano para approved e guardar snapshot do diff local para conferência de preservação, sem stash/reset.
2. Unificar loader V2 e preservar validação antecipada de TOKEN, DATABASE_URL, MINIAPP_SECRET, modo e timeout positivo; manter validação de bind equivalente à atual.
3. Internalizar políticas fixas em local idiomático e remover parsers, erros e campos mortos.
4. Manter WEBHOOK_URL somente em webhook; implementar derivação de segredo separada e reutilizar validação/autenticação existentes, drop false e servidor único.
5. Gerar Direct Link no getMe existente antes de processar updates e atualizar uso do botão de ranking.
6. Simplificar .env.example e Compose; revisar Docker/workflows/scripts sem alterar build/embed/arquitetura.
7. Atualizar documentos ativos e registrar decisões/memória, preservando histórico técnico.
8. Testar configuração mínima, obrigatórios, modo polling/webhook/inválido, timeout inválido, segredo Base64 e tamanhos abaixo/acima/exatos (incluindo rejeição 64 bytes), todos os defaults internos e irrelevância das opções eliminadas.
9. Testar Direct Link com UnoRobotBot e @UnoRobotBot, botão, consulta única getMe; testar segredo derivado determinístico, separado da chave mestre, formato, rotação e header incorreto recusado, além de setWebhook e drop false.
10. Rodar busca global final, go test ./..., go test -race ./..., go vet ./..., build de ./... e cmd/bot. Rodar make check quando houver banco TEST_DATABASE_URL e ferramentas; reportar bloqueio se ausentes. Não iniciar bot conectado nem aplicar migrations em banco real.
11. Conferir diff/status finais contra alterações prévias, documentar resultados e mover plano a done.

## Riscos
- Webhook exige domínio HTTPS externo real: não é possível reduzir a seis envs em todas as instalações sem perder essa informação.
- Rotação de MINIAPP_SECRET altera segredo derivado; startup precisa reaplicar setWebhook e teste deve confirmar header alinhado.
- Refatoração de Config/Web afeta testes e wiring de startup; políticas/auth e transações devem permanecer idênticas.
- make check exige PostgreSQL de teste e npm; race depende de CGO/toolchain. Não usar banco de produção como substituto.
- BotFather precisa ter Direct Mini App com short name ranking configurado; geração de link não configura BotFather.

## Impactos esperados
- Seis opções normais e uma opção condicional para webhook.
- Botão de ranking sempre usa identidade real, sem configuração redundante.
- Defaults e segurança preservados; configuração antiga deixa de ser lida.

## Compatibilidade
- Linux, macOS e Windows: loader Go e defaults preservados.
- Docker: pipeline frontend -> embed -> binário -> servidor único preservado.
- CI/CD: testes e builds existentes atualizados sem publicação/deploy.

## Como testar

### Build
```bash
go build ./...
go build -o /tmp/unobotgo-config-validation ./cmd/bot
```

### Testes
```bash
go test ./...
go test -race ./...
go vet ./...
make check
git diff --check
```
make check somente com TEST_DATABASE_URL apontando a banco de teste disponível; testes de integração nunca em produção.

### Execução
```bash
# Após homologação e apenas em ambiente local configurado:
go run ./cmd/bot --dev
```
Não executar este comando durante a refatoração se implicar conexão real ao bot/migrations não autorizadas.

## Rollback
Reverter somente hunks desta tarefa com comparação ao snapshot inicial; preservar alterações anteriores. Não alterar dados nem planos antigos.

## Observações
Esta fase é auditoria e planejamento; nenhuma implementação autorizada pelo fluxo AGENTS.md antes da aprovação explícita deste plano. Sem subagentes, commit, push, amend, rebase, merge, deploy ou migração de produção.


## Entrega e validação
Implementação e documentação realizadas conforme plano. Frontend prévio preservado via comparação SHA256 dos arquivos. Testes de config/startup/Telegram e go test ./... passaram. git diff --check passou. Busca de opções removidas encontrou somente planos históricos, preservados como rastreabilidade.

Verificações incompletas: race padrão recusou CGO desabilitado; tentativa CGO=1 e vet/build interrompidos sem conclusão verificável. make check recusou ausência de TEST_DATABASE_URL. Tentativas de PostgreSQL temporário interrompidas antes de inicialização completa; nenhum banco real alterado. Usuário solicitou encerrar verificações e realizar commit/push dev. Não declarar suite completa ou build aprovados nesta refatoração. Autorização posterior substitui proibição inicial de commit/push; main e deploy continuam fora de escopo.

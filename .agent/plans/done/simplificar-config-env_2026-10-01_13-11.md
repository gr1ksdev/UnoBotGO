# Plano: simplificar configuração externa V2

## Pedido do usuário
Reduzir env normal a TOKEN, TELEGRAM_MODE, DATABASE_URL, TURN_TIMEOUT, WEB_ADDR e MINIAPP_SECRET; remover configurações internas e gerar Direct Mini App via identidade do bot. Preservar webhook seguro e servidor único. Sem commit/push/deploy.

## Objetivo
Configuração externa pequena, loader sem campos mortos, políticas internas constantes e testes de regressão completos.

## Contexto atual
Branch dev, HEAD d17162c, working tree limpo. Config e Web são loaders separados; Config carrega políticas internas e Web aceita endereço legado, link manual e durations. app.Run encaminha ambos. Webhook faz setWebhook em cada startup, usa URL fornecida e segredo explícito; valida header por comparação constante. getMe em Bot.Run fornece username antes de receber updates. Compose expõe link manual. Docker V2 mantém frontend -> embed -> binário único. Docker V1/raiz legado existe e não será redesenhado.

## Arquivos analisados
- AGENTS.md e .agent/context.md
- internal/config/config.go, web.go, config_test.go
- cmd/bot/main.go, cmd/migrate/main.go, cmd/devseed/main.go
- internal/app/app.go
- internal/telegram/bot.go, transport.go, shared_http.go, ranking.go, client.go
- internal/telegram/transport_test.go, lifecycle_test.go
- .env.example, docker-compose.yml, Dockerfile, Dockerfile.v2, Makefile
- README.md, docs/miniapp.md, docs/v2-telegram.md
- .github/workflows/dev-ci.yml, scripts/dev.mjs e buscas globais
- Documentação oficial Telegram Bot API/setWebhook e Direct Mini Apps

## Arquivos que poderão ser modificados
- .env.example e docker-compose.yml
- internal/config/config.go, web.go, config_test.go e testes adicionais/defaults
- internal/app/app.go e testes pertinentes
- cmd/bot/main.go e consumidores diretos necessários dos loaders
- internal/telegram/bot.go, ranking.go, transport.go, shared_http.go e testes pertinentes
- README.md, docs/miniapp.md, docs/v2-telegram.md e documentação ativa encontrada pela busca
- .agent/context.md, decisions.md e memory/memory.md

## Estratégia de implementação
Manter separação de loader de ferramentas se ela for necessária para não exigir MINIAPP_SECRET do simulador/migrate; reunir a configuração de runtime sem duplicar parsing. Structs representam só valores externos; políticas fixas nomeadas em localização idiomática compartilhada. LOG info, histórico100, TTL2m, limites20000/512, initData1h e migration2m preservados. Remover endereço legado WEBHOOK_LISTEN_ADDR e usar só WEB_ADDR.

Exceção mínima webhook: manter WEBHOOK_URL somente quando modo webhook, pois servidor atrás de proxy não conhece sua URL HTTPS pública. Preservar validação de HTTPS e path dedicado antes de recursos/listener, setWebhook a cada startup e drop=false. Eliminar WEBHOOK_SECRET derivando HMAC-SHA256 sobre domínio/contexto fixo exclusivo (por exemplo unobotgo/v2/telegram-webhook-secret/v1), com MINIAPP_SECRET como chave; codificar saída Base64URL sem padding. Não reutilizar diretamente bytes da chave AES nem alterar referências/cursors/initData. Derivação testada quanto a estabilidade, separação e rotação; rotação do MINIAPP_SECRET também atualiza webhook no startup. Valores antigos deixam de ser lidos sem fallback/depreciação.

Gerar link https://t.me/<username>/ranking no getMe já existente, normalizando @ antes de habilitar ingress. Shortname constante ranking. Remover setter/config manual se desnecessário e adaptar testes. URL externa segue exclusivamente BotFather.

Não ler/exibir valores do .env real nem editá-lo automaticamente; remover apenas template/parsing/documentação, preservando segredos locais. Históricos internos antigos em .agent podem manter referências como registro, diferenciados de documentação ativa; varredura final comprova ausência de parsing/uso operacional das removidas.

## Passos detalhados
1. Completar inventário de consumidores, testes, ferramentas, docs e env por nomes, sem expor segredos.
2. Introduzir políticas constantes e reduzir loaders/structs/errors; manter TOKEN/DATABASE_URL/SECRET obrigatórios no runtime.
3. Implementar derivação isolada do secret webhook e validação de sua URL/rota; preservar header e transportes.
4. Integrar link automático em getMe, remover link manual e atualizar testes de botão/lifecycle.
5. Simplificar env.example e Compose; documentar WEBHOOK_URL opcional exclusiva e BotFather.
6. Atualizar testes: config mínima, envs removidas ignoradas, constantes, durations, modos, segredo Base64 exatamente32bytes, URL webhook e derivação/secret/header/setWebhook/drop=false, links com username/@.
7. Buscar globalmente variáveis eliminadas e campos mortos; revisar diff sem tocar regras, UI ou persistência.
8. Executar Go normal/race/vet/build e make check conforme ferramentas/banco disponíveis. Documentar falhas, resolver regressões e registrar limitações reais.
9. Atualizar memória/decisões/contexto; mover plano para done e entregar relatório com arquivos/testes/Git.

## Riscos
- Webhook sem URL pública: manter WEBHOOK_URL; não tentar descoberta fictícia.
- Rotação de MINIAPP_SECRET muda secret webhook: setWebhook no startup aplica novo valor; secret nunca logado. Testar derivação/validação.
- Ferramentas sem servidor não precisam segredo Mini App: preservar separação intencional sem duplicação.
- Testes race anteriormente tiveram limitação de arquitetura/VMA; executar e relatar se recorrente.
- make check exige TEST_DATABASE_URL e npm ci; confirmar serviço local/teste e ferramentas, sem usar banco de produção ou alterar dados reais.

## Impactos esperados
Seis envs normais e uma excepcional WEBHOOK_URL, nenhum campo de política no config externo. Link de ranking automático; funcionamento de polling/webhook e autenticação preservados.

## Compatibilidade
- Linux, macOS, Windows: Go padrão e envs simples.
- Docker: mesma cadeia build frontend/embed/binário/servidor.
- CI/CD: mesmos gates, sem publicação/deploy ou mudanças em main.

## Como testar

### Build
```bash
go build ./...
go build -o /tmp/unobot-config-validation/unobotgo ./cmd/bot
```

### Testes
```bash
go test ./...
go test -race ./...
go vet ./...
make check
git diff --check
```
make check só usa banco de testes isolado; se faltar ferramenta/serviço, registrar motivo exato sem esconder falha. Busca global exclui secrets e distingue registros históricos de uso vivo.

### Execução
Fixtures/mocks para Telegram e testes de handler; não registrar webhook real nem iniciar bot de produção.

## Rollback
Reverter apenas alterações desta rodada após revisão do diff, sem reset/clean ou apagar trabalho local. Nenhum schema ou dado alterado.

## Observações
AGENTS.md requer aprovação explícita do plano antes da implementação. Este arquivo é o único criado nesta auditoria; código e .env real intactos. Sem commit, push, main, deploy ou migrations em produção.

## Execução e conclusão
- Aprovação explícita recebida, incluindo WEBHOOK_URL condicional e separação criptográfica do segredo. Plano executado na dev a partir do HEAD d17162c; .env real preservado.
- Auditoria completa confirmou apenas cmd/bot consumia Config/LoadWeb; loaders unificados sem exigir segredo de ferramentas independentes. Config possui sete campos externos. Políticas internas em defaults.go e fallback do TokenStore reutiliza limites centrais.
- Derivação isolada HMAC-SHA256 com contexto unobotgo/v2/telegram-webhook-secret/v1, saída43caracteres Base64URL sem padding, validada32bytes. Header constante, setWebhook e preservação de updates mantidos nos caminhos shared/standalone/polling. Novo link no getMe único, sem setter/link manual/URL externa.
- Template, compose, README e docs ativos atualizados. Busca global por11variáveis removidas fora de .env/.agent/dependências/assets não retornou ocorrências. Registros históricos .agent preservados e contexto atual documentado. Docker/workflows/scripts sem aliases restantes; Docker V1 legado preservado.
- Novos testes config/derivação/link/startup/webhook, incluindo modo inválido, duração, tamanho de chave31/32/33/64, formato, rotação, separação, headers e getMe único. Nenhum arquivo frontend/CSS, regra, SQL ou migration modificado.
- Exceção de validação documentada: make check falhou no teste existente de desempate por consultar outubro com fixtures setembro. Mesma falha reproduzida no commit base exportado em /tmp. Ajustado SOMENTE ranking_tiebreak_integration_test.go para consultar o mês das fixtures explicitamente; datas/expectativas/regras preservadas. Isso é estabilização de teste para completar checks, não mudança de ranking.
- Resultados finais: go test ./..., go test -race ./..., go vet ./..., go build ./..., build cmd/bot, make check (incluindo frontend lint/typecheck37testes/build, debugcards e integração Postgres), docker compose config --quiet e git diff --check APROVADOS. Binário final bin/unobotgo.
- PostgreSQL isolado em container efêmero unobotgo-config-audit-20261001/porta15433; banco aplicativo existente preservado. Nenhum webhook real registrado ou chamada Telegram externa realizada. Node24 temporário com SHA256 verificado; npm reportou2vulnerabilidades moderadas preexistentes sem alteração de dependências.
- Bloqueio inicial de cache Go no sandbox resolvido com permissões habilitadas pelo usuário. Falhas iniciais de asserção de segredo no teste corrigidas. Nenhuma falha omitida; nenhum commit/push/main/deploy/produção.

## Autorização posterior de versionamento
Usuário solicitou explicitamente git commit e git push após conclusão e validação da refatoração. Esta autorização substitui a restrição anterior exclusivamente para publicar as alterações revisadas na dev. Estratégia: conferir status/diff, sincronização remota e diff-check; commit sem amend e push explícito HEAD:dev sem force. Nenhum deploy ou mudança em main autorizado.

## Integração para push autorizado
- Commit local f313ff4 criado. Fetch revelou commit remoto3c7c455; primeira tentativa de push recusada sem fast-forward, sem force.
- Integrado origin/dev via merge normal, preservando melhorias remotas de fullscreen e remoção do período do hero, respectivos planos e testes. Refatorações paralelas reconciliadas: contexto canônico unobotgo/v2/telegram-webhook-secret/v1, única função validada DeriveWebhookSecret e método de conveniência remoto delegando a ela. Políticas de drop=false centralizadas; validação remota adicional de URL preservada.
- Testes remotos de configuração preservados em installation_test.go e adaptados ao contexto aprovado; outros testes remotos e locais mantidos. Duplicação automática do campo de mock GetMeCalls removida após falha de compilação; validações completas repetidas.
- Resultado integrado: make check integral aprovado, incluindo45testes frontend, Go/debugcards/vet/build e PostgreSQL isolado; go test -race ./... aprovado. Sem mudanças em main, force push ou deploy.

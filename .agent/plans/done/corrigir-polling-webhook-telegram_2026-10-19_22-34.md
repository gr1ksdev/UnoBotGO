# Plano: corrigir polling, webhook e resiliência do bot Telegram

## Pedido do usuário
O bot fica intermitentemente offline, não responde a comandos nem a consultas inline. Reiniciar a VPS/Coolify não resolve; apenas esperar. O token vazado já foi revogado. O usuário quer uma solução completa e perguntou se webhook seria melhor. A hospedagem é uma VPS Linode com Coolify; há outro bot no mesmo ambiente funcionando.

## Objetivo
Eliminar a classe de falha causada por polling curto/429, tornar o webhook a opção recomendada para produção e documentar a operação com instância única, sem alterar o modelo de estado em memória.

## Contexto atual
- `internal/telegram/bot.go:200` usa `GetUpdatesParams{AllowedUpdates: ...}` sem `Timeout`. O telego v1.10.0 só aplica timeout padrão quando `params == nil`; com params não nulo envia `timeout=0` (short polling).
- `internal/telegram/client.go:178` usa `http.Client{Timeout: 15s}`. Insuficiente para long polling de 30s.
- `SafeAPICaller` só repete 429 com `retry_after <= 5s`; valores maiores viram erro e o telego espera 8s, sem respeitar o valor informado.
- `telego.WithDiscardLogger()` esconde erros de `getUpdates`, incluindo 409, 429 e falhas de rede.
- O webhook já existe, valida o secret e retorna 503 quando a fila está cheia, mas o startup não tem retry nem log de status.
- O estado dos jogos é em memória. Vários jogos em grupos diferentes funcionam no mesmo processo; duas instâncias/containers do bot não são suportadas.
- O Coolify faz rolling update por padrão, criando sobreposição de containers. Isso causa 409 no polling e divisão de updates no webhook.
- A documentação atual está desatualizada sobre capacidades de fila e sobre `GetWebhookInfo`.

## Arquivos analisados
- `internal/telegram/bot.go`
- `internal/telegram/client.go`
- `internal/telegram/shared_http.go`
- `internal/telegram/dispatch.go`
- `internal/telegram/transport.go`
- `internal/telegram/client_test.go`
- `internal/telegram/transport_test.go`
- `internal/telegram/lifecycle_test.go`
- `internal/telegram/mock_test.go`
- `internal/app/app.go`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `docs/v2-telegram.md`
- `github.com/mymmrac/telego@v1.10.0/long_polling.go`
- `github.com/mymmrac/telego@v1.10.0/telegoapi/caller.go`

## Arquivos que poderão ser modificados
- `internal/telegram/client.go`
- `internal/telegram/bot.go`
- `internal/telegram/shared_http.go`
- `internal/telegram/mock_test.go`
- `internal/telegram/client_test.go`
- `internal/telegram/transport_test.go`
- `internal/telegram/bot_test.go` (ou novo `polling_test.go`)
- `docs/v2-telegram.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`

## Estratégia de implementação
1. Corrigir o polling com `Timeout: 30`, `Limit: 100` e timeout HTTP de 45s.
2. Fazer o `SafeAPICaller` respeitar `retry_after` integralmente para `getUpdates`, mantendo retry curto para os demais métodos.
3. Substituir o logger descartado do telego por um logger `slog` que redige tokens e expõe erros de polling.
4. Redigir tokens também nos erros de transporte retornados pelo caller.
5. Melhorar o startup do webhook: retry limitado, log da URL sanitizada e leitura de `getWebhookInfo`.
6. Adicionar monitor periódico de problemas de entrega do webhook.
7. Cobrir com testes: timeout de polling, 429 acima de 5s, redação de token, retry do webhook e 503 por fila indisponível.
8. Atualizar a documentação com a recomendação de webhook no Coolify, instância única e deploy sem rolling update.
9. Registrar decisão e memória ao final.

## Passos detalhados
1. Em `client.go`, definir constantes de timeout de long polling e HTTP, e um helper de espera cancelável.
2. Alterar `NewBot` para usar `http.Client{Timeout: 45s}` e `telego.WithLogger` com logger sanitizado.
3. Reescrever `SafeAPICaller.Call` com loop de tentativas, retry integral de `getUpdates` e redação de erros de rede.
4. Em `bot.go`, usar `GetUpdatesParams` com `Timeout: 30`, `Limit: 100` e retry de long polling explícito.
5. Criar `registerWebhook` com até 3 tentativas e log de `getWebhookInfo`; usar em `runWebhook` e `runSharedWebhook`.
6. Adicionar monitor periódico que loga advertência quando houver updates pendentes ou erro recente de entrega.
7. Atualizar o mock de testes para capturar `GetUpdatesParams`, erros sequenciais de `SetWebhook` e chamadas de status.
8. Adicionar/ajustar testes em `client_test.go`, `transport_test.go` e um teste de polling.
9. Atualizar `docs/v2-telegram.md` com:
   - webhook como recomendação de produção;
   - `TELEGRAM_MODE=webhook` e `WEBHOOK_URL`;
   - Coolify com container name consistente / sem rolling update;
   - uma única instância;
   - deploy perde partidas em memória;
   - timeouts e tratamento de 429.
10. Rodar formatação, testes, race, vet, build e `git diff --check`.
11. Atualizar `.agent/memory/memory.md` e `.agent/decisions.md`; mover este plano para `done`.

## Riscos
- O proxy do Coolify pode não encaminhar o header de secret ou o POST; validar com `getWebhookInfo` e teste manual.
- Retry de webhook com erro permanente atrasa o startup em até 3s.
- O monitor periódico faz uma chamada extra a `getWebhookInfo` a cada 5 minutos.
- O estado continua em memória; deploy perde partidas em andamento.
- Se o Coolify continuar com rolling update, webhook pode dividir updates entre containers.
- O token antigo em arquivos históricos de `.agent` deve continuar revogado; não será reimpresso nem alterado.

## Impactos esperados
- Polling deixa de fazer short polling e de sofrer 429 por uso incorreto de timeout.
- Erros de polling passam a ser visíveis e sanitizados.
- Webhook fica observável e com retry de registro.
- Documentação operacional passa a refletir o requisito de instância única.
- Nenhuma migração de banco ou mudança de contrato de API.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD
- Coolify
- PostgreSQL existente
- Sem mudanças em schema

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

### Execução
```bash
go run ./cmd/bot
```

Para webhook local, usar um túnel HTTPS e:
```env
TELEGRAM_MODE=webhook
WEBHOOK_URL=https://<tunel>/telegram
```

Validar em produção:
- log `connected to telegram bot`;
- log `webhook configured` com URL sanitizada;
- `getWebhookInfo` sem `last_error_message`;
- comandos, inline e callbacks;
- parar o container por 20–30s, enviar mensagem, subir e confirmar reenvio;
- deploy com container antigo parado antes do novo.

## Rollback
Reverter os arquivos alterados ou voltar `TELEGRAM_MODE=polling` com o polling corrigido. O webhook remoto é reaplicado no próximo startup e o polling remove webhooks existentes com `drop_pending_updates=false`.

## Observações
- A aprovação do usuário foi dada antes do registro deste plano: “então pode implementar”.
- Não será feito commit nem push sem novo pedido.
- O token vazado já foi revogado; nenhum token será incluído em logs, testes ou documentação.

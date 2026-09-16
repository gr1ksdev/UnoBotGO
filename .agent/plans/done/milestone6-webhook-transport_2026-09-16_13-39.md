# Plano: milestone6-webhook-transport

## Pedido do usuário
Adicionar transporte Telegram Webhook alternativo ao polling na branch dev.

## Objetivo
Compartilhar o pipeline de updates, configurar webhook com secret, deduplicação em memória, healthz e shutdown correto sem alterar gameplay.

## Estratégia
Adicionar configuração, métodos SetWebhook/DeleteWebhook ao BotAPI, servidor net/http limitado e seguro, roteamento comum para dispatcher, deduplicação por UpdateID e testes de transporte.

## Validação
`go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`, `git diff --check`.

# UnoBotGO

Bot de UNO em Go para o Telegram utilizando modo inline com stickers para visualização e seleção de cartas.

---

## Versões do Projeto

### UnoBotGO V2 (Milestone 3 — Playable Telegram MVP)
- **Executável**: `cmd/bot/main.go`
- **Arquitetura**:
  - Engine desacoplada (`internal/uno`): regras clássicas, atomicidade, revision estrita e política de colocações.
  - Serviço de aplicação (`internal/game`): autorização de atores, isolamento de partidas, locking fino e views públicas/privadas.
  - Telegram Adapter (`internal/telegram`): particionamento de workers por `ChatID`, workers dedicados para inline queries, anti-cheat com tokens de uso único de 128 bits e serialização explícita de `cache_time:0`.
- **Como executar o V2**:
  ```bash
  cp .env.example .env
  # Configure TOKEN no .env
  go run ./cmd/bot
  ```
  Ou via Docker:
  ```bash
  docker build -f Dockerfile.v2 -t unobotgo:v2 .
  docker run --rm --env-file .env unobotgo:v2
  ```

  A imagem oficial da `main` é publicada em
  `ghcr.io/gr1ksdev/unobotgo:latest` e também recebe uma tag imutável
  `sha-<commit>`. O servidor precisa apenas de Docker e das variáveis de
  ambiente descritas em `.env.example`.

---

## Comandos do Bot (V2)

| Comando | Descrição |
|---|---|
| `/novo` | Cria um lobby de partida no grupo (o criador é o responsável administrativo). |
| `/entrar` | Inscreve o usuário na partida aberta ou em andamento. |
| `/iniciar` | Inicia a partida (autorizado apenas para o responsável). |
| `/cancelar` (ou `/kill`) | Cancela a partida (autorizado apenas para o responsável). |
| `/sair` | Sai da partida em andamento (transfere responsabilidade se necessário). |
| `/estado` | Exibe o estado público da partida ativa ou lobby. |
| `/ajuda` | Instruções de como jogar. |

---

## Testes Automatizados

O projeto conta com uma suíte abrangente de testes unitários e de integração com cobertura de concorrência e race detector:

```bash
# Executar todos os testes
go test ./...

# Executar com race detector
go test -race ./...

# Verificar formatação e vet
go vet ./...
gofmt -l .
```

## Build e distribuição

Commits e pull requests em `dev` executam testes, race detector, vet, build e
uma construção da imagem sem publicar. A publicação ocorre somente depois da
promoção da árvore pública para `main`, seguindo o processo descrito em
`docs/branching.md`. O workflow da `main` publica a imagem no GHCR após as
validações.

---

## Documentação Técnica
- [Regras da Engine V2](docs/v2-rules.md)
- [Camada de Aplicação V2](docs/v2-application.md)
- [Adapter Telegram V2 e Roteiro de Aceite](docs/v2-telegram.md)

### Transporte Telegram

Long polling é o padrão. Para webhook, use `TELEGRAM_MODE=webhook`, `WEBHOOK_URL`, `WEBHOOK_SECRET` e `WEBHOOK_LISTEN_ADDR=:8080`; publique o endpoint HTTPS por um proxy externo. `WEBHOOK_DROP_PENDING_UPDATES=false` preserva updates pendentes.

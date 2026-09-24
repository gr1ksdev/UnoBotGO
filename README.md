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

### UnoBotGO V1 (Legado)
- **Executável**: `main.go` (na raiz do repositório)
- Mantido intacto para fins de compatibilidade e histórico.

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
| `/reset` | Recupera o grupo, cancela trabalhos pendentes e apaga a partida e o histórico daquele grupo (responsável ou administrador). |
| `/ajuda` | Instruções de como jogar. |

O `/reset` usa uma fila de recuperação separada. Assim, ele continua disponível
mesmo quando a fila normal do grupo está cheia ou uma operação anterior ficou
presa. Depois da autorização, o bot invalida o trabalho antigo daquele grupo,
remove seu estado e seus tokens e abre uma fila limpa para novos comandos. O
comando não afeta partidas de outros grupos.

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

## Simulador local de partidas

O simulador executa bots diretamente sobre a engine V2. Ele não precisa de
Telegram, banco de dados nem variáveis do `.env`. No modo interativo, informe de
2 a 10 jogadores e escolha `classico`/`1` ou `caseiro`/`2`:

```bash
make simulator
# ou
go run ./cmd/simulator
```

Também é possível automatizar e reproduzir uma partida pela semente:

```bash
go run ./cmd/simulator --players 2 --mode classico --seed 20260923
go run ./cmd/simulator --players 4 --mode caseiro --seed 20260924 --quiet
```

Cada execução mostra a semente utilizada e grava um relatório Markdown em
`.reports/simulations/`. O relatório reúne colocações, estatísticas gerais e por
jogador, possíveis erros e uma linha do tempo que explica bloqueios, reversões,
coringas, +2, +4, empilhamentos, penalidades e desafios de blefe ocorridos. Ele
também informa início, fim e tempo total, além do histórico completo de todas as
ações, eventos da engine e estado da mesa depois de cada jogada.

Use `--max-actions` para alterar o limite defensivo de jogadas e `--output` para
escolher outro caminho para o relatório.

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
- [Auditoria Histórica do V1](docs/v2-audit.md)

### Transporte Telegram

Long polling é o padrão. Para webhook, use `TELEGRAM_MODE=webhook`, `WEBHOOK_URL`, `WEBHOOK_SECRET` e `WEBHOOK_LISTEN_ADDR=:8080`; publique o endpoint HTTPS por um proxy externo. `WEBHOOK_DROP_PENDING_UPDATES=false` preserva updates pendentes.

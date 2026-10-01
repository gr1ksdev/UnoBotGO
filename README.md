# UnoBotGO

Bot de UNO em Go para o Telegram utilizando modo inline com stickers para visualização e seleção de cartas.

---

## Versões do Projeto

### UnoBotGO V2
- **Executável**: `cmd/bot/main.go`
- **Arquitetura**:
  - Engine desacoplada (`internal/uno`): regras clássicas, atomicidade, revision estrita e política de colocações.
  - Serviço de aplicação (`internal/game`): autorização de atores, isolamento de partidas, locking fino e views públicas/privadas.
  - Telegram Adapter (`internal/telegram`): particionamento de workers por `ChatID`, workers dedicados para inline queries, anti-cheat com tokens de uso único de 128 bits e serialização explícita de `cache_time:0`.
- **Como executar o V2**:
  ```bash
  cp .env.example .env
  # Configure TOKEN, DATABASE_URL e MINIAPP_SECRET no .env
  make build
  make run
  ```
  Ou em modo desenvolvimento com Mini App Vite sincronizado:
  ```bash
  make dev
  ```
  Ou via Docker:
  ```bash
  docker build -f Dockerfile.v2 -t unobotgo:v2 .
  docker run --rm -p 8080:8080 --env-file .env unobotgo:v2
  ```
  As migrações do PostgreSQL são verificadas e aplicadas automaticamente na inicialização com advisory lock transacional fail-closed.


  A imagem oficial da `main` é publicada em
  `ghcr.io/gr1ksdev/unobotgo:latest` e também recebe uma tag imutável
  `sha-<commit>`. Ambas são manifestos multi-arquitetura para `linux/amd64` e
  `linux/arm64`; o Docker seleciona automaticamente a variante do servidor. O
  servidor precisa apenas de Docker e das variáveis descritas em `.env.example`.

### UnoBotGO V1 (Legado)
- **Executável**: `main.go` (na raiz do repositório)
- Mantido intacto para fins de compatibilidade e histórico.

---

## Comandos do Bot (V2)

No chat privado, `/start` apresenta o bot e oferece um botão para adicioná-lo a
um grupo. `/help` mostra a lista completa de comandos. O menu do Telegram é
separado por contexto: o privado exibe apenas `/start` e `/help`, enquanto os
grupos exibem os comandos de partida.

| Comando | Descrição |
|---|---|
| `/start` | Mostra a apresentação e o botão para adicionar o bot a um grupo. |
| `/help` (ou `/ajuda`) | Exibe os comandos e as instruções de uso. |
| `/novo` | Cria um lobby de partida no grupo (o criador é o responsável administrativo). |
| `/entrar` | Inscreve o usuário na partida aberta ou em andamento. |
| `/trancar` | Impede novos jogadores de entrar; somente o responsável, no lobby ou durante a partida. |
| `/destrancar` | Permite novas entradas; somente o responsável. |
| `/iniciar` | Inicia a partida quando há pelo menos dois jogadores inscritos. |
| `/cancelar` (ou `/kill`) | Cancela a partida (autorizado apenas para o responsável). |
| `/sair` | Sai da partida em andamento (transfere responsabilidade se necessário). |
| `/estado` | Exibe o estado público da partida ativa ou lobby. |
| `/ranking` | Consulta o ranking histórico acumulado deste grupo; disponível a qualquer membro. |
| `/reset` | Recupera o grupo, cancela trabalhos pendentes e apaga a partida e o histórico daquele grupo (responsável ou administrador). |
| `/config` | Configura o modo padrão (Clássico/Caseiro) e sistema de ranking (Legado/Atualizado) do grupo (admin ou instalador). |

O `/reset` usa uma fila de recuperação separada. Assim, ele continua disponível
mesmo quando a fila normal do grupo está cheia ou uma operação anterior ficou
presa. Depois da autorização, o bot invalida o trabalho antigo daquele grupo,
remove seu estado e seus tokens e abre uma fila limpa para novos comandos. O
comando não afeta partidas de outros grupos.

Ao terminar uma partida pontuada, após o commit no PostgreSQL, o bot envia duas
mensagens separadas: **🏁 Partida encerrada**, com colocação e pontos ganhos naquela
partida, e **🏆 Ranking do grupo**, com os totais históricos atualizados. `/ranking`
mostra esse mesmo ranking, incluindo jogadores que não participaram da última partida.
Legacy usa `1 pt`, `2 pts`, `0 pts`; Updated usa centésimos, como `8,57 pts`.
As medalhas são somente `🥇`, `🥈`, `🥉`; depois vêm `4.`, `5.` etc.
No ranking acumulado, posições são únicas: scores iguais são ordenados pela melhor
colocação na última partida elegível de cada jogador, depois pela conclusão mais
recente e, por estabilidade técnica, pelo UserID crescente. Abandono definitivo
aparece como `Nome · fora do ranking` no resultado, sem posição nem pontos elegíveis.
Rankings extensos exibem as linhas que cabem e a quantidade de jogadores restantes.
Sem partidas pontuadas, o bot informa isso; no privado, orienta consultar em um grupo.

Esta UX está na `dev`, aguardando homologação manual antes de publicação.

---

## Trocar cartas no modo caseiro

Selecione **Caseiro** no lobby para jogar com uma carta extra **🔀 Trocar cartas**
(109 cartas no total). Ao jogá-la, abra **Suas cartas** e escolha outro participante
ou **Manter minha mão**. Depois, abra novamente e escolha a cor. A troca, quando
escolhida, só acontece nessa última etapa e inclui as mãos inteiras após o descarte.
Se for sua última carta, você termina normalmente, sem troca nem escolha de cor.
A carta não pode ser jogada sobre coringa nem responder a uma penalidade +2/+4.
O modo clássico mantém suas 108 cartas. O novo fluxo aguarda homologação no Telegram.

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
coringas, trocas de mãos, +2, +4, empilhamentos, penalidades e desafios de blefe ocorridos. Ele
também informa início, fim e tempo total, além do histórico completo de todas as
ações, eventos da engine e estado da mesa depois de cada jogada.

Use `--max-actions` para alterar o limite defensivo de jogadas e `--output` para
escolher outro caminho para o relatório.

## Build e distribuição

Commits e pull requests em `dev` executam testes, race detector, vet, build e
uma construção AMD64/ARM64 da imagem sem publicar. A publicação ocorre somente
depois da promoção da árvore pública para `main`, seguindo o processo descrito
em `docs/branching.md`. O workflow da `main` publica no GHCR um manifesto com as
duas arquiteturas após as validações.

---

## Documentação Técnica
- [Estado atual do projeto: implementação, validação e publicação](docs/project-status.md)
- [Regras da Engine V2](docs/v2-rules.md)
- [Camada de Aplicação V2](docs/v2-application.md)
- [Adapter Telegram V2 e Roteiro de Aceite](docs/v2-telegram.md)
- [Auditoria Histórica do V1](docs/v2-audit.md)

### Transporte Telegram

Long polling é o padrão e o modo recomendado. Webhook permanece experimental, sem homologação real aprovada; consulte o [estado do projeto](docs/project-status.md#transportes-e-evidência-real). Para webhook, use `TELEGRAM_MODE=webhook` e `WEBHOOK_URL=https://bot.exemplo.com/telegram`; publique esse endpoint HTTPS por um proxy externo encaminhando ao servidor compartilhado (`WEB_ADDR`, padrão `:8080`). A URL pública é necessária para `setWebhook` e não pode ser deduzida do endereço local. O segredo de autenticação é derivado internamente; updates pendentes são sempre preservados.

### Configuração V2

O [.env.example](.env.example) contém somente as configurações normais de instalação:

| Variável | Finalidade / padrão |
|---|---|
| `TOKEN` | Token obrigatório do bot Telegram |
| `TELEGRAM_MODE` | `polling` (padrão) ou `webhook` |
| `DATABASE_URL` | Conexão PostgreSQL obrigatória |
| `TURN_TIMEOUT` | Duration positiva; padrão `2m` |
| `WEB_ADDR` | Endereço HTTP compartilhado; padrão `:8080` |
| `MINIAPP_SECRET` | Exatamente 32 bytes em Base64; gere com `openssl rand -base64 32` |

Somente webhook exige também `WEBHOOK_URL`, com HTTPS e caminho dedicado fora de `/api`, `/assets`, `/healthz` e `/readyz`. O segredo enviado ao Telegram é `Base64URL-sem-padding(HMAC-SHA256(MINIAPP_SECRET decodificado, "unobotgo/v2/telegram-webhook-secret/v1"))`. O contexto separa esse protocolo das referências AES-GCM; a chave original não é enviada. A rotação de `MINIAPP_SECRET` atualiza esse segredo no próximo startup, que reaplica `setWebhook`. Referências antigas também deixam de valer com a rotação, como antes.

O botão **🌐 Ranking Global** usa automaticamente `https://t.me/<username-do-bot>/ranking`, montado a partir do `getMe` já realizado no startup. Configure o Direct Mini App com short name **ranking** e sua URL externa HTTPS no BotFather; essa URL não é configuração do backend.

Políticas internas fixas: logs `info`, histórico 100, tokens inline com TTL `2m`, limite global 20.000 e por usuário 512, validade de initData `1h`, timeout das migrations de startup `2m` e descarte de updates pendentes desativado. Opções antigas não são mais lidas; não há aliases ou fallback de configuração.

### PostgreSQL e Ranking V2

O runtime V2 exige PostgreSQL configurado via `DATABASE_URL`. Com o `.env` configurado, basta iniciar o processo principal:

```sh
go run ./cmd/bot
# Em produção: ./bin/unobotgo
```

As migrations SQL versionadas são embutidas no binário e verificadas/aplicadas automaticamente antes de HTTP, Telegram e workers. A conexão tem prazo de 10 segundos; migrations e verificação têm timeout interno total de 2 minutos. Um advisory lock transacional serializa instâncias, e todo o lote pendente e seu ledger são confirmados em uma única transação. Migrations aplicadas não devem ser editadas: checksum divergente, versão desconhecida, timeout ou erro SQL impedem o startup, sem corrigir o ledger ou executar downgrade. Bancos vazios/parciais são atualizados; bancos já atualizados apenas são verificados. Não há comando manual ou segundo binário obrigatório.

**Configuração e Ranking:**
- Cada grupo possui configuração própria criada sob demanda com os padrões **Clássico** e **Atualizado**.
- O comando `/config` permite que administradores e o usuário que adicionou o bot configurem o modo padrão (`Clássico` / `Caseiro`) e o sistema de ranking (`Legado` / `Atualizado`) através de botões inline interativos.
- Partidas iniciadas usam o snapshot de configuração capturado na criação; alterações posteriores afetam apenas as partidas futuras.
- Ao final de cada partida pontuável (mínimo de 2 participantes elegíveis), o bot persiste o resultado de forma atômica e exibe uma mensagem dedicada anunciando os pontos distribuídos.
- Troca de sistema de ranking em grupos que já acumularam histórico é bloqueada para preservar a integridade das pontuações.

Testes reais de PostgreSQL usam uma base exclusiva de testes e schemas temporários isolados:

```sh
TEST_DATABASE_URL='postgres://postgres:senha@localhost:5432/unobot_test?sslmode=disable' go test -race -tags integration ./internal/storage/postgres/...
```

Documentação de persistência e homologação: [M7](docs/m7-persistence.md).

Em grupos/supergrupos, todo comando deve mencionar o username do bot: `/novo@SeuBot`, `/entrar@SeuBot` ou seu alias `/join@SeuBot`, `/config@SeuBot`, etc. Comandos sem sufixo ou destinados a outro bot são ignorados. No privado, `/start` e `/help` continuam sem sufixo. Grupos já configurados preservam seu sistema de ranking; a migration 0008 altera somente o default para grupos novos.

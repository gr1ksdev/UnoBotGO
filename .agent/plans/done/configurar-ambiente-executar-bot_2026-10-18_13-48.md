# Plano: configurar-ambiente-executar-bot

## Pedido do usuário
O usuário forneceu o token do bot `8691500434:AAGhVsYfhqeqVTrYcTIvn6_sNPYSdrkvvlE` e solicitou que o projeto seja configurado e mantido em execução ("use esse token de bot e deixe o projeto rodando").

## Objetivo
Configurar o ambiente operacional local (variáveis de ambiente `.env`, infraestrutura PostgreSQL via Docker e geração de secret do Mini App) e inicializar o binário do UnoBotGO em segundo plano com polling ativo, garantindo conectividade e atendimento às atualizações do Telegram.

## Contexto atual
- As correções de resiliência e estorno de tokens inline no `Dispatcher` já foram testadas e validadas com sucesso.
- O contêiner PostgreSQL (`unobotgo-db-1`) está ativo e saudável via Docker Compose na porta padrão 5432.
- O frontend web foi compilado com sucesso em `web/dist` e o binário `bin/unobotgo` está construído com os assets embutidos.
- O arquivo `.env` foi criado no repositório (ignorado pelo `.gitignore`).
- As variáveis obrigatórias para o runtime são:
  - `TOKEN`: token fornecido pelo usuário (`8691500434:AAGhVsYfhqeqVTrYcTIvn6_sNPYSdrkvvlE`).
  - `TELEGRAM_MODE`: `polling`.
  - `DATABASE_URL`: `postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable`.
  - `TURN_TIMEOUT`: `2m`.
  - `WEB_ADDR`: `:8080`.
  - `MINIAPP_SECRET`: chave aleatória de 32 bytes em Base64.

## Arquivos analisados
- `.env.example`
- `.gitignore`
- `docker-compose.yml`
- `Makefile`
- `cmd/bot/main.go`
- `internal/config/config.go`
- `internal/app/app.go`

## Arquivos modificados
- `.env` (novo arquivo, ignorado pelo git)

## Estratégia de implementação
1. **Criação do arquivo `.env`:**
   - Preencher com as configurações validadas pelo `internal/config`:
     - `TOKEN=8691500434:AAGhVsYfhqeqVTrYcTIvn6_sNPYSdrkvvlE`
     - `TELEGRAM_MODE=polling`
     - `DATABASE_URL=postgres://unobot:unobot@localhost:5432/unobot?sslmode=disable`
     - `TURN_TIMEOUT=2m`
     - `WEB_ADDR=:8080`
     - `MINIAPP_SECRET=Os5QJq/V8EJTVNjfUat7BCQTuDT/+IHSNLRa5yEa+SM=`
2. **Execução do processo do bot em segundo plano:**
   - Iniciar o binário `./bin/unobotgo` como processo de suporte/daemon contínuo.
   - Monitorar os logs de inicialização confirmando:
     - Conexão e aplicação/verificação de migrations no PostgreSQL.
     - Inicialização do servidor HTTP unificado na porta 8080.
     - Registro dos comandos do bot no Telegram e início do polling (`UpdatesViaLongPolling`).
3. **Validação da saúde do processo:**
   - Checar endpoints de liveness/readiness (`http://localhost:8080/healthz` e `http://localhost:8080/readyz`).
   - Confirmar que o bot está consumindo updates sem erros.

## Passos detalhados
1. Criar `.env` com as variáveis necessárias.
2. Executar `./bin/unobotgo` em background (processo daemon).
3. Testar `GET http://localhost:8080/healthz` e `readyz` com `curl`.
4. Verificar logs de startup no terminal confirmando `database connected`, `migrations completed` e polling iniciado.

## Riscos
- **Conflito de porta HTTP (8080):** Caso a porta 8080 esteja em uso por outro serviço.
  - *Mitigação:* Portas foram previamente verificadas via `lsof` e a porta 8080 está livre.
- **Queda do PostgreSQL:** Se o Docker for encerrado.
  - *Mitigação:* Contêiner `unobotgo-db-1` está com política `restart: always` e status saudável.

## Impactos esperados
- O UnoBotGO ficará online no Telegram respondendo a mensagens, comandos e jogadas inline imediatamente com as correções de resiliência ativas.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD

## Como testar

### Build
```bash
go build -o bin/unobotgo ./cmd/bot
```

### Testes
```bash
curl -I http://localhost:8080/healthz
curl -I http://localhost:8080/readyz
```

### Execução
```bash
./bin/unobotgo
```

## Rollback
Parar o processo do bot e remover o arquivo `.env`.

## Observações
O arquivo `.env` permanece no `.gitignore` e não será versionado no Git.

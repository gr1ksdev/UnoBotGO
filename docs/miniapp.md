# Mini App de Ranking Global do UnoBotGO

Documentação técnica do Mini App de Ranking Global integrado ao UnoBotGO V2.

## 1. Visão Geral e Arquitetura

O Mini App de Ranking Global oferece uma interface visual responsiva e interativa no Telegram para visualização das pontuações do mês vigente (`America/Sao_Paulo`).

- **Executável Único**: O frontend compilado em `web/dist` é embutido no binário Go (`bin/unobotgo`) via `go:embed`. Não há runtime Node.js em produção.
- **Origem Única**: Servidor HTTP nativo em Go expõe tanto as rotas da API (`/api/v1/...`), quanto os arquivos estáticos e o webhook do bot.
- **Startup e Migrações**: No boot, o coordenador `internal/app` adquire advisory lock transacional no PostgreSQL, verifica e aplica migrações pendentes. Se houver falha, encerra imediatamente (fail-closed) sem abrir portas de rede ou iniciar o dispatcher do Telegram.

## 2. Segurança e Autenticação

### Validação de initData (HMAC-SHA256)
- O frontend envia os dados de inicialização recebidos do Telegram no cabeçalho `Authorization: tma <initData>`.
- O backend decodifica os pares de chave-valor em formato URL-encoded, verifica chaves duplicadas, deriva a chave HMAC via `WebAppData` com o `TOKEN` do bot e calcula o hash conforme a especificação oficial do Telegram.
- Comparações de hash em tempo constante (`subtle.ConstantTimeCompare`).
- Validação temporal estrita: `auth_date` com expiração fixa de 1h e tolerância a desvios de relógio futuro de até 30s.

### Referências Opacas e Máscaras de IDs
- O backend **nunca** expõe IDs numéricos reais do Telegram na API JSON ou nas URLs de fotos.
- Chaves estáveis para React (`key`) são derivadas via HMAC com segredo.
- Referências (`group_ref`, cursores e referências de avatar) são cifradas e autenticadas via **AES-GCM** de 256 bits com escopo, tipo, mês e timestamp de expiração (derivadas de `MINIAPP_SECRET`).
- Na interface visual, os identificadores são exibidos no padrão mascarado: `ID ••••1234`.

## 3. Endpoints da API

| Método | Caminho | Descrição |
|---|---|---|
| `GET` | `/api/v1/rankings/groups` | Lista grupos ordenados no mês (`system=updated` ou `legacy`), paginado por keyset |
| `GET` | `/api/v1/rankings/players` | Lista jogadores agregados em todos os grupos no mês vigente |
| `GET` | `/api/v1/rankings/groups/{group_ref}` | Detalhes do grupo selecionado e ranking interno de seus participantes |
| `GET` | `/api/v1/media/{avatar_ref}` | Proxy autenticado de foto de perfil/grupo (retorna 200 com imagem, 202 se em processamento ou 204 se ausente) |
| `GET` | `/healthz` | Probe de liveness |
| `GET` | `/readyz` | Probe de readiness (retorna 503 até que persistência e bot estejam operacionais) |

## 4. Precisão Numérica de Pontuação

- No backend, a pontuação é mantida em centésimos inteiros (`score_units` como `bigint` / `int64`).
- No JSON da API, `score_units` é transmitido como **string decimal** (ex: `"284000"`).
- No frontend TypeScript, a string é convertida para `BigInt` nativo, dividida e formatada com separadores de milhar (`pt-BR`), garantindo precisão matemática exata mesmo para pontuações superiores a $2^{53}$.

## 5. Cache e Proxy de Fotos (Media)

- Cache LRU em memória limitado a 2.000 entradas e 64 MiB.
- Downloads realizados em background por workers dedicados com taxa limitada (ticker de 350ms).
- Validação de MIME raster permitida (`image/jpeg`, `image/png`, `image/webp`) e limite de 2 MiB por foto.
- Redação estrita de tokens e URLs do Telegram em logs e mensagens de erro.


## 6. Configuração e Direct Mini App

Configure `TOKEN`, `DATABASE_URL` e `MINIAPP_SECRET` (exatamente 32 bytes em Base64: `openssl rand -base64 32`). `TELEGRAM_MODE`, `TURN_TIMEOUT` e `WEB_ADDR` têm padrões `polling`, `2m` e `:8080`.

O startup usa o username retornado pelo único `getMe` para construir `https://t.me/<username>/ranking`. O short name `ranking` é fixo. Cadastre o Direct Mini App e sua URL HTTPS externa no BotFather; o backend não precisa duplicar essa URL.

Se usar webhook, a configuração adicional `WEBHOOK_URL` indica seu endpoint público HTTPS com caminho dedicado (por exemplo `/telegram`). A autenticação por header é preservada com um segredo derivado por HMAC-SHA256 e contexto `unobotgo/v2/telegram-webhook-secret/v1`, codificado em Base64URL sem padding. O master secret nunca é enviado ao Telegram. A rotação do master instala o novo segredo via `setWebhook` no startup; updates pendentes não são descartados.

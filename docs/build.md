# Build e distribuição

## Requisitos de compilação

- **Go**: 1.26+
- **PostgreSQL**: 17+

A v3 não tem frontend: o binário é Go puro, sem Node no build nem na imagem.

## Build

```bash
go build -trimpath -o bin/unobotgo ./cmd/bot
```

O executável é standalone e inclui as migrations SQL versionadas.

Ao iniciar `./bin/unobotgo`, o processo conecta PostgreSQL e aplica as
pendências antes do bot. O ledger e os checksums são verificados, com advisory
lock entre instâncias e timeout interno de 2 minutos. Migrations já aplicadas
não devem ser editadas; falhas impedem startup. Não há etapa manual ou segundo
binário de migration.

## Testes e validação de qualidade

```bash
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

A integração com PostgreSQL real exige uma base de testes e a tag de
integração:

```bash
TEST_DATABASE_URL='postgres://postgres:senha@localhost:5432/unobot_test?sslmode=disable' \
  go test -race -tags integration ./internal/storage/postgres/...
```

## Container local

```bash
docker build -f Dockerfile.v2 -t unobotgo:v3 .
docker run --rm -p 8080:8080 --env-file .env unobotgo:v3
```

O build do container é multi-stage: o estágio Go compila o binário standalone e
a imagem final é baseada em distroless não root, sem conter código-fonte.

## CI e GHCR

O workflow `v3-ci` valida o código (testes, race detector, vet, build e imagem
multiarch) sem publicar. O workflow da `main` repete as validações e, somente
em push na `main`, publica a imagem no GitHub Container Registry (GHCR).

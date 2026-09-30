# Build e distribuição

## Requisitos de compilação

- **Go**: 1.26+
- **Node.js**: 24 LTS e npm (necessários apenas para compilação do frontend/desenvolvimento)
- **PostgreSQL**: 17+

## Build unificado

O Mini App React/TypeScript/Tailwind é compilado em `web/dist` e embutido diretamente no binário Go via `//go:embed`:

```bash
make build
```

O comando gera o executável standalone `bin/unobotgo`.

## Desenvolvimento

Para executar simultaneamente o backend em modo dev e o servidor de desenvolvimento do Vite com proxy:

```bash
make dev
```

Ou isoladamente:
- Backend: `make dev-back`
- Frontend: `make dev-web`

## Testes e validação de qualidade

```bash
make test        # Testes Go e Vitest frontend
make vet         # go vet
make check       # Validação completa (requer TEST_DATABASE_URL)
git diff --check
```

## Container local

```bash
docker build -f Dockerfile.v2 -t unobotgo:v2 .
docker run --rm -p 8080:8080 --env-file .env unobotgo:v2
```

O build do container é multi-stage: o estágio Node24 gera os assets estáticos, o estágio Go compila o binário standalone e a imagem final é baseada em distroless não root, sem conter Node, npm ou código-fonte.

## CI e GHCR

O CI de `dev` valida o código (frontend e Go) e constrói a imagem multiarch sem publicação. O workflow da `main` repete as validações e, somente em push na `main`, publica a imagem no GitHub Container Registry (GHCR).

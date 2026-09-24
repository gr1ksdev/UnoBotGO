# Build e distribuição

## Testes locais

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./...
git diff --check
```

## Container local

```bash
docker build -f Dockerfile.v2 -t unobotgo:v2 .
docker run --rm --env-file .env unobotgo:v2
```

Para validar as duas arquiteturas com Buildx:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --file Dockerfile.v2 \
  .
```

`TOKEN` é obrigatório. As demais variáveis V2 e seus valores padrão estão em
`.env.example`. Nenhum token deve ser commitado.

## CI e GHCR

O CI de `dev` valida o código e constrói a imagem sem publicação. O workflow da
`main` repete as validações e, somente em push na `main`, publica:

```text
ghcr.io/gr1ksdev/unobotgo:latest
ghcr.io/gr1ksdev/unobotgo:sha-<commit>
```

Cada tag é um manifesto OCI com variantes `linux/amd64` e `linux/arm64`. O
runtime Docker seleciona automaticamente a variante compatível com o host.

A autenticação usa o `GITHUB_TOKEN`, com `packages: write` somente no job de
publicação. A promoção entre branches continua seguindo `docs/branching.md`,
sem merge entre os históricos independentes.

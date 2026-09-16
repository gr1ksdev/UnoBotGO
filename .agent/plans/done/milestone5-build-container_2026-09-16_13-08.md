# Plano: milestone5-build-container

## Pedido do usuário
Criar CI para dev/main, build Docker V2 e publicação futura no GHCR, sem alterar main.

## Estratégia
Validar Go e a imagem sem publicação em dev. Em main, validar primeiro e publicar apenas o job GHCR com GITHUB_TOKEN e tags latest/SHA. Usar runtime distroless não-root.

## Restrições
Sem gameplay, promoção, merge, push de imagem ou alterações em Makefile.

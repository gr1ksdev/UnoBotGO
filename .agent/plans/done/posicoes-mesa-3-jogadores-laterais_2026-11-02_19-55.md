# Plano: posicoes-mesa-3-jogadores-laterais

## Pedido do usuário
Ajustar o formato das posições da mesa de jogo no WebApp: quando houver 3 jogadores na partida (você + 2 oponentes), ao invés de exibir oponentes no topo, posicionar 1 jogador em cada lado (um na esquerda e outro na direita, com o jogador local embaixo).

## Objetivo
1. Em partidas com 3 jogadores no total (2 oponentes + você):
   - Oponente 1: posicionado à esquerda (`seat-left`).
   - Oponente 2: posicionado à direita (`seat-right`).
   - Jogador local: posicionado embaixo (`local-player`).
   - Topo livre, garantindo distribuição simétrica "um em cada lado" ao redor da mesa central.
2. Em partidas com 2 jogadores no total (1 oponente + você):
   - Oponente 1: posicionado no topo (`seat-top`), de frente para o jogador local.
3. Em partidas com 4 jogadores no total (3 oponentes + você):
   - Oponente 1: esquerda (`seat-left`).
   - Oponente 2: topo (`seat-top`).
   - Oponente 3: direita (`seat-right`).
   - Jogador local: embaixo (`local-player`).
4. Recompilar o bundle de produção do frontend (`web/dist`) via Vite/Deno para que o binário Go embutido (`go:embed all:dist`) sirva os novos arquivos atualizados aos navegadores/Telegram.

## Contexto atual
- `web/src/pages/Game.tsx` renderizava condicionalmente para 2 oponentes as classes `seat-left` e `seat-top`.
- O bundle em `web/dist` não havia sido recompilado após as alterações recentes de CSS e JSX, fazendo com que o navegador continuasse recebendo o bundle antigo (onde os oponentes ficavam lado a lado no topo por conta do flexbox flex-wrap).
- O backend Go embute `web/dist` via `web/embed.go` (`//go:embed all:dist`).

## Arquivos analisados
- `web/src/pages/Game.tsx`
- `web/src/game-v2.css`
- `web/src/styles.css`
- `web/src/components/scene/TableScene.ts`
- `web/embed.go`

## Arquivos que poderão ser modificados
- `web/src/pages/Game.tsx`
- `web/src/game-v2.css`
- `web/dist/*` (recompilação dos assets para o embed Go)

## Estratégia de implementação
1. Em `web/src/pages/Game.tsx`:
   - Atualizar a renderização para `opponents.length === 2`: mapear `opponents[0]` em `seat-wrapper seat-left` e `opponents[1]` em `seat-wrapper seat-right`.
2. Em `web/src/game-v2.css`:
   - Ajustar as propriedades de largura, dimensões de avatar e quebra de texto de `.seat-left` e `.seat-right` para evitar qualquer sobreposição com a mesa central e manter visual limpo mesmo em telas estreitas de smartphones (com media query para telas < 380px).
3. Executar `deno run -A npm:vite build` dentro do diretório `web/` para atualizar os arquivos em `web/dist/`, garantindo que o servidor HTTP do bot sirva o HTML, CSS e JS corretos.
4. Executar os testes do frontend com `deno task test` e verificação de tipagem com `deno task typecheck`.

## Passos detalhados
1. Editar `web/src/pages/Game.tsx`:
   - No bloco `opponents.length === 2`, alterar o segundo assento de `seat-top` para `seat-right`.
2. Refinar estilos em `web/src/game-v2.css`:
   - Garantir que `.seat-left` e `.seat-right` fiquem centralizados verticalmente em 43-45%, com largura compacta e posicionamento seguro.
3. Compilar o frontend:
   - Rodar `cd web && deno run -A npm:vite build` para atualizar `web/dist`.
4. Executar bateria de testes:
   - `cd web && deno task typecheck`
   - `cd web && deno task test`
   - `go test ./internal/...`

## Riscos
- **Sobreposição em telas estreitas:** Oponentes laterais podem encostar nas cartas de compra/descarte em telas de 320-360px de largura se o avatar/texto for muito largo. Mitigação: limitar a largura do container lateral a ~68px e encurtar o nome com reticências (`text-overflow: ellipsis`).

## Impactos esperados
- Partidas com 3 jogadores terão 1 oponente na esquerda, 1 oponente na direita e o jogador local embaixo.
- Partidas com 4 jogadores terão 1 na esquerda, 1 no topo, 1 na direita e o jogador local embaixo.
- Partidas com 2 jogadores terão 1 no topo e o jogador local embaixo.
- O bundle servido pelo bot via Telegram Mini App refletirá imediatamente as novas posições.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD

## Como testar

### Build
```bash
cd web && deno run -A npm:vite build
go build ./cmd/bot
```

### Testes
```bash
cd web && deno task typecheck
cd web && deno task test
go test -v ./internal/...
```

### Execução
```bash
go run ./cmd/bot --dev
```

## Rollback
Desfazer alterações em `web/src/pages/Game.tsx` e `web/src/game-v2.css` via `git checkout` e restaurar `web/dist`.

## Observações
- A engine PixiJS do `TableScene.ts` descobre a posição dos avatares dinamicamente pelo seletor `[data-player-key]`, logo as animações de cartas se moverão automaticamente para as novas coordenadas laterais.

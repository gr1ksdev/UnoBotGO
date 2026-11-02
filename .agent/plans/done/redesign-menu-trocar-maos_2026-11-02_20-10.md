# Plano: redesign-menu-trocar-maos

## Pedido do usuário
O menu de seleção da carta Trocar mãos no WebApp está "de cabeça pra baixo", além de estar feio e não condizente com a identidade visual do jogo.

## Objetivo
1. Redesenhar completamente o menu de seleção da carta "Trocar mãos" (`ChoosingPlayer` / Phase 4) no WebApp para seguir a identidade visual v2 do UnoBotGO (glassmorphism escuro, tipografia MatchoLexend/MatchoNunito, bordas com destaque cyan/neon, badges e ícones).
2. Transformar a lista de oponentes em cards informativos com:
   - Avatar do jogador (foto ou inicial com gradiente).
   - Nome com corte seguro (`ellipsis`).
   - Quantidade de cartas atual com ícone de cartas (informação estratégica crucial para decidir com quem trocar).
   - Botão de ação "Trocar" estilizado em destaque.
3. Separar a ação de "Manter minhas cartas" com estilo secundário elegante (outline suave) abaixo da lista de oponentes.
4. Corrigir qualquer problema de inversão/orientação ("de cabeça pra baixo"), garantindo `transform: none` explícito e ordem natural de leitura (`flex-direction: column`), sem rotações herdadas de animações GSAP de cor.
5. Recompilar o bundle de produção do frontend (`web/dist`) via Vite/Deno para servir imediatamente no bot.

## Contexto atual
- Em `web/src/pages/Game.tsx`, quando `view.phase === 4`, o componente `Choice` renderiza `.target-sheet` com um `<h2>` genérico e botões HTML simples (`<button key={p.key}>{p.name}</button>`), sem avatar, sem contagem de cartas dos oponentes e sem estilização moderna.
- Em `web/src/game-v2.css`, há pouquíssimo estilo para `.target-sheet` (`background: #19383f; padding: 22px;`), caindo nos estilos legados de `styles.css` (`.target-options button` cinza opaco).
- Animações GSAP no mesmo componente `Choice` manipulam transformações e rotações para a escolha de cores, o que pode causar inconsistência de estado de transform caso o contexto GSAP não isole a fase de alvo.

## Arquivos analisados
- `web/src/pages/Game.tsx`
- `web/src/game-v2.css`
- `web/src/styles.css`
- `web/src/pages/Game.test.tsx`

## Arquivos que poderão ser modificados
- `web/src/pages/Game.tsx`
- `web/src/game-v2.css`
- `web/src/pages/Game.test.tsx`
- `web/dist/*` (bundle compilado de produção)

## Estratégia de implementação
1. **Em `web/src/pages/Game.tsx`**:
   - No bloco `target`, estruturar o modal com:
     - Badge superior com ícone de troca de cartas (`🔀`).
     - Título destacado: `Trocar de Mãos`.
     - Subtítulo explicativo: `Escolha um jogador para trocar todas as cartas ou mantenha sua mão.`
     - Linhas interativas de oponentes (`.target-player-row`) exibindo `PlayerAvatar`, nome do oponente e contagem de cartas (`{p.count} cartas` com `Icon name="cards"`).
     - Botão secundário de "Manter minhas cartas ({me.count})" na base.
   - Isolar animações GSAP para que `target-sheet` receba apenas fade-in e scale de entrada limpos, sem qualquer herança de rotação de cor.
2. **Em `web/src/game-v2.css`**:
   - Estilizar `.v2-choice.target-sheet`:
     - Superfície `#0d1e26f5` com `backdrop-filter: blur(16px)`, borda sutil ciano (`#00bcc444`) e sombra suave.
     - `transform: none !important;` garantindo orientação vertical perfeita em todos os dispositivos.
     - Botões de jogadores estilizados com hover, clique e layout horizontal claro.
     - Botão de "Manter minhas cartas" destacado com estilo outline sutil.
3. **Testes e Build**:
   - Atualizar testes em `web/src/pages/Game.test.tsx` para cobrir os novos elementos do modal de Trocar Mãos.
   - Executar `deno task test` e `deno task typecheck`.
   - Executar `deno run -A npm:vite build` para atualizar `web/dist`.

## Passos detalhados
1. Editar `web/src/pages/Game.tsx`:
   - Atualizar a renderização do bloco `target` com componentes ricos (avatar, cards, contagem).
2. Editar `web/src/game-v2.css`:
   - Adicionar regras completas de estilo para `.target-sheet`, `.target-surface`, `.target-player-row`, etc.
3. Testar componentes e lógica com `vitest`:
   - Rodar `cd web && deno task test`.
4. Recompilar o bundle estático:
   - Rodar `cd web && deno run -A npm:vite build`.
5. Validar compilação do bot:
   - Rodar `go build ./cmd/bot`.

## Riscos
- **Telas pequenas (smartphones compactos):** Se houver 3 oponentes, o modal deve caber sem estourar a altura da tela. Mitigação: usar tamanhos compactos (linhas de ~52px) e `max-height` seguro.

## Impactos esperados
- Menu de "Trocar Mãos" visualmente impressionante, condizente com o padrão do jogo.
- Exibição da quantidade de cartas de cada oponente diretamente no menu, tornando a jogada estratégica e intuitiva.
- Orientação vertical correta e sem visual invertido/feio.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- Telegram WebApp / Navegadores mobile

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
```

## Rollback
Restaurar `web/src/pages/Game.tsx` e `web/src/game-v2.css` via `git checkout` e recompilar `web/dist`.

# Plano: refinar-navegacao-header-telegram

## Pedido do usuário
Refinar a navegação visual do Ranking Global e do detalhe de grupo no Telegram Mini App, eliminando duplicações de navegação no topo (evitar duas formas visuais de voltar no detalhe e remover a seta/botão fechar customizado na tela global). Fazer o primeiro bloco visual do frontend ficar integrado logo abaixo do chrome nativo do Telegram com cores contínuas (azul no ranking global e vermelho no detalhe), centralizar perfeitamente os títulos, respeitar safe areas sem hacks e sem tentar desenhar por cima do chrome nativo do Telegram, preservando o fallback web para ambientes fora do Telegram e todos os elementos já aprovados (cards, avatares, marquee, hero, pontuação, filtros).

## Objetivo
1. Eliminar a seta/botão fechar customizado redundante no topo do Ranking Global.
2. No detalhe de grupo, quando o `BackButton` nativo do Telegram estiver disponível, ocultar a seta customizada do hero/header e controlar o retorno exclusivamente pelo `BackButton` nativo do Telegram.
3. Manter um fallback web discreto de botão de voltar no detalhe apenas quando o `BackButton` nativo do Telegram não estiver disponível (ex: desenvolvimento web/testes fora do Telegram).
4. Centralizar geometricamente os títulos (`Ranking Global · {mês}` e `Ranking do grupo`) usando layout em grade simétrica (3 colunas: esquerda, centro, direita), eliminando o compensador artificial `padding-right: 36px`.
5. Garantir que a integração de cor com o Telegram continue perfeitamente sincronizada (`#073b82` para o Global e `#99121f` para o Detalhe) via `setHeaderColor`.
6. Garantir ciclo de vida e cleanup estritos do `BackButton` para evitar acúmulo de listeners em navegações sucessivas.
7. Atualizar e expandir a suíte de testes cobrindo ausência de controles duplicados, uso do `BackButton` nativo, fallback fora do Telegram e navegações repetidas sem acúmulo de listeners.

## Contexto atual
- `web/src/pages/Rankings.tsx`:
  - Na tela global (`!detail`), renderiza `<button className="back-button" aria-label="Fechar Ranking Global" onClick={() => window.Telegram?.WebApp.close?.()}><Arrow /></button>` redundante ao lado esquerdo do título.
  - No detalhe de grupo (`detail`), renderiza `<button className="back-button" aria-label="Voltar ao Ranking Global" onClick={back}><Arrow /></button>` dentro do hero vermelho, duplicando o `BackButton` nativo do Telegram.
  - No CSS (`web/src/styles.css`), `.detail-header .title-bar h1` possui `padding-right: 36px` como compensação visual da seta esquerda.
- `web/src/lib/telegram.ts`:
  - Já gerencia `setHeaderColor` (`#073b82` no global e `#99121f` no detalhe).
  - Já gerencia `--telegram-top`, `--telegram-bottom`, etc. com base em `safeAreaInset` e `contentSafeAreaInset`.
  - Já gerencia `BackButton.show()`, `BackButton.hide()`, `BackButton.onClick()` e `BackButton.offClick()`.
- API oficial do Telegram Mini Apps:
  - Não oferece método para injetar título arbitrário dentro da barra nativa do cliente Telegram (o cliente Telegram controla os botões nativos e mostra o nome do bot).
  - Portanto, a solução oficial e recomendada pelo usuário é a integração visual: chrome nativo com a mesma cor via `setHeaderColor`, seguido imediatamente pelo header do app com título centralizado e sem controles duplicados.

## Arquivos analisados
- `web/src/App.tsx`
- `web/src/App.test.tsx`
- `web/src/pages/Rankings.tsx`
- `web/src/styles.css`
- `web/src/lib/telegram.ts`
- `web/src/lib/telegram.test.tsx`
- `web/src/components/Ranking.tsx`
- `web/src/components/Ranking.test.tsx`
- `Makefile`

## Arquivos que poderão ser modificados
- `web/src/pages/Rankings.tsx`
- `web/src/styles.css`
- `web/src/lib/telegram.ts` (se necessário refinamento de cleanup)
- `web/src/App.test.tsx`
- `web/src/lib/telegram.test.tsx`
- `.agent/memory/memory.md` (registro de convenções e decisões)

## Estratégia de implementação
1. **Detecção do BackButton nativo**:
   - Criar uma checagem reativa e segura para saber se o Telegram Mini App disponibiliza `BackButton` (`Boolean(window.Telegram?.WebApp?.BackButton)`).
2. **Refatoração do Header em `Rankings.tsx`**:
   - Na tela global:
     - Remover o botão `<button className="back-button">` (sem seta customizada e sem botão "Fechar").
     - O topo conterá apenas o título e o ícone de calendário.
   - Na tela de detalhe:
     - Se `hasNativeBack` for verdadeiro (dentro do Telegram com BackButton): não renderizar nenhuma seta customizada no hero.
     - Se `hasNativeBack` for falso (fora do Telegram / dev fallback): renderizar o botão customizado de voltar na coluna esquerda.
   - Estrutura do `.title-bar`:
     - 3 slots: `.title-bar-left` (1fr), `h1` (auto/centralizado), `.title-bar-right` (1fr).
     - Na tela global: `.title-bar-left` fica vazio (ou reservado para equilíbrio métrico), `h1` no centro e o calendário no `.title-bar-right`.
     - No detalhe: `.title-bar-left` contém o fallback apenas se fora do Telegram, `h1` no centro e `.title-bar-right` fica vazio para manter a simetria perfeita.
3. **Ajustes de Estilo em `styles.css`**:
   - `.title-bar`: usar `display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr); align-items: center; min-height: 44px;`.
   - `.title-bar-left`: `display: flex; align-items: center; justify-content: flex-start; min-width: 0;`.
   - `.title-bar-right`: `display: flex; align-items: center; justify-content: flex-end; min-width: 0;`.
   - `.title-bar h1`: centralizado, `margin: 0`, `white-space: nowrap`, `overflow: hidden`, `text-overflow: ellipsis`.
   - Remover `.detail-header .title-bar h1 { padding-right: 36px; }` para que o título do grupo fique perfeitamente centralizado no hero vermelho.
   - Ajustar tipografia responsiva em 360px e 310px para evitar corte ou overflow em viewports compactos.
4. **Ciclo de vida do BackButton**:
   - Garantir que `BackButton.onClick` e `BackButton.offClick` utilizem a mesma referência e que `BackButton.hide()` seja chamado na saída da rota de detalhe.
   - Verificar ausência de vazamento de callbacks em navegações repetidas (`global -> detalhe -> global -> detalhe -> global`).
5. **Preservação de regras e componentes existentes**:
   - Manter gradientes existentes, avatares, scores, cards UNO decorativos no hero, onda SVG, marquee de nomes longos, BottomNavigation, Segmented switches e TanStack Query cache.
6. **Atualização e novos testes**:
   - Atualizar `App.test.tsx` para refletir que no Telegram o detalhe não possui botão customizado de voltar no DOM, navegando via callback do `BackButton` nativo.
   - Adicionar teste garantindo que o Ranking Global não renderiza botão customizado de voltar/fechar.
   - Adicionar teste para navegação de detalhe fora do Telegram (fallback web).
   - Adicionar teste de navegação repetida garantindo que listeners do `BackButton` não acumulam.

## Passos detalhados
1. Inspecionar `web/src/pages/Rankings.tsx` e implementar a lógica condicional de navegação e layout de 3 colunas do `.title-bar`.
2. Atualizar `web/src/styles.css` com a grade 1fr auto 1fr para `.title-bar`, remover a compensação `padding-right: 36px` do detalhe e garantir responsividade em telas estreitas (320px, 360px, 390px, 430px).
3. Inspecionar `web/src/lib/telegram.ts` e refinar o gerenciamento do `BackButton` para máxima robustez contra re-renders e desmontagens.
4. Atualizar testes em `web/src/App.test.tsx` e `web/src/lib/telegram.test.tsx`.
5. Executar:
   - `npm --prefix web run lint`
   - `npm --prefix web run typecheck`
   - `npm --prefix web run test`
   - `npm --prefix web run build`
   - `git diff --check`
6. Atualizar `.agent/memory/memory.md` com as decisões de integração visual e ciclo de vida de controles nativos do Telegram.

## Riscos
- **Risco**: Quebra de testes existentes que procuravam pelo botão customizado `Voltar ao Ranking Global` dentro de ambientes com Telegram mockado.
  - **Mitigação**: Atualizar a expectativa desses testes para validar o uso do `BackButton` nativo e criar um teste dedicado para o fallback web quando o Telegram não possui `BackButton`.
- **Risco**: Título desviar para a esquerda ou direita em telas estreitas com ícone de calendário.
  - **Mitigação**: O uso de grid `1fr auto 1fr` garante que o slot central fique matematicamente alinhado ao centro geométrico do header, enquanto o slot esquerdo e o direito absorvem o mesmo espaço proporcional.
- **Risco**: Título quebrar ou vazar a viewport em telas muito pequenas (ex: 320px).
  - **Mitigação**: Regras responsivas com `font-size: 16.5px` em 360px e `15px` em 310px, com `text-overflow: ellipsis` defensivo.

## Impactos esperados
- Eliminação total da duplicação de navegação no topo (sem duas setas no detalhe e sem seta redundante no global).
- Continuidade visual natural entre o chrome nativo do Telegram e os cabeçalhos azul e vermelho da aplicação.
- Títulos centralizados de forma precisa em ambas as telas.
- Zero acúmulo de listeners do Telegram.

## Compatibilidade
- Linux: Sim
- macOS: Sim
- Windows: Sim
- Docker: Sim
- CI/CD: Sim

## Como testar

### Build
```bash
export PATH="/home/senzu/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web run build
```

### Testes
```bash
export PATH="/home/senzu/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
```

### Execução
```bash
export PATH="/home/senzu/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web run dev
```

## Rollback
Descartar alterações com:
```bash
git checkout -- web/
```

## Observações
A API oficial do Telegram Mini Apps não expõe interface para renderizar títulos customizados dentro da barra nativa do cliente Telegram. A solução adotada é a integração contínua: coloração idêntica da barra nativa com `setHeaderColor`, remoção de botões duplicados no DOM do app e cabeçalho visual alinhado imediatamente abaixo dos controles nativos.

# Plano: fidelidade-visual-mockup-rankings

## Pedido do usuário
Realizar uma segunda passada exclusivamente de UI/UX no frontend (MiniApp Web Telegram) para aproximar a implementação ao máximo do mockup aprovado (`mockup_de_rankings_uno_em_iphones.png`).
- Manter a fidelidade ao mockup como fonte de verdade visual.
- Não alterar backend, API, SQL, regras de ranking, auth, paginação, referências opacas, migrations ou integração Telegram.
- Foco em:
  - Largura, proporções e alinhamento mobile-first.
  - Gradiente azul royal/elétrico no header do Ranking Global.
  - Gradiente carmesim/cereja no header e hero do Ranking do Grupo.
  - Controles segmentados (pills cápsula) com cores corretas:
    - Atualizado ativo em vermelho (`#e53935` / `#ea2d36`).
    - Legado ativo em vermelho (`#e53935` / `#ea2d36`).
    - Grupos ativo em vermelho (`#e53935` / `#ea2d36`).
    - Players ativo em azul elétrico (`#136fe5` / `#1672ec`).
    - Itens inativos com texto ardósia/marinho sobre fundo transparente.
    - Contêineres de controle arredondados (cápsula `rounded-full`) com fundo claro/translúcido.
  - Cartas da lista (Cards de ranking):
    - Top 1: card amarelo suave quente (`#fff8d6` / `#fef5cc`) com medalha de fita dourada e número 1 branco.
    - Top 2: card neutro/prata suave (`#f3f6fa`) com medalha de fita prateada e número 2 branco.
    - Top 3: card bronze/pêssego suave (`#fff0e6`) com medalha de fita bronzeada e número 3 branco.
    - Posições 4+: cards brancos (`#ffffff`) limpos com sombras suaves e números minimalistas sem ponto (`4`, `5`, `6`...), tipografia marinho/carvão em negrito.
    - Avatares maiores (~48px-50px nos cards, ~96px no hero com borda branca de 4px).
    - Tipografia: nomes fortes em marinho escuro (~15.5px, peso 700), ID com 5 bullets (`ID •••••8462`), pontuação em destaque à direita com `pts` em peso semibold.
  - Tela de Detalhe do Grupo:
    - Hero com gradiente vermelho, bordas arredondadas, avatar grande em destaque, nome do grupo, ID mascarado, pontuação grande (~28px) e legenda `Total do grupo no mês de <Mês>`.
    - Cartas decorativas de UNO fanning no canto inferior direito do hero (uma amarela e uma verde com bordas brancas e oval central).
    - Transição com topo arredondado (`rounded-t-[30px]`) para a área branca com ícone de grupo 👥 e título `Ranking interno do grupo · <Mês>`.
  - Validações: `npm run lint`, `npm run typecheck`, `npm run test`, `npm run build`, testes Go afetados e `git diff --check`.
  - Sem commit/push, sem alterar `main`, sem deploy.

## Objetivo
Refatorar a camada visual do frontend (`web/src/styles.css`, `web/src/components/Ranking.tsx`, `web/src/pages/Rankings.tsx`) para alcançar fidelidade máxima com o design do mockup nos três fluxos (Global Grupos, Global Players e Detalhe do Grupo).

## Contexto atual
- A funcionalidade e roteamento já foram homologados.
- O CSS atual em `web/src/styles.css` ainda possui gradientes escuros (quase pretos/ardósia no topo), segmented controls retangulares ou com cores divergentes da referência, cards com altura/espaçamento e contraste reduzidos em relação ao fundo, e o número de posição 4+ com ponto final (`4.`).
- O ícone da seção "Ranking interno do grupo" exibe um peão de xadrez (`♟`) em vez do ícone de pessoas/grupo (`👥`) do mockup.
- O hero da tela de detalhe não possui as proporções, padding e cartas UNO decorativas com as cores exatas do mockup.

## Arquivos analisados
- `mockup_de_rankings_uno_em_iphones.png`
- `web/src/pages/Rankings.tsx`
- `web/src/components/Ranking.tsx`
- `web/src/components/Ranking.test.tsx`
- `web/src/styles.css`
- `web/src/App.test.tsx`

## Arquivos que poderão ser modificados
- `web/src/styles.css`
- `web/src/components/Ranking.tsx`
- `web/src/components/Ranking.test.tsx`
- `web/src/pages/Rankings.tsx`

## Estratégia de implementação
1. **Segmented Controls & Classes Visuais**:
   - Ajustar `web/src/components/Ranking.tsx` para aplicar classes semânticas claras nos segmented buttons (`selected-red` para Atualizado, Legado e Grupos; `selected-blue` para Players).
   - Ajustar o `RankBadge` para renderizar posições > 3 como número limpo (`{position}` sem ponto final), atualizando o teste correspondente.
2. **Ícone da Seção de Detalhes**:
   - Em `web/src/pages/Rankings.tsx`, substituir o caractere `♟` por um SVG de pessoas/grupo 👥 limpo e alinhado com o mockup.
3. **Cartas Decorativas UNO no Hero**:
   - No hero de grupo em `web/src/pages/Rankings.tsx` e `styles.css`, estilizar as duas cartas UNO sobrepostas no canto inferior direito (carta amarela e carta verde, com borda branca, cantos arredondados, fundo colorido e elipse branca central).
4. **Estilização Global e Detalhe em `web/src/styles.css`**:
   - **Gradiente do Header Global**: Royal blue vibrante (`linear-gradient(180deg, #0947ba 0%, #105fce 50%, #1a75ec 100%)`).
   - **Gradiente do Header de Detalhes**: Carmesim rico com profundidade (`linear-gradient(180deg, #700d18 0%, #9e1322 35%, #cf1e2c 70%, #9e1322 100%)`).
   - **Segmented Controls**: Formato cápsula `border-radius: 9999px`, altura ~44px, preenchimento e botões internos `rounded-full` com tipografia 700 e transições suaves.
   - **Painel de Conteúdo**: Fundo `#f0f3f8` com topo arredondado de 28px/30px para criar contraste perfeito com os cards brancos.
   - **Cards de Ranking**:
     - Altura confortável (~76px), border-radius 20px-22px, padding 12px 14px.
     - Top 1: fundo amarelo suave `#fff9db` / gradiente suave `#fff9d6 -> #fef5cc`.
     - Top 2: fundo neutro prata suave `#f3f6fa`.
     - Top 3: fundo pêssego/bronze suave `#fff0e6`.
     - Posições 4+: fundo branco puro `#ffffff` com borda sutil e sombra suave.
     - Medalhas SVG detalhadas com brilho e fita de cauda.
     - Posições 4+: número sem ponto, centralizado, marinho escuro em negrito.
     - Tipografia: nomes marinho escuro (#081534), ID em ardósia com espaçamento correto, score com números em destaque tabular e `pts` integrado.
   - **Hero da Tela de Detalhes**:
     - Card arredondado (radius 24px) com fundo vermelho translúcido, avatar grande de 96px com borda branca de 4px, nome do grupo 22px negrito, pontuação grande 28px e legenda do mês.
5. **Verificações e Testes**:
   - `export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH"`
   - `npm run lint`
   - `npm run typecheck`
   - `npm run test`
   - `npm run build`
   - `go test ./internal/httpapi/...`
   - `git diff --check`

## Riscos
- **Risco**: Quebra de testes de componentes caso alguma asserção de texto ou seletor seja alterada.
  - **Mitigação**: O único teste que referenciava o formato de número era `expect(screen.getByText('4.')).toBeInTheDocument()`, que será atualizado para `'4'`, mantendo todos os outros seletores funcionais e atributos semânticos (`aria-label`, `role`) idênticos.
- **Risco**: Regressão de layout em telas com larguras menores (ex.: 320px).
  - **Mitigação**: Regras responsivas mantidas com `overflow-wrap: anywhere`, `min-width: 0`, e proporções flexíveis nos cards e hero.

## Impactos esperados
- Interface do WebApp Telegram idêntica à referência visual aprovada, transmitindo um acabamento mobile-first premium, profissional e polido.

## Compatibilidade
- Linux / macOS / Windows
- Safari iOS (Telegram WebApp no iPhone) e Android WebView
- Chromium / Desktop WebApp

## Como testar

### Build
```bash
export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH" && cd web && npm run build
```

### Testes
```bash
export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH" && cd web && npm run test && npm run lint && npm run typecheck
go test ./internal/httpapi/...
```

### Rollback
```bash
git checkout -- web/src/
```

## Observações
- Não requer nenhuma alteração de schema/migration ou backend.
- Respeita o compromisso de zero commit/push.

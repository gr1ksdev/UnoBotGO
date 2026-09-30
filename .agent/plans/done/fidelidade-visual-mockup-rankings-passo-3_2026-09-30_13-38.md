# Plano: fidelidade-visual-mockup-rankings-passo-3

## Pedido do usuário
Executar uma terceira passada exclusivamente visual (UI/UX) com base estrita no mockup `mockup_de_rankings_uno_em_iphones.png`, utilizando como referência a viewport padrão de iPhone de aproximadamente 390x844 CSS pixels.
Foco principal:
1. **Tipografia**:
   - Aumentar presença e peso dos nomes (16.5px - 17px, peso 750/800 em marinho escuro).
   - Pontuações maiores e mais impactantes (18px - 19px bold/extra-bold, tabular nums).
   - Títulos dos headers mais destacados (18px, peso 750, tracking refinado).
   - Textos secundários (ID mascarado, legendas) nítidos e harmoniosos (~13px).
2. **Cards de Ranking**:
   - Ocupação de altura melhor aproveitada (altura mínima ~80px - 82px, padding vertical 12px, horizontal 14px, raio 22px).
   - Alinhamento vertical refinado (grid e flex perfeitamente balanceados).
   - Sombras e bordas ultra sutis (`rgba(15, 23, 42, 0.03)`).
3. **Avatares**:
   - Aumentar tamanho dos avatares da listagem para ~54px - 56px de diâmetro, com borda branca fina de alta definição.
   - Avatar do hero da tela de detalhe aumentado para ~102px - 106px com borda branca sólida de 4px e sombra elegante.
4. **Header Global**:
   - Gradiente azul profundo com iluminação rica e tridimensional:
     `linear-gradient(180deg, #073b82 0%, #0d4fa8 35%, #156ad4 75%, #2484eb 100%)`
     com transição suave sobre o topo da área de conteúdo.
5. **Área de Conteúdo**:
   - Fundo mais claro e limpo (`#f8fafd` / `#f6f8fc`), eliminando a sensação acinzentada, preservando a sobreposição de 20px com cantos arredondados de 28px/30px.
   - Cards com fundo branco puro `#ffffff` contrastando elegantemente com a base clara.
6. **Tela de Detalhe do Grupo**:
   - Hero com proporções expandidas e gradiente carmesim acetinado profundo:
     `linear-gradient(180deg, #660a14 0%, #99121f 30%, #cb1d2a 70%, #8f101c 100%)`.
   - Nome do grupo com 23px negrito 800, ID em 13.5px com bom espaçamento, pontuação enorme de 32px extra-bold com `pts` em 20px bold.
   - Cartas UNO decorativas estilizadas no canto direito:
     - Duas cartas fanning out (amarela e verde), com borda branca nítida de 3px, sombra, elipse central e símbolo/desenho interno, cortadas elegantemente pela lateral direita do card.
7. **Sensação Mobile Premium**:
   - Experiência nativa iOS sem aspecto de tabela web.
   - Sem alterações em backend, queries, rotas, regras de negócio ou autenticação.
   - Sem commit/push.

## Objetivo
Alcançar fidelidade visual definitiva e acabamento mobile-first premium no Mini App Web Telegram em viewport de 390px, alinhando tipografia, proporções, densidade, gradientes e elementos decorativos com a referência exata do mockup.

## Contexto atual
- As cores e estruturas principais estão no lugar correto, mas as escalas de texto, tamanho dos avatares, altura dos cards, luminosidade do fundo e detalhes das cartas UNO e hero ainda se comportam de forma excessivamente comprimida/compacta.

## Arquivos analisados
- `mockup_de_rankings_uno_em_iphones.png`
- `.agent/card1_crop.png`
- `.agent/card4_crop.png`
- `.agent/hero_crop.png`
- `.agent/header_crop.png`
- `web/src/styles.css`
- `web/src/pages/Rankings.tsx`
- `web/src/components/Ranking.tsx`

## Arquivos que poderão ser modificados
- `web/src/styles.css`
- `web/src/pages/Rankings.tsx`
- `web/src/components/Ranking.tsx`

## Estratégia de implementação
1. **Markup e Estrutura**:
   - Em `web/src/pages/Rankings.tsx`:
     - Refinar a estrutura das cartas UNO decorativas no hero para conter a elipse central com o caractere/ícone do UNO, garantindo renderização de cartas reais fanned out no canto direito.
2. **Estilização em `web/src/styles.css`**:
   - Ajustar tipografia base, tamanhos de fontes e pesos (`player-name` para 16.5px 750, `score` para 18.5px 800, `masked-id` para 13px 500).
   - Ajustar dimensões dos cards (`min-height: 80px`, `padding: 12px 14px`, grid com coluna de avatar para `56px` e medalha para `32px`).
   - Aumentar diâmetro de avatares para `54px` na listagem e `104px` no hero.
   - Atualizar gradiente do header global para azul profundo acetinado e do hero de detalhe para carmesim com profundidade e brilho radial sutil.
   - Clarear o fundo da área de conteúdo para `#f7f9fc` / `#f8fafd` e manter cards com `#ffffff`.
   - Ajustar as cartas UNO no hero: tamanho ~52px x 82px, borda 3px branca, cantos arredondados, rotação precisa e elipse centralizada com acabamento fiel ao mockup.
3. **Verificações**:
   - Executar `npm run lint`, `npm run typecheck`, `npm run test`, `npm run build`.
   - Executar `go test ./internal/httpapi/...`.
   - Verificar `git diff --check`.

## Riscos
- **Risco**: Quebra de layout em telas ultracompactas (<340px).
  - **Mitigação**: Media queries com `clamp` e regras específicas para telas estreitas mantendo proporções fluidas.

## Impactos esperados
- Interface 100% alinhada ao mockup visual de 390px, com presença marcante, profundidade e densidade de aplicativo nativo de alta qualidade.

## Compatibilidade
- iOS Safari (Telegram WebApp iPhone 390x844 e superiores)
- Android Chrome WebView
- Desktop WebApp

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

## Rollback
```bash
git checkout -- web/src/
```

## Observações
- Nenhuma alteração em backend, banco ou regras de negócio.
- Working tree preservado sem commit/push.

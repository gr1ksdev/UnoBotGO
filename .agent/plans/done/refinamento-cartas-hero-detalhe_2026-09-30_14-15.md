# Plano: refinamento-cartas-hero-detalhe

## Pedido do usuário
Refinamento visual cirúrgico na tela de detalhe do grupo ("Ranking do grupo") do Telegram Mini App, especificamente no conjunto de cartas decorativas no lado direito do hero vermelho:
- Representar 3 cartas grandes físicas de UNO em leque:
  - Vermelha em primeiro plano;
  - Amarela atrás;
  - Verde atrás;
- As cartas devem começar dentro do hero vermelho e ultrapassar geometricamente a borda direita da interface, sendo cortadas naturalmente pelo limite da viewport / app-shell, dando a impressão física de continuarem para fora do celular;
- Não reduzir cartas para caberem e não cortar prematuramente pelo próprio hero card;
- Proporções realistas de carta com borda branca nítida, cantos arredondados e elipse central;
- Legenda do hero simplificada de "Total do grupo no mês de {month}" para "Total em {month}";
- Garantir que nenhum elemento textual ou avatar seja encoberto ou sofra quebra inadequada;
- Zero scroll horizontal em todas as viewports;
- Nenhuma alteração de backend, API, regras, banco, tela global ou commits/push.

## Objetivo
Elevar a fidelidade visual da tela de detalhe de grupo ao mockup aprovado (`mockup_de_rankings_uno_em_iphones.png`), implementando o leque de 3 cartas UNO com transbordamento lateral controlado (overflow) e proporções realistas, ajustando a legenda mensal e atualizando os testes de frontend.

## Contexto atual
- A tela de detalhe exibe o card `.group-hero` com `overflow: hidden;`, que prende e corta as cartas decorativas dentro dos 26px arredondados do próprio card.
- A implementação atual possui apenas 2 cartas minúsculas (`card-yellow` e `card-green`) com dimensões de 54px x 86px contidas no canto inferior direito.
- A legenda exibe `"Total do grupo no mês de {month}"`, enquanto a especificação refinada solicita `"Total em {month}"`.
- `App.test.tsx` valida a string `"Total do grupo no mês de Setembro"`.

## Arquivos analisados
- `web/src/pages/Rankings.tsx`
- `web/src/styles.css`
- `web/src/App.test.tsx`
- `web/src/components/Ranking.tsx`
- `mockup_de_rankings_uno_em_iphones.png`
- `.agent/phone2_edge.png`

## Arquivos que poderão ser modificados
- `web/src/pages/Rankings.tsx` (estrutura das 3 cartas decorativas e atualização da legenda)
- `web/src/styles.css` (regras de overflow, dimensões das cartas, rotações em leque, sombras e stacking)
- `web/src/App.test.tsx` (atualização da asserção da legenda)
- `.agent/decisions.md` (registro da decisão de design e clipping)
- `.agent/memory/memory.md` (atualização da memória persistente)

## Estratégia de implementação
1. **Hierarquia de Clipping & Overflow**:
   - Manter `.app-shell` com `overflow: hidden;` como o container raiz delimitador do dispositivo/viewport móvel.
   - Ajustar `.group-hero` e `.detail-header` para permitir que o leque de cartas decorativas projete-se para fora da borda direita do hero card até o limite lateral do `.app-shell`.
   - O container `.uno-cards` será posicionado absolutamente no canto direito do hero, com `pointer-events: none;` e `aria-hidden="true"`.
2. **Design e Estrutura das 3 Cartas (Vermelha, Amarela, Verde)**:
   - Carta Vermelha: primeiro plano (`z-index: 3`), rotação moderada (~-2° a +4°), posicionada na frente e estendendo-se para a direita.
   - Carta Amarela: plano intermediário (`z-index: 2`), rotação para o topo-direito (~18° a 22°), projetando-se mais para fora.
   - Carta Verde: plano de fundo (`z-index: 1`), rotação mais acentuada (~36° a 42°), projetando-se para o canto inferior direito.
   - Proporções das cartas: escala fluida usando `clamp(86px, 24vw, 110px)` por `clamp(132px, 37vw, 168px)` com `aspect-ratio: 2 / 3.05` aproximado de cartas de baralho.
   - Estilização física: borda branca de ~3px a 3.5px, border-radius de 10px a 12px, elipse central branca (`card-oval`) com gradiente interno sutil e sombra projetada (`box-shadow: -4px 6px 16px rgba(0, 0, 0, 0.35)`) dando profundidade entre as cartas.
3. **Preservação de Conteúdo e Acessibilidade**:
   - Conteúdo de texto (`.group-summary`) e avatar (`.avatar-large`) mantidos com `z-index: 10` e posicionamento flex estável, garantindo legibilidade absoluta.
   - Reduzir margem de colisão lateral ajustando `max-width` do texto se necessário, para que números e títulos fiquem visualmente limpos.
4. **Legenda**:
   - Alterar `<p className="hero-caption">Total do grupo no mês de {month}</p>` para `<p className="hero-caption">Total em {month}</p>`.
   - Atualizar a asserção no teste `web/src/App.test.tsx` linha 166 de `expect(screen.getByText(/Total do grupo no mês de Setembro/)).toBeInTheDocument()` para `expect(screen.getByText(/Total em Setembro/)).toBeInTheDocument()`.

## Passos detalhados

1. **Atualização da tela `Rankings.tsx`**:
   - Modificar a legenda do hero para `"Total em {month}"`.
   - Modificar o markup das cartas em `.uno-cards` para conter 3 cartas:
     - `.card-green` (fundo)
     - `.card-yellow` (meio)
     - `.card-red` (frente)
     Cada qual com sua elipse central `.card-oval`.

2. **Estilização em `web/src/styles.css`**:
   - Ajustar regras de overflow no hero e cabeçalho de detalhe.
   - Definir proporções, sombras, bordas, cores ricas (Vermelho `#ea2328`, Amarelo `#f59e0b`, Verde `#16a34a`) e elipses com ângulo rotacionado característico.
   - Definir posicionamentos absolutos e rotações das 3 cartas criando a impressão física de leque ultrapassando a borda direita do aparelho.
   - Garantir que em telas menores (320px - 390px) e em visualização desktop não ocorra barra de rolagem horizontal.

3. **Atualização do teste `web/src/App.test.tsx`**:
   - Atualizar a expressão regular de busca da legenda para `/Total em Setembro/`.

4. **Verificação de qualidade e regressão**:
   - `npm --prefix web run lint`
   - `npm --prefix web run typecheck`
   - `npm --prefix web run test`
   - `npm --prefix web run build`
   - `go test -count=1 ./...`
   - `go build ./...`
   - `git diff --check`

5. **Documentação e Finalização**:
   - Registrar no `.agent/decisions.md` e `.agent/memory/memory.md`.
   - Mover plano para `.agent/plans/done/`.
   - Apresentar instruções para homologação visual no navegador.

## Riscos
- Risco de overflow horizontal em telas móveis estreitas:
  - Mitigação: O overflow horizontal é contido no nível do `.app-shell` (`overflow-x: hidden`), permitindo que a carta ultrapasse a margem do card do hero mas seja rigidamente cortada na borda física da tela/viewport do app, sem gerar barra de rolagem.
- Risco de sobreposição das cartas sobre o texto do hero:
  - Mitigação: Posicionamento fixado à direita com `pointer-events: none` e texto com `z-index: 10` e largura máxima controlada.

## Impactos esperados
- A tela "Ranking do grupo" atinge fidelidade visual premium com o mockup de referência, com as 3 cartas UNO em leque extrapolando a borda direita do hero.
- A legenda fica mais sucinta e elegante.
- Todos os testes de unidade e integração passam com 100% de sucesso.
- O build de produção do frontend (`web/dist`) fica atualizado para visualização local imediata com `go run -tags debugcards ./cmd/bot`.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD

## Como testar

### Build
```bash
export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH" && npm --prefix web run build && go build ./...
```

### Testes
```bash
export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH" && npm --prefix web run test && go test -count=1 ./...
```

### Execução
```bash
go run -tags debugcards ./cmd/bot
```

## Rollback
Caso necessário, reverter as alterações nos arquivos com:
```bash
git checkout -- web/src/pages/Rankings.tsx web/src/styles.css web/src/App.test.tsx
```

## Observações
- Nenhuma alteração no backend ou endpoints Go.
- Nenhuma alteração na tela global de Grupos e Players.
- Nenhum commit ou push será realizado.
- Todo o trabalho anterior no working tree permanece preservado.

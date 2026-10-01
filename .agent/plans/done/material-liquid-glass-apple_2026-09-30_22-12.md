# Plano: material-liquid-glass-apple

## Pedido do usuário
Consultar documentação Apple e corrigir material da pílula, pois efeito anterior não parece Liquid Glass. Somente frontend/dev, sem commit/push/main/backend/API.

## Objetivo
Trocar fundo leitoso e blur excessivo por material óptico com refração de borda, luminosidade/translucidez e resposta de interação, preservando navegação aprovada.

## Contexto atual
Pílula flutuante tem background76/46% branco e blur22; subpílula ativa84% branca. Isso apaga o fundo. Apple orienta Liquid Glass para camada de navegação, com resposta dinâmica e adaptação de transparência/movimento; adoção nativa via SwiftUI/UIKit/AppKit. Mini App é React/WebView, exigindo equivalente web, sem prometer acesso a material nativo. HIG distingue regular/clear; navegação sobre texto exige contraste. MDN documenta backdrop SVG URL e feDisplacementMap.

## Arquivos analisados
- web/src/components/BottomNavigation.tsx
- web/src/styles.css
- web/src/App.test.tsx
- internal/httpapi/static.go (CSP permite data/blob; somente leitura)
- .agent/context.md
- https://developer.apple.com/documentation/technologyoverviews/adopting-liquid-glass
- https://developer.apple.com/design/human-interface-guidelines/materials
- https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/backdrop-filter
- https://developer.mozilla.org/en-US/docs/Web/SVG/Reference/Element/feDisplacementMap

## Arquivos que poderão ser modificados
- web/src/components/BottomNavigation.tsx
- web/src/hooks/useLiquidGlass.ts (novo)
- web/src/styles.css
- .agent/context.md
- .agent/memory/memory.md
- .agent/decisions.md

## Estratégia de implementação
Hook óptico pequeno sem dependências: mapa de deslocamento derivado da geometria da cápsula, gerado em canvas somente em resize, SVG feDisplacementMap sobre pixels reais do backdrop nos navegadores compatíveis. Centro neutro e borda refrativa; sem screenshot/clonagem de conteúdo/dados. Blur menor, película menos opaca, reflexos de borda e destaque respondendo a pointer. Seleção deriva exclusivamente do tab da URL e desliza como única camada; navegação/link/aria-current preservados. Fallback blur sem SVG/sem canvas, superfície opaca sem backdrop e respeito a reduced transparency/motion/contrast. Não aplicar efeito ao ranking/conteúdo.

## Passos detalhados
1. Registrar plano/autoridade da tarefa e fontes consultadas.
2. Gerar mapa óptico leve com cleanup de resize; integrar SVG local decorativo.
3. Ajustar CSS material/seleção/reflexos/interações e accessibility media, mantendo dimensões/safe areas/reserva.
4. Validar refração visível no browser sobre fundo real de ranking e teste óptico com padrões coloridos; confirmar que diferença de pixels existe com displacement ativado/desativado.
5. Verificar navegação, responsive/safe, perf (sem loops/frame rendering) e fallback.
6. Checks/build/documentação, plano done.

## Riscos
- SVG backdrop não tem comportamento uniforme em engines; enhancement com fallback, validar refração real no engine disponível.
- Transparência deve preservar leitura; blur moderado, luminância/film e labels mais escuros.
- Efeito nativo Apple não é API CSS; informar limite sem interromper trabalho.

## Impactos esperados
- Material mais próximo das propriedades ópticas da referência Apple, mantendo conteúdo/nav.
- Sem dependências/telemetria/captura/requests extra; custo local somente quando cápsula muda dimensão.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD: React/CSS/SVG/canvas padrão; enhancement óptico e fallback conforme suporte.
- Mobile-first e desktop/safe area da pílula preservados.

## Como testar

### Build
```bash
npm --prefix web run build
go build ./...
```

### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
git diff --check
```

### Execução
```bash
npm --prefix web run dev
```
Browser com fixtures locais e diagnóstico óptico, sem backend/migrations.

## Rollback
Reverter só hook/hunks desta rodada, preservar alterações anteriores.

## Observações
Autorizado corrigir após consulta da documentação. Fontes Apple lidas via markdown/JSON oficial pois páginas dependem JS. Manter fonte citada no relatório e distinção entre efeito web/nativo. Homologação visual com usuário.

## Conclusão
Implementado em BottomNavigation.tsx, useLiquidGlass.ts e styles.css. Refração comprovada comparando escala 18/0: 3678 pixels diferentes apenas na pílula. Testes browser em 320/390/430/1280px, insets 0/34px, rotas, refresh, teclado, BackButton, estados e paginação passaram. Contraste aumentado desativa filtro corretamente.

Validação: lint, typecheck, 35 testes, build frontend, git diff --check e go build ./... passaram. Primeiro teste revelou ausência de CSS.supports no jsdom; corrigida detecção de capacidade e suíte repetida com sucesso. Nenhum Go alterado. Sem commit/push/deploy. Material web equivalente, não API nativa Apple; Safari/WebView real não validado nesta máquina.

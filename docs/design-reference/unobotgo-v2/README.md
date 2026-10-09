# UnoBotGO — correção visual v2

Este pacote corrige o afastamento visual do pacote v1. Para cartas e tipografia da partida, a v2 tem precedência sobre os SVGs e fontes de substituição da v1. Não é uma implementação da engine nem do multiplayer.

## Uso

1. Extraia a pasta `unobotgo-v2` em `docs/design-reference/unobotgo-v2/` do seu repositório. Preserve a v1 e suas alterações locais.
2. Abra `referencia.html` no navegador. Os controles alternam turno, coringa, resultado e mãos de 7, 8, 9, 16 e 30 cartas; também demonstram distribuição, compra e virada. A referência foi atualizada para até oito cartas por fileira, agrupadas por cor, sem rolagem horizontal. Os dados são ilustrativos e a revanche é uma simulação local.
3. Entregue `PROMPT-CORRECAO.md` ao Codex que trabalha no repositório. Ele deve substituir a apresentação da partida mantendo a integração real existente.
4. Compare a implementação com as capturas em `references/` e valide nos quatro tamanhos indicados no prompt.

## Conteúdo

- `assets/cards/`: 62 imagens de cartas, incluindo versões coloridas de coringas, mais `card_overlay.png`, que é uma camada visual, não uma carta adicional.
- `assets/backs/default.png`: verso original.
- `assets/atlases/`: dois atlas originais e seus JSONs. As cartas individuais foram recortadas sem redimensionamento e com a rotação corrigida conforme os metadados.
- `assets/fonts/`: Nunito variável, pesos 200–1000, e Lexend Deca Black, peso 900. `fonts.css` fornece os registros locais.
- `assets/effects/`: setas, brilhos, padrões e sombras originais.
- `assets/audio/`: arquivos estáticos originais, opcionais. A referência não reproduz áudio automaticamente.
- `assets/vendor/`: GSAP para a demonstração offline.
- `referencia.html` e `interacao.js`: referência interativa com recursos originais e composição reconstruída a partir das capturas.
- `previews/`: catálogo das cartas e capturas da referência corrigida.
- `asset-manifest.json`: origem e hashes dos recursos extraídos.

## O que é exato e o que é reconstruído

As fontes, os atlas, seus pixels recortados, o verso e os arquivos de efeitos vêm do HAR fornecido. O pacote não contém o HAR, credenciais, mensagens de WebSocket, avatares pessoais extraídos das respostas ou os bundles completos da aplicação original.

O seletor em losango, a coroa, a disposição da mesa e as animações da referência foram reconstruídos para demonstrar o comportamento pedido. Não representam extração completa do código do Matcho. Cores de interface e tempos de movimento são aproximações visuais; os pixels das cartas são originais.

As capturas enviadas pelo usuário são incluídas como referências, e mostram os avatares e nomes que já estavam visíveis nelas. Os exemplos da demonstração usam iniciais e nomes ilustrativos.

A referência usa HTML e GSAP; o prompt pede que o Codex aplique as skills locais de PixiJS e GSAP à partida real. Consulte `MOVIMENTOS.md` para a coreografia e suas durações.

Abra `VERIFICACAO.md` para os resultados dos testes da referência. Esses testes não validam o backend do UnoBotGO.

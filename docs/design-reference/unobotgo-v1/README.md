# UnoBotGO — pacote de implementação visual v1

Referência executável para uma Mini App mobile com ranking unificado entre Telegram Inline e WebApp. É uma proposta nova baseada na captura real do Matcho fornecida nesta conversa. As imagens conceituais anteriores não estavam disponíveis para comparação; não se afirma reproduzi-las.

## Abrir

Abra `index.html` no navegador. Não requer instalação, servidor, internet ou build. Navegue por Jogar, Ranking e Perfil. A partida demonstrativa fica no botão principal da tela inicial.

Links locais para as referências:

| Tela | URL relativa |
| --- | --- |
| Início | `index.html?screen=home` |
| Ranking de jogadores | `index.html?screen=ranking` |
| Ranking de grupos | `index.html?screen=groups` |
| Perfil | `index.html?screen=profile` |
| Partida de dois jogadores | `index.html?screen=game` |
| Resultado | `index.html?screen=result` |
| Ranking carregando | `index.html?screen=ranking&state=loading` |
| Ranking vazio | `index.html?screen=ranking&state=empty` |
| Falha no ranking | `index.html?screen=ranking&state=error` |

Todos os nomes, resultados, tempos, grupos e pontos são dados ilustrativos. Os botões de compra e jogada demonstram seleção e feedback, sem alterar partida ou pontuação. Não há backend, autenticação Telegram, sala real ou WebSocket neste pacote.

## O que entregar ao Codex

Coloque esta pasta no repositório em `docs/design-reference/unobotgo-v1/`. Envie o texto completo de `PROMPT-CODEX.md`. Os arquivos `index.html`, `ui.css`, `ui.js` e `tokens.css` são a referência visual concreta; as imagens em `previews/` registram esse código no navegador.

1. Implementar primeiro o ranking usando as fontes de dados existentes.
2. Reutilizar os componentes na home e no perfil.
3. Integrar a partida com a engine do projeto e transporte em tempo real.
4. Comparar screenshots nas mesmas dimensões e ajustar.

## Conteúdo

| Caminho | Uso |
| --- | --- |
| `tokens.css` | Cores, espaços, raios, tipografia e curva de movimento |
| `assets/cards/` | 55 SVGs próprios: 52 cartas coloridas, dois coringas e um verso |
| `assets/icons/` | 15 ícones vetoriais editáveis |
| `assets/table-texture.svg` | Textura discreta da mesa |
| `assets/manifest.json` | Catálogo e medidas dos assets |
| `docs/DESIGN-SYSTEM.md` | Medidas, componentes, responsividade e estados |
| `docs/INTEGRACAO.md` | Identidade, grupos, resultado único e eventos de partida |
| `docs/REFERENCIA.md` | Evidência limitada ao print e HAR fornecidos |
| `docs/matcho-reference.jpg` | Captura original para consulta visual |
| `PROMPT-CODEX.md` | Instrução para aplicar este pacote ao repositório real |

As cartas representam tipos visuais, não a composição quantitativa de um baralho. A engine determina quantas cópias de cada carta existem. A carta Troca de Mãos não aparece nesta proposta; se já estiver implementada e habilitada no projeto, adicionar um SVG próprio e conectá-la às regras existentes.

## Limites

A referência de partida pronta mostra dois jogadores. O documento de design especifica composição para três a seis, mas essas variantes precisam ser implementadas e verificadas antes de integração final. Regras e pontuação existentes prevalecem sobre qualquer número ilustrativo do protótipo. Não há novas coins, XP, conquistas, lojas ou filtros de temporada.

Os SVGs do pacote foram criados para esta proposta. A captura do Matcho é referência, não arte para publicar no aplicativo. O HAR bruto não acompanha o pacote; a integração deve usar o protocolo do próprio UnoBotGO.

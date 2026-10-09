# Verificação da referência — fileiras e movimento

Verificação local em Chromium headless. Este relatório cobre o pacote de referência, sem validar o servidor ou a aplicação de produção do UnoBotGO.

## Layout

Foram verificados 32 casos: mãos de 1, 2, 7, 8, 9, 15, 16 e 30 cartas em 320×568, 360×800, 390×844 e 430×932.

- Cada fileira tem no máximo oito cartas e margem mínima de 16px nos dois lados.
- A última fileira incompleta permanece centralizada.
- Não há rolagem horizontal da mão nem do documento.
- Em mãos grandes, a última carta pode ser alcançada pela rolagem vertical da região da mão.
- Fontes e imagens carregam localmente. Capturas dos estados de turno, coringa, resultado e duas fileiras estão em `previews/`.

## Movimento

- A distribuição apresenta cartas saindo do monte em sequência para os dois jogadores da demonstração, sem revelar frentes do adversário, e termina com sete cartas na mão local.
- A compra apresenta deslocamento real do monte até a posição da nova carta. Cliques repetidos durante a compra ilustrativa não geram uma segunda carta.
- A virada demonstra frente → verso → frente e restaura o recurso correto ao concluir.
- A escolha de cor mantém todas as pétalas com a mesma cor e permanece visível após um segundo; depois fecha e apresenta a variante de coringa escolhida.
- Fora da própria vez, as cartas estão indisponíveis.
- A revanche local aguarda uma segunda aceitação e reinicia com distribuição animada.
- Nenhum erro JavaScript observado. Resultados estruturados em `previews/checks.json`.

## Limites

A referência é HTML/GSAP com dados ilustrativos e dois jogadores. Não valida PixiJS, dez jogadores, autenticação, WebSocket, pontuação, persistência, reconexão, eventos concorrentes, compras múltiplas nem jogada animada ao descarte. Esses itens estão nas instruções para integração no repositório real.

O agrupamento por cor e o número de cartas por fileira são apresentação. Os tempos são sugestões novas de movimento, não timelines extraídos do original. As imagens/fontes continuam sendo os recursos originais do HAR.

Algumas áreas expostas das cartas são estreitas em 320px por causa da sobreposição. A integração deve verificar o toque em aparelho real, garantir seleção de todas as cartas e fornecer alternativa acessível. A referência tem foco de teclado e nomes acessíveis; não substitui homologação no Telegram.

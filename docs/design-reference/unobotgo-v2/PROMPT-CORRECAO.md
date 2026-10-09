# Correção autorizada: fidelidade visual e comportamento da partida

Implemente esta correção na partida WebApp existente do UnoBotGO. O objetivo é reproduzir a aparência e os comportamentos das referências originais que forneci, com as cartas e fontes reais deste pacote. Estou aprovando a implementação desta correção. Preserve alterações locais. Não faça commit, push ou deploy.

Leia as instruções do repositório e inspecione a integração atual antes de editar. Localize e leia integralmente os SKILL.md das skills de PixiJS e GSAP que já baixei na pasta do projeto. Aplique ambas à apresentação da partida: PixiJS para cena/cartas/camadas e GSAP para os movimentos. Reutilize as versões e estruturas existentes compatíveis com as skills, sem instalar bibliotecas e deixá-las sem uso. Use `docs/design-reference/unobotgo-v2/` como referência prioritária para cartas, tipografia e interação da partida. A v1 desviou do resultado pedido e não deve prevalecer nesses pontos.

## Recursos obrigatórios

Use as imagens originais de `assets/cards/`, o verso de `assets/backs/default.png` e os efeitos fornecidos. Não redesenhe as cartas, não substitua seus números por texto HTML e não mantenha os SVGs pastel da v1. Preserve proporção 256:344 e transparência. `card_overlay.png` não é uma carta jogável. Vincule o recurso ao tipo/cor da engine; mantenha CardID estável durante renderização, animação e comandos.

Use Nunito variável para interface e Lexend Deca Black para os títulos de resultado, conforme `fonts.css`. Não dependa de fontes remotas nem use uma fonte de sistema como substituição permanente. Interface original: Nunito, peso base 550; títulos em Lexend Deca Black 900. Ajuste os demais pesos e tamanhos pela referência.

As variantes `red_wild.png`, `yellow_wild.png`, `green_wild.png`, `blue_wild.png` e as respectivas `*_wild_draw4.png` permitem mostrar a cor ativa no descarte. Isso é apresentação: a carta continua sendo coringa na engine.

## Aparência e movimento

Compare primeiro as capturas em `references/original-*`. Use `referencia.html` como demonstração de estados, não como backend nem como cópia completa do original. As referências `atual-*` mostram problemas a corrigir.

1. **Mesa:** composição compacta e imersiva, verso original, descarte, monte e setas de direção. Remova da área central o excesso de caixas e instruções que afasta o visual do jogo original. Preserve ações necessárias e acessibilidade.
2. **Turno:** avatar do jogador da vez com contorno azul e pulsação suave. Na minha vez, brilho azul difuso junto ao meu avatar e à parte inferior da mesa. Fora da minha vez, todas as minhas cartas ficam visualmente apagadas e indisponíveis. Na minha vez, apenas as jogáveis ficam com cores vivas; as demais continuam apagadas. Reproduza o escurecimento das capturas, sem presumir que precisa ser um filtro grayscale total. Derive turno e jogabilidade do estado autorizado da engine.
3. **Mão:** no máximo oito cartas por fileira, centralizadas, com folga visível nas duas laterais. A nona começa outra fileira; a décima sétima começa a terceira. Esta instrução substitui a rolagem horizontal proposta anteriormente. Não desloque a mão para a direita nem deixe cartas fora do viewport. Centralize também a última fileira incompleta. Em mãos grandes, adapte altura e tamanho sem cobrir monte, descarte e avatares; quando a altura não bastar, permita rolagem vertical somente na região da mão. Teste 1, 2, 7, 8, 9, 15, 16, 17 e 30 cartas. Agrupe por cor (o equivalente a “naipe” no UNO): vermelho, amarelo, verde, azul e coringas. Dentro de cada cor, ordene números e ações de forma consistente. Use ordenação estável por CardID para empates. Reorganize animadamente quando cartas entram/saem, sem trocar o alvo de toque durante a interação nem refazer a distribuição a cada snapshot. A carta selecionada se eleva suavemente, sem cortar sua parte superior nem impedir acesso às fileiras vizinhas. Preserve seleção pelo ID, não pelo índice da lista.
4. **Coringa:** substitua a caixa genérica com quatro botões rotulados por um seletor grande em losango arredondado sobre a mesa escurecida. Vermelho em cima, amarelo à direita, azul embaixo e verde à esquerda. Ao tocar, as quatro pétalas recebem a cor escolhida com transição de aproximadamente 450ms e permanecem nessa cor por aproximadamente 700ms antes de uma saída suave de 350ms. O feedback completo deve ser perceptível, próximo de 1,5s; não desapareça quase imediatamente. Isso é duração de apresentação, não atraso para enviar o comando ao servidor. Preserve nomes acessíveis para cada cor. Não permita confirmação duplicada. Mostre o coringa com a variante da cor ativa após confirmação do servidor. Se houver rejeição, restaure o estado e permita nova escolha.
5. **Animações reais:** use GSAP na implementação para seleção/jogada, compra, reorganização da mão, entrada/saída do seletor e transição do resultado. Siga a coreografia detalhada em MOVIMENTOS.md. Ao iniciar, distribua do monte para cada jogador em sequência alternada, revelando somente minhas cartas. Na compra, a carta deve sair fisicamente do monte, deslocar-se até minha mão, virar para revelar a frente autorizada e acomodar-se na posição correta. Compras múltiplas (+2/+4 e outros eventos já existentes) devem exibir cartas em sequência com intervalos curtos. Nos adversários, anime apenas versos até a posição do jogador e atualize a contagem pública. Jogadas vão da carta escolhida ao descarte. Evite teletransporte, movimentos lineares rígidos e saltos de layout. Use antecipação pequena, aceleração/desaceleração suave e acomodação discreta, sem exagerar em elasticidade. Use brilho suave e passagem de luz na coroa do vencedor. Faça cleanup das timelines/tweens ao mudar de partida ou desmontar. Respeite preferência de movimento reduzido. A animação nunca deve definir a regra, enviar comando duplicado ou atrasar o processamento de eventos. Instalar GSAP sem usá-lo não cumpre o pedido.
6. **Resultado:** sobreposição sobre a própria mesa escurecida, mensagem conforme vitória/derrota, lista real de participantes, vencedor destacado em dourado, coroa com feixe de luz e botão Revanche. Não invente pontos, nem copie os números ilustrativos da referência. Preserve a pontuação e a situação de persistência retornadas pelo serviço.

## Revanche multiplayer

O botão deve registrar aceitação real no servidor e atualizar todos os participantes pelo transporte existente. Mostre quem aceitou e quem falta. Uma única aceitação não reinicia a partida; só inicie quando todos os participantes exigidos tiverem aceitado. Defina explicitamente o comportamento de saída/desconexão, sem reduzir silenciosamente o grupo necessário para iniciar.

Faça a transição para uma nova partida/rodada de forma atômica e idempotente: novo identificador conforme o contrato da engine, nenhum reinício duplicado e votos antigos sem efeito. Preserve o resultado anterior e a finalização transacional; não permita que uma revanche apague um resultado pendente. Se o backend ainda não suporta revanche, implemente o contrato e a sincronização necessários, com testes das condições reais de concorrência.

## Integração existente

Preserve autenticação Telegram, engine, regras, timers oficiais, WebSocket existente, revisões, deduplicação, projeção privada da mão, finalização e ranking. Não troque a integração por uma simulação. Não envie a mão privada de outro jogador ao navegador.

Criar/entrar/jogar deve permanecer disponível pelo fluxo real da Mini App, sem exigir `/novo` e `/entrar` como pré-requisito. Se o fluxo atual ainda obriga isso, corrija os caminhos disponíveis na Mini App, respeitando as salas e políticas já implementadas. Botões Jogar não podem apenas abrir o bot quando prometem jogar na Mini App.

Mantenha a separação das políticas Legacy/Updated e o período mensal. A modalidade Inline/WebApp não cria outra fonte de pontuação. Não altere regras para imitar a referência visual.

## Critérios de entrega

- Teste screenshots 320×568, 360×800, 390×844 e 430×932, com fontes carregadas e recursos reais.
- Verifique estados: minha vez, outra vez, seleção de carta, seletor de cor antes/depois da escolha, resultado, um voto de revanche, todos os votos e reconexão. Exercite composição de dois até dez jogadores.
- Verifique o limite de oito por fileira, margens laterais, centralização das fileiras incompletas e ausência de rolagem horizontal. Verifique que trinta permitem alcançar todas pela região vertical da mão. Grave um vídeo curto de distribuição, compra, virada e escolha de cor; screenshots não comprovam fluidez. Teste nomes longos e avatares ausentes.
- Teste rejeição e duplicação de comandos, eventos atrasados, troca de turno durante animação, desmontagem e revanche concorrente. Reutilize os checks obrigatórios do projeto.
- Entregue a implementação no repositório, capturas comparáveis e relatório dos checks executados. Indique separadamente o que foi testado localmente e o que foi confirmado no Telegram real.
- Não declare concluído só porque o build passou: confira a fidelidade das cartas, fonte, seletor e composição visual antes da entrega.


## Sincronização das animações

Estado autorizado e animação são responsabilidades separadas. Envie a intenção sem aguardar o fim da animação e derive a apresentação do evento/snapshot aceito. Preserve revisão e deduplicação: um evento reaplicado não pode distribuir/comprar outra vez. Reconexão exibe o estado atual sem repetir compras históricas. Se o turno mudar durante uma animação, atualize a disponibilidade imediatamente. Cancelamento, desmontagem, resize e troca de sala devem limpar movimentos e manter a apresentação coerente com o estado real.

O agrupamento por cor é somente organização da mão no cliente, sem alterar a engine. O número de cartas por fileira também é apresentação. A referência local é um guia de movimentos; a entrega deve usar partidas reais e seguir as duas skills locais.

# Movimento e organização da mão

Esta revisão atende ao pedido de uma partida mais fluida e viva. O limite de oito cartas por fileira substitui o carrossel horizontal anterior.

## Mão

- Oito cartas no máximo em cada fileira; o excedente continua na próxima.
- Todas as fileiras centralizadas, com margem mínima de 16px de cada lado na referência.
- Grupos por vermelho, amarelo, verde, azul e coringas. Números e ações têm ordem estável dentro de cada grupo.
- Entrada/saída reorganiza as cartas suavemente, mantendo IDs; não use índice visual como identidade do comando.
- Fileiras podem sobrepor parcialmente a borda inferior das anteriores, preservando números, acesso e seleção. Mãos muito grandes usam rolagem vertical da região da mão.
- A disposição não deve deslocar o documento ou criar rolagem horizontal. Monte/descarte continuam fora da região de rolagem.

## Coreografia sugerida

| Ação | Sequência | Tempo inicial para ajuste |
| --- | --- | --- |
| Distribuir | Monte → jogadores alternados → acomodar; revelar somente a própria mão | Voo de 780ms; intervalo de 90ms entre destinatários na demo de dois jogadores |
| Comprar | Verso sai do monte → voo suave → virada no percurso → posição organizada na mão | Voo de 780ms; virada de 500ms começa aos 200ms |
| Compras múltiplas | Uma carta por vez, com voos parcialmente sobrepostos | Intervalo de 100–160ms, ajustar pela quantidade |
| Jogar | Pequena elevação da seleção → viagem ao descarte → acomodação | 450–650ms, ajustar pela distância |
| Selecionar | Elevação discreta e escala mínima, com retorno suave da seleção anterior | 380ms |
| Reorganizar | Deslocamento entre posições reais antes/depois da mudança | 580ms |
| Abrir seletor | Crescimento suave com opacidade sobre a mesa escurecida | 500ms |
| Escolher cor | Todas as pétalas recebem a cor → mantêm feedback → saída suave | 450ms + 700ms + 350ms |
| Resultado | Entrada suave sobre a mesa → destaque dourado → passagem de luz na coroa | Entrada de 550ms; brilho discreto em ciclo |

Os tempos são pontos de partida de apresentação e não foram extraídos de timelines originais. Distribuição de dez jogadores deve evitar uma espera longa: comprima os intervalos/voos ou organize rodadas parcialmente simultâneas, preservando legibilidade.

O comando de cor vai para o servidor no toque. O tempo do feedback não deve atrasar a lógica da partida nem confirmar uma jogada rejeitada. A animação acompanha o resultado autorizado.

## Escopo da referência

`referencia.html` usa dados ilustrativos e GSAP, mostra distribuição para dois jogadores, compra unitária, virada, reorganização, seleção e coringa. Compras múltiplas, movimento da jogada ao descarte, integração PixiJS e sincronização multiplayer são requisitos do prompt para o repositório real, não funcionalidades implementadas nesta demonstração.

As skills PixiJS/GSAP foram baixadas no projeto do usuário e não estão disponíveis neste pacote. O Codex daquele projeto deve localizá-las, lê-las e aplicá-las.

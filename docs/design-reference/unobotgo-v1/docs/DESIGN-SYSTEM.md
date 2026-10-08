# Design system — medidas e comportamento

## Direção

Mini App vertical, alegre e legível: verde petróleo na mesa; verde lima suave para ações; cartas em coral, amarelo, verde e azul. Formas arredondadas, tipografia forte e poucas áreas de brilho. A composição da partida vem do print fornecido: oponente em cima, duas pilhas no meio, mão e jogador local na base. As cartas novas usam cantos de 16 unidades, círculos discretos, símbolos escuros e um verso com marca geométrica, distintos das artes do Matcho.

`index.html` + `ui.css` são a fonte visual executável. Não transformar esta proposta em dashboard de desktop. Em telas largas, manter a aplicação com no máximo 480 px e centralizar a área mobile.

## Tokens

| Função | Valor |
| --- | --- |
| Fundo | `#10292F` |
| Superfície | `#19383F` |
| Superfície elevada | `#24464D` |
| Texto principal | `#F4F6ED` |
| Texto secundário | `#ADC3C2` |
| Ação principal | `#C6EF83` |
| Texto sobre ação | `#19312D` |
| Carta vermelha | `#FF655F` |
| Carta amarela | `#FFD25B` |
| Carta verde | `#A7E367` |
| Carta azul | `#63B8FF` |
| Borda de superfície | `rgba(204,230,218,.12)` |
| Espaçamentos | 4, 8, 12, 16, 20, 24, 32 px |
| Raios de interface | 12, 20, 28 px |
| Fonte | `ui-rounded, Trebuchet MS, DejaVu Sans, sans-serif` |

A fonte é um fallback local. A família presente no dispositivo pode mudar métricas; fixar a fonte licenciada escolhida pelo projeto e regerar referências quando isso ocorrer. Não extrair a fonte do Matcho como requisito de integração.

## Hierarquia de texto

| Elemento | Tamanho / peso / observação |
| --- | --- |
| Título home | 34 / 800 / entrelinha 1,15 |
| Título de página | 30 / 800 / tracking −1 px |
| Título de bloco | 16 / 700 |
| Número principal | 31 / 800 |
| Nome na lista | 13 / 700 |
| Descrição | 13 / normal / entrelinha 1,55 |
| Informação secundária | 11 / normal |
| Rótulo de navegação | 10 / 700 |

Em 320 px, reduzir o título da home a 31 px e o nome da lista a 12 px. Conteúdo não deve ganhar scroll horizontal; o único scroller horizontal é a mão de cartas. Nomes de lista truncam com reticências; preservar o nome completo em um rótulo acessível ou detalhe de perfil.

## Tela de ranking — referência 390 × 844

| Bloco | Regra |
| --- | --- |
| Margens | 20 px laterais; 24 px no topo |
| Marca | 17 px; ícone 30 × 30 |
| Cabeçalho | Margem superior 30 px; título e descrição juntos |
| Segmento Jogadores / Grupos | Container 18 px de raio; padding 5 px; botão 43 px mínimo |
| Pódio | Três colunas; gap 8 px; base alinhada; reserva de 234 px |
| Avatar do primeiro | 69 × 69; borda amarela 3 px |
| Avatares segundo e terceiro | 55 × 55; borda 3 px |
| Base do pódio | Segundo 64 px; primeiro 84 px; terceiro 50 px |
| Linha de ranking | Mínimo 66 px; raio 18 px; gap 12 px; padding 13 × 12 |
| Avatar na linha | 38 × 38 |
| Minha posição | Mesmo componente, borda `#C6EF8355`; mostra posição real, mesmo fora do top |
| Navegação | Altura aproximada 74 px + safe area; três destinos; fixa na base |
| Espaço inferior do conteúdo | 112 px; permite rolar até o fim sem cobrir linhas |

O pódio contém os três primeiros. A lista começa em quarto para não duplicar participantes. O fixture mostra três linhas seguintes, sem ser limite de produto. Na integração real, permitir paginação de ranking sem deslocar o pódio. Não limitar o ranking aos seis primeiros.

Se houver menos de três participantes, não criar posições falsas: reduzir o pódio aos resultados existentes ou usar lista simples. Empates e sua ordenação seguem a regra do backend. Se o usuário não estiver classificado, mostrar um convite a jogar; não inventar posição. Na aba Grupos, a linha adicional de grupo pessoal só aparece com contexto validado — uma conta pode participar de vários grupos.

## Início, perfil e resultado

Home: um botão claro para começar/abrir partida, atalho para o bot Inline, três métricas reais e prévia de ranking. O texto de demonstração deve desaparecer somente depois de conectar dados reais.

Perfil: identidade Telegram única, total de pontos, posição e estatísticas combinadas. Cada partida do histórico identifica Inline ou Mini App, grupo quando aplicável e colocação. Aproveitamento = vitórias / partidas elegíveis, conforme a contagem oficial do projeto; quando não houver partidas mostrar travessão ou 0 segundo a convenção adotada.

Resultado: classificação confirmada, variação de pontos confirmada, ação para ranking e retorno. Só afirmar que pontos foram contabilizados quando o backend confirmar o commit. No protótipo o resultado é uma tela separada, não conclusão automática de uma partida real.

## Partida — dimensões

| Elemento | Regra |
| --- | --- |
| Área de jogo | `100svh`; adaptar à altura estável reportada pelo Telegram quando disponível |
| Safe areas | Somar áreas seguras do Telegram e dispositivo sem duplicá-las |
| Cabeçalho | Voltar/menu 44 × 44; 16 px de margem lateral |
| Oponente | Centralizado; verso das cartas resumido; avatar 52 × 52 |
| Centro | Flexível, ocupa espaço entre oponente e mão |
| Carta descartada | 88 × 123 px; rotação −8° |
| Monte de compra | 90 × 126 px; camadas de sombra discretas |
| Distância entre pilhas | 39 px na referência de 390 px |
| Mão | Carta 80 × 112; sobreposição 32 px; avanço visível 48 px |
| Mão comprida | Scroll horizontal; preserva largura e símbolo da carta |
| Carta selecionada | Elevação 16 px; z-index maior; espaço extra para ver a carta inteira |
| Jogador local | Avatar 42 px; nome e quantidade; ação contextual ao lado |
| Ação principal | Mínimo 44 px de altura; habilitada com seleção válida |

Dois jogadores: oponente no topo, usuário na base. Três jogadores: dois oponentes no topo, distribuídos igualmente. Quatro: um no topo, um à esquerda, um à direita; usuário na base. Cinco e seis: os oponentes seguem a borda superior e laterais, com badges compactos (avatar 36 px e quantidade), sem tocar pilhas e mão. Esses layouts adicionais são requisitos para implementação, ainda não screenshots prontas deste pacote. Quantidade máxima vem da engine real; não impor seis se o projeto suporta outro máximo.

320 × 568: reduzir área do oponente, versos a uma faixa de 29 px, pilhas a 70–72 × 98–101 px e espaços verticais. A mão continua tocável; controles locais não ficam fora da tela. 390 × 844 e 430 × 932: usar área central flexível, sem esticar as cartas. Landscape deve manter uma composição tocável compacta ou uma orientação recomendada reversível; não travar o usuário numa tela sem ações.

Mais de sete cartas: rolar a mão; não diminuir indefinidamente os alvos. Verificar 1, 2, 7, 15 e 30 cartas. Após uma compra, revelar a carta recebida sem perder a seleção válida; após confirmação, remover a carta pelo ID estável, não pelo índice.

## Estados e movimento

| Evento/estado | Comportamento |
| --- | --- |
| Seleção | 180 ms; elevação 16 px; sem enviar ação automaticamente |
| Confirmação de jogada | Ação via serviço; bloquear duplicata enquanto pendente |
| Carta em movimento | 220–280 ms; eixo de origem até pilha central; carta sempre legível |
| Comprar | 180–240 ms; monte até mão; contagem atualizada por evento confirmado |
| Mudança de turno | 160 ms; ring do avatar e texto indicando o jogador atual |
| Inverter | Setas refletem direção oficial; animação 240 ms, uma vez por evento |
| Escolher cor | Sheet modal com quatro opções de 56 × 56 ou maiores, rotuladas |
| Carta inválida | Opacidade/contraste e estado textual; manter nome legível |
| Conexão perdida | Banner discreto; desabilitar envio; manter snapshot visível |
| Reconectando | Feedback persistente; recuperar snapshot e revision antes de habilitar |
| Jogada recusada | Restaurar seleção e explicar rejeição; sem contabilizar resultado |
| Ranking carregando | Skeleton preserva área do pódio e lista |
| Ranking vazio | Convite a jogar, sem exemplos misturados aos dados reais |
| Falha de ranking | Mensagem e botão tentar novamente |
| Resultado pendente | Mostrar partida finalizada, aguardando confirmação da pontuação |

Respeitar `prefers-reduced-motion`: reduzir viagem de cartas e desativar efeitos repetitivos. Animação não determina estado de jogo. Eventos repetidos ou replay de reconexão não devem repetir efeitos já vistos.

## Acessibilidade e desempenho

Controles de 44 × 44 ou maiores, incluindo região exposta de cartas. SVGs de números e ações dão informação além da cor; leitor de tela recebe cor, tipo e disponibilidade. Avatares reais usam fallback de iniciais. Modal deve manter foco dentro, anunciar título e permitir fechar quando a regra aceitar cancelamento.

DOM é suficiente para ranking e este protótipo. No jogo, manter a solução já usada no projeto ou escolher canvas só após medir necessidade; o pacote não exige GSAP ou PixiJS. Canvas deve ter controles DOM equivalentes para acessibilidade. Evitar blur de tela inteira, loops contínuos de brilho, filtros pesados em 30 cartas e rerender global a cada tick.

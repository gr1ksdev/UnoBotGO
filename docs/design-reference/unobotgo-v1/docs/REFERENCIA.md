# Referências observadas

## Print

Arquivo fornecido: `374488.jpg`, mantido como `matcho-reference.jpg`.

Observação: tela vertical de jogo; avatar e contagem do oponente na área superior; descarte e monte centrais; duas setas curvas indicando direção; mão de cartas sobrepostas perto da base; jogador local abaixo da mão. As barras do Android e os controles do Telegram vistos no print pertencem ao contêiner, não devem ser desenhados como conteúdo da Mini App.

A nova proposta mantém a organização geral e cria identidade própria: verde petróleo + lima, números escuros, verso geométrico, textura de pontos, navegação compartilhada nas áreas fora da partida.

## HAR

Arquivo fornecido: `matcho.dotvhs.com_2026_10_08_09_42_54.har`.

Constatações locais da captura:

- 86 entradas de rede.
- Recursos sob `/socket.io/`, upgrade `wss` com status 101 e 89 mensagens WebSocket registradas no HAR.
- Atlas de cartas PNG com metadados JSON contendo 63 frames.
- Recursos de deck, realce de turno/jogador, setas, sombras, padrões e áudio.

Esses números são sobre esta captura, não o inventário completo do projeto. A presença de arquivos não prova todas as funcionalidades da aplicação. O pacote não faz engenharia reversa do backend, não reutiliza mensagens de sessão e não contém o HAR bruto.

Nenhum áudio, carta, avatar pessoal, fonte, bundle JS ou token do Matcho foi copiado para o produto de demonstração. A captura original é incluída exclusivamente como referência visual solicitada.

## Imagens anteriores

O histórico relata imagens conceituais aprovadas, mas seus arquivos não estavam anexados nesta execução. Esta entrega tem referências novas, renderizadas do código incluído; não são reprodução garantida daquelas imagens.

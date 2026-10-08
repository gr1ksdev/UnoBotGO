# Prompt para aplicar ao repositório UnoBotGO

Implemente o redesign mobile da Mini App e integre partidas WebApp ao UnoBotGO existente. O pacote visual está em `docs/design-reference/unobotgo-v1/`. Leia os arquivos desse diretório antes de começar. Se a pasta estiver em outro lugar, encontre-a no repositório; não substitua as referências por um design genérico.

O usuário quer uma Mini App de Telegram vertical e feita para toque. Use a composição de partida do print Matcho como orientação e a identidade própria do pacote como fonte de implementação. O ranking de jogadores e o ranking de grupos são unificados: partidas Inline e WebApp contam juntas sob as regras atuais.

## Primeiro, compreender o projeto

Leia AGENTS.md aplicáveis e inspecione a branch de trabalho. Localize frontend atual, ranking, perfis, rotas, engine, serviço de partidas, snapshots, IDs/revisions, timers, persistência, privacidade, pontuação e envio de notificações após commit. O pacote foi criado sem acesso ao código; nenhum endpoint descrito nele deve ser presumido existente.

Não sobrescreva alterações locais do usuário. Use a stack e o gerenciamento de dependências já presentes quando adequados. Inspecione documentação oficial atual dos pontos de integração Telegram e transporte utilizados. Não reinvente regras de UNO nem a política de pontos.

## Referências concretas obrigatórias

1. `index.html`, `ui.css`, `ui.js`: modelo executável de seis telas.
2. `tokens.css`: cores, raios, espaços e fonte.
3. `assets/cards/`: cartas próprias SVG, 80 × 112 de viewBox.
4. `assets/icons/`: ícones próprios; use-os ou adapte-os ao componente SVG da stack.
5. `docs/DESIGN-SYSTEM.md`: medidas, animações, estados e regras de responsividade.
6. `docs/INTEGRACAO.md`: invariantes do ranking e sincronização.
7. `previews/`: screenshots renderizadas; comparar com a implementação nas mesmas dimensões.

O protótipo usa dados fictícios e scripts locais, não é frontend pronto para produção. Converta seus componentes para a stack do projeto e substitua fixtures por fontes reais. Não copie `innerHTML` de fixtures para renderizar conteúdo vindo de usuários/APIs.

## Ordem de execução

Comece pelo ranking, que estabelece a linguagem visual; depois perfil e home; finalmente partida e resultado. Continue até concluir os itens viáveis no repositório, sem encerrar a tarefa só com plano ou uma tela isolada. Se faltar uma informação indispensável, entregue o restante verificável e descreva o bloqueio concreto.

### 1. Ranking

Crie/reutilize `RankingScreen`, `RankingTabs`, `Podium`, `RankingRow`, `MyPosition`, `Avatar`, `BottomNav` e estados Loading/Empty/Error.

- Apenas duas categorias: Jogadores e Grupos. Não criar categorias Inline/WebApp.
- Pódio segundo/primeiro/terceiro, conforme a referência.
- Lista começa após o pódio e aceita paginação real.
- Minha posição real em destaque quando existir; não deixar fixture #12.
- Pontos formatados segundo o sistema existente, sem fazer conta com floats na UI.
- Estados com menos de três resultados, empates, nomes longos e avatares ausentes.
- Grupo só recebe resultado da partida com vínculo validado no servidor.
- Ranking respeita preferências de privacidade e exclusão do projeto.
- Não adicionar coins, XP, conquistas, temporadas ou filtros sem suporte existente.

### 2. Perfil e início

Um perfil por identidade Telegram, pontuação e estatísticas combinadas. Histórico identifica modalidade e contexto de grupo. Sem duplicar resultados de reconexão. Home abre o fluxo real de sala existente ou o novo fluxo mínimo implementado; o atalho Telegram usa o username/configuração real do bot. Não hardcode bot, ID, token ou usuário.

### 3. Partida

Reutilize engine e serviço. O servidor é autoritativo. A UI envia comando com identidade da partida, ID de ação e controle de revision equivalente ao já disponível. Reutilize a finalização transacional e idempotente para pontuar.

- Dois jogadores: oponente no topo, pilhas no centro, mão e usuário na base.
- Implementar e verificar layouts para o número de jogadores suportado pela engine; as variantes 3–6 estão descritas, não prontas no protótipo.
- Cartas como SVG do pacote ou sprites derivados deles; nunca usar screenshot recortada como UI inteira.
- Mais de sete cartas usa scroll horizontal; nenhum zoom que transforme a mão em alvos minúsculos.
- Seleção eleva carta e habilita confirmação. Jogar só quando servidor aceitar.
- Coringa abre escolha de cor acessível; +2, +4, bloqueio e inversão seguem regras reais.
- Mostrar jogador atual, timer oficial e quantidade de cartas sem revelar mão alheia.
- Comprar, jogar, desistir e sair respeitam permissões/regras existentes.
- Reconexão recupera snapshot; gaps, duplicatas e revision antiga são tratados.
- Fim de partida confirmado encerra controles de turno, inclusive com dois jogadores.
- Finalização sem commit de score não mostra pontuação inventada.
- Se Troca de Mãos já existir, criar visual próprio e conectar seleção de destinatário às regras atuais.

Canvas/PixiJS/GSAP não são obrigatórios. Use a solução mais simples que mantenha fluidez e a arquitetura do projeto. Se já houver renderer adequado, integre-o. Movimento das cartas vem dos eventos confirmados; nunca usar animação como engine.

### 4. Visual e responsividade

Reproduza os tokens e a hierarquia da referência. Fundo petróleo, acento lima, cards coloridos, superfícies arredondadas. Não fazer dashboard administrativo, mesa horizontal de desktop, bordas neon intensas ou emojis como ícones.

Respeite safe areas do Telegram/dispositivo e altura estável da Mini App. Navegação inferior aparece no início/ranking/perfil e some durante a partida. Header do Telegram e barra do sistema não são elementos do aplicativo.

Verifique 320 × 568, 360 × 800, 390 × 844 e 430 × 932. Teste 1, 2, 7, 15 e 30 cartas, nomes longos e a contagem máxima de jogadores. O conteúdo das páginas pode rolar; mesa e controles precisam caber na área útil. O contraste e os rótulos de cartas não dependem só da cor.

## Critérios de conclusão

- Dados reais em ranking e perfil; nenhum mock apresentado como resultado verdadeiro.
- Uma única operação canônica de finalização e score para Inline e WebApp.
- Testes de duplicata, grupo forjado, identidade forjada, revision antiga e encerramento de dois jogadores.
- Checks obrigatórios da stack e testes existentes apropriados passam.
- Screenshots da implementação nas dimensões de referência, com comparação visual e correção das diferenças materiais.
- Fotos/avatares não escondem informação, nomes longos não empurram pontuação para fora da tela.
- Commit/deploy/publicação seguem a autorização e as regras do repositório; não publicar produção apenas por terminar o redesign.

No relato final, diga o que foi integrado, o que foi testado e limitações materiais. Não afirmar que já existe jogo online só porque a tela renderiza. Se uma tela ainda estiver em demonstração, identifique isso claramente.

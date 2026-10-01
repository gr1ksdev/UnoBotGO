# Decisão: massa fictícia e ferramenta de devseed para homologação do Mini App de Ranking Global

## Data
2026-09-30

## Contexto
O Mini App de Ranking Global do UnoBotGO necessita de validação visual e funcional em cenários extremos antes da configuração de domínio HTTPS, BotFather e abertura no Telegram real (listas longas > 50 itens para paginação e scroll infinito, nomes longos e com emojis, pontuações elevadas de até 25.000 pts, empates exatos com múltiplos critérios de desempate, fallbacks de avatares sem chamadas de rede, e isolamento completo entre Atualizado e Legado). Ao mesmo tempo, era imperativo garantir segurança absoluta fail-closed para impedir qualquer execução acidental em ambientes de produção ou poluição de dados reais.

## Decisão tomada
1. Namespace estrito e reservado para fixtures de desenvolvimento:
   - Chat IDs: intervalo negativo `[-990000000999, -990000000001]`.
   - User IDs: intervalo positivo `[9900000001, 9900000999]`.
   - Game IDs: prefixo `devseed_game_*`.
2. Camadas de proteção fail-closed:
   - Exigência mandatória de `APP_ENV=development` ou `ALLOW_DEV_SEED=1`.
   - Validação explícita de host da URL do PostgreSQL (bloqueio automático de hosts com substrings como `prod`, `production`, `live`, etc.).
   - Mascaramento de credenciais da URL do banco nos logs e saída de terminal.
3. Transacionalidade e Idempotência:
   - Limpeza cirúrgica prévia (`Clean`) executada dentro da mesma transação antes da inserção dos dados determinísticos, impedindo duplicação de pontuações ou acúmulo descontrolado caso o comando seja executado múltiplas vezes.
   - Remoção sem uso de `TRUNCATE`, respeitando a ordem de chaves estrangeiras (`completed_game_players`, `completed_games`, `player_group_monthly_stats`, `player_group_stats`, `known_group_users`, `group_configs`).
4. Cobertura determinística de cenários visuais e de produto:
   - **Atualizado**: 80 grupos (paginação >50 ativa), 180 jogadores, grupo gigante (60 jogadores), grupo pequeno (2 jogadores), empates com desempate por última partida, nomes longos com overflow/ellipsis, nomes com emojis variados, pontuações até 25.000,00 pts (2.500.000 units), e jogadores em múltiplos grupos.
   - **Legado**: 65 grupos, 140 jogadores, pontuações inteiras puras de até 2.450 pts (245.000 units).
   - Mês corrente calculado em `America/Sao_Paulo` via `ranking.RankingLocation()`, sincronizado com a engine de ranking.
   - Fallbacks de avatar nativos (iniciais e ícones), sem chamadas remotas de Bot API durante o seed.
5. Ferramental CLI e Make:
   - Pacote `internal/devseed` desacoplado e reutilizável com suite de testes unitários e de integração.
   - CLI `cmd/devseed` com subcomandos `miniapp`, `clean-miniapp` e `help`.
   - Alvos `make seed-miniapp` e `make clean-miniapp-seed`.

## Motivo
Garantir ambiente completo para testes de estresse de interface e performance sem riscos de integridade, mantendo separação estrita e determinística entre dados de homologação e dados reais.

## Impacto
Desenvolvedores e homologadores podem popular e limpar o banco local instantaneamente em segundos para testar qualquer aspecto do Mini App no navegador sem necessidade de simulações manuais de partidas.

---

# Decisão: refinamento visual cirúrgico do leque de cartas UNO no hero de detalhe de grupo

## Data
2026-09-30

## Contexto
O usuário solicitou um refinamento visual muito específico na tela de detalhe do grupo ("Ranking do grupo"): as cartas decorativas do hero deveriam deixar de ser ícones contidos para se tornarem 3 cartas físicas grandes de baralho UNO dispostas em leque rotacionado (Vermelha em primeiro plano, Amarela no meio, Verde atrás), ultrapassando geometricamente a borda direita do hero e do aparelho, cortadas naturalmente pela borda física da viewport sem corte prematuro pelo card. Além disso, a legenda deveria ser simplificada para "Total em {month}".

## Decisão tomada
1. Leque de 3 cartas grandes com proporção realista de baralho:
   - Dimensões fluídas: `clamp(92px, 25vw, 106px)` de largura por `clamp(144px, 39vw, 166px)` de altura.
   - Borda branca sólida nítida de `3.5px`, cantos arredondados com `border-radius: 12px`, sombras projetadas profundas (`box-shadow: -4px 6px 18px rgba(0, 0, 0, 0.42)`).
   - Elipse central (`card-oval`) inclinada em `-30deg` com borda branca e brilho sutil.
2. Posicionamento e Rotação em Leque:
   - Carta Vermelha: primeiro plano (`z-index: 3`), rotação de `3deg`, `bottom: 10px`, `right: -32px`.
   - Carta Amarela: plano intermediário (`z-index: 2`), rotação de `18deg`, `bottom: 44px`, `right: -36px`.
   - Carta Verde: plano de fundo (`z-index: 1`), rotação de `34deg`, `bottom: -22px`, `right: -28px`.
   - Ponto de pivô/origem: `transform-origin: 30% 90%`.
3. Clipping e Transbordamento:
   - `.group-hero` ajustado com `overflow: visible;`, permitindo que as cartas passem para fora do card hero.
   - `.app-shell` mantido com `overflow: hidden;`, garantindo corte natural das cartas no limite lateral do dispositivo/viewport móvel com zero scroll horizontal.
   - A parte inferior das cartas projeta-se abaixo do hero e é naturalmente coberta pela curvatura do topo do `.ranking-panel`.
4. Legenda e Hierarquia:
   - Legenda simplificada para `"Total em {month}"` em `Rankings.tsx` e teste unitário atualizado em `App.test.tsx`.
   - Avatar e `.group-summary` garantidos com `z-index: 10`, assegurando que textos fiquem sempre à frente e nunca sofram sobreposição indevida.
5. Preservação Total de Escopo:
   - Nenhuma alteração em backend, rotas, regras de negócio ou telas globais.

## Motivo
Fidelidade total ao conceito visual do mockup aprovado, proporcionando a ilusão óptica de cartas reais que continuam fisicamente para fora da tela do smartphone.

## Impacto
Acabamento visual de alto padrão com forte apelo temático de UNO na tela de grupo, mantendo conformidade rigorosa com todos os testes e builds.

---

# Decisão: terceira passada de fidelidade visual de UI/UX (proporção, densidade, tipografia e acabamento nativo 390px)

## Data
2026-09-30

## Contexto
A homologação da segunda passada indicou a necessidade de refinar proporção, densidade, presença tipográfica, acabamento do hero, cartas UNO decorativas e clareamento do fundo, tomando a viewport de 390x844px do iPhone como referência 1:1 com o mockup.

## Decisão tomada
1. Tipografia e Presença:
   - Nomes com presença ampliada: `font-size: 16.5px`, `font-weight: 750` em marinho escuro `#081534`.
   - Pontuação de cards ampliada para `font-size: 18px`, `font-weight: 800` com `pts` em `14px` peso 600.
   - Títulos de header em `18px`, peso 750, perfeitamente centralizados.
2. Dimensões e cards:
   - Cards com altura mínima de `80px`, preenchimento vertical mais harmonioso (`12px 16px 12px 12px`), cantos com `border-radius: 22px`.
   - Posição 4+ com número em `17.5px` peso 750 marinho escuro, perfeitamente centralizado.
3. Avatares:
   - Ampliados para `56px` x `56px` na listagem com anel branco de alta definição.
   - Avatar do hero na tela de detalhe ampliado para `104px` x `104px` com borda sólida branca de `4px` e sombra refinada.
4. Header e Hero da tela de detalhe:
   - Gradiente carmesim acetinado profundo (`linear-gradient(180deg, #660a14 0%, #99121f 30%, #cb1d2a 70%, #90111e 100%)`).
   - Nome do grupo em `23px` peso 800, pontuação grande em `32px` peso 850 com `pts` em `20px` bold.
   - Cartas UNO decorativas estilizadas no canto inferior direito (`54px` x `86px`), sobrepostas e inclinadas, com borda branca sólida de `3px`, sombra e elipse central branca, parcialmente cortadas pela lateral direita do card como no mockup.
5. Fundo clareado:
   - Painel de conteúdo clareado para `#f8fafd`, eliminando a sensação acinzentada e elevando o contraste dos cards brancos `#ffffff`.
6. Preservação:
   - Nenhuma alteração em backend, queries, rotas, regras de negócio ou autenticação.

## Motivo
Alcançar fidelidade visual definitiva e acabamento indistinguível da referência nativa em 390px.

## Impacto
Sensação autêntica de aplicativo nativo mobile premium de alta qualidade no Telegram WebApp.

---

# Decisão: segunda passada de fidelidade visual de UI/UX do Mini App baseada estritamente no mockup oficial

## Data
2026-09-30

## Contexto
Após homologação funcional da Mini App de ranking, os componentes visuais necessitavam de refinamento minucioso para alcançar fidelidade máxima com o design aprovado no mockup (`mockup_de_rankings_uno_em_iphones.png`).

## Decisão tomada
1. Header Global com gradiente azul royal elétrico:
   - Substituído o gradiente escuro anterior por `linear-gradient(180deg, #0947ba 0%, #0d54c7 40%, #156ddf 80%, #1a75ec 100%)`.
2. Segmented controls em formato cápsula (`rounded-full`):
   - Contêineres claros com formato pílula e botões internos `rounded-full` com tipografia semibold/bold (14.5px).
   - Cores exatas do mockup:
     - `Atualizado` e `Legado` ativos em vermelho vibrante (`#ea2328`).
     - `Grupos` ativo em vermelho vibrante (`#ea2328`).
     - `Players` ativo em azul elétrico (`#0262f6`).
     - Abas inativas sobre fundo transparente com texto ardósia/marinho (`#4b5770`) de peso 600.
3. Cards de ranking com proporções e contraste fiéis:
   - Cards com altura confortável de 76px e border-radius de 22px sobre fundo de painel `#f0f3f8`.
   - Top 1: fundo amarelo suave quente (`linear-gradient(90deg, #fff9d8 0%, #fef5cc 100%)`) com medalha de fita dourada e número 1 branco.
   - Top 2: fundo neutro prata suave (`#f4f6fa`) com medalha de fita prateada e número 2 branco.
   - Top 3: fundo pêssego/bronze suave (`#fff0e6`) com medalha de fita bronzeada e número 3 branco.
   - Posições 4+: cards brancos com borda sutil, sombra suave e número de posição limpo (sem ponto final, centralizado em 16px negrito).
   - Tipografia de nomes em marinho escuro (#081534), ID com 5 bullets (`ID •••••8462`) e pontuação tabular com `pts` semibold.
4. Header e Hero da tela de detalhe:
   - Gradiente carmesim rico (`linear-gradient(180deg, #700d18 0%, #9e1322 35%, #cf1e2c 70%, #9e1322 100%)`).
   - Hero com avatar ampliado de 92px com borda branca de 4px, nome com 21px negrito, pontuação grande 28px e legenda do mês.
   - Cartas UNO decorativas estilizadas no canto inferior direito (carta amarela e verde fanning com borda branca e elipse central).
   - Título da seção interna atualizado com ícone de grupo 👥 (`UsersIcon`).
5. Zero alterações de backend:
   - Toda a lógica de API, queries, autenticação, paginação e regras de negócio permaneceu 100% intocada.

## Motivo
Garantir acabamento visual mobile-first premium indistinguível do mockup aprovado.

## Impacto
Experiência do usuário consistente, polida e moderna no Telegram WebApp em iOS e Android.

---

# Decisão: Mini App Telegram de ranking global mensal, API unificada e assets embutidos

## Data
2026-09-30

## Contexto
O UnoBotGO necessitava de uma experiência visual rica para exibição do ranking global mensal (universos Atualizado e Legado), com listas de grupos e jogadores, detalhe do grupo, fotos e anonimização de identificadores, mantendo um único executável em produção sem dependência de runtime Node.js.

## Decisão tomada
1. Arquitetura unificada e runtime standalone:
   - Frontend em React 19, TypeScript, Vite, Tailwind v4 e TanStack Query compilado em `web/dist` e embutido no binário Go (`bin/unobotgo`) via `//go:embed`.
   - Node.js utilizado unicamente na etapa de build/desenvolvimento; imagem Docker final distroless nonroot sem Node ou arquivos fontes.
   - Entrypoint único gerenciado por `internal/app`: orquestra configuração, migrações automáticas sob advisory lock PostgreSQL com fail-closed estrito, servidor HTTP unificado e inicialização do dispatcher/bot do Telegram.
2. Segurança e privacidade:
   - Autenticação obrigatória de rotas de dados e mídia via cabeçalho `Authorization: tma <initData>`, validado por HMAC-SHA256 em tempo constante.
   - IDs numéricos do Telegram protegidos: interface exibe `ID ••••1234`; referências de grupos, cursores de paginação e URLs de mídia utilizam tokens criptografados e autenticados com AES-GCM (derivados de `MINIAPP_SECRET`).
3. Integridade e precisão matemática:
   - `score_units` transmitido como string decimal no JSON e processado via `BigInt` no TypeScript, prevenindo perda de precisão acima de 2^53.
   - Mês corrente calculado exclusivamente no backend com fuso horário `America/Sao_Paulo`. Cursores com mês defasado retornam 409 `ranking_period_changed`.
4. Proxy e cache de avatares:
   - Pacote `internal/media` implementa cache LRU em memória (até 2.000 fotos e 64 MiB), rate limiter por ticker e workers dedicados. Downloads do Telegram são restritos à API oficial com sanitização total de credenciais e logs.

## Motivo
Eliminar sobrecarga no chat do Telegram mantendo alta fidelidade visual ao mockup oficial, garantindo segurança de credenciais, ausência de vazamento de dados de grupos/jogadores e simplicidade operacional de deploy com um único binário.

## Impacto
O ranking global fica acessível no Telegram tanto por botão inline nos rankings quanto diretamente via Mini App. A integridade estatística, regras de engine e migrações anteriores permanecem 100% preservadas.

---

# Decisão: melhoria visual e explicativa do comando /config via blockquotes contextuais

## Data
2026-09-30

## Contexto
A mensagem do comando `/config` exibia apenas o modo e sistema de ranking selecionados sem detalhar as regras ativas no momento, obrigando os usuários a deduzirem os efeitos de cada modo ou ranking.

## Decisão tomada
1. Formatação contextual: `RenderGroupConfig` foi enriquecido para incluir exatamente dois blocos de citação (`<blockquote>...</blockquote>` em HTML):
   - Um bloco para o modo selecionado (`🎮 Clássico` ou `🎮 Caseiro`) com seu respectivo resumo.
   - Um bloco para o sistema de ranking selecionado (`🏆 Legado` ou `🏆 Atualizado`) com seu respectivo resumo.
2. Separador visual: inserido o separador `────────────` em linha própria estritamente entre os dois blockquotes.
3. Centralização e reatividade: como `handleConfig` (comando inicial) e os callbacks de modo (`cfg_mode`) e ranking (`cfg_rank`) já consom exclusivamente `RenderGroupConfig`, a edição da mensagem via `EditMessageText` reflete instantaneamente a alteração, garantindo que o resumo antigo desapareça e o novo seja exibido sem qualquer discrepância.
4. Preservação: botões inline, callbacks, permissões (admin/instalador), dados persistidos e regras de gameplay/ranking mantidos 100% inalterados.

## Motivo
Garantir clareza para os administradores do grupo no momento da configuração, eliminando ambiguidades sobre combinações de cartas ou cálculo de pontuação, mantendo a mensagem compacta e padronizada.

## Impacto
Interface mais intuitiva no Telegram com atualização dinâmica e sem acoplamento adicional no banco ou regras de negócio.

---

# Decisão: suporte a ranking mensal no privado (/ranking) e observação de título de grupos

## Data
2026-09-29

## Contexto
O comando `/ranking` precisava funcionar no privado do bot para apresentar ao usuário suas pontuações mensais por grupo, sem misturar os sistemas Atualizado e Legado, e exibindo o nome humano do grupo ao invés de ChatID cru.

## Decisão tomada
1. Bifurcação transparente: se a mensagem for privada (`!isGroup`), o comando `/ranking` chama `handlePrivateRanking(ctx, chatID, actorID)` usando estritamente o UserID do remetente (`msg.From.ID`).
2. Isolamento de pontuações: sistemas `Atualizado` e `Legado` renderizados em seções separadas, cada um com seu próprio `Total · ...` calculado por soma de inteiros (`score_units`). Jamais são somados em um total geral.
3. Persistência de título dos grupos e Migração 0007 (`0007_group_title.up.sql`):
   - Adicionada coluna `title text NOT NULL DEFAULT ''` na tabela `group_configs` e índice `player_group_monthly_stats(user_id, month_start)`.
   - Criado método `ObserveGroupTitle(ctx, chatID, title)` usando update condicional:
     `WHERE EXCLUDED.title <> '' AND group_configs.title IS DISTINCT FROM EXCLUDED.title`
     garantindo que strings vazias/nulas não sobrescrevam títulos já conhecidos e que updates só ocorram em caso de alteração real.
   - Títulos observados automaticamente em mensagens de comando em grupos e no evento `HandleMyChatMember`.
   - Fallback determinístico: se o grupo não possuir título no banco, exibe `Grupo <ChatID>`.
4. Consulta SQL única: `ListUserMonthlyRankings` busca todas as linhas mensais elegíveis do usuário em uma única query com ordenação determinística (`s.score_units DESC`, `s.last_finished_at DESC`, `COALESCE(NULLIF(c.title, ''), 'Grupo ' || s.chat_id::text) ASC`, `s.chat_id ASC`).
5. Proteção de limites: `RenderUserMonthlyRankings` respeita o limite de 4000 unidades UTF-16, com truncamento ordenado e aviso de linhas omitidas (`… e mais X grupos.`), preservando o `Total` acumulado com o valor real integral.

## Motivo
Eliminar N+1 queries, reaproveitar dados já armazenados em `player_group_monthly_stats`, assegurar privacidade do usuário (sem vazar dados de outros membros), manter consistência de tipos e formatação pt-BR sem floats e sem chamadas remotas de API do Telegram por consulta de ranking.

## Impacto
O bot agora atende `/ranking` tanto em grupo (ranking competitivo do grupo) quanto no privado (resumo mensal pessoal). A arquitetura existente de gameplay e persistência permanece 100% preservada.

---

# Decisão: transformação do ranking acumulado em ranking mensal particionado

## Data
2026-09-29

## Contexto
O produto evoluiu para transformar o ranking visível do grupo em ranking mensal, iniciando automaticamente um ranking novo a cada virada de mês, sem resets manuais ou deleções físicas, preservando o histórico completo.

## Decisão tomada
1. Adotar `player_group_monthly_stats` particionado por `(chat_id, user_id, month_start date)` com índice otimizado para ranking `(chat_id, month_start, score_units DESC, user_id)`.
2. Fuso horário canônico de transição estritamente `America/Sao_Paulo` (carregado via `_ "time/tzdata"`). A virada do mês ocorre às 00:00:00 do dia 01 em Brasília.
3. Partida encerrada pertence integralmente ao mês do seu `finished_at`.
4. Persistência atômica: `RecordCompletedGame` atualiza `player_group_stats` (all-time) e `player_group_monthly_stats` (mês) na mesma transação.
5. Migration `0006_monthly_ranking.up.sql` cria a tabela e executa backfill idempotente a partir de `completed_games` e `completed_game_players`.
6. Desempate do mês: a CTE `latest` filtra exclusivamente as partidas daquele mês (`date_trunc('month', (finished_at AT TIME ZONE 'America/Sao_Paulo'))::date = $2`), impedindo que jogos de meses passados influenciem o desempate corrente.

## Motivo
Garante determinação unívoca de períodos, preservação integral do histórico para futuros rankings globais ou relatórios, integridade transacional sem duplicação de pontos e determinismo absoluto independente do timezone da máquina hospedeira.

## Impacto
O ranking agora exibe o mês atual no título (`🏆 Ranking do grupo · <NomeDoMês>`). A virada de mês é 100% lógica no banco de dados.

---

# Decisão: posições únicas e desempate pela última participação elegível

## Data
2026-09-29

## Contexto
Na homologação de 64bb97d, usuário substituiu expressamente a regra de empate competitivo. A repetição de medalhas era comportamento previamente especificado, não erro de acúmulo.

## Decisão tomada
Ordenação no SQL: score DESC, última colocação elegível ASC, finished_at dessa participação DESC, UserID ASC. Consulta única com DISTINCT ON e JOIN; renderer sequencial. Não duplicar informações em stats ou criar migration. Sem referência histórica, NULLS LAST. Em timestamps iguais entre jogos do mesmo jogador, GameID DESC estabiliza qual registro é considerado último.

## Motivo
Dados existentes são suficientes; resultado individual e timestamps já persistidos na transação homologada. Score permanece autoritativo em stats; partidas sem elegibilidade/commit não entram no desempate.

## Impacto
Substitui a decisão anterior de 1/1/3. Consulta lê histórico elegível para selecionar últimas participações e ordena antes do LIMIT; sem N+1. Nenhum commit/push autorizado nesta homologação: mudanças devem permanecer no working tree da dev. Nome “.” não foi alterado.

---

# Decisão histórica: leitura acumulada e encerramento pós-commit (empate substituído acima)

## Data
2026-09-29

## Contexto
Persistência cumulativa já homologada; Telegram anunciava apenas ganhos da partida e não oferecia leitura do histórico.

## Decisão tomada
Interface de leitura separada da escrita, com serviço no pacote ranking e consulta PostgreSQL exclusiva de player_group_stats. Mesmo snapshot para sistema, total e prefixo ordenado/limitado. Renderer compartilhado para comando e mensagem automática; duas mensagens pós-commit substituem resumo redundante. Empates por score exato; ordem secundária por UserID somente para estabilidade. Limite conservador UTF-16 e linhas completas.

## Motivo
Preservar a transação/semântica homologada, impedir anúncios antes do commit, reutilizar arquitetura existente, evitar queries por jogador e UI de paginação.

## Impacto
Nenhum schema, fórmula, elegibilidade ou conversão muda. Consultas trazem até 512 registros e contagem exata; examinam stats do grupo para contar e validar compatibilidade. Falha de leitura orienta repetir /ranking, sem regravar pontos. Homologação manual pendente na dev, sem publicação.

---

# Decisão: encerramento formal da Milestone M7 no escopo homologado e adiamento (DEFERRED) do import antigo

## Data
2026-09-27

## Contexto
A fundação de persistência PostgreSQL, configuração persistente por grupo, snapshots imutáveis por jogo, rankings Legacy e Updated, regras de elegibilidade e abandono, persistência atômica síncrona pós-jogo, mensagens de pontuação pós-commit, comando `/config` com permissões de admin/instalador e bloqueio de alternância de ranking com histórico acumulado foram implementados, testados exaustivamente e homologados manualmente no Telegram real.
O escopo de importação de ranking legado a partir de mensagens textuais antigas (M7.5), embora possua fundação de parser, staging e reconciliação criada, depende de decisões de produto adicionais (política de conversão Legacy → Updated, multiplicadores, resolução de ambiguidades e UX Telegram).

## Decisão tomada
1. Encerrar formalmente a Milestone M7 considerando como concluído e homologado todo o escopo de persistência, configuração e rankings Legacy e Updated.
2. Adiar formalmente o import de ranking antigo para uma milestone futura, marcando-o explicitamente como DEFERRED ("Old ranking import is deferred to a future milestone").
3. Manter a fundação técnica existente em `internal/rankingimport` e a migration de staging `0004_imports.up.sql` intactas na branch `dev`, sem expor comandos, sem mutação de pontuação e sem conversões arbitrárias.
4. Classificar explicitamente toda a pasta `.agent/` (`plans/`, `memory/`, `decisions.md`, `context.md`) e `AGENTS.md` como artefatos de controle interno de desenvolvimento exclusivos da branch `dev`, mantendo-os estritamente fora da árvore pública da branch `main` e de qualquer promoção.

## Motivo
Garantir previsibilidade técnica, rastreabilidade arquitetural e permitir que o produto estável e homologado avance de forma segura, evitando acumular escopo não prioritário e assegurando que os artefatos de governança interna do agente não vazem para o repositório público em conformidade com as regras do projeto e `docs/branching.md`.

## Impacto
A M7 atinge seu encerramento com sucesso e homologação comprovada no Telegram real. A base de código na `dev` fica pronta para auditoria e planejamento de promoção para `main`. Nenhuma quebra ou funcionalidade inacabada é exposta aos usuários do bot.

---

# Decisão: UX Telegram de configuração de grupos, permissões e proteção de histórico

## Data
2026-09-27

## Contexto
A Milestone M7 implementou a fundação de persistência com `group_configs`, mas faltava a interface de usuário no Telegram para os grupos configurarem seu modo de jogo padrão (`Classic` / `Caseiro`) e sistema de ranking (`Legacy` / `Updated`). Além disso, era necessário determinar quem tem permissão para alterar configurações, como receber eventos de entrada do bot no grupo (`my_chat_member`), e como lidar com trocas de ranking quando já existem pontuações registradas.

## Decisão tomada
1. Comando único `/config` em grupos baseado em botões inline para alternar modo padrão e sistema de ranking, marcando visualmente o item ativo.
2. Permissão restrita a: administradores atuais do grupo OU usuário que adicionou o bot (se ainda for membro atual). Em caso de falha de consulta à API do Telegram, falhar fechado (`groups.ErrForbidden`).
3. Bloquear a troca do sistema de ranking (`Legacy` ↔ `Updated`) caso o grupo já contenha pontuações registradas em `player_group_stats` ou partidas pontuadas em `completed_games`, retornando `groups.ErrNeedsProductDecision` e exibindo um alerta claro ao usuário, preservando a configuração anterior sem corrupção ou migrações mágicas de pontos.
4. Escutar updates de `my_chat_member` em `allowedUpdates` e filtrar transições reais de instalação/reentrada (`left`/`kicked` -> `member`/`administrator`). Apenas quando houver um ator válido no update, gravar `installed_by_user_id` e atualizar `KnownGroupUser`. Enviar mensagem curta de boas-vindas com botão `[ ⚙️ Configurar ]` (`cfg_open_`). Updates irrelevantes (promoções/demissões de cargo do bot) são descartados sem envio de mensagens.
5. O setup permanece 100% opcional e nunca impede a criação ou execução de partidas com `/novo` (defaults `Classic` + `Legacy`).

## Motivo
Garantir uma interface simples, segura e autoexplicativa sem poluir a lista de comandos com múltiplos `/set...`, evitar que usuários não autorizados alterem as regras do grupo, e proteger a integridade estatística dos rankings contra inconsistências de cálculo histórico até que haja uma definição de produto para migração ou coexistência.

## Impacto
Administradores e quem adicionou o bot podem gerenciar as configurações facilmente. Partidas em andamento continuam isoladas com seus snapshots congelados. Grupos novos recebem uma mensagem discreta de boas-vindas. O histórico de pontuações existente fica protegido contra alterações de sistema.

---

# Decisão: desabilitar desafio de blefe para +4 contra +2 no Caseiro e regras de reentrada


## Data
2026-09-26

## Contexto
Durante a homologação real do UnoBotGO V2 no Telegram foram encontrados dois comportamentos inconsistentes:
1. No modo Caseiro, quando um jogador responde a um +2 com um +4 (jogada válida pelo stacking `StackWildDrawFourOnTwo`), se o jogador possuir uma carta da cor anterior, o sistema considerava a jogada blefe culpado em caso de desafio. No entanto, nessa situação o +4 é um counter legal e não uma jogada sob a restrição clássica de Wild Draw Four.
2. Um jogador que saía da partida com `/sair` ficava banido permanentemente de voltar (`ErrAlreadyJoined`), mesmo com a sala aberta, enquanto um jogador com colocação recebia o mesmo erro genérico sem distinção.

## Decisão tomada
1. Identificar na engine se o `WildDrawFour` foi jogado como counter de +2 pendente sob `StackWildDrawFourOnTwo`. Nesses casos, definir `DrawFourChallengeable = false` e `Bluffing = false`, omitir a opção de blefe nas views/inline e rejeitar com `ErrInvalidAction` tentativas de desafio forçado.
2. Permitir que jogadores que saíram (`Status == Left`) e não possuem colocação reentrem via `/entrar` caso a sala esteja aberta, recebendo nova mão de 7 cartas e sendo alocados na cauda lógica da ordem existente (algoritmo padrão de late join), sem criar registros duplicados em `s.Players`.
3. Bloquear permanentemente jogadores que já conquistaram colocação (`WentOut` / presente em `Placements`) com erro explícito `ErrAlreadyFinished` e mensagem informativa: `"🏁 Você já terminou esta partida e não pode entrar novamente."`.
4. Fixar a ordem de checagem no `JoinGame`: 1) Já finalizou (`ErrAlreadyFinished`); 2) Já está ativo (`ErrAlreadyJoined`); 3) Sala trancada (`ErrRoomLocked`).

## Motivo
Manter a coerência lógica do modo Caseiro com regras de stacking, evitar acusações falsas de blefe, permitir que jogadores que saíram por engano retornem sem trapacear (recebem mão nova e vão para o fim da fila) e garantir a invariante de que um jogador nunca pode obter múltiplas colocações na mesma partida.

## Impacto
O modo Clássico e o blefe normal permanecem 100% inalterados. Salas trancadas continuam bloqueando novos jogadores e reentrantes com a mensagem de trancamento. Jogadores já colocados não podem reentrar sob nenhuma circunstância.

---

# Decisão: registrar file_id autenticado para sticker cinza de Trocar cartas

## Data
2026-09-26

## Contexto
O sticker cinza de Trocar cartas causava erro `400 DOCUMENT_INVALID` no `answerInlineQuery` porque o `file_id` configurado não era reconhecido pela Bot API (`400 wrong file_id` em `getFile`). Um contorno textual provisório havia sido colocado, mas causava inconsistência visual no menu inline.

## Decisão tomada
Fazer o upload direto do asset `assets/stickers/swap_hands_grey.png` via `sendSticker` autenticado pelo bot oficial, obter o `file_id` gerado pelo próprio Telegram (`CAACAgEAAxkDAAMoarc5AnTTNQ_W6bTz1yaQVlhRR20AAtwHAAJGhrhFYMGxG-e10Xc9BA`), atualizar `StickersGrey["swap_hands"]` e remover o contorno de `InlineQueryResultArticle`, restaurando `InlineQueryResultCachedSticker` nativo para a carta indisponível.

## Motivo
Garantir que o Telegram reconheça e sirva o sticker no inline mode sem expirar ou falhar por descompasso de credenciais entre bots, preservando a interface uniforme de cartas na mão do jogador.

## Impacto
A carta Trocar cartas quando indisponível volta a ser visualizada como sticker cinza escurecido idêntico às demais cartas bloqueadas, sem erros 400 e sem tokens de jogada.

---

# Decisão: distinguir threads comuns de tópicos de fórum

## Data
2026-09-25

## Contexto
Usuário recebe aviso de fórum usando /entrar em grupo sem tópicos. O filtro bloqueava IsTopicMessage ou qualquer MessageThreadID.

## Decisão tomada
Usar exclusivamente IsTopicMessage como indicador de tópico nos handlers de comandos e reset. Preservar autenticação, restrições de chat e bloqueio de tópicos reais.

## Motivo
MessageThreadID também identifica threads comuns conforme contrato Message da Bot API; sua presença não confirma tópico de fórum.

## Impacto
Comandos passam a funcionar em threads comuns. Sem suporte adicional a tópicos ou mudança de roteamento de partidas. Payload real do incidente não foi capturado; homologação no grupo continua pendente.

---

# Decisão: entrega administrativa descartável por build tag

## Data
2026-09-25

## Contexto
Usuário solicitou entregar cartas existentes a jogadores, exclusivamente pelo ID 7595607953, e escolheu excluir a ferramenta do build normal.

## Decisão tomada
Isolar handler, serviço, método transacional da engine e testes em arquivos com tag debugcards. Um hook no handler normal chama implementação tagged ou stub sem efeito. Não acrescentar tipos de ação/evento permanentes nem registrar ajuda/comandos públicos. Entrega usa CardsDrawn existente e revisão estrita.

## Motivo
Permitir remover a ferramenta por build e por arquivos, mantendo autorização no serviço e conservação do inventário. Evitar alterar regras e evidências de efeitos pendentes.

## Impacto
Build de teste requer -tags debugcards. Build normal, Docker e pipeline não incorporam a funcionalidade. Desativação exige trocar executável/reiniciar; não oculta fontes do repositório. Sem mudanças de configuração ou dependências.

---

# Decisão: representar troca indisponível como texto

## Data
2026-09-25

## Contexto
Resposta inline falha com DOCUMENT_INVALID quando jogador com carta de troca fica impedido de usá-la. Usuário autorizou correção sem plano.

## Decisão tomada
Restaurar artigo textual apenas para SwapHands indisponível, sem token de ação. Preservar sticker colorido e regras existentes.

## Motivo
Retirar da resposta o documento cinza suspeito sem impedir a visualização da carta ou alterar a partida. A causa remota permanece hipótese até homologação.

## Impacto
Carta bloqueada aparece como texto no menu. Asset/ID cinza permanecem para investigação. Nenhuma mudança em regras, turnos ou desafio de blefe.

---

# Decisão: Troca de mãos como escolha de jogador exclusiva do caseiro

## Data
2026-09-25

## Contexto
O usuário pediu uma carta com sticker próprio que troca mãos completas após seleção de jogador em menu semelhante ao coringa, apenas no modo caseiro. Aprovou o plano com "sim".

## Decisão tomada
Adicionar uma carta SwapHands no baralho caseiro (109 cartas), controlada por AllowSwapHands. Criar ChoosingPlayer e ChoosePlayer/TargetID, mantendo a cor ativa e as restrições de coringa do modo. Proibir uso como última carta. Trocar mãos restantes atomicamente e só então anunciar UNO. Seleção usa tokens existentes com validação de ator/revisão e alvo ativo. Preservar inventários injetados com CustomDeck serializável; reconstruir apenas o padrão quando o modo muda no lobby.

## Motivo
A escolha de jogador tem semântica distinta da escolha de cor. Uma fase própria impede confusão entre alvos, mantém invariantes e permite reaproveitar segurança e apresentação inline existentes sem publicar mãos. Preservar decks injetados mantém testes determinísticos e contratos de inventário.

## Impacto
Novos valores são anexados às enumerações para preservar os antigos. Snapshots da feature exigem código compatível. Seeds do caseiro mudam de trajetória pelo inventário maior. Clássico e executável V1 permanecem com o comportamento anterior. Simulador, views, documentação e testes cobrem a nova fase. Homologação visual do sticker real continua pendente.

---

# Decisões

# Decisão: Autorização da ação CallBluff no Service da aplicação

## Data
2026-09-23

## Contexto
Ao desafiar o blefe de um +4 Coringa através do sticker inline `option_bluff`, a jogada era rejeitada com o erro `invalid action` (`⚠️ <jogador>: Jogada não aceita: invalid action. Abra Suas cartas novamente.`).

## Decisão tomada
Incluir `uno.CallBluff` no switch de autorização de ações inline em `internal/game/service.go` (`case uno.PlayCard, uno.DrawCard, uno.PassTurn, uno.ChooseColor, uno.CallBluff:`), permitindo que a ação seja devidamente autenticada e enviada à engine `uno.Game`.

## Motivo
Garantir que a ação de blefe não seja descartada prematuramente pela camada de aplicação antes de alcançar as regras da partida.

## Impacto
O blefe agora é processado normalmente pelo bot no Telegram.

---

# Decisão: Seleção de modo no lobby e travamento pós-início

## Data
2026-09-23

## Contexto
Ao enviar o comando `/novo`, a mensagem do lobby exibia o botão "🃏 Suas cartas", mesmo sem nenhuma carta distribuída aos jogadores. O usuário solicitou que esse botão fosse substituído pela seleção de modo (Clássico ou Caseiro) e que essa seleção permanecesse disponível até o início da partida com `/iniciar`, momento a partir do qual a alteração de modo não é mais permitida.

## Decisão tomada
1. Na engine (`internal/uno`):
   - Adicionar `SetRules ActionType = 11` e `RulesChanged EventType = "rules_changed"`.
   - Adicionar campo `Rules Rules` em `Action`.
   - No `Game.Apply`, permitir a ação `SetRules` exclusivamente quando `Phase == Lobby`, atualizando as regras e emitindo `RulesChanged`. Se já iniciado, rejeitar com `ErrGameStarted`.
2. No serviço (`internal/game`):
   - Autorizar `uno.SetRules` no mesmo chat para o responsável (`actor.PlayerID == entry.ownerID`).
   - Adicionar método `s.SetRules(ctx, actor, gameID, rules)`.
3. No teclado do Telegram (`internal/telegram/commands.go`):
   - Em `makeGameButtons`, durante `Phase == uno.Lobby`, retornar botões inline de seleção de modo: `[ ✅ 🎻 Clássico ]  [ 🏠 Caseiro ]` (ou vice-versa com base nas regras ativas).
   - Durante as fases de jogo ativo (`TakingTurn`, `ChoosingColor`), retornar exclusivamente `[ 🃏 Suas cartas ]`.
4. No handler de callbacks (`internal/telegram/callbacks.go`):
   - Tratar prefixos `mode_classic_` e `mode_caseiro_`.
   - Validar que a partida ainda está em `Phase == Lobby`. Se já iniciada, exibir alerta no Telegram informando que o jogo já começou e o modo não pode ser alterado.
   - Validar que o solicitante é o responsável pela partida (`view.OwnerID`). Se não for, alertar que apenas o responsável pode alterar.
   - Atualizar as regras via `s.SetRules` e editar em tempo real o texto do lobby (`RenderLobby`) e os botões (`makeGameButtons`).

## Motivo
Eliminar o botão prematuro "Suas cartas" durante o lobby, proporcionar uma experiência intuitiva e rápida de configuração de modo diretamente no grupo e garantir que as regras da partida fiquem travadas após o início.

## Impacto
Interface mais clara e organizada no lobby, sem confusão para novos jogadores e com total garantia de imutabilidade das regras após o início do jogo.

---

# Decisão: Restituição do blefe no +4 Coringa e ordenação das cartas da mão estilo V1

## Data
2026-09-23

## Contexto
O usuário relatou que a opção de blefe havia sumido quando um jogador descartava +4 Coringa, impedindo a vítima de desafiar o blefe. Além disso, as cartas da mão no teclado inline apareciam espalhadas e desordenadas (ex: vermelhas misturadas com verdes).

## Decisão tomada
1. Criar a ação `uno.CallBluff` e o evento `uno.BluffCalled` na engine.
2. Na engine (`internal/uno`):
   - Ao jogar `WildDrawFour`, verificar se o descarte foi um blefe (se o jogador possuía cartas da cor ativa na mão antes de jogar o +4) e guardar `Bluffing bool`.
   - Ao escolher a cor (`ChooseColor`), criar `s.PendingBluff = &BluffInfo{Actor: actor, Target: target, Bluffing: bluffing}`.
   - Tratar `CallBluff`: se teve sucesso (o autor blefou), o autor recebe a penalidade de compra `DrawCounter`; se não teve sucesso (não blefou), o desafiante recebe `DrawCounter + 2` cartas. Em ambos os casos a penalidade é aplicada e a vez passa para o próximo jogador.
   - Limpar `PendingBluff` caso qualquer outra ação seja executada.
3. Expor `CanCallBluff bool` na `PublicGameView` de `internal/game/views.go`.
4. No Telegram inline (`internal/telegram/inline.go`):
   - Ordenar a mão do jogador (`sortHand`) seguindo o critério da V1: Vermelho -> Azul -> Verde -> Amarelo -> Coringas (+4 e Wild), ordenados numericamente e depois por ação internamente.
   - Quando `view.Public.CanCallBluff` for verdadeiro e for a vez da vítima, anexar o sticker de blefe `option_bluff` ("BQADBAADygIAAl9XmQABJoLfB9ntI2UC").
5. No Telegram renderer (`internal/telegram/renderer.go`):
   - Renderizar o resultado da ação de blefe ("Blefe pego!" ou "<jogador> não blefou!").

## Motivo
Restauração integral da regra clássica de desafio de blefe do +4 Coringa e melhoria ergonômica da visualização de cartas na mão do jogador, garantindo organização visual limpa e paridade total com a V1.

## Impacto
Mecânica de blefe do UNO restabelecida com fidelidade à V1 e cartas perfeitamente agrupadas por cor e valor na interface inline.

---

# Decisão: Restauração integral das regras de cartas da V1 na engine V2

## Data
2026-09-23

## Contexto
O usuário solicitou o retorno de todas as regras de jogabilidade de cartas originais da V1, preservando 100% dos textos, layouts e formatações da V2 atual. Na V1 existiam comportamentos clássicos específicos como não bater com Coringa, proibição de jogar Coringa sobre Coringa, +4 livre sem restrição de cartas na mão, e a mecânica de compra livre onde o turno não passa compulsoriamente se a carta comprada for incompatível.

## Decisão tomada
1. Adicionar quatro novas flags configuráveis na struct `Rules` da engine (`internal/uno`):
   - `NoWildFinish`: impede que um jogador vença/feche o jogo descartando uma carta especial (Wild ou WildDrawFour) como sua última carta.
   - `NoWildOnWild`: impede jogar Coringa sobre Coringa (Wild sobre Wild, +4 sobre Wild, etc.) na rodada comum quando não estiver respondendo a empilhamento de penalidade.
   - `AllowWildDrawFourAlways`: permite descartar +4 a qualquer momento, sem a restrição oficial da Mattel de verificar se o jogador tem a cor ativa na mão.
   - `FreePlayAfterDraw`: após comprar uma carta voluntariamente, não passa a vez compulsoriamente; o jogador pode descartar qualquer carta válida de sua mão ou acionar a ação `PassTurn`.
2. Habilitar todas essas flags em `BotRules()` (modo Clássico do bot) e em `CaseiroRules()` (modo Caseiro do bot).
3. No `CaseiroRules()`, manter adicionalmente as regras de cruzamento de penalidades (`StackWildDrawFourOnTwo` e `StackDrawTwoOnWildFour`), respeitando a paridade com a V1.
4. Manter `ClassicRules()` sem essas flags para compatibilidade estrita do manual Mattel nos testes de conformidade.
5. Preservar inalterada toda a camada de apresentação em HTML, formatações, queries dinâmicas, menções seguras e reação festiva `🥳` no UNO.

## Motivo
Fidelidade total à experiência de jogo consolidada na versão 1 do UnoBotGO, mantendo a estabilidade arquitetural e a segurança de concorrência da V2.

## Impacto
Jogabilidade idêntica ao bot clássico, sem quebras visuais e com suite de testes 100% aprovada.

---

# Decisão: Empilhamento de +4 coringa e invalidação de cache de cartas

## Data
2026-09-23

## Contexto
Após jogar um +4 Coringa, o bot na engine V2 aplicava imediatamente a compra de 4 cartas e pulava compulsoriamente a vez do próximo jogador, tirando a interatividade e a possibilidade de rebater (+4 sobre +4). Além disso, os botões inline estáticos com `g_<GameID>` faziam com que o cache do cliente do Telegram entregasse a visão de seleção de cores do primeiro jogador quando outro jogador abria o botão "Suas cartas".

## Decisão tomada
1. Adicionar `StackWildDrawFour: true` em `BotRules()`, permitindo que ao escolher a cor do +4 a penalidade seja acumulada em `DrawCounter` e o próximo jogador receba a sua vez normalmente para rebater com outro +4 ou recolher as cartas voluntariamente.
2. Anexar a revisão atual da partida no botão `🃏 Suas cartas` (`g_<GameID>_<revision>`), tornando a query string sempre dinâmica a cada turno/jogada para invalidar o cache local dos clientes do Telegram.
3. Permitir parse de query inline flexível ignorando sufixos após o `GameID`.
4. Durante `ChoosingColor`, exibir exclusivamente o aviso de espera e o resumo das próprias cartas para jogadores espectadores/não-escolhedores, sem expor seletores de cor nem stickers.

## Motivo
Eliminar a sensação de pulo de turno automático e garantir que cada jogador veja com fidelidade suas próprias cartas no Telegram.

## Impacto
Fluxo de jogo do +4 alinhado ao comportamento esperado do bot, com controle completo de turnos e sem vazamento de opções ou cache compartilhado entre usuários.

---

# Decisão: Correção de estado global e concorrência V1

## Contexto
O bot mantinha estado global em memória e apresentava problemas de cartas fantasmas ao limpar lobbies não iniciados (vazamento de referências) e exibição incorreta de cartas em inline query ao jogar em múltiplos chats devido à falta de contexto do chat nas requisições inline.

## Decisão tomada
1. Reverter a exibição de ChatID no input do inline query, mantendo a caixa de texto limpa.
2. Sincronizar o ponteiro global de jogo ativo `UserIDCurrent[userID]` de forma proativa e invisível ao usuário:
   - A cada mensagem ou comando recebido do usuário no grupo (em `handleMessage`), caso ele tenha um player associado àquele chat.
   - A cada transição de turno no jogo (ao iniciar a partida, ao passar o turno após uma jogada inline, ao kickar, skippar ou quando um jogador sai).
3. Encapsular a limpeza de referências de jogadores de um jogo na função privada `removeGamePlayers` do `GameManager` e utilizá-la tanto no encerramento normal/forçado quanto na limpeza de lobbies não iniciados (`CleanGames`).
4. Implementar getters thread-safe (`GetCurrentPlayer` e `GetPlayersForUser`) para ler os mapas globais sob Mutex.
5. Adotar o mecanismo de `AntiCheat` idêntico ao repositório original `jh0ker/mau_mau_bot` para invalidar naturalmente o cache local de inline queries no Telegram:
   - Anexar o sufixo `:AntiCheat` nos IDs dos resultados inline (ex: `r_7:0`).
   - Extrair, validar e incrementar o `player.AntiCheat++` no `handleChosenInlineResult`.
6. Corrigir a rotação de turno (`Turn`) e listagem de jogadores (`Players`) em `game.go` para navegar no sentido anterior (`.Prev`) quando `Reversed` for `true`.
7. Otimizar a liberação de locks de `game.Lock()` no `handleChosenInlineResult` para ser realizada de forma antecipada (antes de chamadas de rede do Telegram e do `UpdateCurrentPlayer`), prevenindo deadlocks.

## Motivo
Garante a integridade do estado e evita o descompasso na sincronização do inline query sem expor IDs internos no campo de texto de digitação do Telegram. A liberação antecipada de locks previne deadlocks de Mutex e otimiza a latência. A correção em game.go alinha a rotação ao comportamento real de jogo UNO.

## Impacto
O bot agora é estável em cenários com múltiplos jogos paralelos e limpa totalmente a memória ao cancelar lobbies, sem deixar resíduos de cartas ou sessões fantasma para os usuários. A carta reverse agora muda de fato a rotação do jogo e não causa travamentos concorrentes.

# Decisão: fronteira da engine V2 e políticas de rodada

## Data
2026-09-14

## Contexto
O usuário aprovou a implementação da Milestone 1 após auditoria do V1. Escolheu
bloquear +4 ilegal inicialmente, manter UNO automático e preservar continuidade
por colocação/entrada tardia. Inline multigrupo usará seleção explícita futuramente.

## Decisão tomada
Criar internal/uno sem Telegram, SQL ou estado global. Game encapsula State, ações
são aplicadas numa cópia e só confirmadas após validação; revision incrementa uma
vez por ação aceita. Snapshot/Restore fazem cópia profunda, e cartas físicas têm
IDs. Regras Classic são independentes das políticas FirstWinner/Placements e
AllowLateJoin. Dez registrados por jogo, sem reentrada; falta de cartas falha
atomicamente. Cor ativa não modifica a carta Wild. Challenge fica para depois.

## Motivo
Permitir testes determinísticos e futura troca de interface, eliminar acoplamento
e corrupção parcial, e preservar a dinâmica desejada sem chamar as adaptações de
regras oficiais. A implementação V1 não é substituída durante construção da engine.

## Impacto
Nova engine com biblioteca padrão apenas. M2 adicionará serviço, índices,
MemoryRepository e mutex privado por jogo. A engine requer acesso serializado;
Estado não serializa locks/RNG. M3 conectará Telegram, snapshots privados serão
convertidos em views autorizadas, tokens vincularão resultados a usuário e partida.
Ranking/Match não entram na engine. PostgreSQL do V1 permanece intacto.

## Documentação
- docs/v2-rules.md: API, diferenças de regras, atomicidade e limites.
- docs/v2-audit.md: arquitetura V1, riscos e fatos verificados na Bot API.


# Decisão — M2: aplicação, administração independente e ownership único

## Data
2026-09-15

## Contexto
M1 possui engine de dono único e snapshots privados completos. Usuário aprovou
M2 com criador não participante, gestão independente, múltiplos chats por jogador
e retenção de somente resumos públicos dos últimos 100 jogos encerrados.

## Decisão tomada
Service exportado e manager privado em internal/game. Runtime único por partida,
mutex privado e índices publicados em seção curta sob lock global. Não criar
MemoryRepository: não há responsabilidade distinta sem persistência real.
Start/Cancel usam solicitante real, autorizado pelo OwnerID no serviço; exceção
mínima da guarda de participação na M1, mantendo dealer ativo e validações de regras.
Views não expõem snapshots. PlayerView não recebe target: somente mão do Actor.
Encerramento libera índices/runtime e arquiva só view pública com FIFO configurável.

## Motivo
Evitar duas fontes de verdade, falsificação do autor administrativo, inscrição
implícita, vazamento de mãos e serialização desnecessária entre partidas.

## Impacto
M3 consumirá Service e fará autenticação/seleção multigrupo. Sem tokens ou Telegram
nesta entrega. Context cancelado após aceitação não interrompe publicação. Recovery
futuro exige envelope State+metadata e reconstrução de índices, fora de I/O sob locks.
Engine permanece independente; V1 e dependências/configuração permanecem intactos.

## Documentação
- docs/v2-application.md: API, autorização, lifecycle, locking, privacidade e limites.
- docs/v2-rules.md: contrato administrativo mínimo atualizado da M1.

# Decisão — M3: Playable Telegram MVP, concorrência e tokens de uso único

## Data
2026-09-15

## Contexto
A Milestone 3 conecta a aplicação V2 ao Telegram sem acoplamento de engines ou banco de dados externo. O V1 sofria com descompasso de cache (`omitempty` omitindo `cache_time:0`), concorrência não serializada por chat, locks retidos em I/O de rede e identificação frágil de partidas inline.

## Decisão tomada
1. Executável V2 em `cmd/bot/main.go` consumindo `internal/game.Service`.
2. Dispatcher com 8 filas particionadas por `ChatID` (buffer 32) para mensagens e jogadas, e 4 workers separados (buffer 64) para consultas inline.
3. Tokens criptográficos de 128 bits (`TokenStore`) para vincular cada resultado inline a uma ação de revisão estrita, consumidos atomicamente e invalidados por evento ou tempo.
4. `InlineRequestConstructor` para contornar `omitempty` na serialização de `answerInlineQuery`, emitindo `cache_time:0` e `is_personal:true`.
5. `SafeAPICaller` com sanitização de logs (nunca expor tokens ou URLs autenticadas) e retry limitado exclusivo para HTTP 429.
6. Isolamento total entre V1 e V2: nenhum código legado é alterado.

## Motivo
Garantir concorrência estritamente ordenada por grupo, respostas inline imediatas e não bloqueantes, segurança contra anti-cheat/repetição e conformidade com as regras de confidencialidade e integridade do bot.

## Impacto
O V2 torna-se jogável de ponta a ponta no Telegram. A engine e o serviço mantêm zero dependência de transporte. Sem persistência externa, ranking ou Match nesta milestone.

## Documentação
- docs/v2-telegram.md: arquitetura detalhada e roteiro de homologação manual.
- README.md: visão geral e comandos.

# Decisão: Paridade V1 de empilhamento +2, seletor de cor limpo e grito de UNO

## Data
2026-09-15

## Contexto
Na jogabilidade do V1, jogar um `+2` não pulava o oponente imediatamente; transferia o turno para que o jogador pudesse ver suas cartas e rebater com outro `+2` (acumulando a penalidade) ou comprar as cartas acumuladas. No seletor de cor, o jogador recebia 4 opções de cor limpas e um 5º artigo de resumo da mão, sem stickers cinzas misturados. Por fim, o anúncio de UNO devia ser enviado em mensagem dedicada no grupo com reação festiva 🥳.

## Decisão tomada
1. Adicionada regra `StackDrawTwo: true` a `uno.BotRules()` e campo `DrawCounter int` em `uno.State`.
2. Em `internal/uno/game.go`:
   - Quando `s.Rules.StackDrawTwo && card.Rank == DrawTwo`: `s.DrawCounter += 2` e a vez passa para o próximo jogador sem skip.
   - Enquanto `s.DrawCounter > 0`: apenas cartas `DrawTwo` podem ser jogadas; se o jogador comprar, recebe `s.DrawCounter` cartas de uma vez, zera o contador e o turno avança.
3. Em `internal/telegram/inline.go`:
   - Na fase `ChoosingColor`, se o jogador for o selecionador da cor, retorna exclusivamente 4 artigos de cor ("Escolha sua cor") e 1 artigo de resumo das cartas da mão ("Cartas: ..."), retornando imediatamente sem misturar stickers cinzas.
   - Na fase `TakingTurn` com `DrawCounter > 0`, o sticker de compra exibe "Comprando X cartas" e as cartas não-+2 da mão ficam desabilitadas (cinzas).
   - Ao detectar `uno.UnoAnnounced` em `HandleChosenInlineResult`, o bot envia uma mensagem separada no grupo (`<link> <b>Gritou UNO!</b>`) e aplica a reação `🥳` via `SetMessageReaction`.
4. Interface `BotAPI` estendida com `SetMessageReaction(ctx context.Context, params *telego.SetMessageReactionParams) error`.

## Motivo
Garantir paridade completa com a dinâmica clássica apreciada pelos jogadores no V1, eliminando a frustração de perder o turno compulsoriamente sem poder visualizar a mão ou rebater com outro +2, mantendo a interface inline limpa na escolha de cores e celebrando o grito de UNO com a reação festiva solicitada.

## Impacto
Controle total do jogador preservado, interface do seletor alinhada ao V1 e suporte nativo à reação festiva via Bot API sem introduzir regressões ou alterar a separação arquitetural da engine.

# Decisão: Simplificação de mensagens, remoção do botão de atualizar e liberação de /iniciar

## Data
2026-09-15

## Contexto
O usuário solicitou remover o botão "🔄 Atualizar estado" do teclado inline nas mensagens do jogo, remover a linha de cabeçalho `🃏 UnoBotGO`, ocultar a contagem de cartas `(X cartas)` dos jogadores nas mensagens públicas da mesa e permitir que qualquer membro do chat possa dar `/iniciar` (e não apenas quem criou a partida via `/novo`), mantendo `/cancelar` exclusivo do responsável.

## Decisão tomada
1. Em `internal/game/service.go`: autorização de `uno.StartGame` atualizada para exigir apenas que o autor esteja no chat da partida (`actor.ChatID != 0`), sem exigir que seja `entry.ownerID`. O comando `uno.CancelGame` permanece restrito exclusivamente ao `entry.ownerID`.
2. Em `internal/telegram/commands.go`: `makeGameButtons` simplificado para conter apenas o botão `🃏 Suas cartas`, eliminando o botão `🔄 Atualizar estado`.
3. Em `internal/telegram/renderer.go`:
   - Removido cabeçalho `🃏 <b>UnoBotGO</b>\n\n` de `RenderPublicState`.
   - Na lista "Jogadores em jogo:", removida a contagem `(X cartas)`. Se o jogador estiver com 1 carta, exibe `⚠️ <b>UNO!</b>`.
   - Em `RenderLobby`, atualizado o texto para `Use /iniciar para começar!`.

## Motivo
Atender à preferência do usuário por mensagens mais compactas e limpas no grupo, manter a privacidade das mãos (sem expor a contagem exata de cartas de cada um a cada lance) e trazer paridade com o V1 onde qualquer membro podia iniciar a partida quando houvesse quórum.

## Impacto
Mensagens no chat do Telegram ficam mais limpas, diretas e com menos botões desnecessários, melhorando a experiência mobile dos usuários.



# Decisão

## Data
2026-09-16

## Contexto
A Milestone 6 adiciona Webhook como transporte alternativo ao long polling.

## Decisão tomada
Usar um pipeline único de updates, servidor `net/http` interno com validação de `X-Telegram-Bot-Api-Secret-Token`, `SetWebhook` explícito em todo startup webhook, `DeleteWebhook(false)` ao iniciar polling e deduplicação em memória por `UpdateID`.

## Motivo
Preservar a jogabilidade existente, aplicar alterações de segredo sem operação manual e evitar processamento duplicado sem adicionar infraestrutura persistente.

## Impacto
Webhook exige URL HTTPS pública terminada externamente; a deduplicação é perdida após reinício.


# Decisão: encerramento e menções contextuais do V2

## Data
2026-09-23

## Contexto
A confirmação final oferecia acesso a uma mão indisponível. O scheduler podia publicar timeout fora de ordem. Todos os nomes apontavam ao próprio jogador. O usuário aprovou o plano corretivo e decidiu preservar a regra terminal existente de stacking.

## Decisão tomada
- Preservar a engine; teclados e erros seguem a view atual/terminal. Invalidar tokens existentes no encerramento natural ou por saída, mantendo validação server-side como autoridade.
- Separar descoberta de candidatos de timeout da aplicação; GameID, ChatID, jogador, revision, fase e prazo são revalidados sob mutex. Mutação e mensagem executam na mesma fila de chat. Limpar turnStarted ao encerrar.
- Centralizar destino das menções em Renderer.PlayerLink: UserID real somente para responsável atual, BotID nos demais casos. Usar view resultante em confirmações/eventos; obter BotID de GetMe antes do ingresso.
- Manter g_<GameID> visível, separado dos tokens de ação one-use. Não há contexto oculto equivalente no botão Inline Mode. Não adicionar aliases, mudar formato ou usar seleção global por usuário.
- Preservar penalidades imediatas, cor pendente e stacking terminal atual (sem nova compra automática nem chance de rebater). Não modificar gameplay, infraestrutura ou transportes.

## Motivo
Corrigir os caminhos que oferecem ou anunciam turnos obsoletos sem inventar outra fonte de estado nem sacrificar segurança/multigrupo. A política de menções precisa refletir o responsável posterior à ação, não seu autor anterior.

## Impacto
Final sem convite para jogar; candidates antigos tornam-se no-op; mesma implementação em polling e webhook. Novas mensagens apontam ao bot exceto pelo jogador responsável. Mensagens históricas não são reescritas em massa. Aceitação visual de tg://user?id=<BotID> continua pendente em clientes reais, conforme roteiro documentado.

# Decisão: simulador local desacoplado dos transportes

## Data
2026-09-23

## Contexto
Era necessário executar partidas automáticas com quantidade e modo escolhidos pelo operador, detectar falhas e explicar cartas especiais sem depender do Telegram ou duplicar as regras da engine.

## Decisão tomada
Criar `internal/simulation` sobre a API pública de `internal/uno` e expô-lo por `cmd/simulator`. Usar uma seed única para embaralhamento e decisões, validar o snapshot após cada ação e gerar relatório Markdown a partir de ações, eventos e estados resumidos. Manter o simulador fora de `internal/game` e `internal/telegram`.

## Motivo
A engine é a fonte de verdade das regras e já oferece ações transacionais, snapshots, `CanPlay`, shuffler injetável e validação estrutural. Um adaptador local separado testa esse contrato sem credenciais, rede, banco ou efeitos no bot em produção.

## Impacto
Partidas de 2–10 bots nos modos Clássico e Caseiro podem ser reproduzidas por seed. Relatórios ficam em `.reports/simulations/`, fora do Git. O simulador cobre lógica de jogo; transporte, stickers, callbacks e filas Telegram permanecem fora de seu escopo.
# Decisão: recuperação isolada e geração por grupo

## Data
2026-09-24

## Contexto
O bot legado em Python podia deixar um grupo lento ou sem respostas após uma ação desconhecida. Na V2, grupos compartilham workers particionados e um comando colocado na fila comum não conseguiria recuperar um shard saturado.

## Decisão tomada
Processar `/reset` em uma fila administrativa independente. Após autenticar o responsável ou administrador do grupo, cancelar o contexto anterior, avançar a geração do chat e encaminhar novos trabalhos para uma fila dedicada limpa. Remover atomicamente no serviço a partida ativa, índices, histórico e runtime daquele chat, tombstonar referências antigas e invalidar os tokens retornados. Proteger todas as classes de worker com recuperação de panic.

## Motivo
A via de recuperação precisa continuar acessível quando o caminho comum falha e precisa isolar ações antigas sem reiniciar o bot inteiro ou interromper outros grupos. A autorização via Telegram evita que um membro comum use a limpeza para sabotar partidas.

## Impacto
O grupo pode criar uma nova partida imediatamente após o reset. Tarefas da geração anterior são descartadas e o estado removido não pode ser republicado por referências antigas. O mecanismo não recupera processo morto, indisponibilidade global da API ou código externo que ignore cancelamento; esses casos ainda dependem do supervisor do processo e dos timeouts de rede.

---
# Decisão: imagem OCI AMD64 e ARM64

## Data
2026-09-24

## Contexto
A publicação da `main` gerava somente `linux/amd64`, impedindo o uso direto da mesma tag oficial em servidores ARM64. A branch pública também precisa continuar separada dos artefatos internos existentes em `dev`.

## Decisão tomada
Construir `linux/amd64` e `linux/arm64` com Buildx, usando `$BUILDPLATFORM` no estágio Go e `$TARGETOS/$TARGETARCH` no cross-compile. Publicar um único manifest list nas tags `latest` e `sha-<commit>`. Continuar promovendo a `main` por allowlist sobre seu histórico próprio.

## Motivo
Uma referência multi-arquitetura simplifica deploys e evita depender de emulação para compilar o binário. A allowlist mantém código V1, relatórios e memória de agentes fora da distribuição pública.

## Impacto
Docker seleciona automaticamente a imagem AMD64 ou ARM64. O CI de `dev` passa a validar ambas sem publicar; a `main` publica ambas após os testes. O tempo do build remoto pode aumentar por produzir duas variantes.

---
# Decisão: preservar contador no empilhamento cruzado Caseiro

## Data
2026-09-24

## Contexto
Ao responder um `+2` com `+4` no modo Caseiro, a engine permitia a jogada, mas substituía a penalidade pendente de 2 por 4 durante a escolha de cor.

## Decisão tomada
Somar o valor do `+4` ao `DrawCounter` existente quando qualquer regra compatível de empilhamento do `+4` estiver ativa. Manter os modos e o baralho sem alterações.

## Motivo
Empilhamento representa uma única penalidade acumulada. Resolver a escolha de cor não deve apagar cartas já pendentes.

## Impacto
`+2 → +4` passa a 6 e pode continuar acumulando. O Clássico não passa a aceitar combinações cruzadas, pois a validação de jogabilidade e suas flags não mudaram.

---

# Decisão: sticker cinza para Trocar cartas

## Data
2026-09-25

## Contexto
A carta Trocar cartas não possuía variante visual desabilitada. Quando não era jogável, o adapter inline substituía somente essa carta por um artigo textual, diferente das demais cartas da mão.

## Decisão tomada
Criar uma variante estática cinza da arte existente, registrá-la pela Bot API com o file ID `CAACAgEAAxkBAAER8aRqtlf6ZtRKfAj02K5AnlVcRz_W_AACVAcAAkaGsEXgXGCANqlQKz0E` e usar o fluxo genérico de `InlineQueryResultCachedSticker` indisponível. O resultado mantém prefixo `grey_` e não recebe token de ação.

## Motivo
Manter consistência visual na mão e conservar a mesma proteção já usada pelas outras cartas indisponíveis.

## Impacto
A carta indisponível aparece escurecida e sua seleção não executa jogada. Regras, frequência e sticker colorido permanecem iguais.

---

# Decisão: bloquear +4 sobre +4 somente no Caseiro

## Data
2026-09-25

## Contexto
O Caseiro herdava de `BotRules` a permissão de responder uma penalidade `+4` com outro `+4`. A mensagem pública também repetia a direção em uma linha textual e nas setas da lista de jogadores.

## Decisão tomada
Sobrescrever `StackWildDrawFour` para `false` em `CaseiroRules`, preservando as flags independentes de respostas cruzadas. Remover a linha textual de direção e manter `➡️`/`⬅️` entre os jogadores. Não alterar `BotRules`.

## Motivo
Aplicar a regra solicitada apenas ao modo citado e retirar informação visual duplicada sem esconder o sentido da rodada.

## Impacto
No Caseiro, `+4 → +4` é indisponível, enquanto `+2 → +4` e `+4 → +2` continuam válidos. O modo Clássico mantém seu comportamento. O estado Telegram fica mais compacto.

---

# Decisão: comandos Telegram por contexto

## Data
2026-09-25

## Contexto
O chat privado reutilizava a ajuda completa como resposta de `/start`, e o menu padrão expunha comandos de grupo em todos os contextos. Não havia acesso direto para adicionar o bot a um grupo.

## Decisão tomada
Separar `/start` de `/help`, gerar o deep link de grupo com o username retornado por `GetMe` e registrar comandos nos escopos padrão, privado e grupos. Tratar o payload `startgroup=true` como confirmação de adição, sem iniciar partida. Manter `/ajuda`, `/kill` e `/start` sem payload em grupo como aliases compatíveis.

## Motivo
Dar ao primeiro contato uma apresentação curta, manter a ajuda legível e mostrar em cada chat somente os comandos relevantes.

## Impacto
O privado mostra `/start` e `/help`; os grupos mostram comandos de partida e `/help`. Mudanças de username passam a exigir apenas reinício do bot, sem alteração de código.

---


# Decisão: status operacional separado da arquitetura

## Data
2026-09-26

## Contexto
Documentação histórica não distinguia maturidade nem diferenças atuais entre árvores independentes.

## Decisão tomada
Manter docs/project-status.md público e específico quanto às branches, com dimensões separadas de implementação, testes e homologação. Promover somente esse documento e README nesta milestone, sem merge.

## Motivo
Evitar tratar commit, CI ou simulação como homologação Telegram e não publicar novidades de gameplay incidentalmente.

## Impacto
Codemaps permanecem internos/históricos; nenhuma arquitetura ou gameplay alterados. Polling recomendado; webhook experimental por aceite real insuficiente. Default main desejada depende de autenticação administrativa indisponível neste ambiente.


# Decisão: admissão como metadata e ordem lógica no renderer

## Data
2026-09-26

## Contexto
Milestone solicita late join justo, controle de entradas e mensagens compactas. A engine auditada já insere corretamente na cauda lógica; exibição anterior usa ordem física.

## Decisão tomada
Preservar algoritmo de entrada e cobri-lo com regressões; renderer percorre a ordem a partir do atual conforme Direction. Locked pertence a managedGame, com autorização exclusiva de owner e checagem atômica com Join sob entry.mu. Lock/unlock não incrementa revisão da engine.

## Motivo
Evitar alterar Reverse ou adicionar regras de elegibilidade futuras; administração não é jogada e não deve invalidar tokens nem mexer no prazo do turno.

## Impacto
Estado de sessão exposto publicamente sem mãos; zero persistência nova. Ordem visual representa o próximo ciclo no sentido atual. Reverse/Skip futuros continuam produzindo seus efeitos normais. Alterações somente dev; homologação Telegram pendente.

# M7 — persistência síncrona e fundação PostgreSQL

## Data
2026-09-27

## Contexto
HEAD auditado e72cd66 na dev. Plano M7 aprovado com alterações explícitas do usuário.

## Decisão tomada
pgx/v5 e pool, migrations SQL embutidas aplicadas explicitamente pelo cmd/migrate, ledger com checksum e advisory lock transacional. Bot exige DATABASE_URL, conexão e schema atual antes do Telegram. Finalização aguarda transação/commit; sem worker de resultados, outbox ou snapshots privados. Resultado pendente fica em memória para retry, sem promessa de sobrevivência a crash.

## Motivo
Manter gameplay em memória e separar dados duráveis, garantindo idempotência por GameID sem infraestrutura adicional. Nenhuma regra competitiva bloqueada foi decidida.

## Impacto
M7.1 exige preparar PostgreSQL antes de iniciar cmd/bot. Engine, simulator e testes unitários continuam independentes do banco. CI ganha PostgreSQL isolado. Integração dos resultados será feita nas próximas submilestones.

# M7 — scores e fechamento sem política inventada

## Data
2026-09-27

## Contexto
Fórmulas aprovadas, mas elegibilidade/requisitos mínimos/abandono/late join ainda bloqueados.

## Decisão tomada
Unidades int64 em centésimos, Updated arredondado half-up a duas casas. DTO público final guarda apenas participantes, nomes, colocações existentes e metadados de participação. Resultado sem policy_version é persistido como needs_product_decision, score NULL, sem tocar stats. Runtime ainda não ativa uma política competitiva. Foundation transacional aceita scores calculados somente com policy_version explícita, validando fórmula. Nenhuma política de produção foi criada.

## Motivo
Implementar cálculo, auditoria e transação testável sem decidir elegibilidade arbitrária. Resultado imutável retido em mapa de memória independente do histórico; adapter aguarda COMMIT fora dos locks de game e reconhece somente sucesso. Sem worker/outbox.

## Impacto
Partidas oficialmente encerradas já podem persistir auditoria; canceladas/ativas não. Ranking automático e mensagens de pontos continuam pendentes de decisão. Resultados pending não são promovidos/recalculados automaticamente; aplicação futura exige operação auditável específica. Falha de conexão mantém resultado para retry síncrono; crash antes de COMMIT ainda perde RAM.

# M7 — elegibilidade por conclusão aprovada

## Data
2026-09-27

## Contexto
Após auditoria do HEAD 2b52344, o usuário definiu a regra definitiva e autorizou implementação na dev, com commits sem push.

## Decisão tomada
N é exclusivamente a quantidade de concluintes com placement válido. Abandonados definitivos e participantes apenas do lobby ficam na auditoria com posição ausente e zero pontos, fora de N e das stats de partidas concluídas. Late join e saída/reentrada não penalizam quem concluiu. Departure usa os placements reais; N<2 é persistido sem concessão para ambos sistemas. Cancelled continua excluído. Sem cronologia extra ou alteração em internal/uno.

## Motivo
Aplicar as regras explícitas de produto, preservando engine/placements e fórmula/centésimos/half-up da fundação. Não há necessidade de distinguir late join original de reentrada para elegibilidade.

## Impacto
Policy completed-placements-v1 é aplicada numa cópia do resultado final. Validação rejeita placement duplicado, posição em Left, gaps e score divergente. N<2 será identificado no storage como insufficient_eligible_players. Dados antigos pending não serão pontuados retroativamente.

# Decisão: troca opcional e resolução por cor

## Data
2026-09-28

## Contexto
A base bce47d0 obrigava troca em ChoosePlayer, mantinha cor e proibia última carta. Usuário aprovou explicitamente as exceções após auditoria.

## Decisão tomada
- Última SwapHands encerra autor pelo lifecycle normal, sem escolher alvo/cor ou transferir mão vazia. Cor anterior permanece se houver continuidade.
- Com cartas restantes, ChoosePlayer/KeepHand guarda decisão; ChooseColor resolve mãos, cor e turno atomicamente. Uma revisão por ação aceita, não uma por fluxo inteiro.
- Preservar timeout atual: nenhuma expiração/seleção automática nas escolhas pendentes.
- Saída do alvo invalida a seleção e exige nova escolha alvo/manter, sem fallback para outro jogador.

## Motivo
Evitar troca obrigatória e término artificial de um alvo por mão vazia; preservar invariantes, tokens e arquitetura de escolhas.

## Impacto
KeepHand e HandKept explícitos; ColorChoice reutilizado com metadados de troca. Snapshot pendente novo requer runtime compatível. Sem mudança de M7/GroupConfig/ranking, Classic, transporte ou banco. Somente dev; sem push, main intacta.


# Decisão: hero integrado e fallback de apresentação

## Data
2026-09-30

## Contexto
Usuário rejeitou card interno do hero; nome '.' é preservado intencionalmente no backend a partir de dados observados do Telegram.

## Decisão tomada
Remover a aparência de caixa interna e compactar somente estilos do detalhe. Aplicar nome neutro 'Jogador' apenas no frontend para nomes sem conteúdo visual útil (pontuação/espaços/controles), inclusive iniciais do avatar.

## Motivo
Atender composição visual solicitada sem alterar dados históricos, API, contratos ou semântica competitiva.

## Impacto
Ranking/ordenação/pontuação intactos; fallback vale para apresentação de jogadores no Mini App. Homologação visual no celular pendente, anexos novos não recebidos.


# Decisão: fidelidade visual acima de densidade no detalhe

## Data
2026-09-30

## Contexto
Usuário reprovou a miniaturização e forneceu referência aprovada; o objetivo anterior de compactação deixou de ser critério visual.

## Decisão tomada
Recuperar escala do hero, score, avatar, cartas físicas, sheet e rows, individualmente, com presença e profundidade. Manter resumo diretamente no hero e desktop com as mesmas dimensões internas do mobile.

## Motivo
A referência e a nova hierarquia solicitada têm precedência sobre números/densidade da rodada anterior.

## Impacto
Alteração somente de apresentação, preservando backend e regras. Mais altura visual é intencional. Aceite final depende de homologação pelo usuário.


# Decisão: reutilizar stickers originais como assets estáticos

## Data
2026-09-30

## Contexto
Decoração do hero era desenhada em CSS. O único PNG local era cinza; cartas coloridas originais eram referências no mapa de stickers do bot.

## Decisão tomada
Recuperar uma vez g_0/y_0/r_0 e manter WebP intactos em web/src/assets/cards, importados pelo Vite no componente HeroCards. Registrar origem sem credenciais.

## Motivo
Usar as artes reais já utilizadas pelo projeto sem nova geração de imagens, integração runtime ou endpoint de backend.

## Impacto
~31KB adicionais no bundle estático, preservando auth/API/pontuação/Telegram bot. Nova composição amplia avatar e melhora alinhamento sem recriar card interno; homologação visual pendente.


# Decisão: onda decorativa sem alteração de layout

## Data
2026-09-30

## Contexto
Design aprovado; usuário solicita apenas avatar ligeiramente maior/centralizado e duas lombadas rasas entre hero e sheet.

## Decisão tomada
SVG vetorial como máscara alpha de um pseudo-elemento da sheet, responsivo a100% de largura e24px de altura. Pseudo-elemento sobreposto ao hero, sem espaço em fluxo; contorno substitui curva anterior. Fundo horizontal compartilhado mantém continuidade. Avatar+8px com pequeno deslocamento do grid; cartas só recebem offset8px.

## Motivo
Solução localizada, sem dependência externa, sem alterar hero/lista/dados/markup funcional e com SVG fácil de manter.

## Impacto
Somente styles.css e assets/hero-wave.svg em produto. Lista e altura hero verificadas idênticas; auth/API/ranking intactos. Homologação visual pendente.


# Decisão: seção principal no footer, sistema no header

## Data
2026-09-30

## Contexto
Usuário aprovou visual e solicitou substituir segmented Grupos/Players por bottom navigation mobile, sem alterar backend/API.

## Decisão tomada
Links React Router alimentados pelos mesmos system/tab da URL, aria-current=page e SVGs sem dependências. Footer fixed restrito ao max-width do app, safe-area compartilhada com reserva de conteúdo, montado apenas nas páginas globais. Detalhe volta sempre a groups no mesmo sistema. Page controla Telegram BackButton; root App deixa de sobrescrever o estado dele.

## Motivo
Hierarquia clara entre sistema e seção, com deep links e refresh preservados, sem estado de aba duplicado. Corrigir competição de hooks no BackButton é necessário para preservar navegação direta do detalhe.

## Impacto
Só frontend: BottomNavigation, RankingsPage, CSS, App/hook de Telegram e testes. Ranking/score/paginação/autenticação/endpoints inalterados. Footer permanece disponível em loading/error/empty. Homologação visual pendente.


# Decisão: footer flutuante com vidro CSS

## Data
2026-09-30

## Contexto
Usuário pediu navegação separada em pílula, com aparência Liquid Glass, em vez de barra colada ao rodapé.

## Decisão tomada
Manter links/estado e acessibilidade da navegação existente, alterando apenas CSS para cápsula max340, margens externas/safe e material translúcido via backdrop-filter com fallback opaco. Safe area é margem inferior externa e a reserva do conteúdo acompanha a geometria.

## Motivo
Atender formato flutuante sem redesenhar app, introduzir dependências ou alterar comportamento.

## Impacto
Somente CSS e documentação nesta rodada; checks e navegação preservados. Efeito CSS inspirado no vidro, sem prometer material nativo Apple. Homologação visual pendente.

# Decisão: material óptico web da bottom navigation

## Data
2026-09-30

## Contexto
A pílula translúcida com blur intenso não apresentava a refração solicitada. Consultadas documentação Apple de adoção de Liquid Glass e HIG Materials.

## Decisão tomada
Usar deslocamento SVG do backdrop real com mapa de borda gerado localmente por canvas no resize, película mais transparente, reflexos e seleção deslizante. Manter fallback CSS e preferências de acessibilidade.

## Motivo
A aplicação React não pode usar diretamente o material nativo SwiftUI/UIKit. A solução web reproduz propriedades ópticas sem duplicar conteúdo nem adicionar dependências.

## Impacto
Somente frontend. Chromium validado com capturas comparativas: 3678 pixels alterados exclusivamente no retângulo da pílula. Safari/Telegram em dispositivo requerem homologação visual.
# Decisão: fullscreen nativo com cabeçalho próprio

## Data
2026-10-01

## Contexto
Usuário solicitou usar o cabeçalho do Mini App em vez da barra normal do Telegram.

## Decisão tomada
Solicitar `requestFullscreen()` na inicialização única do App em clientes com suporte à API 8.0; manter `expand()` como fallback, sem insistir ao navegar. Preservar controles nativos e margens seguras, atualizando layout pelos eventos fullscreen. Ajustar `setHeaderColor` por página para contraste nativo.

## Motivo
É a API oficial para fullscreen. Não existe autorização da API para remover controles nativos obrigatórios; mantê-los acessíveis evita sobreposição e preserva saída/navegação.

## Impacto
Somente integração frontend. Nenhuma alteração de dados, ranking, API ou backend. Homologação em Telegram móvel necessária; sem commit/push/deploy.


# Decisão: configuração V2 mínima e webhook derivado

## Data
2026-10-01

## Contexto
Usuário aprovou refatoração de configuração e autorizou depois commit/push dev, interrompendo verificações adicionais.

## Decisão tomada
Loader único com seis configurações normais e WEBHOOK_URL exclusiva de webhook. Políticas fixas em internal/config/defaults.go. Secret token derivado de MINIAPP_SECRET por HMAC-SHA256 com contexto unobotgo/telegram/webhook-secret/v1 e Base64 URL sem padding, separado dos contextos de refs/IDs. Direct Link /ranking construído no getMe existente.

## Motivo
A URL pública é indispensável ao setWebhook e não decorre do bind interno. A derivação elimina configuração redundante sem enviar a chave mestre e preserva autenticação em tempo constante.

## Impacto
Sem alteração de schema, ranking, auth initData, AES/refs ou frontend nesta refatoração. Testes focados e go test ./... passaram; race, vet/build completos e make check ficaram pendentes após interrupções e pedido de encerrar checks. make check recusou ausência de TEST_DATABASE_URL; race padrão recusou CGO desabilitado. Publicação somente dev por autorização posterior; sem deploy/main.

# Diagnóstico de fluidez: UnoBotGO × Matcho

Data: 2026-10-11. Escopo autorizado: inspeção do HAR, medição local com fixtures, execução de harnesses e relatório. Nenhum arquivo de produção ou dependência foi alterado nesta análise.

## Resumo

Não encontrei um limitador explícito de FPS no UnoBotGO. A cena Pixi fica parada quando não há trabalho e desenha uma vez por tick GSAP quando a cena está suja. No desenvolvimento, o `app.render()` levou 7,4–8,7 ms somados durante um gesto de arraste de 2,1–2,4 s; o wrapper de chamadas `drawArrays`/`drawElements` somou 0–0,4 ms no mesmo intervalo. São tempos de CPU observados localmente, e a submissão WebGL não inclui a conclusão da GPU. Isso não confirma nem exclui custo de rasterização no telefone.

Os candidatos restantes mais bem sustentados pelo código são trabalho disparado por snapshots que mudam estado visual e a criação simultânea de voos na distribuição. `layout()` mede os alvos do DOM e atualiza os nós Pixi a cada sincronização; quando seu `staticKey` muda, destrói e reconstrói a mesa e os oponentes. A distribuição cria um voo Pixi por carta de todos os participantes: 14 voos para dois jogadores e 70 para dez. O ambiente local não mediu de modo estável a cadência desses voos: os contadores de render variaram de 2 a 33 chamadas entre casos embora a contagem de voos fosse consistente. O custo percebido em dispositivo real continua **inconclusivo**.

Uma diferença estrutural que merece um A/B é a atualização da projeção das cartas voando. O UnoBotGO usa `PerspectiveMesh` com grade explícita 8×8. O Matcho também cria uma malha com quatro cantos e recalcula a projeção; embora o bundle Pixi declare default 10×10 para sua classe `PerspectiveMesh`, o alias usado na rota não pode ser vinculado com certeza a essa classe no HAR. A grade Uno 8×8 não prova custo maior que o Matcho. A hipótese a testar é a quantidade de malhas/sombras simultâneas e seu custo total, não simplesmente “reduzir a grade”.

As medições RAF corrigidas do Chromium headless contam callbacks de observação, não quadros apresentados. No arraste, o p95 por etapa ficou perto de 66,6 ms e o máximo entre 66,7 e 83,4 ms; as lacunas máximas históricas de 316–350 ms foram contaminadas por callbacks de setup/etapas anteriores e foram retiradas como métrica por ação. O navegador usa SwiftShader e interações Playwright/CDP. Isso não sustenta a percepção de 50 FPS nem uma alegação de 60/120 FPS. A confirmação exige captura no Telegram físico com apresentação real.

## Base, ambiente e preservação

Base examinada: branch `dev`, HEAD `7b85f69`. A árvore já continha as alterações locais do pacote de experiência e do patch desempenho/fullscreen, além de arquivos de documentação, harnesses e assets ainda não rastreados. Foram preservados. `git status` foi conferido antes e depois. Não usei `reset`, `clean`, reapliquei pacotes, alterei dependências, editei código de produção ou fiz commit/push/deploy.

- macOS 26.7.1; Node 24.21.0.
- Chromium Playwright 149.0.7827.55, headless, com `--enable-unsafe-swiftshader` (renderização por software).
- Fixtures locais: viewport 390×844, DPR 2, toque habilitado; backing store do canvas 780×1688.
- PixiJS 8.22.0, GSAP 3.15.0, React/ReactDOM 19.3.0, Vite 7.3.6, Playwright 1.61.1 e Vitest 3.2.7, confirmados com `npm ls --depth=0`. Não há FPS fixado por `ticker.fps`/`gsap.ticker.fps` no fluxo da cena.
- No código Matcho, GSAP é 3.15.0 e React/ReactDOM 19.2.7. O bundle mostra APIs Pixi 8.11.0 ou posteriores, mas não expõe um número exato confiável do Pixi instalado; não atribuo uma versão precisa ao Matcho.

O harness local está em `.reports/diagnostico-fluidez/diagnostic-check.mjs`. As coletas históricas de movimento ficam em `runs/development-animated/` e `production-animated/`; as passadas exploratórias com `recovery=true` continuam preservadas em `development-final/` e `production/`, mas não medem animações. Em 2026-10-11, dois defeitos do harness foram corrigidos e os mesmos seis casos foram refeitos em `runs/development-corrected/` e `production-corrected/`. As pastas antigas não foram sobrescritas.

## Inventário sanitizado do HAR

O HAR tem 43 entradas e 1.968.245 bytes de respostas registrados. Há 11 scripts, 2 folhas CSS, 12 respostas PNG, 2 referências WOFF2, 2 JSON, 3 HTML, 1 SVG, 1 WebP, 1 JPEG, 6 XHR, 12 fetches, 1 WebSocket e 1 ping, entre outros tipos. O inventário por índice, MIME, tipo de recurso, tamanho e hash está em `.reports/diagnostico-fluidez/har/inventory.json`; nomes de arquivo são hashes locais, sem URL ou query string.

- 37 respostas têm conteúdo no HAR; 26 corpos estáticos foram extraídos para `.reports/diagnostico-fluidez/har/`. HTML, favicon e conteúdos não necessários não foram extraídos. Dois corpos de imagem com possível conteúdo pessoal foram deliberadamente omitidos e marcados no inventário.
- Seis entradas não têm corpo utilizável, incluindo as duas referências JSON e as duas fontes WOFF2. Os nomes das fontes aparecem em referências CSS, mas seus bytes não estão no HAR.
- O HAR contém CSS, SVG, WebP, PNGs de interface/cartas e o PNG do atlas de cartas. O JSON de descritores do atlas não está disponível no corpo capturado.
- Há nomes de arquivos de áudio no manifesto JavaScript, mas nenhum corpo MP3/OGG utilizável capturado. A presença no manifesto prova referência, não download ou decodificação bem-sucedidos naquela sessão.
- O arquivo `014-8de099b66920.pretty.js` é a rota dinâmica do jogo Matcho. O arquivo `004-2d4626ac4c30.pretty.js` agrega módulos Pixi e infraestrutura, incluindo Geometry, Filter, RenderTarget e `browserAll`; esses módulos não são lógica específica do jogo. Os demais chunks incluem dependências/UI/SDK. Um JS de terceiros não foi classificado como código do jogo apenas por estar na mesma página.
- Foram extraídos e formatados somente corpos estáticos; nenhum bundle foi executado e nenhum endpoint capturado foi reproduzido. Não foram copiados para este relatório tokens, query strings, URLs, conteúdo de perfil ou identificadores pessoais. O HAR original não foi modificado.

## Caminho atual do UnoBotGO

A cena é criada de forma assíncrona por [`useTableMotion.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/hooks/useTableMotion.ts:11). O efeito de layout encaminha cada nova referência de `view`, seleção ou callback `release` a `scene.sync()`. O componente recria `release` em cada render, então a dependência pode disparar uma sincronização adicional; `TableScene.sync()` retorna cedo quando a referência da view também é a mesma e nada está pendente ([`Game.tsx`](/Users/gabriel/Projetos/UnoBotGO/web/src/pages/Game.tsx:300), [`useTableMotion.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/hooks/useTableMotion.ts:11), [`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:520)).

`TableScene.init()` inicializa WebGL com `resolution=min(devicePixelRatio, 2)`, `autoStart:false`, ticker Pixi não compartilhado e recursos de interação Pixi desligados. O atlas e as texturas auxiliares são carregados antes de marcar a cena pronta; os 63 frames do atlas viram referências de textura para as cartas ([`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:66)). Os botões DOM da mão são hit targets acessíveis vazios; os pixels das cartas são desenhados pelo Pixi ([`Game.tsx`](/Users/gabriel/Projetos/UnoBotGO/web/src/pages/Game.tsx:457), [`game-v2.css`](/Users/gabriel/Projetos/UnoBotGO/web/src/game-v2.css:148)).

A animação GSAP e o desenho não estão em dois loops de jogo: `requestRender()` marca a cena suja, registra um callback não prioritário no ticker GSAP e `flushRender()` chama `app.render()` depois de atualizar as timelines. O listener é removido em repouso. Não existe um RAF contínuo próprio da cena; o único RAF explícito nela libera o estado de scroll controlado depois da animação ([`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:131)). No harness, a janela de repouso produziu zero renderizações Pixi em todas as seis configurações.

`layout()` refaz mapa de botões, mede mesa, descarte, assentos, setas, área da mão e cada botão de carta com `getBoundingClientRect()`. Reutiliza o mapa de sprites estáveis pela identidade física da carta, mas executa os setters de textura/tamanho/sombra e solicita render ao final de cada layout. Quando `staticKey` muda, a mesa antiga é destruída e refeita, incluindo pilha, descarte, setas e sprites de cartas dos oponentes; a chave inclui turno, fase, direção, descarte e quantidade de cartas de oponentes ([`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:332)). A medição de layout no Chromium foi baixa (<2 ms somados por fase), mas não representa a GPU nem garante o custo de um telefone.

O scroll físico interrompe movimentos pendentes e recompõe a cena; o scroll controlado por compra executa `layout()` em cada `onUpdate` da timeline ([`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:207), [`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:310)). O arraste diretamente atualiza posição/rotação do grupo a cada `pointermove` entregue e solicita render, enquanto easing fica no retorno/seleção ([`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:230)). O harness de performance confirmou erro de posição zero no mesmo evento de movimento recebido; isso não mede toque até o pixel apresentado.

Cada voo cria `Container`, sombra, `PerspectiveMesh`, marcador DOM e timeline. Em cada atualização calcula cantos, troca textura quando revela a face, atualiza tamanho/posição/alpha da sombra e solicita render ([`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:428)). A mão reutiliza nós por ID e destrói os removidos; os voos são destruídos ao completar/cancelar. O descarte mantém até cinco camadas. Não há culling Pixi por viewport para os sprites da mão atrás da máscara.

A apresentação de eventos de jogo usa snapshots aceitos do servidor. `acceptSnapshot()` rejeita revisões antigas, mas aceita nova view na mesma revisão; `scene.sync()` só anima eventos contíguos de snapshots não marcados como recovery ([`useGame.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/hooks/useGame.ts:11), [`TableScene.ts`](/Users/gabriel/Projetos/UnoBotGO/web/src/components/scene/TableScene.ts:520)). `send()` envia imediatamente pelo WebSocket e marca `pending`; o UI aguarda confirmação para reabilitar ações. Isso pode parecer uma pausa antes do movimento, mas não é evidência de frame perdido: esta coleta usou socket local sem RTT real. A continuidade da animação após o snapshot e o tempo de espera pela confirmação são medidas diferentes.

O atlas é carregado via `Assets.load<Spritesheet>()`; áudio é opt-in e cria `HTMLAudio` por evento quando habilitado. Fontes usam `font-display: swap`; sons/fontes não foram identificados como carga durante o arraste no fixture. A tela tem um brilho animado de turno por opacity/transform e um anel local animado por box-shadow ([`game-v2.css`](/Users/gabriel/Projetos/UnoBotGO/web/src/game-v2.css:50), [`game-v2.css`](/Users/gabriel/Projetos/UnoBotGO/web/src/game-v2.css:105)). Não foram removidos nem desabilitados efeitos.

## Comparação de arquitetura observada no HAR

| Aspecto | Matcho capturado | UnoBotGO atual | Significado para fluidez |
| --- | --- | --- | --- |
| Renderer | `Application.init()` com resolução limitada a DPR 2 e antialias; configuração “potato” reduz a resolução à metade (`014…pretty.js:2734`). | WebGL, DPR limitado a 2, antialias e desenho manual sob demanda (`TableScene.ts:69`). | Configuração base semelhante. O Matcho tem uma opção explícita de qualidade reduzida; o HAR não mostra que estivesse ligada. |
| Ticker/render | Um listener no ticker GSAP observa timelines ativas e inicia/para `app.ticker` Pixi (`014…pretty.js:2469`). Pixi desenha no próprio ticker quando ligado. | O callback GSAP atualiza e chama `app.render()` na mesma cadência, após a timeline; desliga no repouso. | Uno evita um segundo ciclo de RAF do renderer e tende a manter o desenho no mesmo tick da animação. O método Matcho pode ter faseamento entre seus dois tickers; o HAR não mede o efeito visual. |
| Malha de perspectiva | A rota chama `setCorners()` em malha Pixi; o default da classe no chunk é 10×10 (`014…pretty.js:1565`, `004…pretty.js:16218`). | A malha de voo declara 8×8 e chama `setCorners()` a cada atualização (`TableScene.ts:436`, `463`). | O Matcho passa os quatro cantos e não declara `verticesX/Y`; o Pixi capturado define default 10×10 para `PerspectiveMesh`, mas o alias de classe não é rastreável com certeza no HAR. A grade Uno 8×8 não permite concluir que sua projeção seja mais cara. |
| Cartas e sombras | A carta é um container com face e sombra; carta da mão também recebe overlay. Drag no touch usa tween de 100 ms para a posição e tweens para inclinação/sombra (`014…pretty.js:1425–1512`). | Sprite e sombra por carta, mais mesh/sombra por voo. O drag atualiza imediatamente no evento recebido. | Matcho não prova maior continuidade: o movimento touch tem easing intermediário, e cada engine recebe eventos/timing diferentes. O drag direto do Uno reduz atraso calculado, mas não mede presentation latency. |
| Mão/layout | Cartas mantidas como objetos Pixi; posições/reordenação animadas com GSAP. | Pixels Pixi e hit targets DOM posicionados por layout React; posições medidas da geometria DOM. | A divisão DOM/Pixi é uma diferença, não defeito. Pode custar em snapshots e scroll se o layout for medido repetidamente. |
| Pilhas e cache | A pilha de descarte usa `cacheAsTexture(true)` e atualiza a textura ao crescer; remove cache ao recolher pilha (`014…pretty.js:1360–1383`). | Pilhas simples são reconstruídas por `staticKey`; histórico visível limitado a cinco cartas. | O cache Matcho é uma otimização localizada da pilha; não demonstra que todo o jogo use cache ou sprite pool. O conjunto Uno pequeno pode não compensar render texture e invalidação. |
| Criação/limpeza | `new V(...)` cria nós de cartas em respostas de mão; não encontrei pool sistêmico no trecho. A pilha usa cache parcial. | Nós da mão são reutilizados por ID; cada voo é novo e destruído no complete/cancel; cleanup remove observers, listeners e timelines (`TableScene.ts:171–192`, `474`). | O HAR não sustenta a recomendação genérica de “copiar pooling do Matcho”. Deve-se medir memória real antes de introduzir pool. |
| Pausa/retomada | Escuta visibility/deactivated e pausa/retoma fila e ticker, com listeners removidos no destroy (`014…pretty.js:2399–2416`, `2724`). | Feedback para ao ocultar; ticker Pixi não roda em repouso, e a cena remove o listener GSAP ao terminar (`Game.tsx:265`, `TableScene.ts:141–169`). | Lifecycle de cada app difere; não prova vazamento nem melhora de FPS. |
| Efeitos/recursos | Manifesto registra atlas, imagens e áudio; corpos/fontes/áudios não estão todos capturados. Há RAF próprio para partículas (`014…pretty.js:3148–3172`). | Atlas e texturas principais aguardados antes de cena pronta; áudio opcional por ação; não há loop de partículas contínuo da cena. | O HAR prova referências e alguns bytes, não sucesso de carregamento, decodificação, frame pacing ou execução de todos os caminhos. |

Os links oficiais consultados são [PixiJS: render loop](https://pixijs.com/8.x/guides/concepts/render-loop), [PixiJS: desempenho](https://pixijs.com/8.x/guides/concepts/performance-tips), [PixiJS: `PerspectiveMesh` 8.22](https://pixijs.download/v8.22.0/docs/scene.PerspectiveMesh.html) e [GSAP ticker](https://gsap.com/docs/v3/GSAP/gsap.ticker/). A documentação do Pixi destaca que `app.render()` submete comandos, sem esperar a GPU terminar; por isso o tempo instrumentado não mede o trabalho completo da GPU. A documentação do GSAP descreve o ticker sincronizado a RAF, coerente com o callback do UnoBotGO depois das atualizações do ticker.

## Cenários e medições

Foram repetidos os mesmos seis casos: 2 e 10 participantes × 7, 15 e 30 cartas iniciais. Cada fixture local exercita repouso, seleção/retorno, scroll, 20 movimentos touch via CDP e cancelamento, compra de uma carta, compra de três, distribuição, coringa com confirmação local, 30 snapshots idênticos e recuperação. Não há backend nem RTT real.

### Retificação do harness e rastreabilidade

A versão antiga mantinha o cursor RAF em Node, mas tentava lê-lo como `window.__diag.rafCursor`; também havia um `return` antes da atualização do cursor. Assim, `raf.interval` repetia o histórico acumulado desde a inicialização em vez dos callbacks daquela ação. **Todas as estatísticas RAF por etapa nas pastas `development-animated/` e `production-animated/` são inválidas como medidas de etapa**, inclusive p50/p95/máximo e contagem de lacunas. Os campos cumulativos registrados separadamente naquela versão continuam sendo históricos do documento, não dados por ação. A menção anterior a pausas máximas de 316–350 ms durante ações não pode ser atribuída à ação; parte podia ser do setup ou de fases anteriores.

O harness corrigido envia o cursor absoluto da última leitura ao browser, delimita os callbacks novos entre duas marcações, e publica separadamente `raf.interval` da etapa e `raf.cumulative` do histórico retido. Só calcula mediana, p95, máximo e lacunas quando há pelo menos dois intervalos observados; contagens de callbacks/amostras continuam explícitas. Uma assertion determinística injeta uma lacuna de 968 ms antes do começo de uma etapa e confirma que o máximo/p95 seguinte ficam em 16 ms; também confirma que a leitura sucessiva não reutiliza as amostras. `node --check` e a própria execução do harness passaram.

O segundo defeito estava no cenário “compra de três”: depois da compra de uma carta, o fixture recriava `N+3` cartas a partir dos IDs `c0…`, produzindo somente duas cartas novas em relação à mão `N+1`. **As medições antigas de compra de três são, na realidade, de duas cartas novas**; não descrevem uma compra de três. Isso afeta os contadores/custos da etapa `drawMany` e conclusões que associavam esses custos a três voos. A versão corrigida preserva todos os IDs existentes, acrescenta três IDs únicos, atualiza a contagem pública e envia `cards_drawn.count=3`. Em cada execução, as assertions conferiram +1/+3 cartas e contagem/evento; os marcadores DOM identificaram todos os voos. Em desenvolvimento, a instrumentação individual de `TableScene.fly` também confirmou exatamente os três CardIDs novos. As evidências corrigidas ficam em diretórios próprios e incluem esses IDs e marcadores por cenário.

### Resultados por etapa nas seis fixtures

A tabela mostra callbacks RAF, não quadros apresentados. Os p50/p95/máximos abaixo vêm exclusivamente dos timestamps de callbacks dentro da etapa. Estatísticas cumulativas ficam em `end.raf.cumulative` nos JSON; não devem ser interpretadas como métricas da etapa. Os dois ambientes usaram a mesma viewport 390×844, DPR 2 e Chromium headless/SwiftShader.

| Etapa | Desenvolvimento corrigido | Build de produção corrigido | Interpretação |
| --- | --- | --- | --- |
| Repouso (~1,1 s) | p50 16,7 ms; p95 16,7–16,8 ms; máximo 16,8 ms | p50 16,7 ms; p95 16,7–16,8 ms; máximo 16,8 ms | Etapa sem ação manteve intervalos regulares neste ambiente. |
| Seleção/retorno | p95 66,7–83,4 ms; máximo 83,3–100,1 ms | p95 66,7–83,4 ms; máximo 66,7–116,6 ms | Há lacunas no callback de observação durante a etapa, mas isso não é contagem de frames perdidos na tela. |
| Arraste/cancelamento | p50 16,7 ms; p95 66,6–66,8 ms; máximo 66,7–83,4 ms | p50 16,7 ms; p95 66,6–66,7 ms; máximo 66,7–83,4 ms | O máximo cai de 316–350 ms reportado na tabela antiga para 67–83 ms quando setup e etapas anteriores são excluídos. O p95 continua alto em headless. |
| Compra de uma | p95 66,7–100 ms; máximo 66,7–100 ms; 1 ID/1 marcador por caso | p95 66,7–100 ms; máximo 66,7–116,7 ms; 1 ID/1 marcador por caso | Contagem/evento e voo individual coerentes. |
| Compra de três | p95 66,7–100 ms; máximo 66,8–116,7 ms; 3 IDs/3 marcadores e 3 chamadas `fly` por caso | p95 66,7–100 ms; máximo 83,3–100,1 ms; 3 IDs/3 marcadores por caso | Agora corresponde a exatamente três cartas novas. A produção não expõe o wrapper privado `fly`; seus marcadores DOM foram conferidos. |
| Distribuição | p95 16,7–66,7 ms; máximo 66,5–100 ms | p95 16,7–66,8 ms; máximo 50–83,4 ms | Variação permanece alta entre casos, coerente com muitas animações simultâneas; não estima FPS apresentado. |
| Coringa/confirmação local | p95 50–50,1 ms; máximo 66,7–83,4 ms | p95 50 ms; máximo 66,7–66,8 ms | Confirmação fixture local; sem latência de rede real. |
| 30 snapshots idênticos | p95 16,7–66,6 ms; máximo 66,6–66,7 ms | p95 16,7–16,8 ms; máximo 49,9–50,1 ms | Burst artificial, mantido separado de comportamento do servidor real. |

No arraste, o desenvolvimento registrou 26–28 chamadas a `app.render()` e 7,4–8,7 ms somados no wrapper; o tempo síncrono dos wrappers WebGL ficou em 0–0,3 ms. Em produção, a instrumentação de `app.render()` não é instalada; os draws WebGL foram 122–130 e o submit síncrono somou 0–0,4 ms. O `ScriptDuration` CDP do arraste foi 34–45 ms em dev e 17–18 ms em produção; `LayoutDuration`, abaixo de 1 ms nos dois. Isso descreve trabalho observado no host, não o custo completo da GPU nem a apresentação na tela.

Nas compras corrigidas, dev registrou 14–17 renders Pixi para uma carta e 17–26 para três; o `app.render()` somou 6,6–8,4 ms e 7,6–11,2 ms, respectivamente. WebGL contou 69–79 draws para uma carta e 84–122 para três em dev; em produção, 69–83 e 84–122. A execução anterior de “três” só adicionava duas cartas, então aqueles intervalos antigos (13–24 renders e 63–117 draws) ficam registrados apenas como evidência histórica de uma fixture de duas cartas novas e não sustentam custo de compra de três.

A instrumentação WebGL mede o tempo síncrono dos wrappers `drawElements`/`drawArrays`, não execução da GPU. O `ScriptDuration`/`LayoutDuration` vem de métricas CDP, e a leitura do heap não força coleta. O contador RAF é um callback de observação; não é `framesPresented`, `requestVideoFrameCallback` de canvas nem captura do display. Os harnesses anteriores também mediram renderer p95 de 163 ms neste ambiente, apesar de cadência mediana de RAF de 16,8 ms, mostrando por que mediana isolada não basta. Não foi habilitada uma gravação de timeline que separe Paint, Raster, Composite e pausas de GC; esses custos permanecem sem medição nesta coleta.

As saídas `development-corrected/` e `production-corrected/` têm `summary.json` e JSON por caso; os resumos contêm zero erros de página. As evidências anteriores foram preservadas. As pastas `har/` e `runs/` não incluem credenciais nem corpos de páginas HTML.

## Diagnóstico classificado e correções para uma próxima etapa

| Prioridade | Achado | Classe | Correção possível, impacto e risco | Como validar se resolver |
| --- | --- | --- | --- | --- |
| 1 | A distribuição cria um voo/mesh/sombra por carta: 14–70 voos em uma cena; cada timeline atualiza projeção e sombra. O Matcho também anima cartas, mas serializa/distribui por `animQueue`; o HAR não quantifica os frames apresentados. | Hipótese de custo alto em distribuição, confirmado que há multiplicação de objetos | Testar limite de concorrência ou escalonamento visual mantendo todas as animações e duração aparente. Risco: alterar ritmo/ordem percebidos e tornar o fim da distribuição mais longo. Não priorizar mudar a resolução da malha sem A/B visual e traço físico. | A/B no mesmo telefone e build, com marcação dos voos ativos/renderizações, gravação de tela a 120/240 fps ou trace de frames físicos; comparar p95 de frame, contagem de pausas e aparência em 2/10 jogadores. |
| 2 | `staticKey` inclui contagem de oponentes e flags de turno/fase; qualquer mudança destrói e recria a mesa/oponentes. `sync/layout` ainda faz medição completa mesmo quando a composição está quase igual. | Trabalho/reconstrução confirmado pelo código; impacto de fluidez inconclusivo | Atualizar separadamente a pilha do deck, o realce de turno, a contagem de oponentes e a direção, preservando os containers estáveis. Cachear leituras de geometria até resize/layout real. Risco: estado visual parcial, sprites obsoletos ou máscara/posição desatualizada. | Contar `Container`/`Sprite` destruídos por snapshot, `getBoundingClientRect` e layout; testar snapshot repetido, compra, 10 jogadores, resize/scroll, comparação pixel a pixel e trace de performance. |
| 3 | Scroll do dedo chama `stop()+layout()`; scroll de compra chama `layout()` em todo update da timeline. | Trabalho confirmado; gargalo de scroll não reproduzido com fidelidade por CDP | Separar cálculo geométrico da atualização visual, reutilizar posições e redesenhar apenas os alvos afetados, sem mudar o scroll vertical. Risco: voos pousarem em coordenada antiga ou ficar uma carta dessincronizada. | Arraste/rolagem físicos em 7/15/30 cartas; comparar atualizações de layout e frame pacing, conferir última fileira, margens e voo chegando ao slot após rolagem. |
| 4 | Anel local anima `box-shadow`; brilho de turno anima `opacity`/`transform`. | Possível custo de pintura/composição; headless não isolou camadas ou pintura | Medir paint/composite em trace real. Se confirmado, preservar brilho e transição com camada/representação que evite repintar sombra a cada frame. Risco: brilho, borda ou contraste diferentes e uso de camada/GPU maior. | Capturar Paint/Composite no Chrome Android e comparação visual; repetir bateria com animação ativa, sem remover efeito. |
| 5 | Os callbacks RAF headless ficam regulares em repouso, mas várias etapas ativas têm p95 de 50–100 ms e lacunas máximas de até 116,7 ms. | Medida de callback por etapa confirmada; relação com a tela física inconclusiva | Não alterar produção com base isolada nesses números. O harness pode atrasar o próprio browser e SwiftShader não representa a GPU do telefone. | Correlacionar trace de frames apresentados, main thread, GPU e temperatura em captura física de Telegram Android/iOS. |
| 6 | Comando de jogo aguarda snapshot aceito; tempo de rede pode atrasar o início do voo, enquanto o tween em execução é local. | Distinção de arquitetura confirmada; RTT atual não medido | Em próxima medição, marcar `send`, ACK, snapshot aceito, primeiro frame de animação e duração do voo separadamente. Não aplicar previsão otimista antes de verificar regras/consenso. Risco: confundir latência/rejeição de regra com apresentação. | Repetir com RTT controlado e dispositivo real; comparar tempo comando→ACK com intervalos entre frames depois do início. |

Não há evidência suficiente para recomendar remover efeitos, trocar engine, copiar bundle do Matcho, reduzir DPR globalmente ou aumentar uma configuração de FPS. O Matcho tem um modo “potato” que reduz resolução, mas não há informação no HAR de estar ativo e o teste local não comparou resolução. Qualidade adaptativa só deve ser considerada depois de uma captura física mostrar limitação GPU.

## O que o HAR permite concluir

O HAR permite inspeção estática de um código de jogo Matcho que usa Pixi, GSAP, imagens e manifestos de áudio; permite verificar escolhas concretas como ticker, mesh de perspectiva, `cacheAsTexture`, RAF de partículas, gestos touch, resolução DPR 2 e listeners de visibilidade. Não prova que o código capturado rodou em cada cenário, que recursos sem corpo carregaram, que uma sessão teve uma taxa de quadros específica, que o renderer usou GPU física, ou que o Matcho estava mais rápido sob o mesmo aparelho/estado/rede. Também não identifica o valor de `setting_potato` nem uma versão Pixi exata.

## Captura adicional necessária no Telegram físico

Para resolver a dúvida, é necessária uma gravação do **mesmo build de produção** em Telegram Android e iOS compatíveis, com modelo do aparelho, versão do Telegram/OS, 60/120 Hz da tela, viewport/DPR e modo de economia/temperatura registrados. Gravar ao menos 10 s de repouso, seleção/retorno repetida, 10 s de arraste contínuo, compra de 3 cartas e distribuição com 2 e 10 jogadores; repetir cada cenário algumas vezes depois de aquecer a partida. Exportar trace de Chrome DevTools/Perfetto no Android com Main, Frames, Raster/GPU e memória; no iOS, gravação de alta taxa e Instruments/Core Animation equivalentes. Para Matcho, capturar os mesmos cenários e dispositivo/rede. Medir frames apresentados/intervalos reais e pausas p95, além do atraso toque→display. HAR, ticker callbacks e Chromium headless não substituem essa captura.

## Reproduzir localmente

Com Chromium Playwright instalado:

```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web test
npm --prefix web run build

# Desenvolvimento, em outro terminal:
npm --prefix web run dev -- --port 5174 --strictPort

# Cenários dev:
DIAG_MODE=development-corrected \
DIAG_DEV=1 \
DIAG_OUT=.reports/diagnostico-fluidez/runs/development-corrected \
DIAG_URL=http://127.0.0.1:5174 \
node .reports/diagnostico-fluidez/diagnostic-check.mjs

# Build de produção; depois do build, em outro terminal:
cd web && npx vite preview --host 127.0.0.1 --port 4173 --strictPort

# Mesmos cenários no build:
DIAG_MODE=production-corrected \
DIAG_OUT=.reports/diagnostico-fluidez/runs/production-corrected \
DIAG_URL=http://127.0.0.1:4173 \
node .reports/diagnostico-fluidez/diagnostic-check.mjs

# Harnesses existentes:
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_PERFORMANCE_OUTPUT=.reports/diagnostico-fluidez/harnesses/performance node web/scripts/performance-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_MOTION_OUTPUT=.reports/diagnostico-fluidez/harnesses/motion node web/scripts/motion-check.mjs
UNO_VISUAL_URL=http://127.0.0.1:5174 UNO_EXPERIENCE_OUTPUT=.reports/diagnostico-fluidez/harnesses/interaction node web/scripts/experience-check.mjs
```

## Verificações desta análise

Lint, typecheck, 128 testes frontend e build passaram. Os harnesses existentes de performance, movimentos e interação passaram; a matriz de layout foi executada para validar oito cartas por fileira, margens, rolagem vertical e tamanhos de mão. Não houve acesso ao Telegram físico, teste real de som/vibração nesta coleta, trace de GPU física, captura de frames apresentados nem comparação lado a lado com Matcho em dispositivo equivalente. O resultado final separa contagem/custo de trabalho local de FPS medido: **FPS físico continua desconhecido**.

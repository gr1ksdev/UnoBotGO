# Plano: experiência completa da partida a partir do HAR

## Pedido do usuário
“Faça o pacote completo meu irmão. Eu confio em você”, após discussão dos JS de geometria/browser, código da partida, atlas, efeitos e sons da referência.

## Objetivo
Completar a apresentação e interação da partida real com recursos originais disponíveis e comportamentos adaptados ao PixiJS/GSAP existente. Entregar pacote ZIP e patch consolidado, sem exigir aplicar o patch anterior.

## Contexto atual
Branch dev, HEAD 7b85f69510ad5be1d2bd46b9a7e7dab16a798084. Alterações do patch de fluidez permanecem locais e serão preservadas/incluídas. WebSocket, engine, privacidade, consenso de revanche, fontes, imagens e fileiras de oito já existem. Último HAR contém os mesmos chunks previamente analisados e atlas/efeitos; fontes e sons não têm corpos nesse HAR, mas áudios reais existem no pacote v2 do repositório. Skills PixiJS/GSAP citadas na memória não estão neste checkout; utilizar APIs atuais das dependências instaladas e documentação oficial. Autorização explícita atual dispensa nova pergunta de aprovação.

## Arquivos analisados
- AGENTS.md; .agent/context.md, memory/memory.md, decisions.md
- web/src/components/scene/TableScene.ts; hooks/useTableMotion.ts
- web/src/pages/Game.tsx, Game.test.tsx; game-v2.css
- web/src/lib/cardMotion.ts, handLayout.ts e testes
- web/scripts/motion-check.mjs, visual-check.mjs; package.json
- HAR atual, Geometry, browserAll, MatchoRoute JS/CSS e manifestos de atlas
- docs/design-reference/unobotgo-v2/assets/audio e effects
- Documentação oficial PixiJS PerspectiveMesh e GSAP lifecycle/matchMedia

## Arquivos que poderão ser modificados
- Cena, hook, página, CSS, bibliotecas de interação/feedback e seus testes
- web/public/assets/audio e efeitos adicionais confirmados
- Scripts de navegador; docs e .agent para evidência e memória

## Estratégia de implementação
Manter DOM para alvos acessíveis e comandos; Pixi para pixels e gestos visuais. Arrastar carta selecionada ao descarte aciona o mesmo comando validado usado pelo segundo toque. Retorno/cancelamento nunca envia comando. Novas revisões, perda de conexão, resize e unmount cancelam gestos. Manter rolagem vertical das fileiras. Adaptar tempos/curvas e recursos do HAR, sem copiar bibliotecas compiladas, protocolo alheio, cookies ou tokens. Sons locais opcionais e feedback tátil com API Telegram disponível; falha de áudio não interfere no jogo. Efeitos só por eventos novos confirmados e não por recovery/histórico.

## Passos detalhados
1. Registrar autorização, preservar baseline e inventariar recursos disponíveis.
2. Implementar seleção com escala/sombra, hover, arraste com inclinação e retorno elástico; proteger CardID e deduplicação.
3. Diferenciar distribuição, compra, descarte próprio/adversário e reorganização; transições de disponibilidade e efeitos de turno/inversão/bloqueio/cor.
4. Integrar sons existentes com preferência persistida, limite de simultaneidade e descarte de áudio obsoleto; opções de som/vibração no menu.
5. Completar entrada escalonada do resultado, brilho, revanche e efeitos sem pontuação fictícia.
6. Testar cancelamentos, rejeições, gestos reais touch, revisões, reduced-motion, som e layouts 320x568/360x800/390x844/430x932, até 30 cartas/dez jogadores.
7. Gravar prévia identificada de teste, documentar proveniência/limitações e empacotar patch consolidado e guia.

## Riscos
- Captura não é código fonte completo; manter adaptação própria e mencionar faltas.
- Gestos não podem bloquear rolagem de mão nem emitir jogadas duplicadas.
- Timelines não podem acessar sprites destruídos nem repetir eventos de recovery.
- Autoplay/áudio/haptics variam por cliente; usar opt-in e falha silenciosa.
- Backend real e dispositivo físico não são comprovados por fixtures visuais.

## Impactos esperados
- Apresentação mais viva e fiel, recursos existentes em uso, interação acessível preservada.
- Pacote único aplicável à base indicada, sem commit/push/deploy.

## Compatibilidade
- Linux/macOS/Windows: código TypeScript e APIs web portáveis.
- Docker/CI/CD: assets locais, build e CSP atuais; sem novos serviços/dependências.

## Como testar
### Build
```bash
npm --prefix web run build
```
### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
node web/scripts/motion-check.mjs
node web/scripts/visual-check.mjs
git diff --check
```
### Execução
```bash
npm --prefix web run dev -- --host 127.0.0.1
```
Harness identificado intercepta snapshots; não contata conta Telegram real.

## Rollback
Reverter somente hunks desta tarefa, preservando mudanças do usuário. Não apagar referências, histórico, planos ou pontuação. Sem reset/clean.

## Observações
Plano registrado diretamente como aprovado por autorização explícita na conversa. Documentação e prévia final devem separar validação local de homologação Telegram/GPU real.

## Conclusão

Implementação concluída. 117 testes frontend, lint/tipos/build, git diff --check, harnesses de movimento/interação, 324 composições finais, 37 seleções estreitas e 36 listas de resultado passaram. Atlas 63 frames e 14 áudios verificados. Render sob demanda validado com contador WebGL em repouso. Prévia local gravada. Guia e limites em docs/experiencia-completa-har.md. Pacote consolidado contém etapa anterior; não aplicar ambos. Sem commit/push/deploy e sem novo E2E Go/PostgreSQL ou homologação física.

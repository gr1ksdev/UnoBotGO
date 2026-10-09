# Correção: dois toques, descarte e coringa

Implementada localmente após aprovação do plano em 2026-10-09. Sem novo commit, push ou deploy; alterações e artefatos anteriores preservados.

## Comportamento

- Primeiro toque na carta jogável seleciona por CardID e eleva com GSAP. Segundo toque na mesma carta envia play imediatamente, sem botão separado. Tocar outra carta apenas muda seleção. Há guarda síncrona contra repetições, bloqueio durante pending/desconexão e disponibilidade da engine. Teclado usa as mesmas ativações acessíveis.
- A parte elevada também recebe toque, dentro de sua faixa exclusiva e da folga entre fileiras. Não muda o alvo por índice nem a geometria usada pelo PixiJS.
- Descarte conserva até cinco faces públicas observadas, com offsets/rotações discretos e sombra original. A face anterior fica visível sob a nova carta em voo; o topo confirmado aparece ao aterrissar. Revisões/IDs deduplicam histórico; resize/recovery recompõem sem replay. Na reconexão, só existem as faces públicas conhecidas, sem inventar histórico nem acessar mão alheia.
- Cada nova escolha restaura vermelho/amarelo/verde/azul e cancela feedback/saída antigos, vinculada à sala/carta/fase. Funciona mesmo depois de escolhido/sent já terem sido limpos. ACK rápido conserva 450+700+350 ms; rejeição libera nova tentativa. Coringa na fase de escolha fica neutro, em vez de herdar a cor ativa anterior; a variante colorida aparece após confirmação.
- Engine, autenticação, WebSocket, finalização, políticas e ranking permanecem intactos.

## Checks desta correção

| Check | Resultado |
| --- | --- |
| make check com PostgreSQL de testes | Passou: lint, TypeScript, 109 testes frontend, build, Go/vet/builds debugcards, integração PostgreSQL/app e diff; check.log |
| Build após ajuste de toque na parte elevada | Passou; build.log |
| Browser de movimentos | Passou; motion.log e motion-checks.json. Inclui segundo toque na parte elevada, ID, rejeição/duplicação, turno/recovery, quatro pétalas após feedback anterior, coringa neutro, pilha visível durante voo nos quatro tamanhos, resize/unmount e falha de assets |
| Visual nos quatro tamanhos | Passou: 16 composições (mãos 7/30, jogadores 2/10); estados adicionais, fontes, último cartão, safe areas, nomes longos e avatares ausentes; visual.log e visual/checks.json |
| Dois clientes com Go/WS/engine/PostgreSQL reais e race | Passou: 39 jogadas, 23 compras, cinco cores, finalização normal, retries sem duplicação, rankings/histórico, reconexão e consenso de revanche; e2e.log e production/browser.json |
| Backend da partida local | Um resultado persistido, dois retries, um histórico por conta, uma notificação, dois jogadores e um grupo no ranking; production/backend.json |

Falhas iniciais de verificação foram corrigidas: narrowing de Position no callback TypeScript; teste isolado interrompido por npm ci concorrente, substituído pelo make check completo sequencial aprovado. Nenhuma falha foi ignorada.

## Evidências

- discard-in-flight-{320,360,390,430}.png e discard-pile-{320,360,390,430}.png: carta anterior permanece sob a jogada; pilha final com recursos reais. Capturas revisadas visualmente.
- next-wild-four-colors-320.png: nova escolha após feedback vermelho completo; quatro cores próprias e coringa neutro.
- visual/: estados nos quatro tamanhos 320×568, 360×800, 390×844 e 430×932. A matriz de 324 do trabalho anterior foi preservada; esta correção executou os 16 casos especificados acima.
- comparisons/: originais fornecidos à esquerda, fixtures da implementação atual à direita.
- production/video/movimentos-reais-11s.mp4: 10,56 s em velocidade normal, trechos da partida real local com distribuição, compra/virada e cor; segments.json registra as fontes/cortes.
- production/video/dois-toques-descarte-4s.mp4: seleção elevada e jogada de coringa pelo segundo toque, seguida do seletor; jogada-segment.json registra o corte conferido por frames. A sobreposição do seletor acompanha a fase autorizada sem atrasar o comando.
- Vídeos completos account-1.webm/account-2.webm preservados localmente. generate-evidence.py reproduz comparações e vídeo principal.

## Escopo da confirmação

Confirmado localmente em Chromium com toque emulado, API/WebSocket/engine/finalizador/PostgreSQL reais e CSP de produção. SDK/HMAC/identidades e consultas de membros são exclusivos do teste. A variante final da área de toque elevada também foi exercitada no bundle estático atualizado.

Não confirmado no Telegram real nem em aparelho físico. Não foram alterados BotFather, URL publicada, produção ou processos do bot. Nenhum novo commit/push/deploy foi feito nesta correção.

## Versionamento autorizado posteriormente
Usuário autorizou commit e push para dev. Código, documentação, checks/logs finais, capturas principais do descarte/coringa nos quatro tamanhos, comparações e vídeos curtos são versionados. Capturas complementares e vídeos completos ficam preservados localmente. O gerador de cortes requer os vídeos completos; os JSONs de segmentos registram suas fontes. Deploy continua fora do escopo.

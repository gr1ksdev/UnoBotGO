# Plano: webapp-mesa-4-jogadores-e-bloqueio-iniciada

## Pedido do usuário
O usuário solicitou quatro modificações para o modo de jogo WebApp / WebSocket:
1. **Redução da capacidade para 4 jogadores:** Atualmente a mesa suporta até 6 (ou 10) jogadores; limitar a no máximo 4 participantes.
2. **Posicionamento dos 4 jogadores na mesa:** Disposição em cruz com todos centralizados:
   - 1 embaixo (jogador local / Você)
   - 1 em cima (centralizado horizontalmente)
   - 1 na esquerda (centralizado verticalmente)
   - 1 na direita (centralizado verticalmente)
3. **Descrição do convite com lista de jogadores:** Na mensagem com o link da mesa gerada pelo botão de compartilhamento, mostrar os nomes dos jogadores que já estão na sala (ex: `Jogadores na sala (X/4): • Jogador 1 • Jogador 2`).
4. **Bloqueio e tela para partidas já iniciadas:** Quando a partida começar e alguém tentar entrar pelo link de convite (ou abrir a sala), exibir uma tela/mensagem clara informando que aquela partida já foi iniciada e não aceita mais participantes.

## Objetivo
Implementar a limitação de 4 jogadores para salas WebApp, ajustar a disposição dos avatares e assentos na mesa no CSS/React para a cruz centralizada (top, bottom, left, right), incluir a lista de jogadores atuais no texto de compartilhamento do link da sala e criar uma tela de bloqueio informativa ("Partida já iniciada") quando um usuário tentar ingressar em uma partida em andamento.

## Contexto atual
- `internal/httpapi/live.go` e `internal/httpapi/rooms.go`:
  - `Capacity` é retornado como 10 no endpoint de live e preview.
  - `previewInvite` e `joinInvite` calculam `can_join` com base em `len(v.Players) < 10`.
- `web/src/pages/Game.tsx` e `web/src/styles.css`:
  - Os oponentes são todos colocados em uma linha superior (`.opponents`) no topo da mesa.
  - As posições dos oponentes na mesa usam classes `.players-2`, `.players-3`, `.players-4`, `.players-5`, `.players-6`.
  - O componente `ShareRoom` em `web/src/components/ShareRoom.tsx` apenas compartilha a URL crua sem listar os participantes atuais da sala.
- `web/src/pages/Invite.tsx`:
  - Se `can_join` for falso, exibe apenas uma mensagem genérica de rodapé: `"Esta sala está cheia ou não aceita mais participantes."`. Não há destaque especial quando o motivo é o jogo já ter começado.

## Arquivos analisados
- `internal/httpapi/live.go`
- `internal/httpapi/rooms.go`
- `internal/httpapi/rooms_test.go`
- `web/src/api/client.ts`
- `web/src/components/ShareRoom.tsx`
- `web/src/pages/Game.tsx`
- `web/src/pages/Invite.tsx`
- `web/src/pages/Invite.test.tsx`
- `web/src/styles.css`
- `web/src/components/scene/TableScene.ts`

## Arquivos que poderão ser modificados
- `internal/httpapi/live.go`
- `internal/httpapi/rooms.go`
- `internal/httpapi/rooms_test.go`
- `web/src/api/client.ts`
- `web/src/components/ShareRoom.tsx`
- `web/src/pages/Game.tsx`
- `web/src/pages/Invite.tsx`
- `web/src/pages/Invite.test.tsx`
- `web/src/styles.css`

## Estratégia de implementação

1. **Capacidade máxima de 4 jogadores (Backend & Frontend):**
   - Em `internal/httpapi/rooms.go`:
     - Definir capacidade de salas livres (`ranking.RoomFree` ou salas webapp) como 4 em `previewInvite` e `joinInvite`.
     - Validar que novas entradas recusam quando `len(v.Players) >= 4`.
   - Em `internal/httpapi/live.go`:
     - Configurar `Capacity: 4` quando `v.RoomType == ranking.RoomFree` ou `v.Origin == "webapp"`.
   - Em `web/src/pages/Game.tsx`:
     - Ajustar o lobby para mostrar `X de 4` vagas.

2. **Posições em Cruz na Mesa (Top, Bottom, Left, Right):**
   - O jogador local (`footer.local-player`) permanece centralizado embaixo (`bottom`).
   - Os até 3 oponentes serão distribuídos nas posições:
     - 1 oponente (partida 1v1): Top (centralizado no topo).
     - 2 oponentes (partida de 3): Top e Left (ou Left e Right).
     - 3 oponentes (partida de 4):
       - Oponente 1: **Esquerda (Left)**, centralizado verticalmente (`top: 50%; transform: translateY(-50%)`).
       - Oponente 2: **Cima (Top)**, centralizado horizontalmente (`left: 50%; transform: translateX(-50%)`).
       - Oponente 3: **Direita (Right)**, centralizado verticalmente (`top: 50%; transform: translateY(-50%)`).
   - Como o `TableScene.ts` busca dinamicamente as coordenadas de cada elemento com `[data-player-key]`, a cena PixiJS e as cartas animadas passarão automaticamente a voar para as posições da esquerda, topo, direita e base sem quebrar o canvas.
   - Ajustar `web/src/styles.css` para estilizar `.seat-left`, `.seat-top`, `.seat-right`, com avatares e contadores legíveis e centralizados em telas mobile e desktop.

3. **Descrição da Mensagem com Link e Jogadores da Sala:**
   - Em `web/src/components/ShareRoom.tsx`:
     - Receber a lista de jogadores atuais (`players: Seat[]`).
     - Montar o texto com os participantes:
       ```
       🎮 Mesa de UNO no UnoBotGO!
       Jogadores na sala (X/4):
       • Jogador 1 (Criador)
       • Jogador 2
       
       Entre pelo link:
       ```
     - Integrar esse texto formatado em `navigator.share({ title, text, url })`, na cópia para clipboard e via Telegram share URL (`https://t.me/share/url?url=...&text=...`).

4. **Tela de Bloqueio Informativa para Partidas Já Iniciadas:**
   - Em `internal/httpapi/rooms.go` (`previewInvite`):
     - Retornar campos `phase` e `started: v.Phase != uno.Lobby`.
   - Em `web/src/pages/Invite.tsx`:
     - Se `view.started || view.phase !== 0`:
       - Renderizar um card de bloqueio com ícone, eyebrow `PARTIDA EM ANDAMENTO`, título `Esta partida já foi iniciada!`, texto explicativo de que não é mais possível entrar porque o jogo já começou, e botão para voltar ao início (`/play`).
   - Em `web/src/pages/Game.tsx`:
     - Se um espectador não participante abrir a rota `/game/:id` e a partida já tiver começado (`!me && view.phase !== 0`), exibir uma tela amigável avisando que a partida já está em andamento com botão para voltar ao início.

## Passos detalhados
1. Atualizar `internal/httpapi/rooms.go` e `internal/httpapi/live.go` para limitar capacidade em 4 e expor `phase`/`started` no preview de convite.
2. Atualizar testes backend em `internal/httpapi/rooms_test.go` para a nova capacidade.
3. Atualizar tipos em `web/src/api/client.ts`.
4. Atualizar `web/src/components/ShareRoom.tsx` para formatar a descrição com os nomes dos jogadores inscritos.
5. Atualizar `web/src/pages/Game.tsx` e `web/src/styles.css` para posicionar os oponentes em Top, Left e Right centralizados ao redor da mesa.
6. Atualizar `web/src/pages/Invite.tsx` para renderizar a tela de partida já iniciada com aviso claro.
7. Atualizar e rodar os testes frontend (`npm test` no diretório `web`).
8. Rodar suíte completa de testes (`go test ./...` e `npm test`).

## Riscos
- **Quebra do cálculo de posições do PixiJS:** O `TableScene` depende de `getBoundingClientRect()` dos avatares `[data-player-key]`.
  - *Mitigação:* Manter a mesma estrutura de tags e classes dos badges de jogador, alterando apenas seu posicionamento no layout CSS ao redor da mesa.
- **Conflito de telas estreitas (viewport < 360px):** Jogadores na esquerda e direita podem encavalar com o centro da mesa.
  - *Mitigação:* Ajustar paddings e tamanhos compactos dos badges laterais para caber confortavelmente em telas a partir de 320px.

## Impactos esperados
- Salas WebApp com experiência clássica de 4 jogadores no formato de cruz (mais organizado e focado).
- Links compartilhados com visibilidade imediata de quem já está na sala.
- Jogadores que chegarem atrasados recebem uma mensagem clara e elegante de que a partida já começou.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD.
- Compatível com telas mobile e navegadores desktop.

## Como testar

### Build
```bash
go build ./cmd/bot
cd web && npm run build
```

### Testes
```bash
go test -v ./internal/httpapi/...
cd web && npm test
```

### Execução
```bash
go run ./cmd/bot
```

## Rollback
Caso necessário reverter:
```bash
git checkout -- internal/httpapi web
```

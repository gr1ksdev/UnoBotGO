# Plano: restaurar-empilhamento-mais-dois-e-seletor-cor

## Pedido do usuário
O usuário relatou três ajustes cruciais de jogabilidade e experiência visual para restaurar o comportamento clássico do V1:
1. **Controle sobre o `+2` (Rebater / Empilhar)**:
   - "se eu jogo um +2 o proximo jogador é pulado automaticamente sem a posibilidade de ver as cartas, isso tira o controle do usuario, e na versao antiga tinha isso, eu quero de volta"
   - Quando alguém joga `+2`, o próximo jogador NÃO deve ser pulado automaticamente. Ele deve receber a vez, poder ver suas cartas e, se tiver outro `+2`, poder rebater (empilhar). Se não tiver ou não quiser rebater, clica em "Comprar X cartas", puxa a penalidade e passa a vez.
2. **Seletor de cor do Coringa no padrão V1**:
   - "Quando eu jogo um coringa e vou escolher a cor, eu tbm quero que voce retorne para a versao antiga"
   - Exibir 4 opções limpas de cor (`Escolha sua cor` -> `❤️ Vermelho`, etc.) e um 5º item com o resumo em texto das cartas da mão para consulta.
   - Não misturar stickers cinzas com as opções de cor.
3. **Grito de UNO em mensagem separada com reação 🥳**:
   - "O grito de uno, eu quero em uma mensagem separada e com uma reacao de 🥳"
   - Quando um jogador fica com apenas 1 carta, o grito de UNO deve ser enviado em uma mensagem dedicada no grupo e receber a reação com emoji 🥳.

## Objetivo
1. **Na engine (`internal/uno`)**:
   - Adicionar regra `StackDrawTwo bool` a `uno.Rules` (ativada em `BotRules()`).
   - Adicionar campo `DrawCounter int` em `uno.State` (gerenciado atomicamente e clonado em `Snapshot`).
   - Quando `StackDrawTwo` estiver ativa e for jogado um `DrawTwo`:
     - Em vez de aplicar penalidade e pular imediatamente, somar `s.DrawCounter += 2` e passar a vez normalmente para o próximo jogador (`next = s.next(actor, 1)`).
   - Quando for o turno de um jogador e `s.DrawCounter > 0`:
     - Apenas cartas do tipo `DrawTwo` são válidas para descarte (rebater/empilhar). Outras cartas retornam `ErrCardNotPlayable`.
     - Se o jogador executar `DrawCard`: compra `s.DrawCounter` cartas de uma vez, zera `s.DrawCounter = 0` e o turno avança automaticamente para o próximo jogador.
2. **Na camada de aplicação (`internal/game`)**:
   - Expor `DrawCounter int` em `PublicGameView`.
3. **No Telegram Adapter (`internal/telegram`)**:
   - Adicionar `SetMessageReaction(ctx context.Context, params *telego.SetMessageReactionParams) error` à interface `BotAPI`.
   - Em `renderer.go`: se `view.DrawCounter > 0`, exibir aviso de penalidade acumulada na mesa.
   - Em `inline.go`:
     - Se `view.Public.DrawCounter > 0`: a opção de compra vira "Comprar X cartas". Cartas na mão que não forem `+2` aparecem desabilitadas (cinzas).
     - Na fase `ChoosingColor`: retornar apenas os 4 artigos de cor ("Escolha sua cor") e 1 artigo de consulta da mão ("Cartas: ..."), retornando imediatamente sem adicionar stickers cinzas.
     - Ao processar `ChosenInlineResult`: se houver evento `UnoAnnounced`, enviar mensagem separada no grupo (`📢 [Nome] Gritou UNO!`) e adicionar a reação `🥳` na mensagem enviada.

## Contexto atual
- `g.play` em `internal/uno/game.go` tratava `DrawTwo` aplicando penalidade imediata no próximo jogador e pulando seu turno (`next = s.next(actor, 2)`).
- Em partidas de 2 jogadores, isso fazia o jogador que jogou o `+2` jogar novamente em seguida, sem que o adversário tivesse a chance de abrir as cartas nem de contra-atacar.
- Na fase `ChoosingColor`, o bot misturava os 4 artigos de cor com todos os stickers cinzas da mão do jogador.
- O aviso de UNO vinha apenas embutido na confirmação da jogada ou no texto da mesa.

## Arquivos analisados
- `internal/uno/rules.go`
- `internal/uno/state.go`
- `internal/uno/game.go`
- `internal/game/views.go`
- `internal/telegram/client.go`
- `internal/telegram/mock_test.go`
- `internal/telegram/renderer.go`
- `internal/telegram/inline.go`
- `internal/telegram/inline_test.go`
- `actions.go` e `results.go` (referências do V1)

## Arquivos que serão modificados
- `internal/uno/rules.go`
- `internal/uno/state.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/game/views.go`
- `internal/telegram/client.go`
- `internal/telegram/mock_test.go`
- `internal/telegram/renderer.go`
- `internal/telegram/inline.go`
- `internal/telegram/inline_test.go`

## Estratégia de implementação
1. **Engine (`internal/uno`)**:
   - Adicionar `StackDrawTwo bool` a `Rules` (`BotRules()` = true).
   - Adicionar `DrawCounter int` a `State`.
   - Em `g.play`:
     - Se `s.Rules.StackDrawTwo && card.Rank == DrawTwo`:
       `s.DrawCounter += 2`
       `next = s.next(actor, 1)`
   - Em `playable`:
     - Se `s.DrawCounter > 0 && card.Rank != DrawTwo`: retornar `ErrCardNotPlayable`.
   - Em `takeAction` (`DrawCard`):
     - Se `s.DrawCounter > 0`: comprar `s.DrawCounter` cartas, resetar `s.DrawCounter = 0`, passar turno para `s.next(p.ID, 1)`.
2. **Camada de Aplicação (`internal/game`)**:
   - Adicionar `DrawCounter int` a `PublicGameView` e repassar em `buildPublicView`.
3. **Telegram Adapter (`internal/telegram`)**:
   - `client.go`: Adicionar `SetMessageReaction` a `BotAPI` e implementar em `SafeAPICaller` (chamando `telegoBot.SetMessageReaction`).
   - `mock_test.go`: Implementar `SetMessageReaction` gravando chamadas em `SentReactions`.
   - `inline.go`:
     - Se `view.Public.Phase == uno.ChoosingColor`:
       - Se for o jogador da vez: 4 artigos de cor ("Escolha sua cor") e 1 artigo de resumo das cartas. Retornar imediatamente sem stickers cinzas.
       - Se for outro jogador: artigo de espera.
     - Se `view.Public.DrawCounter > 0`:
       - Sticker de draw exibe quantidade a comprar.
       - Apenas cartas `DrawTwo` ficam jogáveis na mão.
     - Em `HandleChosenInlineResult`:
       - Ao detectar `ev.Type == uno.UnoAnnounced`:
         Enviar mensagem separada: `📢 [Link] <b>Gritou UNO!</b>`
         Em seguida, adicionar reação `🥳` nessa mensagem via `SetMessageReaction`.

## Passos detalhados
1. Modificar `internal/uno` com suporte a `StackDrawTwo` e `DrawCounter`.
2. Atualizar `internal/game/views.go` repassando `DrawCounter`.
3. Atualizar `internal/telegram/client.go` e `internal/telegram/mock_test.go` com `SetMessageReaction`.
4. Atualizar `internal/telegram/inline.go` com seletor de cor, empilhamento e grito de UNO separado com reação.
5. Adicionar testes em `internal/uno/game_test.go` e `internal/telegram/inline_test.go`.
6. Executar suíte completa de testes com `go test -count=1 -race ./...`.

## Riscos
Nenhum. A flag `StackDrawTwo` mantém `ClassicRules()` intacto para regras oficiais estritas.

## Impactos esperados
- O jogador que recebe um `+2` agora pode ver suas cartas e rebater com outro `+2` ou comprar as cartas acumuladas.
- O seletor de cor fica limpo, sem stickers cinzas misturados e permitindo consultar a mão.
- O grito de UNO gera uma mensagem dedicada com a reação festiva 🥳.

## Compatibilidade
- Linux, macOS, Windows, Docker: Sim.

## Como testar
### Testes automatizados
```bash
go test -count=1 -race ./...
```

## Rollback
Reverter os arquivos modificados.

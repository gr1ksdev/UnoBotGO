# Plano: corrigir-coringa-mais-quatro-e-cache-cartas

## Pedido do usuário
O usuário relatou dois problemas ao jogar a carta +4 Coringa (Wild Draw 4):
1. **Ver cartas idêntico entre os jogadores**:
   - "quando um player joga um +4 coringa, para o outro player o ver cartad fica iguao ao do player que jogou o +4 coringa"
2. **Pulo automático do próximo jogador**:
   - "alem disso quando selecionado uma cor ele pula o proximo player automaticamente."

## Objetivo
1. **Não pular o próximo jogador após o +4**:
   - Alinhar a mecânica do `+4 Coringa` em `BotRules()` com a mesma filosofia já aplicada ao `+2` (`StackDrawTwo`):
     - Ao jogar o +4 e escolher a cor, acumular a penalidade no `DrawCounter` (`+4`) e passar o turno normalmente para o próximo jogador (alvo), sem executar penalidade forçada imediata e sem pular a sua vez.
     - O próximo jogador recebe o turno, pode ver suas cartas e a penalidade acumulada (`⚠️ Penalidade acumulada: comprar X cartas!`).
     - Se tiver outro `+4`, pode rebater/empilhar (`StackWildDrawFour`).
     - Se não tiver ou não quiser rebater, clica na opção de compra ("Comprar X cartas"), puxa a penalidade e a vez avança.
2. **Eliminar cache e sincronizar "Suas cartas"**:
   - Adicionar a revisão atual da partida na query do botão `🃏 Suas cartas`: `g_<gameID>_<revision>`. Dessa forma, o cliente do Telegram invalida o cache local a cada jogada/revisão e nunca reaproveita as opções da rodada anterior ou do outro jogador.
   - Tratar a query inline em `inline.go` extraindo o `gameID` mesmo com o sufixo `_<revision>`.
   - Na fase `ChoosingColor`, para jogadores que não são o `ColorChooserID`:
     - Exibir claramente o aviso "Aguardando escolha de cor" e um resumo de suas próprias cartas, retornando imediatamente (sem poluição de stickers ou reaproveitamento de seletores de cor).

## Contexto atual
- Em `internal/uno/game.go` (`choose`), quando a cor do +4 é escolhida e `StackDrawTwoOnWildFour` é falso, a engine executa `g.penalty(s, next, 4, events)` e faz `next = s.next(next, 1)`. Isso aplica a penalidade compulsoriamente e pula a vez da vítima sem que ela possa interagir ou rebater.
- Em `internal/telegram/commands.go`, o botão `makeGameButtons` gera a query estática `g_%s` (apenas `view.GameID`). O cliente do Telegram faz cache local do inline query por string de busca; quando o segundo jogador clica em "Suas cartas", o Telegram entrega o resultado do primeiro jogador em cache.
- Em `internal/telegram/inline.go`, durante `ChoosingColor`, se quem abre a mão não for o autor da escolha, o bot inseria o artigo de espera mas continuava a execução para o loop de stickers cinzas.

## Arquivos analisados
- `internal/uno/rules.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/uno/lifecycle_test.go`
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/inline.go`
- `internal/telegram/inline_test.go`
- `.agent/memory/memory.md`

## Arquivos que poderão ser modificados
- `internal/uno/rules.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/telegram/commands.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/inline.go`
- `internal/telegram/inline_test.go`

## Estratégia de implementação
1. **Regras da Engine (`internal/uno`)**:
   - Em `Rules`: adicionar `StackWildDrawFour bool`.
   - Em `BotRules()`: definir `StackWildDrawFour: true`.
   - Em `playable`: se `s.DrawCounter > 0`, permitir `card.Rank == WildDrawFour` quando `s.Rules.StackWildDrawFour` for verdadeiro.
   - Em `choose`: se `pending.DrawCount != 0` e `s.Rules.StackWildDrawFour`:
     - Acumular `s.DrawCounter += pending.DrawCount`.
     - O próximo jogador da vez é o alvo (`next = pending.Target`), sem pular e sem aplicar `penalty` forçada.
2. **Invalidação de Cache Inline no Telegram (`internal/telegram`)**:
   - Em `makeGameButtons`: gerar `SwitchInlineQueryCurrentChat: fmt.Sprintf("g_%s_%d", view.GameID, view.Revision)`.
   - Em `buildGameSelectorResults`: usar `fmt.Sprintf("g_%s_%d", g.GameID, g.Revision)`.
   - Em `HandleInlineQuery`: fazer o parse de `rawQuery` com prefixo `g_`, usando `strings.SplitN(queryBody, "_", 2)[0]` para obter o `GameID`.
   - Em `buildPlayerHandResults`: quando `view.Public.Phase == uno.ChoosingColor` e `actorID != view.Public.ColorChooserID`, adicionar artigo de espera e resumo das próprias cartas, retornando `results, ""` imediatamente.

## Passos detalhados
1. Atualizar `internal/uno/rules.go` com `StackWildDrawFour` ativo em `BotRules()`.
2. Atualizar `internal/uno/game.go` para suportar acúmulo de +4 e resposta/rebate de +4 com `DrawCounter`.
3. Atualizar `internal/telegram/commands.go` com a query dinâmica revisionada `g_<gameID>_<revision>`.
4. Atualizar `internal/telegram/inline.go` para suportar a query revisionada e limpar a resposta de espera no `ChoosingColor`.
5. Atualizar e criar testes unitários em `internal/uno` e `internal/telegram`.
6. Rodar toda a suite de testes (`go test ./...`) e verificar ausência de regressões.

## Riscos
- *Risco:* Testes legados de `internal/uno` que supõem que +4 em `ClassicRules` ou `BotRules` aplicam penalidade imediata.
  - *Mitigação:* `ClassicRules()` manterá `StackWildDrawFour: false`, preservando exatamente a especificação de Mattel. Apenas `BotRules()` (usado pelo bot) terá a mecânica dinâmica ativa. Testes específicos de `BotRules` serão ajustados para validar o novo fluxo.

## Impactos esperados
- O próximo jogador após um +4 não é pulado de surpresa; ele ganha a vez para comprar as cartas ou rebater com outro +4.
- Ao clicar em "Suas cartas", cada jogador visualiza exclusivamente a sua própria mão, eliminando interferência de cache entre contas/jogadores.

## Compatibilidade
- Linux / Termux / macOS / Windows / Docker

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -v ./internal/uno/...
go test -v ./internal/telegram/...
go test ./...
```

## Rollback
Descartar alterações com `git checkout` dos arquivos modificados.

## Observações
Preservar o isolamento do motor de regras e a serialização dos handlers por chat.

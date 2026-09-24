# Plano: restaurar-regras-v1

## Pedido do usuário
"olha, vamos fazer assim. volte todo as regras de jogo da v1, pfvr. mantendo os textos como estao"

## Objetivo
Restaurar fielmente todas as regras de jogabilidade e mecânicas de cartas originais do V1 na engine (`internal/uno`), mantendo 100% inalterados os textos, layouts visuais, menções, stickers e mensagens do Telegram:

1. **Empilhamento e resposta a penalidades**:
   - `+2` soma `DrawCounter += 2` e passa para o próximo jogador, que pode rebater com outro `+2` ou comprar as cartas acumuladas e passar a vez.
   - `+4` soma `DrawCounter += 4`, pede a escolha de cor, e passa para o próximo jogador, que pode rebater (com outro `+4` ou `+2` no modo caseiro) ou comprar as cartas acumuladas e passar a vez, sem ser pulado automaticamente.
2. **Proibição de bater com carta especial (Wild / Coringa / +4)**:
   - Se o jogador tiver apenas 1 carta na mão (`len(p.Hand) == 1`) e ela for um Coringa ou +4 (`Rank >= Wild`), ela NÃO pode ser jogada para vencer a partida.
3. **Proibição de Coringa sobre Coringa (Wild sobre Wild / Special sobre Special)**:
   - Não é permitido jogar Coringa ou +4 sobre outro Coringa ou +4 no topo do descarte (salvo se for resposta para empilhar penalidade).
4. **+4 Coringa livre (sem restrição de cor na mão)**:
   - Remover a restrição da Mattel que bloqueava o +4 se o jogador tivesse a cor ativa na mão; no V1, o +4 pode ser jogado a qualquer momento.
5. **Compra livre (Draw)**:
   - Ao comprar 1 carta voluntariamente, o turno NÃO passa automaticamente se a carta comprada não for jogável.
   - O jogador pode jogar qualquer carta jogável da sua mão, ou clicar no botão "Passar".
6. **Manter os textos exatamente como estão**:
   - Preservar toda a camada de apresentação em HTML, formatação de status, links com menção segura, layout inline, grito de UNO separado com reação `🥳` e queries revisionadas.

## Contexto atual
- A engine V2 (`internal/uno`) foi originalmente desenvolvida baseando-se estritamente nas regras do manual da Mattel (W2085), que bloqueavam o +4 caso o jogador tivesse a cor, forçavam que após comprar só a carta recém-comprada pudesse ser descartada (passando a vez se ela fosse inválida), e permitiam bater com Coringa ou jogar Coringa sobre Coringa.
- Em `docs/v2-rules.md`, essas diferenças foram registradas como desvios intencionais da V1 que não haviam sido portados. O usuário agora solicita restaurar integralmente o comportamento do V1.

## Arquivos analisados
- `player.go` (regras V1: `cardPlayable`, `PlayableCards`)
- `game.go` (regras V1: `PlayCard`, `Turn`, `firstCard`)
- `actions.go` (regras V1: `doDraw`, `doPass`, `playCard`)
- `internal/uno/rules.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/uno/lifecycle_test.go`
- `internal/telegram/renderer.go`
- `internal/telegram/inline.go`
- `docs/v2-rules.md`

## Arquivos que poderão ser modificados
- `internal/uno/rules.go`
- `internal/uno/game.go`
- `internal/uno/game_test.go`
- `internal/uno/lifecycle_test.go`

## Estratégia de implementação
1. **Configuração de Regras em `internal/uno/rules.go`**:
   - Adicionar os campos na struct `Rules`:
     - `NoWildFinish bool`: impede vencer jogando Coringa / +4 como última carta.
     - `NoWildOnWild bool`: impede jogar Coringa / +4 sobre outro Coringa / +4.
     - `AllowWildDrawFourAlways bool`: permite jogar +4 mesmo tendo cartas da cor ativa.
     - `FreePlayAfterDraw bool`: após compra voluntária, permite jogar qualquer carta jogável da mão ou passar (sem passar turno compulsoriamente).
   - Ativar todas essas flags em `BotRules()` (e por herança em `CaseiroRules()`).
   - Manter `ClassicRules()` sem essas flags para conformidade clássica nos testes de compatibilidade Mattel.
2. **Lógica em `internal/uno/game.go`**:
   - Em `playable`:
     - Se `s.Rules.NoWildFinish && len(p.Hand) == 1 && card.Rank >= Wild`: retornar `ErrCardNotPlayable`.
     - Se `s.Rules.NoWildOnWild && top.Rank >= Wild && card.Rank >= Wild`: se não for empilhamento válido, retornar `ErrCardNotPlayable`.
     - Se `card.Rank == WildDrawFour`: validar `CanPlayDrawFour` apenas se `!s.Rules.AllowWildDrawFourAlways`.
     - Se `s.DrawnCardID != ""`: só restringir a `s.DrawnCardID == id` se `!s.Rules.FreePlayAfterDraw`.
   - Em `takeAction`:
     - No `case DrawCard`: ao comprar 1 carta voluntariamente, só passar o turno automaticamente se `!s.Rules.FreePlayAfterDraw`. Com a flag ativa, mantém a vez no jogador, permitindo que ele jogue ou passe.
3. **Validação por Testes**:
   - Adicionar cenários para cada uma das regras restauradas em `internal/uno/game_test.go`.
   - Rodar a suite completa de testes (`go test ./...`) e `go vet ./...`.

## Passos detalhados
1. Adicionar as novas flags de regras em `internal/uno/rules.go` e habilitá-las em `BotRules()`.
2. Implementar as validações em `playable` e a compra livre em `takeAction` no `internal/uno/game.go`.
3. Atualizar os testes em `internal/uno/game_test.go` e `internal/uno/lifecycle_test.go`.
4. Executar os testes unitários e verificar o comportamento sem erros.

## Riscos
- *Risco:* Testes legados de colocação ou ciclo determinístico podem esperar vitória com carta Wild ou descarte restrito.
  - *Mitigação:* `ClassicRules()` permanece intocado; testes de `BotRules` serão ajustados com cartas não-wild para vitória e respeitando as novas validações.

## Impactos esperados
- Jogabilidade idêntica ao V1 clássico (sem bater com coringa, sem coringa sobre coringa, +4 livre, compra livre, empilhamento dinâmico).
- Apresentação visual e textos no Telegram 100% preservados conforme a V2 atual.

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
Nenhum texto do Telegram será alterado.

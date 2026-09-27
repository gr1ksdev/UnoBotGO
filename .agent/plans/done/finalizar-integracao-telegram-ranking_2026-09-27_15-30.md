# Plano: finalizar-integracao-telegram-ranking

## Pedido do usuário
Consulte o ai-memory do workstream atual e recupere o contexto da última sessão do Codex. Identifique a tarefa que estava em andamento, o que já foi implementado e o que ainda falta. Depois continue de onde o Codex parou.

## Objetivo
Completar o Passo 3 da continuação da Milestone M7 (aprovada para elegibilidade definitiva): finalizar a integração em `internal/telegram` para envio da mensagem adicional de pontuação após confirmação de commit do resultado no PostgreSQL, adicionar testes unitários completos em `internal/telegram/results_test.go`, atualizar `docs/m7-persistence.md` com o status atualizado e realizar o commit organizado na branch `dev`.

## Contexto atual
- A sessão do Codex (`01a0d65e-86e5-75a2-b370-035a043845ca`) executou a auditoria de elegibilidade e implementou os Passos 1 e 2 da continuação aprovada:
  - Commit `9dc0cd8`: `feat(ranking): define eligibility from completed placements` (cálculo determinístico de elegibilidade e pontuação pura).
  - Commit `25f7f90`: `feat(storage): persist awards only for eligible finishers` (migration 0005, persistência transacional e stats apenas para elegíveis com N>=2).
- No momento em que o Codex encerrou, os seguintes arquivos estavam modificados no working tree (não commitados):
  - `internal/telegram/bot.go`
  - `internal/telegram/commands.go`
  - `internal/telegram/inline.go`
  - `internal/telegram/results.go`
- A lógica de disparo de notificação pós-commit (`notifyPoints`) após o envio da mensagem compacta de fim de jogo foi implementada nos handlers (`bot.go`, `commands.go`, `inline.go`), mas ainda faltam:
  1. Testes unitários completos em `internal/telegram/results_test.go` validando o envio da mensagem de pontuação, formatação (Legacy e Updated), ordenação de colocados vs fora do ranking, HTML escaping, e não reenvio em retries já persistidos (`AlreadyPersisted`).
  2. Atualização do documento `docs/m7-persistence.md` documentando a conclusão da elegibilidade definitiva e o comportamento de anúncio no Telegram.
  3. Verificação de build/vet/race e testes gerais.
  4. Criação do commit `feat(telegram): announce committed ranking points after game completion`.

## Arquivos analisados
- `.agent/plans/approved/m7-persistencia-ranking_2026-09-27_13-33.md`
- `docs/m7-persistence.md`
- `internal/telegram/bot.go`
- `internal/telegram/commands.go`
- `internal/telegram/inline.go`
- `internal/telegram/results.go`
- `internal/telegram/results_test.go`
- `internal/ranking/eligibility.go`
- `internal/storage/postgres/results.go`

## Arquivos que poderão ser modificados
- `internal/telegram/results_test.go`
- `docs/m7-persistence.md`
- `.agent/memory/memory.md` (se necessário atualizar notas de memória)
- (E os já modificados pelo Codex que serão validados e commitados: `internal/telegram/bot.go`, `internal/telegram/commands.go`, `internal/telegram/inline.go`, `internal/telegram/results.go`)

## Estratégia de implementação
1. Revisar as alterações já presentes em `internal/telegram/`:
   - `finalizeOutcome`: retorna callback `notify` somente se commit teve pontuação e não era retry já persistido.
   - `notifyPoints`: envia a mensagem formatada para o chat do jogo usando `renderPoints`.
   - `renderPoints`: formatação consistente com Legacy ("📊 Resultado do ranking") e Updated ("📊 Pontuação da partida"), separador decimal `,` com duas casas para Updated, inteiro para Legacy, escape HTML e sufixo `(fora do ranking)` para não elegíveis.
   - Disparo nos 3 pontos de encerramento (`inline.go`, `commands.go`, `bot.go` timeout) executado estritamente APÓS o envio da mensagem final do jogo.
2. Adicionar testes unitários detalhados em `internal/telegram/results_test.go`:
   - Teste de `renderPoints` para Legacy e Updated com jogadores elegíveis, jogadores não elegíveis (abandonos) e nomes com caracteres HTML especiais.
   - Teste do fluxo de finalização com mock BotAPI verificando:
     - Ordem das mensagens: mensagem de fim de jogo enviada primeiro, depois mensagem de pontuação.
     - Persistência com N<2 (`insufficient_eligible_players` / `Scored=false`): nenhuma mensagem de pontuação enviada.
     - Falha de persistência: resultado mantido em memória, nenhuma mensagem de pontuação enviada.
     - `RetryPendingResults`: ao ter sucesso, envia a mensagem de pontuação.
     - `RetryPendingResults` em resultado `AlreadyPersisted`: não reenvia mensagem de pontuação.
3. Atualizar `docs/m7-persistence.md` para refletir os commits da continuação de elegibilidade e o anúncio de resultados no Telegram.
4. Executar toda a suíte de testes: `go test ./...`, `go test -race ./...`, `go vet ./...`, `go test -tags debugcards ./...`.
5. Criar o commit correspondente na branch `dev`.

## Passos detalhados
1. Implementar novos casos de teste em `internal/telegram/results_test.go`.
2. Executar `go test -v ./internal/telegram/...` e `go test -race ./internal/telegram/...`.
3. Atualizar `docs/m7-persistence.md` com o relatório dos novos commits e comportamento do Telegram.
4. Executar verificação completa (`go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`, `git diff --check`).
5. Gerar commit `feat(telegram): announce committed ranking points after game completion`.

## Riscos
- Risco de mensagens duplicadas ou envio antes do commit: mitigado pelo retorno do callback `notify` que só é instanciado após o retorno com sucesso de `RecordCompletedGame` e chamado após o envio da mensagem principal.
- Risco de escape incorreto no HTML: mitigado pelo uso de `html.EscapeString` no `DisplayName` do jogador.

## Impactos esperados
- Jogadores recebem a pontuação em mensagem logo após a mensagem final de partida concluída ranqueável.
- Partidas com abandonos mostram claramente os abandonadores com `(fora do ranking)` e não pontuados.
- Partidas com N<2 não recebem mensagem de pontuação.

## Compatibilidade
- Linux: Sim
- macOS: Sim
- Windows: Sim
- Docker: Sim
- CI/CD: Sim

## Como testar

### Build
```bash
go build ./...
go build -tags debugcards ./...
go vet ./...
```

### Testes
```bash
go test ./...
go test -race ./...
go test -tags debugcards ./...
git diff --check
```

### Execução
```bash
go run ./cmd/bot
```

## Rollback
Desfazer o commit com `git revert` ou `git reset HEAD~1` (localmente na branch `dev`).

## Observações
As decisões de produto relativas a setup interativo via botões, troca de sistema de ranking em grupo com histórico e aplicação de importações legadas permanecem bloqueadas conforme o plano M7 principal.

# Plano: atualizar-template-ranking-privado

## Pedido do usuário
Atualizar o template de mensagem do comando `/ranking` no privado para incluir os emojis de seção (`⚡ Atualizado` e `🕹️ Legado`), marcadores de lista (`•`) para cada grupo participante, e ajuste de espaçamento (quebra de linha simples após o cabeçalho da seção e linha em branco antes do `Total`).

Exemplo do novo formato:
```text
🏆 Seus rankings · Setembro

⚡ Atualizado
• UNO da Galera · 30,00 pts
• Amigos do UNO · 10,00 pts

Total · 40,00 pts

🕹️ Legado
• Jogatina BR · 0 pts

Total · 0 pts
```

## Objetivo
1. Modificar os cabeçalhos de seção em `RenderUserMonthlyRankings`:
   - `⚡ Atualizado`
   - `🕹️ Legado`
2. Adicionar o prefixo `• ` em cada linha de grupo.
3. Ajustar o espaçamento:
   - Quebra de linha simples (`\n`) entre o cabeçalho da seção e o primeiro item da lista.
   - Linha em branco (`\n\n`) entre o último item da lista e o `Total · ...`.
   - Linha em branco (`\n\n`) entre seções.
4. Ajustar a formatação de itens omitidos por limite de mensagem UTF-16 para manter a consistência de marcadores (`• … e mais X grupos.`).
5. Atualizar os testes unitários correspondentes em `internal/telegram/ranking_test.go`.

## Contexto atual
- O `/ranking` privado já está implementado e validado funcionalmente no working tree da dev.
- O renderizador atual em `internal/telegram/ranking.go` exibe as seções com títulos `Atualizado` e `Legado` sem emojis, sem marcador `•` nas linhas de grupo e com linha em branco dupla após o cabeçalho da seção.

## Arquivos analisados
- `internal/telegram/ranking.go`: Função `RenderUserMonthlyRankings` e helper `privateRankingRemaining`.
- `internal/telegram/ranking_test.go`: Testes unitários de renderização (`TestRenderUserMonthlyRankings_*` e `TestPrivateRankingCommand_*`).

## Arquivos que poderão ser modificados
- `internal/telegram/ranking.go`: Ajuste de strings de cabeçalho, marcadores de linha e espaçamentos.
- `internal/telegram/ranking_test.go`: Atualização das strings esperadas nos testes unitários.

## Estratégia de implementação
1. Em `internal/telegram/ranking.go`:
   - No `renderSection`, mudar `header := prefix + secHeader + "\n"` (uma única quebra de linha após o título da seção).
   - Cada linha de grupo: `fmt.Sprintf("• %s · %s", ...)`.
   - Em `renderSection(rankings.Updated, "⚡ Atualizado", first)` e `renderSection(rankings.Legacy, "🕹️ Legado", first)`.
   - Ajustar `privateRankingRemaining` para incluir o bullet `• … e mais %d grupos.`.
2. Em `internal/telegram/ranking_test.go`:
   - Atualizar asserts dos testes `TestRenderUserMonthlyRankings_UpdatedOnly`, `TestRenderUserMonthlyRankings_LegacyOnly`, `TestRenderUserMonthlyRankings_BothSeparated`, `TestRenderUserMonthlyRankings_HtmlEscapingAndFallback` e `TestPrivateRankingCommand_ScopesToSenderAndNoButtons`.
3. Validar:
   - `go test -race ./internal/telegram/...`
   - `go test ./...`
   - `go vet ./...`
   - `git diff --check`

## Riscos
- Risco zero de regressão em lógica de negócio ou persistência (mudança puramente visual e cosmética de formatação de string).

## Impactos esperados
- Melhoria visual e de legibilidade do ranking privado no Telegram, com ícones temáticos e marcadores claros de lista.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -race ./internal/telegram/...
go test ./...
go vet ./...
git diff --check
```

### Execução
Executar testes de unidade e verificar a renderização exata conforme template fornecido pelo usuário.

## Rollback
Reverter alterações em `internal/telegram/ranking.go` e `internal/telegram/ranking_test.go` via `git restore`.

## Observações
- Não fazer commit ou push.
- Não mexer na branch main.
- Nenhuma alteração em regras de persistência ou banco de dados.

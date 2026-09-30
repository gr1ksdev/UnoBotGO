# Plano: melhoria-ux-config

## Pedido do usuário
Realizar melhoria visual e de UX na mensagem do comando `/config` no Telegram:
- Explicar de forma curta em blockquotes apenas o modo de jogo e o sistema de ranking atualmente selecionados.
- Exibir exatamente dois blockquotes: um para o modo e um para o sistema de ranking.
- Incluir o separador `────────────` em linha própria entre os dois blockquotes.
- Textos dos resumos de modo:
  - Clássico: `🎮 Clássico` / `Regras padrão do bot, sem as combinações extras do modo Caseiro.`
  - Caseiro: `🎮 Caseiro` / `Permite combinações extras entre cartas de compra, como +4 sobre +2 e +2 da cor escolhida sobre +4.`
- Textos dos resumos de ranking:
  - Legado: `🏆 Legado` / `Todos os jogadores elegíveis, exceto o último colocado, recebem +1 ponto.`
  - Atualizado: `🏆 Atualizado` / `A pontuação varia conforme a colocação: quanto melhor a posição, mais pontos o jogador recebe.`
- Rodapé atualizado:
  ```text
  Selecione abaixo para alterar.
  As mudanças afetarão apenas as próximas partidas criadas.
  ```
- Garantir atualização dinâmica ao clicar nos botões de modo/ranking, refletindo os novos resumos e removendo os antigos.
- Preservar regras de gameplay, ranking, permissões, botões e callbacks.
- Não mexer em banco de dados ou migrations.
- Não fazer commit ou push.

## Objetivo
Centralizar a formatação da mensagem no método `RenderGroupConfig` de `Renderer` utilizando HTML compatível com o Telegram (`<blockquote>...</blockquote>`), garantindo que tanto a abertura inicial via `/config` (ou botão de boas-vindas) quanto a edição via callbacks de modo e ranking reflitam instantaneamente a seleção ativa com seus respectivos resumos e separador.

## Contexto atual
- A mensagem de `/config` é gerada por `r.RenderGroupConfig(config groups.Config)` em `internal/telegram/renderer.go`.
- O envio inicial é feito em `internal/telegram/commands.go` (`handleConfig`) com ParseMode HTML.
- As atualizações de modo (`cfg_mode`), ranking (`cfg_rank`) e reabertura (`cfg_open`) em `internal/telegram/callbacks.go` já utilizam exatamente `r.RenderGroupConfig(cfg)` e editam a mensagem com `telego.ModeHTML`.
- O ParseMode atual de ambas as mensagens é HTML.

## Arquivos analisados
- `internal/telegram/renderer.go`
- `internal/telegram/callbacks.go`
- `internal/telegram/commands.go`
- `internal/telegram/renderer_test.go`
- `internal/telegram/config_test.go`

## Arquivos que poderão ser modificados
- `internal/telegram/renderer.go`
- `internal/telegram/renderer_test.go`
- `internal/telegram/config_test.go`

## Estratégia de implementação
1. Em `internal/telegram/renderer.go`:
   - Criar helpers unexported:
     - `groupModeSummary(mode groups.Mode) (title string, desc string)`
     - `groupRankingSummary(system groups.RankingSystem) (title string, desc string)`
   - Atualizar `RenderGroupConfig` para montar a string HTML com os dois blockquotes e o separador `────────────` entre eles, seguido do rodapé em duas linhas.
2. Como `internal/telegram/commands.go` e `internal/telegram/callbacks.go` já chamam `r.RenderGroupConfig(cfg)`, a atualização dinâmica pós-callback e pós-comando funcionará imediatamente de forma idêntica e consistente.
3. Em `internal/telegram/renderer_test.go`:
   - Criar testes unitários para testar as 4 permutações de configurações (Clássico+Legado, Clássico+Atualizado, Caseiro+Legado, Caseiro+Atualizado).
   - Validar a presença de exatamente 2 tags `<blockquote>` e 2 `</blockquote>`.
   - Validar a presença e posição do separador `────────────`.
   - Validar que as opções não selecionadas não aparecem.
4. Em `internal/telegram/config_test.go`:
   - Validar no fluxo de callbacks que, ao alternar entre Clássico e Caseiro ou Legado e Atualizado, a mensagem editada reflete os novos textos e remove os resumos anteriores.
   - Garantir que permissões e regras de autorização permanecem intactas.

## Passos detalhados
1. Implementar helpers `groupModeSummary` e `groupRankingSummary` e atualizar `RenderGroupConfig` em `internal/telegram/renderer.go`.
2. Adicionar suíte de testes unitários para `RenderGroupConfig` em `internal/telegram/renderer_test.go`.
3. Adicionar asserções de conteúdo dinâmico pós-callback em `internal/telegram/config_test.go`.
4. Executar toda a suíte de testes:
   - `go test ./...`
   - `go test -race ./...`
   - `go test -tags debugcards ./...`
   - `go vet ./...`
   - `go build ./...`
   - `git diff --check`
5. Apresentar os resultados, confirmações de não-execução de commit/push e exemplos de texto para as quatro combinações.

## Riscos
- **Risco**: Quebra de formatação HTML no Telegram caso tags não sejam fechadas.
  - **Mitigação**: Uso de tags HTML estáticas rigorosamente balanceadas (`<blockquote><b>...</b>\n...</blockquote>`) sem interpolação de input do usuário.
- **Risco**: Divergência entre envio inicial e edição via callback.
  - **Mitigação**: Ambos já utilizam `RenderGroupConfig` exclusivamente, garantindo 100% de paridade.

## Impactos esperados
- Usuários que abrem `/config` ou alteram opções verão explicações claras e contextuais do que está ativo no momento, tornando a experiência intuitiva e evitando dúvidas sobre o comportamento das partidas no grupo.

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
```

### Testes
```bash
go test -v -race ./internal/telegram/...
go test -race ./...
```

### Execução
Validar visualização com dados de teste e verificar a estrutura do texto retornado por `RenderGroupConfig`.

## Rollback
Desfazer alterações no working tree:
```bash
git checkout -- internal/telegram/renderer.go internal/telegram/renderer_test.go internal/telegram/config_test.go
```

## Observações
- Não requer nenhuma alteração de schema/migration.
- Não afeta gameplay ou cálculo de ranking.
- Nenhuma alteração de botões ou callbacks além da atualização do texto já existente.

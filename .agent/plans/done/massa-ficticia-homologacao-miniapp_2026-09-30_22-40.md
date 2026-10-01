# Plano: massa-ficticia-homologacao-miniapp

## Pedido do usuário
Criar uma massa de dados fictícios especificamente para homologação visual e funcional do Mini App de Ranking Global do UnoBotGO, permitindo testar casos extremos (listas grandes, paginação >50 itens, nomes longos, emojis, empates/tie-breaks, ausência de avatar/fallbacks, grupos pequenos de 2 jogadores, médios e grandes com 50-80 jogadores, scores altos até 25.000,00 pts e isolamento estrito entre os sistemas Atualizado e Legado).
A solução deve:
- Exercitar o fluxo real de persistência e queries PostgreSQL, sem mocks no frontend;
- Ter proteção robusta contra execução acidental em produção (fail-closed, exigindo `APP_ENV=development` ou `ALLOW_DEV_SEED=1`);
- Ser determinística, identificável por faixa de IDs reservados e GameIDs prefixados;
- Ser 100% idempotente (execuções repetidas não duplicam scores nem criam registros duplicados);
- Fornecer comando simples para popular (`make seed-miniapp` / `go run ./cmd/devseed miniapp`) e comando simples para cleanup cirúrgico (`make clean-miniapp-seed` / `go run ./cmd/devseed clean-miniapp`);
- Não criar migrations nem alterar schema em banco;
- Respeitar a regra temporal oficial de `America/Sao_Paulo` para o mês corrente;
- Ser mantida 100% na branch `dev` no working tree, sem commits ou pushes.

## Objetivo
Implementar o utilitário de desenvolvimento `cmd/devseed` e pacote `internal/devseed`, com suporte a população determinística e limpeza reversível de fixtures de homologação do Mini App, targets correspondentes no `Makefile` e testes automatizados de segurança, idempotência, isolamento de sistemas, agregação e tie-breaks.

## Contexto atual
- O Mini App consome `/api/v1/rankings/groups`, `/api/v1/rankings/players` e `/api/v1/rankings/groups/{ref}`.
- O backend consulta `player_group_monthly_stats`, `group_configs`, `completed_games` e `completed_game_players` (para desempates no detalhe via CTE `latest`).
- O limite padrão de paginação é 50 itens (`limit=50`).
- Atualmente o banco de desenvolvimento contém poucos registros manuais, insuficientes para validar scroll infinito, paginação, tie-breaks múltiplos, truncamento de nomes de 80 caracteres ou grupos com dezenas de jogadores.
- Não há ferramenta oficial de dev seed para o ranking no repositório.

## Arquivos analisados
- `internal/storage/postgres/migrations/` (tabelas: `group_configs`, `completed_games`, `completed_game_players`, `player_group_stats`, `player_group_monthly_stats`, `known_group_users`)
- `internal/storage/postgres/global_rankings.go` (queries reais de grupos, players e detalhe do Mini App)
- `internal/storage/postgres/results.go` (`RecordCompletedGame`, inserções e updates acumulados e mensais)
- `internal/storage/postgres/ranking.go` (`ListGroupRanking`, query de ranking com tie-break)
- `internal/ranking/global.go` (`GlobalRequest`, `GlobalPage`, `GlobalService`)
- `internal/ranking/ranking.go` e `internal/ranking/eligibility.go` (cálculo de scores, validação e idempotência)
- `internal/ranking/time.go` (cálculo canônico de mês em `America/Sao_Paulo`)
- `Makefile` (padrões de targets e convenções de build)
- `cmd/migrate/main.go` e `cmd/bot/main.go` (padrões de inicialização de CLI)

## Arquivos que poderão ser criados/modificados
- `cmd/devseed/main.go` (novo: CLI executável para `seed` e `clean`)
- `internal/devseed/seed.go` (novo: gerador de massa determinística, validações de ambiente, transação e remoção cirúrgica)
- `internal/devseed/seed_test.go` (novo: testes unitários e de integração de segurança, idempotência, tie-breaks, sistemas)
- `Makefile` (modificado: adição dos targets `seed-miniapp` e `clean-miniapp-seed`)
- `.agent/memory/memory.md` (modificado: registro de memória técnica)
- `.agent/decisions.md` (modificado: registro da decisão de design)

## Estratégia de implementação

1. **Namespace e Identificação Determinística dos Dados Fictícios**:
   - Chat IDs reservados para grupos do seed: faixa negativa `[-990000000001, -990000000999]` (claramente fora de supergrupos normais).
   - User IDs reservados para jogadores do seed: faixa positiva `[9900000001, 9900000999]` (distinta de contas reais).
   - Game IDs reservados: prefixo `devseed_game_<system>_<seq>`.
   - Remoção cirúrgica no cleanup baseada estritamente nessa tripla de restrições (`chat_id BETWEEN ...`, `user_id BETWEEN ...`, `game_id LIKE 'devseed_%'`), garantindo que nenhum dado real seja tocado.

2. **Proteção Forte contra Execução em Produção (Fail-Closed)**:
   - O comando exige explicitamente a variável `APP_ENV=development` ou `ALLOW_DEV_SEED=1`.
   - Verifica `DATABASE_URL`: se vazia ou se apontar para hosts de produção proibidos (ex: domínios de prod, porta de produção remota se configurada), encerra imediatamente com código de erro 1.
   - Exibe aviso prévio com nome do banco mascarado e ambiente antes de prosseguir.
   - Todo o processo de seed executa dentro de uma transação PostgreSQL única (com `tx.Commit()` somente ao final ou `rollback` imediato em caso de erro).

3. **Geração Realística da Massa de Homologação**:
   - **Mês Corrente**: calculado dinamicamente via `ranking.MonthStart(time.Now().In(ranking.RankingLocation))`, garantindo que funcione em qualquer mês.
   - **Universo Atualizado**:
     - 80 grupos com títulos variados (curtos, médios, longos de 70+ caracteres, emojis "UNO 🎴", "Só +4 😈", "Família do UNO 🃏", acentos e números, e 1 caso com título vazio para testar fallback `Grupo ••••XXXX`).
     - 180 jogadores com nomes variados (simples, compostos, longos, emojis, acentos, 1 caso vazio para testar `Jogador ••••XXXX`).
     - Grupos variados:
       - 1 grupo pequeno com exatamente 2 jogadores;
       - Vários grupos médios com 8 a 15 jogadores;
       - 1 grupo grande com 60 jogadores para testar scroll longo e performance;
     - Usuários multi-grupo: jogadores como "Freddy" presentes em 5 grupos distintos para validar agregação na aba Players.
     - Caso de display name mais recente: jogador que participou com nome antigo em jogo anterior e nome novo "Freddy UNO" no jogo mais recente, validando a precedência do nome mais novo.
     - Variedade de scores: de `0,00` a `25.000,00 pts`, passando por `5,00`, `125,50`, `999,00`, `1.250,00`, `9.999,50`.
     - Casos de empate para validação de tie-breaks:
       - Grupos com mesmo score desempatados por `last_completed_game_at`;
       - Grupos com mesmo score e mesma data desempatados por `name COLLATE "C"`;
       - Jogadores com mesmo score desempatados por `last_completed_game_at` e `display_name`;
       - Ranking interno de grupo com mesmo score desempatado por `last_placement` da partida mais recente do mês.
   - **Universo Legado**:
     - 65 grupos e 140 jogadores completamente isolados do Atualizado.
     - Sobreposição intencional de UserIDs com o Atualizado, validando que os scores não se misturam nem se somam.
     - Scores no padrão legado (`1 pt`, `2 pts`, ..., `2500 pts`).
   - **Persistência Consistente**:
     - Para cada grupo, cria a configuração em `group_configs` com `ranking_system` e `title`.
     - Registra partidas e participantes em `completed_games` e `completed_game_players` com `scoring_status = 'scored'`, `finished_at` distribuídos ao longo do mês corrente e posições válidas.
     - Popula `player_group_stats` e `player_group_monthly_stats` preservando todos os invariantes do schema e dos tie-breakers (onde o CTE `latest` lê as partidas).

4. **Idempotência**:
   - `INSERT ... ON CONFLICT (chat_id, user_id, month_start) DO UPDATE` ou verificação prévia: rodar `make seed-miniapp` repetidas vezes resulta no mesmo estado determinístico exato, sem dobrar scores nem duplicar grupos.
   - `make clean-miniapp-seed` repetido é um no-op seguro que remove apenas os registros das faixas reservadas e encerra com sucesso.

5. **Interface de Comandos e Relatório**:
   - `make seed-miniapp`: executa `go run ./cmd/devseed miniapp` com `APP_ENV=development`.
   - `make clean-miniapp-seed`: executa `go run ./cmd/devseed clean-miniapp` com `APP_ENV=development`.
   - Emite relatório estruturado ao final da execução informando mês corrente, total de grupos/jogadores por sistema, casos especiais gerados e namespace utilizado.

## Passos detalhados

1. **Implementar `internal/devseed/seed.go`**:
   - Definir constantes de faixas reservadas de IDs e prefixos determinísticos.
   - Implementar validação estrita de segurança e ambiente (`validateEnvironment(databaseURL)`).
   - Implementar gerador de fixtures (listas de nomes realistas, emojis, casos extremos, distribuição de datas no mês atual).
   - Implementar método `Seed(ctx, pool)` dentro de transação PostgreSQL com suporte a idempotência.
   - Implementar método `Clean(ctx, pool)` dentro de transação PostgreSQL com deleção cirúrgica nas 6 tabelas afetadas.
   - Implementar estrutura de relatório `Report` e formatação textual.

2. **Implementar `cmd/devseed/main.go`**:
   - Carregar `.env` via `godotenv.Load()`.
   - Validar flags e argumentos (`miniapp`, `clean-miniapp`).
   - Conectar ao banco PostgreSQL usando `pgxpool`.
   - Executar seed ou cleanup e imprimir relatório.

3. **Atualizar `Makefile`**:
   - Adicionar targets `.PHONY: seed-miniapp clean-miniapp-seed`.
   - Chamar `cmd/devseed` com as variáveis necessárias.

4. **Implementar `internal/devseed/seed_test.go`**:
   - Teste de segurança (falha sem `APP_ENV=development` ou `ALLOW_DEV_SEED=1`).
   - Teste de idempotência (duas execuções consecutivas geram exatamente o mesmo score e contagem).
   - Teste de cleanup (remove apenas fixtures e deixa banco limpo).
   - Teste de isolamento entre Atualizado e Legado.
   - Teste de agregação multi-grupo de players.
   - Teste de paginação (>50 itens gerando `next_cursor`).
   - Teste de ordenação de empates conforme regras de tie-break.

5. **Verificações e Testes**:
   - `go test -count=1 ./...`
   - `go vet ./...`
   - `go build ./...`
   - Testes de integração se `TEST_DATABASE_URL` estiver disponível.
   - `git diff --check`

6. **Atualização da documentação e encerramento do plano**:
   - Registrar no `.agent/decisions.md` e `.agent/memory/memory.md`.
   - Mover plano para `.agent/plans/done/`.
   - Apresentar comandos e instruções para uso local.

## Riscos
- **Risco**: Afetar dados reais de grupos ou jogadores em banco local caso já existam partidas reais.
  - **Mitigação**: Faixa rígida e reservada de IDs negativos (`[-990000000999, -990000000001]`), UserIDs específicos (`[9900000001, 9900000999]`) e GameIDs com prefixo `devseed_game_`. O cleanup nunca usa `TRUNCATE` nem `DELETE` irrestrito.
- **Risco**: Execução acidental fora de desenvolvimento.
  - **Mitigação**: Fail-closed obrigatório checando `APP_ENV=development` ou `ALLOW_DEV_SEED=1`.
- **Risco**: Scores acumulando em execuções repetidas do seed.
  - **Mitigação**: O seed define os valores absolutos finais de `score_units`, `completed_games` e `wins` de forma determinística com upsert / reset seguro dentro da transação, garantindo total idempotência.

## Impactos esperados
- O Mini App poderá ser homologado visualmente com centenas de registros, testando paginação, scrolling longo, nomes longos, empates e transições entre Atualizado e Legado.
- Os dados reais do sistema permanecerão completamente intactos e o cleanup removerá 100% da massa de teste em segundos.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD

## Como testar

### Build
```bash
go build ./cmd/devseed && go build ./...
```

### Testes
```bash
go test -count=1 ./internal/devseed/... && go test -count=1 ./...
```

### Execução de Homologação
```bash
APP_ENV=development make seed-miniapp
APP_ENV=development make clean-miniapp-seed
```

## Rollback
Caso necessário, remover os novos arquivos e descartar alterações no Makefile:
```bash
rm -rf cmd/devseed internal/devseed
git checkout -- Makefile
```

## Observações
- Nenhuma alteração em regras de negócio ou de gameplay.
- Nenhuma migration permanente será adicionada ao projeto.
- Não serão feitas chamadas à API do Telegram para avatares falsos.
- Nenhum commit ou push será realizado.

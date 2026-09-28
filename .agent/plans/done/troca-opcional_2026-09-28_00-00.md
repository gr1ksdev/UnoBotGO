# Plano: Trocar Mãos opcional

## Pedido do usuário
Implementar na dev troca opcional via Inline Mode, cor obrigatória após alvo/manter, exceto última carta que termina imediatamente. Sem push/main.

## Objetivo
Preservar engine pura, ações revisionadas, tokens e fluxo de cores; resolver transferência somente na cor.

## Contexto atual
HEAD bce47d0, árvore limpa, pull ff-only concluído. Auditoria e testes base aprovados em uno/game/telegram/simulation/ranking/groups e debugcards. Race local bloqueado por VMA 39/48. Troca atual obrigatória, imediata em ChoosePlayer, sem cor; última carta recusada. M7/ranking/config não serão alterados.

## Arquivos analisados
README, docs/project-status, v2-rules, v2-telegram, v2-application, branching e m7-persistence; internal/uno, game, telegram e simulation (ações, snapshots, views, tokens, dispatcher, timeout, resultados e testes).

## Arquivos que poderão ser modificados
Engine action/event/state/game e testes; autorização do Service e testes; inline/renderer e testes; simulador/report e testes; documentação necessária e registros internos.

## Estratégia de implementação
KeepHand explícita; ChoosePlayer guarda alvo em ColorChoice sem transferir mãos. ChoosingColor reutilizado, com indicação de resolução de SwapHands e alvo opcional. ChooseColor aplica troca/manutenção e cor atomicamente; uma revisão por ação. Última carta usa completePlay imediatamente, preservando a cor anterior quando há continuidade. Escolhas não recebem timeout.

## Passos detalhados
1. Estender ação/evento e estado pendente com validação/cópia.
2. Selecionar alvo/manter abre cor; remover alvo durante escolha invalida a seleção e reabre ChoosingPlayer, sem fallback para outro jogador.
3. Reutilizar inline/cores/tokens; apresentar resultado real por eventos, não por suposição.
4. Atualizar testes existentes e adicionar casos de fluxo, última carta, stale, isolamento, timeout e invariantes; adaptar simulador.
5. Rodar test -count=1, race, vet, build, variantes debugcards e simulação; atualizar docs, revisar e criar commits locais coerentes.

## Riscos
Revisões/hand swaps duplicados; snapshot inconsistente; abandono de alvo; emissão prematura de UNO/placement; efeito em ranking terminal.

## Impactos esperados
Manter sempre disponível; nenhuma transferência antes da cor; última carta encerra somente autor conforme fluxo normal, incluindo último remanescente normal a dois.

## Compatibilidade
Linux/macOS/Windows/Docker/CI: sem dependências novas. Novos snapshots pendentes exigem runtime compatível; partidas ativas continuam em RAM. Nenhuma migration/config/transporte.

## Como testar
### Build
```bash
go build ./...
go build -tags debugcards ./...
```
### Testes
```bash
go test -count=1 ./...
go test -count=1 -race ./...
go vet ./...
go test -count=1 -tags debugcards ./...
git diff --check
```
### Execução
Simulador local com seeds; relatórios em /tmp. Homologação Telegram posterior pelo usuário.

## Rollback
Reverter commits locais sem reset destrutivo; não promover ou enviar.

## Observações
Decisões aprovadas explicitamente: última carta não abre escolhas; r+1/r+2/r+3; manter timeout atual. Sem outras decisões de produto pendentes.

## Execução concluída
Implementação e testes realizados conforme plano. Suítes normais/debugcards, vet/build e diff check aprovados. Race bloqueado no ambiente por VMA 39/48, não por asserção. Simulações Caseiro/Clássico concluídas. Homologação real fica pendente para o usuário. Commits apenas locais na dev; main intacta, sem push.

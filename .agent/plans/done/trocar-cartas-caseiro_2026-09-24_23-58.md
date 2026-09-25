# Plano: trocar-cartas-caseiro

## Pedido do usuário
Adicionar carta Trocar cartas, exclusiva do modo caseiro. Ao jogar o sticker `CAACAgEAAxkBAAER8VtqteJsR8-zG10NFeLTIZyxuZYsBQACBwkAAkkSsEU562tb90Ja3D0E`, apresentar menu semelhante ao coringa para escolher outro jogador e trocar integralmente as mãos.

## Objetivo
Implementar carta, escolha de destinatário e troca transacional na engine V2, integradas ao serviço, Telegram e simulador.

## Contexto atual
A V2 separa engine em internal/uno, aplicação em internal/game e Telegram em internal/telegram. Ambos os modos usam ClassicDeck de 108 cartas. CaseiroRules deriva de BotRules. O coringa abre ChoosingColor com escolhas inline vinculadas ao jogador e revisão. As regras atuais impedem terminar com coringa, jogar coringa sobre coringa e responder penalidade com coringa simples. O lobby permite alternar regras. Há simulador que precisa reconhecer toda nova fase. Árvore de trabalho limpa na análise.

## Arquivos analisados
- AGENTS.md
- .agent/context.md
- .agent/memory/memory.md
- go.mod
- Makefile
- internal/uno/card.go
- internal/uno/deck.go
- internal/uno/rules.go
- internal/uno/action.go
- internal/uno/state.go
- internal/uno/game.go
- internal/game/service.go
- internal/game/views.go
- internal/telegram/inline.go
- internal/telegram/renderer.go
- internal/telegram/callbacks.go
- internal/telegram/stickers.go
- internal/telegram/tokens.go
- internal/simulation/strategy.go

## Arquivos que poderão ser modificados
- internal/uno/{card,rules,action,state,game,event,errors}.go e testes relacionados
- internal/game/{service,views,manager}.go e testes relacionados, conforme integração do lobby e ciclo de vida
- internal/telegram/{inline,renderer,stickers,callbacks,tokens}.go e testes relacionados
- internal/simulation/{strategy,runner,report}.go e testes relacionados
- README.md e docs/v2-{rules,application,telegram}.md
- .agent/context.md, .agent/memory/memory.md e .agent/decisions.md
- Este plano, movido entre pending, approved e done

## Estratégia de implementação
Adicionar tipo próprio de carta sem cor e regra explícita habilitada apenas no caseiro. Criar fase e ação próprias de seleção de jogador, mantendo a escolha de cor independente. Reutilizar fluxo inline e tokens opacos existentes. Trocar apenas as mãos restantes, após descartar a carta, sem mudar assentos, sentido ou identidades. Expor somente informação pública do alvo.

Propostas de regras para aprovação:
- Uma cópia adicional no baralho caseiro (109 cartas); clássico permanece com 108.
- Conservar a cor ativa da mesa; o menu escolhe apenas jogador.
- Aplicar as restrições atuais de coringa: não usar como última carta, sobre outro coringa ou para responder +2/+4 pendente.
- Escolher somente outro participante ativo. Troca obrigatória, confirmada exclusivamente por quem jogou a carta.
- Após confirmação, passar ao próximo jogador na ordem e direção atuais; recalcular anúncios de UNO pelas mãos resultantes.

## Passos detalhados
1. Após aprovação, mover plano para approved.
2. Acrescentar carta e habilitação por regra; selecionar inventário adequado inclusive ao alternar modos no lobby com participantes já inscritos. Preservar contratos de decks injetados e snapshots; rejeitar carta especial sob regras desabilitadas.
3. Implementar estado de escolha, alvo na ação, eventos e erros, validação e cópia de snapshots. Bloquear demais jogadas enquanto a escolha estiver pendente. Trocar mãos atomicamente e avançar turno.
4. Tratar saídas, entrada tardia, cancelamento e encerramento durante escolha. Seguir política do coringa para impedir saída do responsável enquanto escolhe e evitar que timer pule a escolha. Revalidar alvos e revisões após mudanças de participantes.
5. Autorizar ação no serviço e expor responsável/alvos públicos sem mãos; conferir temporização e integração do ciclo de vida.
6. Mapear sticker fornecido e nome da carta. Oferecer menu inline com nomes dos demais ativos; comunicar espera e resultado da troca. Para carta indisponível sem sticker cinza fornecido, usar representação existente segura, sem permitir jogada inválida.
7. Garantir rejeição de seleção por terceiros, tokens repetidos/antigos, alvo próprio ou inativo; atualizar mãos exibidas após troca e preservar revisão estrita.
8. Atualizar estratégia e relatório do simulador para escolha determinística e descrição da troca.
9. Adicionar testes de engine, serviço, Telegram e simulador; executar verificações abaixo e registrar limitações reais.
10. Documentar regras aprovadas e decisões, atualizar memória/contexto e mover plano para done.

## Riscos
- Comparações atuais por intervalo de ranks e fases podem classificar a carta incorretamente.
- Alternância de modo no lobby precisa atualizar inventário sem perder cartas ou quebrar decks de teste.
- Escolhas antigas podem referenciar mãos ou participantes alterados; validar revisão e participação.
- Nova fase precisa de validação, encerramento e suporte do simulador para evitar partidas presas.
- Sticker deve ser validado visualmente no Telegram; testes locais não confirmam disponibilidade do file_id.

## Impactos esperados
- Nova mecânica exclusiva do caseiro e baralho com uma carta adicional.
- Menu de jogadores após a carta, troca integral e atualização das contagens públicas.
- Sem dependências externas adicionais, migrations ou alterações no executável V1.

## Compatibilidade
- Linux: build e testes Go.
- macOS: alterações em Go sem dependência específica de plataforma.
- Windows: mesma lógica Go; não presumir homologação nativa.
- Docker: preservar executável e contratos existentes.
- CI/CD: manter comandos e exigir regressões aprovadas; sem publicação neste plano.

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test ./...
go test -race ./internal/uno ./internal/game ./internal/telegram ./internal/simulation
go vet ./...
git diff --check
```
Cobrir troca exata e conservação de cartas, independência de snapshots, sequência de turno em ambos os sentidos, UNO, todas as restrições propostas, menus e autorizações, saídas/cancelamento e alternância do lobby. Validar que clássico não contém nem aceita a carta. Registrar se race detector não estiver disponível.

### Execução
```bash
go run ./cmd/simulator --players 4 --mode caseiro --seed 20260924
go run ./cmd/simulator --players 4 --mode classico --seed 20260924
```
Homologação manual em bot de teste: jogar sticker, abrir escolha como no coringa, selecionar participante, conferir mãos e turno. Executar bot somente em ambiente de teste configurado, sem afetar produção.

## Rollback
Reverter exclusivamente alterações desta feature em commit separado, preservando trabalho alheio e histórico dos planos. Partidas em memória com a nova carta não devem ser restauradas em versão que não a reconheça; realizar eventual troca de versão fora de partidas ativas.

## Observações
Somente este plano foi criado nesta etapa; implementação depende de aprovação explícita conforme AGENTS.md. Quantidade de cópias e regras complementares são propostas, pois não foram especificadas pelo usuário. Não inclui commit, deploy ou envio de mensagens reais pelo bot.

## Aprovação e conclusão

- Aprovado pelo usuário em 2026-09-25: "sim".
- Implementação concluída em 2026-09-25, com as regras propostas aprovadas.
- Engine, serviço, Telegram e simulador integrados; documentação, contexto, memória e decisões atualizados.
- Validações aprovadas: `go test ./...`, `go build ./...`, `go vet ./...`, `go test -race ./internal/uno ./internal/game ./internal/telegram ./internal/simulation`, `git diff --check`.
- Simulações de 4 jogadores, seed 20260924: caseiro 119 ações, clássico 37 ações, ambas concluídas normalmente. Relatórios locais em `.reports/simulations/`.
- Testes antigos ajustados para a nova fase e para a trajetória diferente da seed caseira; estatística de blefe descoberto mantém cobertura por eventos explícitos.
- Homologação visual do sticker na API real do Telegram pendente, sem executar bot de produção.
- Sem commit ou deploy nesta tarefa.

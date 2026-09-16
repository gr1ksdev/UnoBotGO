# Memória atual — 2026-09-15 — M3

- Aprovação explícita: “Aprovado. Implemente a Milestone 3 conforme este plano.”
- Plano: telegram-mvp-v2_2026-09-15_14-05.md.
- Entry point V2 criado em `cmd/bot/main.go`, totalmente desacoplado do `main.go` legado.
- Concorrência de chat: 8 filas particionadas por ChatID (`hash(ChatID) % 8`) com 32 slots de capacidade para serializar comandos e jogadas da mesma partida.
- Concorrência inline: 4 workers dedicados lendo de fila com capacidade 64 para responder queries inline sem bloquear nem ser bloqueados por operações de chat.
- Backpressure seguro: tarefas descartadas sem mutação quando canais saturam.
- TokenStore privado: tokens aleatórios de 128 bits base64url gerados por `crypto/rand`. Consumo atômico sob mutex, TTL de 2 minutos (configurável), limite global (20.000) e por usuário (512) com evicção FIFO e limpeza oportunista. Invalidação imediata em caso de cancelamento/saída.
- Bypass omitempty telego v1.10.0: `InlineRequestConstructor` emitindo explicitamente `cache_time:0`, `is_personal:true` e `next_offset:""` para `answerInlineQuery`.
- `SafeAPICaller`: retry estrito de apenas 1 vez para HTTP 429 se `retry_after <= 5s`; timeouts e erros de rede não têm retry automático; URLs sanitizadas para nunca vazar tokens nos logs.
- Comandos suportados: `/novo`, `/entrar`, `/iniciar`, `/cancelar` (alias `/kill`), `/sair`, `/estado`, `/ajuda` (alias `/start` em grupos/privado).
- Matching de comandos case-insensitive (`strings.EqualFold(targetBot, h.botUsername)`): aceita `/novo@UnoGoBrBot` e `/novo@unogobrbot`.
- Regra de lobby no `/novo`: criador NÃO é inscrito automaticamente (permanece responsável observador com 0 inscritos até enviar `/entrar`).
- Stickers limpos no chat: remoção de botões de validação nos stickers jogados e eliminação de edições de markup.
- Teclado inline de jogo simplificado: `makeGameButtons` gera exclusivamente o botão `🃏 Suas cartas` (removido botão redundante `🔄 Atualizar estado`).
- Mensagem de estado mais enxuta: removido cabeçalho `🃏 UnoBotGO` de `RenderPublicState` e eliminada a contagem de cartas `(X cartas)` dos jogadores na lista pública (mantendo apenas o alerta `⚠️ UNO!` para quem estiver com 1 carta).
- Permissão de `/iniciar` flexibilizada: qualquer participante do chat pode usar `/iniciar` quando houver pelo menos 2 inscritos no lobby (paridade com V1). O comando `/cancelar` continua restrito ao responsável/owner da partida.
- Modo inline em grade/carrossel horizontal: exclusão de artigos de texto no fluxo de cartas durante a partida; apenas `InlineQueryResultCachedSticker` são emitidos.
- Empilhamento de +2 (`StackDrawTwo: true` em `BotRules()`): jogar +2 soma 2 a `DrawCounter` e passa a vez ao próximo jogador sem pular. O próximo jogador pode contra-atacar com outro +2 ou comprar a penalidade acumulada (passando a vez). Enquanto `DrawCounter > 0`, apenas cartas +2 são jogáveis.
- Seletor de cor estilo V1 limpo: na fase `ChoosingColor`, o jogador da vez recebe 4 artigos de cor ("Escolha sua cor") e 1 artigo de resumo das cartas ("Cartas (toque para estado do jogo):"), retornando imediatamente sem stickers cinzas misturados.
- Grito de UNO separado com reação festiva: quando um jogador atinge 1 carta na mão (`uno.UnoAnnounced`), o bot envia uma mensagem dedicada no grupo e adiciona a reação festiva `🥳` via `SetMessageReaction`.
- Carta inicial sempre numérica (`NumberedStart: true` em `BotRules()`): a partida nunca inicia com cartas de ação (+2, Skip, Reverse, Wild), garantindo início limpo sem penalidades na largada e todos com 7 cartas.
- Envio do sticker da carta virada: no `/iniciar`, o bot envia o sticker da primeira carta do topo no grupo antes da mensagem de status.
- Aceite manual do Telegram: testes automatizados 100% aprovados; roteiro de homologação manual detalhado em `docs/v2-telegram.md`.

---

## Memória histórica da M2 (2026-09-15)

- Aprovação explícita: “Aprovado. Implemente a Milestone 2 conforme este plano revisado.”
- Plano vigente: camada-aplicacao-v2_2026-09-15_13-30.md; anterior preservado como histórico.
- Owner é metadata administrativa independente de inscrição; somente owner inicia/cancela na M2.
- Owner que sai/obtém colocação transfere ao turno resultante ou primeiro ativo no lobby.
- Lobby sem sucessor conserva owner observador e permanece aberto até cancelamento explícito.
- Último Wild mantém participação ativa até escolher cor; WentOut/Left não recebem mão privada.
- Service aceita Actor autenticado pelo adapter; nunca confiar em Actor vindo diretamente de payload.
- Ordem de locks: entry -> índices; lookup solta índices antes de esperar entry. Nenhum I/O externo sob locks.
- Publicação de índices sempre termina após Apply aceito, mesmo com cancelamento de contexto.
- Histórico final público FIFO: padrão100, zero desativa; sem mãos/runtime, perdido ao reiniciar.
- Sem MemoryRepository nesta milestone; future recovery deve validar State+metadata e reconstruir índices.
- M1 preserva proibição de reentrada e compra não jogável passa turno automaticamente.
- Ver docs/v2-application.md antes de implementar M3. Escopo M2 não inclui adapter/tokens.

---

## Memória histórica anterior

# Memória atual — 2026-09-14

## V2 Milestone 1

- Usuário aprovou o plano com "Implement the plan.".
- Decisões confirmadas: +4 ilegal bloqueado inicialmente; UNO automático;
  múltiplos grupos com seleção explícita futura; preservar colocações e entrada tardia.
- Engine nova em internal/uno; V1 permanece executável na raiz.
- IDs físicos, snapshot com cópia profunda, Restore validado, Apply atômico por cópia,
  revision estrita e erros via errors.Is. Snapshots contêm mãos privadas e ficam no servidor.
- Injeção de deck/shuffler permite testes determinísticos; runtime RNG não é persistido.
- Sem empilhamento Classic; somente carta comprada pode ser jogada após compra.
- Policy FirstWinner ou Placements; dez participantes registrados, sem reentrada;
  entrada tardia opcional; cancelamento/saída distinguem motivo de término.
- Falta de cartas retorna ErrDeckEmpty sem mutação parcial; nenhuma carta é fabricada.
- Game não é thread-safe: manager da M2 será responsável pelo mutex privado por partida.
- Testes incluem 40 partidas determinísticas completas e regras/segurança/inventário/recovery.
- Conferir docs/v2-rules.md e docs/v2-audit.md antes das próximas milestones.

## Correções de contexto V1

- PostgreSQL já é obrigatório no startup V1 e guarda ranking/modo por grupo.
- Inline atual usa HTTP próprio com cache_time=0 explícito; telego tem omitempty.
- IDs atuais são aparência:índice; não existe validação AntiCheat/revision no handler atual.
- InlineQuery e ChosenInlineResult não trazem chat_id. Não inferir destino por
  inline_message_id. A seleção deve ficar vinculada à partida na emissão do resultado.
- Passar no race detector atual não cobre os caminhos concorrentes defeituosos do V1.

---

## Histórico anterior (preservado; consultar correções acima)

# Memória do Projeto - UnoGoBot

## Stack
- **Linguagem:** Go
- **Biblioteca Telegram:** telego (github.com/mymmrac/telego)
- **Estado:** Em memória (sem banco de dados)

## Arquitetura
- `main.go` — Entry point, inicializa bot e long polling
- `config.go` — Constantes de configuração (token, waiting_time, etc.)
- `card.go` — Definição de cartas, cores, valores, especiais, stickers
- `deck.go` — Baralho (shuffle, draw, dismiss, fill)
- `player.go` — Jogador (lista duplamente ligada em anel)
- `game.go` — Estado do jogo (regras, turnos, efeitos)
- `gamemanager.go` — Gerenciador de múltiplos jogos
- `results.go` — Construção de resultados inline
- `actions.go` — Ações do jogo (jogar carta, comprar, pular, etc.)
- `inline.go` — Handlers de inline query + chosen inline result
- `commands.go` — Handlers de comandos (/novo, /entrar, etc.)
- `errors.go` — Erros customizados

## Convenções
- Comandos em português (/novo, /entrar, /sair, etc.)
- Modo inline via stickers (usando file_ids do projeto jh0ker/mau_mau_bot)
- Jogador implementado como lista duplamente ligada em anel
- `sync.Mutex` por jogo para concorrência
- Context全局 compartilhado (`botCtx`)

## Decisões técnicas
- Usar telego ao invés de python-telegram-bot (migração Python → Go)
- Estado em memória (leve, sem dependências externas)
- Long polling (sem necessidade de webhook para dev)
- Stickers do projeto original (CARDS_CLASSIC_COLORBLIND)
- Roteamento proativo do contexto de jogo ativo (UserIDCurrent) no início de turnos e interações em grupo, mantendo o inline query livre de parâmetros de chat expostos, com leituras de mapas globais sob Mutex e sufixo de anti-cheat nos IDs dos resultados inline para invalidar o cache do Telegram de forma nativa e idêntica ao repositório original. Suporte a inversão de rotação de turnos (Reversed) no Game e redução do escopo de Mutex em inline handlers para evitar deadlocks de concorrência.

## Problemas conhecidos
- Stickers podem não funcionar se o pacote de stickers original for alterado
- Modo de jogo "text" não implementado completamente (sempre usa stickers)
- Bot precisa de `/setinline` e `/setinlinefeedback` no BotFather

## Histórico de correções (21/06/2026)
- **Cache:** `CacheTime = 1` em vez de 0 — telego usa `omitempty` que omite 0 do JSON; Telegram default 300s.
- **Anti-cheat:** Result IDs com timestamp `time.Now().UnixNano()` para invalidar cache do cliente.
- **Parsing anti-cheat:** Aceita formato 2 ou 3 partes (`<id>:<anticheat>` ou `<id>:<timestamp>:<anticheat>`).
- **Regras de cartas:** `cardPlayable` reescrita igual ao Python original (bloqueia +4 em +2 c/ draw_counter > 0, special-on-special, etc).
- **EndGame:** Adicionado `EndGameByGame` para encerrar quando jogador já saiu. `game.Started = false` setado antes de `afterAction`.
- **Notificação anti-cheat:** Mensagem enviada ao jogador se ação expirou.
- **Ordenação:** Cartas no inline agora ordenadas por cor (r, b, g, y) e valor.
- **Seletor de cores:** Sem duplicação de emoji; `Title` mostra nome da cor.
- **displayName:** `@@` corrigido para `@`.
- **Cores da mão:** Mostra emojis das cores disponíveis durante escolha de cor.

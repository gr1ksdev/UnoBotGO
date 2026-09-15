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

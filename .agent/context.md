# Contexto atual — V2 Milestone 2 (2026-09-15)

- M2 implementada em `internal/game`; Service é a API do futuro adapter M3.
- Manager privado possui runtime UNO e mutex por partida; lock global só protege índices/resumos.
- Create não inscreve responsável. Owner observador pode Start/Cancel; dealer é participante ativo.
- Engine só recebeu exceção de participação para Start/Cancel, preservando solicitante verdadeiro.
- Índices chat/jogador são de sessões abertas/participação ativa, sem current game global.
- PlayerView só expõe mão do próprio Actor confiável; PublicView não contém snapshots/mãos.
- Encerramento descarta runtime e mantém somente resumos públicos FIFO (100 por padrão, configurável).
- MemoryRepository adiado: snapshots transitórios, sem segunda fonte de verdade ou recovery implementado.
- Sem Telegram V2, tokens, ranking, Match, timers ou backend persistente nesta entrega; V1 intacto.
- Contrato atual: `docs/v2-application.md`; regras: `docs/v2-rules.md`; auditoria histórica: `docs/v2-audit.md`.

---

## Registro histórico da M1 (decisões futuras abaixo foram atualizadas pela M2)

# Contexto atual — V2 Milestone 1 (2026-09-14)

- O executável V1 permanece na raiz, em `package main`, com telego v1.10.0.
- V1 exige PostgreSQL para ranking/configuração; somente partidas são efêmeras em RAM.
- `internal/uno` é a nova engine independente de Telegram, SQL, ambiente e relógio.
- Actions transacionais por cópia, revision estrita, cartas físicas com IDs, snapshots serializáveis.
- `ClassicRules`: primeiro vencedor, sem entrada tardia. `BotRules`: colocações e entrada tardia.
- Ambos usam regras Classic das cartas, UNO automático e bloqueio de +4 ilegal.
- Manager/locks por partida, MemoryRepository e serviço serão Milestone 2; adapter V2 será M3.
- Engine é de dono único: chamadas ao mesmo Game devem ser serializadas pelo futuro manager.
- Documentação autoritativa: docs/v2-rules.md e docs/v2-audit.md.
- O módulo Go continua github.com/malbs/UnoGoBot; V1 não foi migrado nem removido.

---

## Contexto histórico do V1 (pode conter informações superadas)

# Contexto do Projeto - UnoGoBot

## Stack e Ferramentas
- **Linguagem**: Go (1.20+)
- **Biblioteca**: `github.com/mymmrac/telego` para interações com a API do Telegram.
- **Configurações**: Gerenciadas via variáveis de ambiente carregadas pelo `godotenv`.
- **Persistência**: Totalmente em memória (ram). Sem banco de dados relacional ou chave-valor persistente.

## Objetivos e Requisitos
1. Permitir que múltiplos grupos iniciem e joguem partidas de UNO de forma concorrente e isolada.
2. Usar o inline mode do Telegram com Stickers para uma experiência gráfica de exibição e seleção de cartas.
3. Manter a integridade de concorrência usando locks (`sync.Mutex`) no gerenciador de jogos e estados de jogo individuais.

## Padrões Internos
- **Listas Circulares**: Os jogadores são encadeados em um anel usando referências de ponteiros `Next` e `Prev`. Isso facilita a alteração de turnos (`Turn`) e efeitos de inversão (`Reverse`).
- **Ponteiros Globais**: O `GameManager` rastreia o jogo ativo atual de cada usuário (`UserIDCurrent`) e a lista de jogos ativos de cada usuário (`UserIDPlayers`) para saber como direcionar as requisições que chegam sem ChatID (como as queries inline).
- **Parâmetros de Contexto**: A inline query utiliza parâmetros de string (como o ID do chat) passados via switch do botão "Suas cartas" para manter o alinhamento de contexto no ambiente multi-grupo.

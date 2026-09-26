# Plano: corrigir-falso-topico

## Pedido do usuário
Corrigir aviso de fórum ao usar /entrar e comandos em grupo comum, confirmado pelo usuário como sem tópicos.

## Objetivo
Não confundir threads de mensagens comuns com tópicos de fórum.

## Contexto atual
HandleMessage e HandleReset recusam `msg.IsTopicMessage || msg.MessageThreadID != 0`. A API e os tipos da telego descrevem MessageThreadID como identificador de thread ou tópico; IsTopicMessage identifica mensagem de tópico. O payload real do incidente não foi capturado, portanto a condição inadequada é confirmada no código, mas a causa específica ainda exige homologação.

## Arquivos analisados
- internal/telegram/commands.go
- internal/telegram/commands_test.go
- internal/telegram/debugcards.go
- internal/telegram/debugcards_test.go
- tipos Message da telego v1.10.0
- Documentação oficial: https://core.telegram.org/bots/api#message

## Arquivos que poderão ser modificados
- internal/telegram/commands.go
- internal/telegram/commands_test.go
- internal/telegram/debugcards_test.go
- .agent/context.md
- .agent/memory/memory.md
- .agent/decisions.md
- Este plano

## Estratégia de implementação
Usar apenas IsTopicMessage nos dois filtros de tópicos. Thread ID isolado não implica fórum. Preservar regras de chat, autenticação, remetente e autorização. Não adicionar suporte a partidas em tópicos reais nem exceção por ID numérico de thread.

## Passos detalhados
1. Após aprovação, mover plano para approved.
2. Substituir a condição nos handlers comum e de recuperação por msg.IsTopicMessage, com comentário explicando distinção.
3. Testar /novo e /entrar com IsTopicMessage=false e MessageThreadID não zero, verificando inscrição no serviço.
4. Testar /reset autorizado com thread comum, entrando pelo handler comum e pela entrada direta de recuperação; confirmar que sem autorização continua recusado.
5. Preservar bloqueio de IsTopicMessage=true, com ID zero ou não zero, sem mutação da partida.
6. No build debugcards, cobrir /dar com ID de thread comum para garantir que alcança o handler autorizado, mantendo entrega e autorização existentes.
7. Executar testes nas duas variantes e registrar conclusão em memória/contexto/decisões. Mover plano para done.

## Riscos
- Payload real com IsTopicMessage=true exigiria investigação adicional; não afirmar que esse payload já foi observado.
- Ignorar indicador de tópico liberaria fluxo sem roteamento apropriado; por isso o bloqueio de IsTopicMessage permanece.
- Alterações anteriores ainda não commitadas devem ser preservadas.

## Impactos esperados
Comandos em threads comuns deixam de ser recusados como fórum. Partidas continuam isoladas por grupo e tópicos reais permanecem bloqueados.

## Compatibilidade
- Linux, macOS e Windows: somente lógica Go.
- Docker: nenhuma alteração na imagem/configuração.
- CI/CD: sem alteração no pipeline; verificar variantes normal e debugcards.

## Como testar

### Build
```bash
go build ./...
go build -tags debugcards ./...
```

### Testes
```bash
go test ./internal/telegram
go test -tags debugcards ./internal/telegram
git diff --check
```

### Execução
Homologar /entrar no grupo comum afetado após atualização do bot. Não executar bot de produção automaticamente. Se persistir, coletar apenas metadados de chat/tópico necessários, sem tokens ou texto privado.

## Rollback
Reverter apenas este ajuste e testes, preservando correção do sticker e comando de depuração já implementados. Não usar reset destrutivo.

## Observações
Sem plano dispensado explicitamente para esta nova correção; aguarda aprovação conforme AGENTS.md. Não inclui commit, push ou deploy.


## Conclusão
- Aprovado pelo usuário com "sim" e implementado em 2026-09-25.
- Dois filtros corrigidos; testes de comandos, reset e /dar em threads comuns adicionados/ajustados, mantendo proteção de tópicos reais.
- go test ./internal/telegram e go build ./... passaram com e sem -tags debugcards; git diff --check aprovado.
- Memória/contexto/decisões atualizados; mudanças anteriores preservadas. Homologação real pendente. Sem commit, push ou deploy.

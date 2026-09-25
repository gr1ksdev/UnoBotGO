# Plano: reorganizar-comandos-privados

## Pedido do usuário
Reorganizar os comandos, principalmente no chat privado. O `/start` deve mostrar boas-vindas e uma descrição breve do bot, sem mencionar V2 ou Go/Golang, indicar `/help` e oferecer um botão para adicionar o bot a um grupo. O username do bot deve ser obtido automaticamente. O `/help` deve explicar os comandos em formato blockquote e terminar informando que esta é uma versão brasileira feita em Golang com base no `@unopybot`.

## Objetivo
Separar claramente a apresentação inicial da ajuda detalhada, criar um fluxo privado próprio e registrar menus de comandos adequados para chats privados e grupos, usando sempre o username retornado por `getMe` no startup.

## Contexto atual
- Em chats privados, `/start`, `/ajuda` e `/help` retornam exatamente o mesmo `RenderHelp`.
- O texto atual de ajuda contém `UnoBotGO V2`, contrariando o texto desejado para a apresentação.
- Não existe mensagem exclusiva de boas-vindas nem botão para adicionar o bot a grupos.
- O startup já chama `getMe`, valida `me.Username` e injeta esse valor em `CommandHandler.botUsername`; portanto não é necessário criar configuração manual nem fixar `@unorobotsadlahkdajlkbot` no código.
- O botão pode usar o deep link `https://t.me/<username>?startgroup=true`, construído com o username obtido no startup.
- O comando `/help` já é aceito como alias em chats privados e grupos, mas o menu registrado no Telegram publica apenas `/ajuda` e os comandos de grupo em um único escopo padrão.
- `RenderHelp` usa HTML, e o Telegram aceita `<blockquote>...</blockquote>` nesse modo de parse.
- O workspace possui mudanças anteriores aprovadas e ainda não commitadas. Esta entrega deve preservá-las.

## Arquivos analisados
- `internal/telegram/commands.go`
- `internal/telegram/renderer.go`
- `internal/telegram/bot.go`
- `internal/telegram/client.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/renderer_test.go`
- `internal/telegram/bot_test.go`
- `internal/telegram/mock_test.go`
- `docs/v2-telegram.md`
- `README.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`

## Arquivos que poderão ser modificados
- `internal/telegram/commands.go`
- `internal/telegram/renderer.go`
- `internal/telegram/bot.go`
- `internal/telegram/commands_test.go`
- `internal/telegram/renderer_test.go`
- `internal/telegram/bot_test.go`
- `internal/telegram/mock_test.go`
- `README.md`
- `docs/v2-telegram.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- este plano, movido entre `pending`, `approved` e `done`

## Estratégia de implementação
Criar uma renderização exclusiva para `/start`, curta e voltada ao primeiro contato. O texto proposto será:

```text
👋 Bem-vindo ao UnoBotGO!

Jogue UNO com seus amigos diretamente nos grupos do Telegram. Crie partidas, escolha o modo de jogo e use sua mão pelo menu privado.

Use /help para conhecer todos os comandos.
```

Essa mensagem não citará V2, linguagem ou detalhes internos. Abaixo dela haverá um teclado inline com um botão `➕ Adicionar a um grupo`, cujo URL será montado dinamicamente como `https://t.me/<username>?startgroup=true`.

Reescrever `RenderHelp` para apresentar cada comando dentro de um bloco `<blockquote>`, incluindo `/start`, `/help`, os comandos de grupo e os aliases relevantes `/ajuda` e `/kill`. Ao final, fora do blockquote, constará que o bot é uma versão brasileira desenvolvida em Go (Golang), baseada no `@unopybot`. O trecho que ensina a abrir a mão continuará usando `@<username>` obtido automaticamente.

Registrar comandos por escopo no startup:
- padrão: `/help`, garantindo fallback limpo;
- chats privados: `/start` e `/help`;
- grupos: `/novo`, `/entrar`, `/iniciar`, `/estado`, `/sair`, `/cancelar`, `/reset` e `/help`.

Os aliases `/ajuda` e `/kill` continuarão aceitos, mas não ocuparão espaço duplicado no menu. O `/start` em grupos continuará com a compatibilidade existente como alias de `/iniciar`, embora apareça somente no menu privado.

## Passos detalhados

1. Após aprovação, mover este plano para `approved`.
2. Adicionar `RenderWelcome` sem referências a V2 ou Golang.
3. Criar o teclado privado com o deep link de adição a grupo montado a partir de `botUsername`.
4. Separar o tratamento privado: `/start` usa boas-vindas e botão; `/help` e `/ajuda` usam a ajuda detalhada.
5. Reformatar `RenderHelp` com `<blockquote>` e descrições de todos os comandos e aliases relevantes.
6. Adicionar o rodapé brasileiro/Golang/baseado em `@unopybot` somente à ajuda.
7. Registrar menus de comandos por escopo, priorizando `/start` e `/help` no privado e comandos de jogo nos grupos.
8. Adaptar o mock para registrar todas as chamadas de `SetMyCommands` com seus escopos.
9. Adicionar testes dos textos, HTML, username dinâmico, URL do botão, separação `/start`/`/help` e menus por escopo.
10. Atualizar README, documentação, contexto, memória e decisão técnica.
11. Executar gofmt, testes focados repetidos, suíte completa, vet, build e `git diff --check`.
12. Mover o plano concluído para `done`.

## Riscos
- **Username fixo ou incorreto:** reutilizar exclusivamente `me.Username` obtido por `getMe` e testar com um username fictício diferente do bot real.
- **Deep link inválido:** gerar o formato oficial `https://t.me/<username>?startgroup=true` e verificar o URL exato no teste.
- **HTML malformado:** manter o conteúdo estático e testar abertura/fechamento de `<blockquote>` e parse mode HTML.
- **Menu privado poluído por comandos de grupo:** usar `BotCommandScopeAllPrivateChats` e validar o escopo no mock.
- **Remover compatibilidade existente:** conservar `/ajuda`, `/kill` e `/start` em grupos como aliases aceitos pelo parser.
- **Misturar informação técnica no `/start`:** testar que a mensagem não contém `V2`, `Go` ou `Golang`; a menção técnica ficará apenas em `/help`.
- **Conflitar com alterações pendentes:** revisar o diff agregado sem limpar nem restaurar arquivos do sticker e das regras anteriores.

## Impactos esperados
- O chat privado terá uma apresentação curta e um caminho direto para adicionar o bot a grupos.
- `/help` se tornará a ajuda principal e exibirá comandos de forma mais legível.
- O menu de comandos privado mostrará apenas ações úteis naquele contexto.
- O menu dos grupos continuará oferecendo os comandos de partida.
- Alterações futuras no username via BotFather serão absorvidas após reiniciar o bot, sem editar o código.

## Compatibilidade
- Linux: nenhuma dependência nova.
- macOS: nenhuma dependência nova.
- Windows: nenhuma dependência nova.
- Docker: usa o mesmo startup e `getMe`; nenhuma alteração de imagem.
- CI/CD: nenhuma alteração de workflow; testes continuam sem acessar a rede.

## Como testar

### Build
```bash
go build ./...
go vet ./...
```

### Testes
```bash
go test ./internal/telegram -run 'TestCommandHandler_Private|TestRenderer_(Welcome|Help)|TestBot_RunAndShutdown' -count=20
go test ./...
git diff --check
```

### Execução
```bash
go run ./cmd/bot
```

Homologação manual: no privado, executar `/start`, conferir texto e botão de adição; abrir o seletor de grupos pelo botão; executar `/help` e conferir blockquote, descrições e rodapé. Em um grupo, conferir o menu e a ajuda sem perder os comandos de partida.

## Rollback
Restaurar o tratamento conjunto de `/start` e ajuda, o `RenderHelp` anterior e o registro de comandos em escopo padrão por uma nova alteração. Não usar reset destrutivo nem descartar as mudanças pendentes anteriores.

## Observações
- O botão será exibido apenas na resposta privada de `/start`.
- A palavra Golang aparecerá somente no rodapé de `/help`, conforme solicitado.
- Plano aprovado pelo usuário com `sim` em 2026-09-25.

## Resultado da implementação

- `/start` privado agora mostra boas-vindas, descrição curta e referência a `/help`, sem citar V2 ou Go/Golang.
- O botão `➕ Adicionar a um grupo` usa `https://t.me/<username>?startgroup=true`, preenchido com o username recebido por `GetMe`.
- O payload que o Telegram envia após a inclusão (`/start@bot true`) orienta `/novo` e `/help`, sem tentar iniciar uma partida. `/start` sem payload continua compatível com `/iniciar` em grupos.
- `/help` lista comandos dentro de `<blockquote>`, mantém os aliases e termina com a identificação brasileira em Go (Golang), baseada no `@unopybot`.
- Menus registrados: fallback `/help`; privado `/start` e `/help`; grupos seis comandos de partida, `/reset` e `/help`.
- O mock registra todas as chamadas de `SetMyCommands`, permitindo validar conteúdo e escopo sem rede.
- README, documentação Telegram, contexto, memória e decisão técnica foram atualizados.

### Validação executada

```text
go test ./internal/telegram -run 'TestCommandHandler_(Private|StartGroup)|TestRenderer_WelcomeAndHelp|TestBot_RunAndShutdown' -count=20  PASS
go test ./...                                                                                                                   PASS
go vet ./...                                                                                                                    PASS
go build ./...                                                                                                                  PASS
gofmt + git diff --check                                                                                                        PASS
```

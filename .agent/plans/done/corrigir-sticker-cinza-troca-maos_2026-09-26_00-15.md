# Plano: corrigir-sticker-cinza-troca-maos

## Pedido do usuário
Corrigir o bug da carta cinza de "Trocar cartas". Atualmente a carta indisponível é exibida como artigo textual ("🔀 Trocar cartas — indisponível agora") para contornar o erro `400 DOCUMENT_INVALID`. O usuário quer retirar o texto (que fica feio no menu inline) e restabelecer o sticker cinza funcional para essa carta.

## Objetivo
Resolver a causa raiz do `DOCUMENT_INVALID`: o `file_id` configurado em `StickersGrey["swap_hands"]` está inválido no Telegram (`400 Bad Request: wrong file_id or the file is temporarily unavailable`). Obter um `file_id` oficial e persistente para o bot atual enviando `assets/stickers/swap_hands_grey.png` via `sendSticker` para o chat do usuário (`7595607953`), atualizar o mapeamento em `internal/telegram/stickers.go`, remover a exceção de artigo textual em `internal/telegram/inline.go` e restabelecer os testes de regressão de sticker cinza.

## Contexto atual
- A reprodução do erro confirmou que `CAACAgEAAxkBAAER8aRqtlf6ZtRKfAj02K5AnlVcRz_W_AACVAcAAkaGsEXgXGCANqlQKz0E` falha em `getFile` da Bot API com erro `400 wrong file_id`.
- Na Bot API do Telegram, cada bot possui identificadores de arquivo (`file_id`) próprios; referências a arquivos de outros bots ou IDs incorretos são rejeitados com `DOCUMENT_INVALID` quando passados em `InlineQueryResultCachedSticker`.
- O asset visual estático cinza já existe pronto em `assets/stickers/swap_hands_grey.png` (PNG RGBA 342×512, 206 KB).
- O código atual em `internal/telegram/inline.go` possui um contorno temporário inserindo um `InlineQueryResultArticle` que evitou o erro na homologação, mas que destoa visualmente da mão do jogador.
- As demais cartas indisponíveis usam `InlineQueryResultCachedSticker` com ID cinza e `InputMessageContent` seguro que informa a indisponibilidade sem modificar o estado do jogo.

## Arquivos analisados
- `AGENTS.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- `assets/stickers/swap_hands_grey.png`
- `internal/telegram/stickers.go`
- `internal/telegram/inline.go`
- `internal/telegram/swap_test.go`
- `.agent/plans/done/sticker-cinza-troca-maos_2026-09-25_11-01.md`

## Arquivos que poderão ser modificados
- `internal/telegram/stickers.go`
- `internal/telegram/inline.go`
- `internal/telegram/swap_test.go`
- `docs/v2-telegram.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- Este plano, movido entre `pending`, `approved` e `done`

## Estratégia de implementação
1. Utilizar um script seguro em scratch para carregar o token do `.env` (sem exibi-lo) e enviar `assets/stickers/swap_hands_grey.png` via `sendSticker` da Bot API oficial para o usuário autorizado `7595607953`.
2. Capturar o `result.sticker.file_id` gerado pelo Telegram na resposta direta dessa chamada.
3. Testar imediatamente com `getFile` para garantir que o bot reconhece o novo `file_id` com HTTP 200.
4. Atualizar a constante `StickersGrey["swap_hands"]` em `internal/telegram/stickers.go` com o novo `file_id`.
5. Em `internal/telegram/inline.go`, remover a cláusula condicional de `SwapHands` que gerava `InlineQueryResultArticle`, permitindo que ela caia no fluxo padrão de sticker cinza (`InlineQueryResultCachedSticker`) com prefixo `grey_` e mensagem de proteção, idêntica às outras cartas.
6. Atualizar `internal/telegram/swap_test.go` para validar que a carta de troca indisponível emite `InlineQueryResultCachedSticker` com o novo ID e sem token de ação de jogo.
7. Rodar a suíte completa de testes (`go test ./...` e `go test -tags debugcards ./...`), `go vet ./...` e compilação do bot.

## Passos detalhados

1. Mover o plano para `.agent/plans/approved/`.
2. Executar o script de upload seguro para obter o `file_id` do sticker cinza diretamente do Telegram para este bot.
3. Validar a resposta do `getFile` para o novo ID.
4. Editar `internal/telegram/stickers.go` inserindo o `file_id` validado.
5. Editar `internal/telegram/inline.go` removendo o artigo de texto e restaurando o sticker cinza.
6. Editar `internal/telegram/swap_test.go` restaurando o teste de verificação do sticker cinza e ausência de ação.
7. Rodar testes de unidade e integração:
   - `go test -v ./internal/telegram -run 'TestSwap.*'`
   - `go test ./...`
   - `go test -tags debugcards ./...`
   - `go vet ./...`
   - `go build ./...`
   - `go build -tags debugcards ./cmd/bot`
8. Atualizar documentação técnica e memória persistente (`.agent/memory/memory.md`, `.agent/context.md`, `.agent/decisions.md`).
9. Mover o plano para `.agent/plans/done/`.

## Riscos
- **Token exposto em logs:** o script roda em ambiente local privado e não imprime o token nem respostas com URLs autenticadas.
- **Falha no upload:** o usuário `7595607953` já possui interação aberta com o bot (`@unorobotsadlahkdajlkbot`), garantindo que `sendSticker` seja entregue com sucesso.
- **Ação acidental ao clicar na carta cinza:** a carta cinza gera ID com prefixo `grey_` e sem token de ação no `TokenStore`, garantindo que tocar nela não execute nenhuma jogada.

## Impactos esperados
- Menu "Suas cartas" com stickers visuais completos e homogêneos para todas as cartas da mão.
- Carta "Trocar cartas" indisponível exibida com sticker cinza escurecido, sem o texto provisório.
- Eliminação definitiva do erro `400 DOCUMENT_INVALID`.

## Compatibilidade
- Linux: execução e testes locais.
- macOS / Windows / Docker: compatível (IDs de arquivo no Telegram independem de SO).
- CI/CD: testes não dependem de chamadas externas de rede, garantindo 100% de sucesso no pipeline.

## Como testar

### Build
```bash
go build ./...
go build -tags debugcards ./cmd/bot
```

### Testes
```bash
go test ./internal/telegram -run 'TestSwap.*' -v
go test ./...
go test -tags debugcards ./...
go vet ./...
```

### Execução
```bash
go run -tags debugcards ./cmd/bot
```
Abrir o Telegram, entrar em uma partida no modo Caseiro com a carta "Trocar cartas" na mão (entregue via `/dar troca`). Quando a carta estiver indisponível (ex: sob efeito de +2 ou quando outro jogador jogar coringa), clicar em `🃏 Suas cartas` e verificar se a carta aparece escurecida com o sticker cinza, sem erros de carregamento e sem itens de texto na grade.

## Rollback
Caso o Telegram volte a apresentar qualquer instabilidade com o arquivo, reverter `inline.go` para o artigo de texto.

## Observações
Nenhum arquivo de código foi alterado antes desta aprovação. Todo o histórico anterior e trabalho não commitado foi rigorosamente preservado.

# Plano: sticker-cinza-troca-maos

## Pedido do usuário
Explicar por que a carta Trocar cartas apareceu poucas vezes e por que, quando indisponível, foi mostrada como item textual. Criar uma versão cinza do sticker, obter seu `sticker_id` no Telegram e usar essa versão no menu inline como ocorre com as demais cartas não jogáveis.

## Objetivo
Produzir uma variante visual desabilitada fiel ao sticker colorido existente, registrá-la no Telegram e substituir o fallback textual da carta `SwapHands` por `InlineQueryResultCachedSticker` cinza, sem criar token de jogada nem alterar regras ou frequência da carta.

## Contexto atual
- A branch `dev` está limpa e sincronizada com `origin/dev` no commit `f8b52b2`, que adicionou a carta.
- O modo Caseiro adiciona exatamente uma `SwapHands` ao `ClassicDeck`, resultando em 109 cartas. Por isso ela é rara: com dois jogadores, somente 14 cartas são distribuídas inicialmente.
- A carta colorida já possui o file ID `CAACAgEAAxkBAAER8VtqteJsR8-zG10NFeLTIZyxuZYsBQACBwkAAkkSsEU562tb90Ja3D0E`.
- O sticker original foi baixado de forma autenticada para análise: é um WebP estático de 342×512, com a arte “swap hands”.
- `StickersGrey` não contém a chave `swap_hands`.
- `buildPlayerHandResults` possui uma exceção explícita: quando `SwapHands` não é jogável, gera o artigo “Trocar cartas — indisponível agora” porque o commit anterior não recebeu uma versão cinza.
- As demais cartas usam `GetCardStickerGreyID` e retornam um `InlineQueryResultCachedSticker` sem token de ação.
- Um `sticker_id` não pode ser inventado localmente. O Telegram só o fornece depois que o arquivo é enviado/registrado pela Bot API. O bot configurado é `@unorobotsadlahkdajlkbot`, mas o projeto não possui `CHAT_ID` ou `USER_ID` de teste.

## Arquivos analisados
- `AGENTS.md`
- `internal/telegram/stickers.go`
- `internal/telegram/inline.go`
- `internal/telegram/swap_test.go`
- `internal/uno/card.go`
- `internal/uno/game.go`
- `internal/uno/swap_test.go`
- `docs/v2-telegram.md`
- `README.md`
- `.agent/plans/done/trocar-cartas-caseiro_2026-09-24_23-58.md`
- sticker colorido obtido via Telegram Bot API para inspeção visual
- par normal/cinza existente da carta vermelha 1, usado como referência do estado desabilitado

## Arquivos que poderão ser modificados
- `assets/stickers/swap_hands_grey.png` ou `.webp`
- `internal/telegram/stickers.go`
- `internal/telegram/inline.go`
- `internal/telegram/swap_test.go`
- `docs/v2-telegram.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`
- este plano, movido entre `pending`, `approved` e `done`

## Estratégia de implementação
Usar o sticker colorido como alvo de edição e gerar, com a ferramenta de imagem, uma variante desabilitada. A composição, bordas, setas, mãos, cartas e textos “swap hands” serão preservados; somente a paleta ficará escura, dessaturada e acinzentada, seguindo o contraste das cartas indisponíveis existentes.

O arquivo final será normalizado para sticker estático aceito pelo Telegram: PNG ou WebP, uma dimensão com 512 px, outra com no máximo 512 px e tamanho abaixo do limite da Bot API. O upload será feito pelo bot para um chat de teste informado pelo usuário. A resposta de `sendSticker` fornecerá `result.sticker.file_id`; somente então esse valor será incluído em `StickersGrey["swap_hands"]`.

No renderer inline, a exceção textual será removida. A carta indisponível seguirá o fluxo genérico de sticker cinza, com ID local prefixado por `grey_`, sem `ActionToken`; selecionar o resultado continuará sem executar jogada.

## Passos detalhados

1. Após aprovação, mover este plano para `approved`.
2. Editar o sticker original com a ferramenta de imagem, preservando integralmente desenho e texto e alterando somente o tratamento visual para desabilitado.
3. Inspecionar visualmente o resultado e normalizar dimensões/formato para a Bot API.
4. Enviar o arquivo ao chat de teste explicitamente fornecido pelo usuário usando o bot configurado e capturar o `file_id` retornado, sem registrar ou imprimir o token do bot.
5. Adicionar `swap_hands` ao mapa `StickersGrey` com o novo ID.
6. Remover o artigo textual especial de `inline.go`, usando o mesmo caminho de sticker cinza das outras cartas.
7. Testar que a versão jogável usa o sticker colorido, a indisponível usa o cinza, não cria token de ação e um resultado `grey_` não altera o jogo.
8. Atualizar documentação, contexto, memória e decisão.
9. Executar gofmt, testes focados, suíte completa, vet, build e `git diff --check`.
10. Mover o plano concluído para `done`.

## Riscos
- **Alteração indesejada de texto/desenho pela geração:** usar edição com invariantes rígidas, inspecionar e iterar antes do upload.
- **Arquivo rejeitado pelo Telegram:** validar formato, dimensões e tamanho antes de enviar.
- **Upload em chat errado:** exigir um chat ID de teste fornecido explicitamente; não usar grupo de produção presumido.
- **Vazamento do token:** ler `.env` apenas no processo local, nunca exibir URL autenticada, token ou resposta que o contenha.
- **Sticker cinza gerar jogada:** manter ID `grey_`, não criar `ActionToken` e cobrir com teste.
- **Aumentar a frequência sem decisão:** nenhuma mudança será feita na quantidade de cópias; continuará uma em 109 no modo Caseiro.

## Impactos esperados
- A carta Trocar cartas não jogável aparecerá visualmente como sticker desabilitado, igual às demais cartas.
- O artigo textual “indisponível agora” deixará de aparecer para essa carta.
- O novo `sticker_id` ficará associado em código e poderá ser reutilizado pelo bot.
- Regras, inventário, probabilidade e fluxo de troca de mãos permanecerão iguais.

## Compatibilidade
- Linux: geração/validação local e testes Go.
- macOS: uso do file ID não depende do sistema.
- Windows: uso do file ID não depende do sistema.
- Docker: o runtime continua usando somente IDs remotos; o asset local não é necessário para execução.
- CI/CD: o arquivo visual será estático e os testes continuarão sem chamar Telegram.

## Como testar

### Build
```bash
go build ./...
go vet ./...
```

### Testes
```bash
go test ./internal/telegram -run 'TestSwap.*Sticker|TestSwapInline' -count=20
go test ./...
git diff --check
```

### Execução
```bash
go run ./cmd/bot
```

Homologação manual: abrir uma mão que contenha `SwapHands` quando não for a vez ou quando alguma restrição impedir a jogada; confirmar que aparece o sticker cinza e que selecioná-lo não altera revisão, mão ou turno.

## Rollback
Restaurar o fallback textual, remover somente a entrada `StickersGrey["swap_hands"]`, os testes e o asset desta entrega por um novo commit. Não apagar o sticker remoto nem usar reset destrutivo; um file ID não referenciado é inofensivo.

## Observações
- Aprovação recebida com o chat ID privado `7595607953`.
- A variante cinza foi gerada e normalizada em `assets/stickers/swap_hands_grey.png` (PNG RGBA, 342×512, 206642 bytes).
- O primeiro envio foi recusado porque o chat ainda não havia iniciado o bot. Após o usuário enviar `/start`, `sendSticker` entregou a imagem na mensagem 27. O ID informado pelo cliente Telegram, `CAACAgEAAxkBAAER8aRqtlf6ZtRKfAj02K5AnlVcRz_W_AACVAcAAkaGsEXgXGCANqlQKz0E`, foi validado via `getFile`; ele resolve para o mesmo `file_unique_id` `AgADVAcAAkaGsEU` da resposta original.
- `StickersGrey["swap_hands"]` usa o ID confirmado pelo usuário. O fallback textual foi removido e a carta segue o renderer cinza genérico.
- O teste cobre o ID colorido, o ID cinza, o tipo `InlineQueryResultCachedSticker`, a ausência de token de ação e a imutabilidade da partida ao selecionar um resultado `grey_`.
- Validações concluídas: teste focado repetido 20 vezes, `go test ./...`, `go vet ./...`, `go build ./...` e `git diff --check`.
- Arte criada com a ferramenta integrada de geração de imagens em modo de edição, usando o sticker colorido como alvo e uma carta cinza existente como referência; o prompt preservou composição, textos, proporção e transparência, alterando somente a paleta para o estado indisponível.

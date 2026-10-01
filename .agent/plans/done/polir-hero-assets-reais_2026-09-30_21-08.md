# Plano: polir-hero-assets-reais

## Pedido do usuário
Refinar somente visual do detalhe do grupo, com auditoria curta seguida de implementação; usar cartas reais do projeto, avatar maior e composição mais polida. Somente dev, sem commit/push/main/deploy.

## Objetivo
Substituir cartas genéricas por artes originais do bot e equilibrar avatar, texto e decoração no hero integrado.

## Contexto atual
Rota /groups/:groupRef em App; RankingsPage renderiza hero, Avatar e Score, e RankingCard no painel. Styles.css contém todos os estilos. Decoração atual usa spans/ovais CSS. Assets locais têm somente swap_hands_grey.png, inadequado para fan colorido. Stickers coloridos são file IDs em internal/telegram/stickers.go; auditoria recuperou via getFile somente leitura as artes g_0/y_0/r_0 em /tmp/unobot-real-cards, sem mudar integração e sem mensagens. WebP com alpha, 342x512, ~10KB cada.

## Arquivos analisados
- web/src/App.tsx
- web/src/pages/Rankings.tsx
- web/src/components/Ranking.tsx
- web/src/styles.css
- web/vite.config.ts
- web/tsconfig.json
- web/src/App.test.tsx
- web/scripts/build.mjs
- internal/telegram/stickers.go
- internal/config/config.go
- assets/stickers/swap_hands_grey.png
- Ranking do Grupo em Estilo UNO.png
- .agent/context.md

## Arquivos que poderão ser modificados
- web/src/pages/Rankings.tsx
- web/src/components/HeroCards.tsx (novo)
- web/src/assets/cards/green-zero.webp (novo)
- web/src/assets/cards/yellow-zero.webp (novo)
- web/src/assets/cards/red-zero.webp (novo)
- web/src/assets/cards/README.md (novo)
- web/src/styles.css
- .agent/context.md
- .agent/memory/memory.md
- .agent/decisions.md

## Estratégia de implementação
Importar WebP pelo Vite num componente decorativo pequeno, sem lógica ou dependência remota em runtime. Substituir spans genéricos por imgs com alt vazio/aria-hidden, proporção intrínseca e sombras usando alpha. Ampliar avatar para 108px, deslocar levemente para cima e alinhar resumo com composição referência. Ajustes individuais de nome, ID, score e período sem scale geral e sem card interno. Manter rows e funções atuais; sutilezas de medalhas/bordas podem ser ajustadas.

## Passos detalhados
1. Registrar autorização e plano; mover para approved.
2. Copiar artes recuperadas sem alterar pixels; registrar origem/keys em README sem credenciais/URLs autenticadas.
3. Criar HeroCards e trocar decoração do hero.
4. Refinar grid, avatar, texto, gradiente e posicionamento/sombras das cartas, com breakpoints móveis.
5. Validar no navegador com API/Telegram simulados, assets reais servidos localmente, nomes extensos, scores extremos e sistemas Updated/Legacy. Confirmar navegação e ausência de overflow de conteúdo.
6. Executar checks frontend e build/testes Go após assets embed; registrar resultados e mover plano para done.

## Riscos
- Cartas reais possuem desenhos mais ricos que mockup; limitar área visível e preservar margem dos textos.
- Avatar maior comprime texto no narrow mobile; reservar largura para conteúdo e ajustar decoração em telas muito estreitas.
- Validação de dados reais no ambiente de produção não autorizada; confirmar consumo da mesma API por código, testes existentes e fixtures no browser, sem executar app/migrations em produção.

## Impactos esperados
- Artes originais offline, hero mais intencional e avatar mais importante.
- ~31KB de imagens estáticas adicionais; API e pontuação intactas.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD: WebP/import Vite suportados pelo build existente; assets incorporados no mesmo binário.

## Como testar

### Build
```bash
npm --prefix web run build
go build ./...
```

### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
git diff --check
go test -count=1 ./...
```

### Execução
```bash
npm --prefix web run dev
```
Browser com fixtures locais somente; sem iniciar backend/migrations.

## Rollback
Reverter apenas hunks/novos assets desta rodada, mantendo demais alterações e planos históricos.

## Observações
Autorização explícita do pedido: auditoria e depois implementação. Sem commit/push/main. Segredos usados somente em memória no download read-only, não incluídos no frontend nem logs/README. Homologação visual final com usuário.


## Resultado
- HeroCards e três WebP originais incorporados pelo Vite, com proveniência em README. Sem alteração de pixels ou dependências adicionais no projeto.
- Avatar 108px, offset vertical -4px; nome 30px, score 46px, ID 16px e período 15px. Padding/colunas/offset decorativo ajustados individualmente. Cartas width 128px e height auto, rotações 22/18/7 graus. Hero integrado sem caixa interna; lista preservada com leve ajuste da sombra dourada.
- Node 24.21.0: lint, typecheck, 31 testes frontend e build aprovados. Vite emitiu três WebP (~31KB). Go test -count=1 ./... e Go build ./... aprovados com novos assets embed; git diff --check aprovado.
- Browser com fixtures locais validou 280/320/360/390/430/1280px, nomes longos, score int64 máximo, assets sem erro, autenticação de fetch (fixture), Legacy/Updated e back preservando parâmetros. Foto de teste carregou pela mesma cadeia auth/blob/objectURL. Conteúdo sem overflow horizontal; red-card bounding box sem sobreposição com summary quando visível.
- Capturas reais do browser em /tmp/unobot-visual-validation/real-cards-{390,430,desktop}.png e real-cards-with-avatar-390.png. Dados simulados; não houve consulta a dados de produção, cujo consumo segue o mesmo código não modificado.
- Nenhum commit/push/main/deploy/migration. Alterações anteriores preservadas. Homologação visual pendente com usuário.

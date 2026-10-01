# Plano: bottom-navigation-rankings

## Pedido do usuário
Mover Grupos/Players para bottom navigation fixa. Manter Atualizado/Legado no header, URL e sistema, ocultar barra no detalhe e retornar sempre a Grupos. Só dev/working tree; sem commit/push/main/deploy/backend/API.

## Objetivo
Navegação principal acessível, fixa e responsiva, respeitando safe area sem sobrepor conteúdo.

## Contexto atual
RankingsPage usa useSearchParams para system/tab e Render Segmented Tipo de ranking no painel. App-shell tem max-width480 e margin20 no desktop. useTelegram alimenta variáveis safe-area; chamado por App e Page. Ambos controlam BackButton, podendo root hide sobrescrever show ao abrir deeplink de detalhe. Callback back atual preserva tab inclusive players; novo requisito exige groups.

## Arquivos analisados
- web/src/App.tsx
- web/src/pages/Rankings.tsx
- web/src/components/Ranking.tsx
- web/src/App.test.tsx
- web/src/lib/telegram.ts
- web/src/styles.css
- web/src/api/client.ts
- web/src/hooks/useRanking.ts
- web/package.json
- .agent/context.md

## Arquivos que poderão ser modificados
- web/src/components/BottomNavigation.tsx (novo)
- web/src/pages/Rankings.tsx
- web/src/App.tsx
- web/src/lib/telegram.ts
- web/src/App.test.tsx
- web/src/styles.css
- .agent/context.md
- .agent/memory/memory.md
- .agent/decisions.md

## Estratégia de implementação
BottomNavigation recebe system/tab já derivados da URL e renderiza Link React Router com aria-current=page, ícones SVG existentes/mesma abordagem e destaque discreto. Não mantém estado nem consulta API. Página monta barra só global, fora de pending/error/empty; remove antigo Segmented de seção, mantendo seletor de sistema e queries.
CSS barra fixed bottom0, largura100%/max480 centralizada, conteúdo60px e safe bottom via max(env, --telegram-bottom). Mesma fórmula reservada no padding-bottom global; desktop bottom20/margens/radius do app. Safe lateral também preservada. Detalhe mantém estilos sem barra/padding extra, com back para groups no mesmo sistema.
Hook useTelegram ganha opção de não controlar BackButton para root App; Page segue proprietário desse botão. Autenticação/initData/eventos de insets e APIs intactos.

## Passos detalhados
1. Registrar plano e seguir autorização explícita de auditoria/implementação.
2. Criar BottomNavigation com links e ícones, sem novas dependências.
3. Montar só em global, remover segmented e fixar retorno de detalhe a groups.
4. Aplicar CSS fixed/safe area/reserva/global e desktop limitado ao container.
5. Evitar root apagar BackButton do detalhe via opção no hook.
6. Adaptar/adicionar testes de URL, troca preservando sistema, deep links, retorno/TelegramBackButton e barra em loading/error/empty.
7. Validar frontend, navegador mobile/desktop/safe area/últimos rows, build embed e documentação.

## Riscos
- Barra fixed pode cobrir último item: fórmula única de altura/safe em conteúdo e nav, validar scroll final.
- Deep links precisam usar query atual como única fonte de verdade; evitar estado duplicado.
- Dois hooks controlando BackButton: ajustar ownership sem mudar autenticação.

## Impactos esperados
- Hierarquia sistema no topo / seção no footer; detalhe aprovado preservado.
- Sem alterações de backend/API/ranking/paginação/score/initData.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD: frontend padrão.
- Mobile-first/Telegram safe areas; desktop largura480 centralizada, sem escala.

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
```

### Execução
```bash
npm --prefix web run dev
```
Browser local com API/Telegram simulados para navegação/safe areas; não iniciar backend/migrations. Nenhum Go será alterado.

## Rollback
Reverter apenas hunks desta rodada e BottomNavigation; preservar working tree anterior e planos.

## Observações
Usuário autoriza implementação após auditoria sem nova pergunta. Sem commits/push/main/deploy. Tests extras necessários por ser alteração de navegação, sem testes espelho de CSS.


## Resultado
- BottomNavigation criada com links/ícones/aria-current e estados visuais discretos, sem estado local/dependência. Segmented Grupos/Players removido; system-switch permanece no header.
- Footer só global, fixed/centralizado/max480, conteúdo60 +border1 +safe bottom. Padding global usa60+safe+16; desktop bottom20 e raio32 inferior. Detalhe aprovado sem footer/reserva extra; back UI/Telegram/error fallback sempre groups no mesmo sistema.
- Root App chama useTelegram com manageBackButton=false e Page permanece proprietária, evitando ocultar botão do detalhe ao montar via deep link. Auth/initData/eventos/endpoint/query intactos.
- Node24.21.0: lint/typecheck/test(35)/build aprovados; git diff --check aprovado. go build ./... aprovado com assets embed; sem Go alterado.
- Browser fixtures em320/390/430/1280 e safe bottom0/34: nav alinhada ao app, max480, alturas61/95, sem overflow; último card43px acima do footer no scroll final. Verificados active state, Enter/teclado, refresh, preservação do sistema, volta por UI/Telegram e deeplink com tabplayers retornando groups, ausência da barra no detalhe, loading/error/empty e paginação6 itens.
- Capturas em /tmp/unobot-visual-validation/bottom-nav-players.png, bottom-nav-mobile-safe.png e bottom-nav-desktop.png. API/Telegram simulados; nenhum banco de produção acessado.
- Arquivos de produto alterados nesta rodada: components/BottomNavigation.tsx (novo), pages/Rankings.tsx, App.tsx, lib/telegram.ts, App.test.tsx e styles.css; memória/context/decisions atualizados. Alterações anteriores preservadas.
- Somente dev, nenhum commit/push/main/deploy/backend/API/ranking/migration; nova homologação visual com usuário.

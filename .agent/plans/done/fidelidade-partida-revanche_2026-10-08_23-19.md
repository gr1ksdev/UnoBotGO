# Plano: fidelidade da partida e revanche

## Pedido do usuário
Correção autorizada da partida real, usando o pacote v2; preservar mudanças locais, sem commit, push ou deploy.

## Objetivo
Reproduzir recursos e estados originais com animações GSAP e implementar consenso multiplayer de revanche seguro.

## Contexto atual
React/Vite, Go/engine UNO, projeções privadas, WebSocket com revisões/receipts e finalizador transacional. SVGs v1 e resultado separado; revanche ausente. Pacote v2 localizado em Downloads/unobotgo-v2, ainda não copiado ao repositório.

## Arquivos analisados
- AGENTS.md e .agent/context.md
- internal/game/{service,manager,views,finalization,results}.go
- internal/httpapi/live.go
- web/src/{pages/Game.tsx,hooks/useGame.ts,hooks/useTableMotion.ts,styles.css,api/client.ts}
- web/scripts/visual-check.mjs e Makefile
- /home/gabriel/Downloads/unobotgo-v2/{README.md,fonts.css,referencia.html,references/original-*}

## Arquivos que poderão ser modificados
- docs/design-reference/unobotgo-v2/ e web/public/assets/
- internal/game/ e internal/httpapi/live.go com testes
- web/src/pages/Game.tsx, hooks, api/client.ts e estilos
- web/scripts/ e testes frontend
- docs/miniapp-webapp.md e .agent/{memory/memory.md,context.md,decisions.md}
- .reports/partida-v2/

## Estratégia de implementação
Copiar recursos originais sem substituir v1 histórica; mapear cartas por rank/cor mantendo CardID e variantes apenas no descarte. Composição compacta, mão centralizada por largura medida, toque por faixa exposta, seletor em losango e resultado sobre a mesa. Votos no serviço protegidos pelo lock da partida, grupo fixo dos participantes finais, finalização confirmada antes de criar e publicar atomicamente nova engine com novo ID. Transporte existente transmite consenso e ponte para nova rodada; desconexão não exclui participantes e saída impede consenso até retorno.

## Passos detalhados
1. Importar pacote v2 e conferir capturas e hashes; verificar cartão de troca sem inventar asset.
2. Implementar contrato de votos e transição atômica/idempotente com testes de concorrência, pending e duplicatas.
3. Integrar projeção e WebSocket sem expor mãos de terceiros.
4. Ajustar recursos, fontes, mão, turno, seletor e resultado, respeitando reduced motion e cleanup.
5. Validar fluxo real criar/entrar/jogar já presente e preservar políticas.
6. Executar checks, matriz visual 4 tamanhos, estados e mãos/participantes; salvar capturas e relatório de limites locais/Telegram.
7. Registrar decisões/memória e mover plano para done somente após concluir.

## Riscos
- Conflito com alterações locais existentes: editar incrementalmente e não restaurar arquivos.
- Corridas de votos/criação e resultado pendente: lock e publicação atômica, IDs estáveis e bloqueio pelo pendingResults.
- Sobreposição de toque e corte na mão: validar geometria e toque real no Chromium estreito.

## Impactos esperados
- Recursos e composição v2, preservando regras e autenticação.
- Revanche exige todos os participantes originais; nenhuma exclusão automática por desconexão.

## Compatibilidade
- Linux, macOS, Windows: Go e Vite existentes.
- Docker e CI/CD: assets locais embutidos no bundle; sem fontes remotas.

## Como testar

### Build
```bash
npm --prefix web run build
go build ./...
```

### Testes
```bash
make check
CGO_ENABLED=1 go test -race ./internal/game ./internal/httpapi
npm --prefix web run test:visual
```

### Execução
```bash
npm --prefix web run dev
```

## Rollback
Reverter somente os hunks desta tarefa, preservando alterações anteriores. Não apagar históricos nem resultados.

## Observações
Aprovação explícita já dada no pedido atual. Telegram real depende de contas/ambiente; registrar somente o que for de fato exercitado. Sem publicar.


## Execução e validação
- Implementação concluída localmente conforme escopo aprovado; recursos v2 originais, contratos e testes integrados.
- make check passou (69 testes frontend), race game/httpapi passou; E2E browser/engine/WebSocket/PostgreSQL com race passou duas vezes; 180 casos visuais + 36 listas de resultado + 4 derrotas + safe areas/fontes/toque passaram.
- Relatório e comparações: .reports/partida-v2/README.md e comparisons.html.
- Homologação Telegram real/aparelho físico não realizada; SDK e identidades locais explicitamente identificados. Sem commit/push/deploy.

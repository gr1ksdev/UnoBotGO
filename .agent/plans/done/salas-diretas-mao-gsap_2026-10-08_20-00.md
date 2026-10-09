# Plano: salas diretas, mão acessível e GSAP

## Pedido do usuário
Criar/admitir/compartilhar/iniciar pela Mini App, mão grande rolável e GSAP coordenado pelos eventos confirmados; cores integradas; evidências com dois clientes, quatro viewports e vídeo. Autorização explícita nesta mensagem; sem commit/push/deploy.

## Objetivo
Completar experiência jogável direta mantendo engine, autorizações, socket e finalização canônica existentes.

## Contexto atual
Alterações não commitadas da continuação anterior preservadas. Rotas reais e ranking funcionam; criação/admissão ainda via bot. GSAP ausente; CSS local não representa eventos. Mão tem 80px/48px de avanço e seleção altera margem causando deslocamento. Score requer grupo/config existente.

## Arquivos analisados
AGENTS.md; planos/contexto/memória anteriores; referência unobotgo-v1; game/service/manager/views/events/results; HTTP live/references; grupos/repos/postgres e ranking; frontend Home/Game/hooks/styles; integração browser existente. Docs oficiais Telegram, GSAP e Flip consultadas.

## Arquivos que poderão ser modificados
internal/httpapi (salas, convites, grupos e socket); game (journal público confirmado); postgres (catálogo de grupos conhecidos); app/telegram (verificação de vínculo); web páginas/hooks/componentes/estilos/libs e lock; testes/harness; docs e .agent/.reports.

## Estratégia de implementação
Grupos conhecidos ou username público resolvido no servidor, com associação atual verificada pela API Telegram, sem aceitar chat ID livre ou política nova. Criar pelo serviço e inscrever criador; convite selado com expiração e autorização atual em admissão; capacidade/lock/revision preservados. Journal de eventos públicos com IDs/revision opacos ao frontend e indicação de recuperação. GSAP timeline/context, fantasmas de cartas entre bounds medidos e reorganização dos CardsID; baseline em reconexão não repete eventos. Mão 88px/60px avanço, bordas e seleção acessíveis; modal de cores 2×2 animado na mesa.

## Passos detalhados
1. Implementar contratos de criação/opções/grupos/convite/admissão e testes de falsificação/permissões/idempotência.
2. Conectar Home criar sala, convite, lobby e entrada pela WebApp; compartilhar link direto.
3. Expor journal público de eventos confirmado pelo serviço, sem mãos alheias, e baseline de reconexão.
4. Corrigir geometria da mão e implementar GSAP: distribuição/compra/jogada local/oponente/seleção/turno/efeitos/cores/resultado, cleanup e reduced motion.
5. E2E real dois navegadores criando/entrando pela UI, jogo inteiro, reconexão e finalização/ranking. Registrar screenshots/vídeo de animações.
6. Verificar quatro viewports, mãos1/2/7/15/30, até10jogadores, primeira/última carta e seleção sem corte. Comparar referência e corrigir.
7. Checks stack/race/integração, docs/exatidão de limites, memória e plano concluído.

## Riscos
Descoberta de grupos não é listagem universal Telegram: oferecer grupos conhecidos e validação de username público; grupos privados podem requerer presença conhecida do usuário/bot. Salas pontuadas mantêm contexto de grupo real. Ativas/pending continuam memória; nenhuma durabilidade nova presumida. Animações podem ser canceladas por snapshots rápidos e nunca governam turno.

## Impactos esperados
Bot permanece compatível; criação/admissão WebApp não requer comandos. Pontuação/transação invariantes. Bundle cresce com GSAP, sem renderer novo.

## Compatibilidade
Linux/macOS/Windows, Docker/CI; Chromium para captura, PostgreSQL para integração, Telegram Mini App autenticada em produção.

## Como testar
### Build
```bash
npm --prefix web run build
go build ./...
```
### Testes
```bash
make check
go test -race ./...
UNO_BROWSER_E2E=1 go test -race -tags integration -run TestTwoBrowserWebAppGame ./internal/httpapi
npm --prefix web run test:visual
```
### Execução
```bash
make dev
```

## Rollback
Reverter somente arquivos/hunks desta etapa, preservando mudanças anteriores/.env e referências; não apagar dados.

## Observações
Não solicitar nova aprovação: implementação das correções já explicitamente autorizada. Sem commit/push/deploy ou configuração remota automática. Vídeo usa identidades de teste e API/socket/engine/banco reais.

## Conclusão
Implementado e verificado; evidências .reports/miniapp-immersive e instruções docs/miniapp-webapp.md. make check isolado, race e E2E passaram;120 layouts/20ranking/4safeareas e touch. Sem commit/push/deploy; homologação Telegram real e durabilidade em memória explicitadas.

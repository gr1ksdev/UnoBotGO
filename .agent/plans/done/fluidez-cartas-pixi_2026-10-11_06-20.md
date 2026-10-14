# Plano: fluidez das cartas e do seletor de cores

## Pedido do usuário
Prosseguir com a implementação no UnoBotGO a partir das capturas e da análise dos recursos originais: distribuição, compra, giro de cartas, mão centralizada em fileiras com no máximo oito cartas e escolha de cor suave e perceptível.

## Objetivo
Melhorar a apresentação PixiJS/GSAP existente com perspectiva no giro, sombras acompanhando a elevação e uma coreografia de abertura, preenchimento e fechamento do seletor. Preservar identidade visual, regras e estado autorizado pelo servidor.

## Contexto atual
Clone limpo da branch dev, HEAD 7b85f69510ad5be1d2bd46b9a7e7dab16a798084. A branch já possui assets e fontes originais, WebSocket autenticado, revanche por consenso, fileiras de oito cartas, ordenação por cor/valor/ID e feedback de cor de aproximadamente 1,5 segundo. O giro atual apenas comprime um Sprite horizontalmente; distribuição e compra usam o mesmo voo sem sombra individual. Skills locais PixiJS/GSAP não estão versionadas; dependências instaladas incluem PixiJS 8.22 e GSAP 3.15. A implementação foi autorizada na conversa por “Vamos prosseguir entao para a implementacao?”. Nenhum commit, push ou deploy nesta tarefa.

## Arquivos analisados
- AGENTS.md, .agent/context.md, .agent/memory/memory.md, .agent/decisions.md
- web/package.json, package-lock.json
- web/src/components/scene/TableScene.ts, hooks/useTableMotion.ts
- web/src/pages/Game.tsx, game-v2.css, lib/handLayout.ts e testes
- web/scripts/motion-check.mjs, visual-check.mjs
- Documentação e implementação instalada de PerspectiveMesh
- Recursos públicos JS/CSS extraídos das capturas HAR fornecidas pelo usuário

## Arquivos que poderão ser modificados
- web/src/components/scene/TableScene.ts
- web/src/lib/cardMotion.ts e teste da projeção
- web/src/pages/Game.tsx, web/src/game-v2.css
- web/scripts/motion-check.mjs
- docs/ e .agent/ para verificação, memória, decisões e plano

## Estratégia de implementação
Trocar a compressão plana do voo por PerspectiveMesh com projeção dos quatro cantos, alternando verso/frente no ponto de giro. Manter Container por voo e sombra separada. Acrescentar sombra às cartas da mão e coreografia escalonada no seletor; comando de cor continua imediato e feedback não depende da latência. Reutilizar disposição, acessibilidade, composição de até dez participantes, revisões e recuperação já existentes.

## Passos detalhados
1. Registrar baseline de testes frontend e conferir harnesses de navegador.
2. Implementar projeção pura com teste de orientação, ponto de giro e dimensões finais.
3. Integrar voos com perspectiva, sombra e aterrissagem; preservar CardID, tint e descarte público.
4. Coordenar abertura, pétalas e preenchimento do seletor com feedback de 1,5 segundo; preservar recuperação após rejeição e supersessão por nova escolha.
5. Verificar testes, lint, tipos e build; executar harness de movimento e composições nas quatro dimensões.
6. Corrigir falhas materiais, registrar evidência e limitações, atualizar memória e decisões, mover plano para done e entregar patch revisável.

## Riscos
- Geometria degenerada no ponto de giro: manter largura mínima e projeção finita.
- Voo fora de sincronia em reconexão, rolagem ou resize: manter cancelamento e composição imediata.
- Assets de Troca de Mãos têm proporção nativa diferente: manter dimensões do slot na projeção.
- Efeitos precisam respeitar prefers-reduced-motion, destruir filhos e liberar timelines.
- Teste local não comprova comportamento no Telegram ou GPU de um aparelho físico.

## Impactos esperados
- Cartas com giro perceptível, profundidade e sombras em compra/distribuição.
- Escolha de cor com movimento escalonado e permanência suficiente.
- Sem alterações em contratos HTTP/WebSocket, pontuação, autenticação ou persistência.

## Compatibilidade
- Linux: ambiente de desenvolvimento e Chromium local.
- macOS e Windows: TypeScript e APIs web portáveis, sem caminhos absolutos em produção.
- Docker: bundle Vite e CSP existentes.
- CI/CD: scripts npm atuais; nenhuma publicação.

## Como testar
### Build
```bash
npm --prefix web run build
```
### Testes
```bash
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run test
node web/scripts/motion-check.mjs
node web/scripts/visual-check.mjs
git diff --check
```
Verificar distribuição, compra, descarte, duplicatas, recuperação, tint durante voo, escolha repetida/rejeitada, nova escolha, redução de movimento, mãos de 1/2/7/8/15/30 cartas e até dez participantes. Capturas em 320x568, 360x800, 390x844 e 430x932. Harness usa dados identificados de teste.
### Execução
```bash
npm --prefix web run dev -- --host 127.0.0.1
```
Harness de navegador intercepta backend; jogo real requer serviço Go e PostgreSQL configurados. Não imprimir credenciais.

## Rollback
Reverter somente alterações desta tarefa. Nenhum reset/clean, migration ou operação em ambiente externo. Assets e histórico existentes permanecem preservados.

## Observações
Plano registrado após inspeção e aprovado pela autorização explícita já presente na conversa. A aprovação não autoriza publicar no GitHub. Usar APIs oficiais instaladas; não copiar bibliotecas compiladas da captura, sessões, cookies ou protocolo de outro serviço.

## Conclusão
Implementação e verificação concluídas em 2026-10-11. Todos os checks frontend e 324 casos da matriz visual passaram. Evidências e limitações em docs/fluidez-cartas-pixi.md. Alterações entregues em patch revisável; nenhum commit, push ou deploy.

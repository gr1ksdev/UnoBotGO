# Plano: rolagem de nomes longos

## Pedido do usuário
Mostrar nomes completos de grupos e players com rolagem para a esquerda quando não couberem.

## Objetivo
Aplicar animação medida pelo overflow real, sem alterar dados ou layout do ranking.

## Contexto atual
Título do hero e nomes de players usam ellipsis; nomes globais podem quebrar linha. Branch dev com alterações anteriores preservadas.

## Arquivos analisados
- web/src/pages/Rankings.tsx
- web/src/components/Ranking.tsx
- web/src/styles.css
- web/src/components/Ranking.test.tsx

## Arquivos que poderão ser modificados
- web/src/components/ScrollingName.tsx
- web/src/components/ScrollingName.test.tsx
- web/src/components/Ranking.tsx
- web/src/pages/Rankings.tsx
- web/src/styles.css
- .agent/memory/memory.md

## Estratégia de implementação
Componente reutilizável mantém uma única cópia do nome acessível. ResizeObserver mede texto e container; animação CSS ocorre apenas se overflow superar 1px, com velocidade constante e pausas. Recalcular em mudança de nome, viewport e fonte. Redução de movimento desativa animação e permite rolagem manual/foco.

## Passos detalhados
1. Criar componente com observadores e limpeza.
2. Usar em título do grupo e nomes dos cards globais/internos.
3. Adicionar estilos de rolagem e alternativa acessível.
4. Testar overflow real simulado, resize e nomes curtos; validar frontend e navegador.

## Riscos
- Animação desnecessária: medir overflow real.
- Movimento desconfortável: respeitar prefers-reduced-motion.
- Mudança de fontes/largura: observar ambos os elementos e fontes.

## Impactos esperados
- Nomes completos visíveis sem ampliar cards; ranking e API intactos.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD; resize do window como fallback.

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
git diff --check
```

### Execução
```bash
npm --prefix web run dev
```

## Rollback
Desfazer exclusivamente esta integração e componente, preservando alterações anteriores.

## Observações
Implementação diretamente solicitada e autorização anterior de execução após auditoria. Sem commit/push/backend.

## Resultado
Implementado componente compartilhado e dois testes cobrindo overflow/resize/cleanup/troca de nome/fallback sem ResizeObserver. Lint, typecheck, 37 testes, build e git diff --check aprovados. Erro inicial de tipo this no teste corrigido e validações repetidas com sucesso. Browser Chromium: nomes longos no hero/cards deslizam; curtos estáticos; 390/430px sem overflow; redução de movimento permite scroll manual. Navegação, deep links, BackButton, sistema e estados preservados. Nenhum arquivo Go ou backend alterado, sem commit/push.

# Plano: escurecer seleção da pílula

## Pedido do usuário
Escurecer levemente a seleção da aba com acabamento de vidro desfocado.

## Objetivo
Trocar a película branca da seleção por cinza translúcido e blur discreto.

## Contexto atual
Bottom navigation aprovada, seleção branca em .glass-selection; branch dev.

## Arquivos analisados
- web/src/styles.css

## Arquivos que poderão ser modificados
- web/src/styles.css
- .agent/memory/memory.md

## Estratégia de implementação
Ajustar somente a superfície da seleção, preservando dimensões, animação e contraste dos labels. Manter alternativa de acessibilidade sem blur.

## Passos detalhados
1. Substituir gradiente branco por cinza translúcido e blur de 3px.
2. Desativar blur nas preferências de transparência/contraste.
3. Validar frontend e diff; registrar resultado.

## Riscos
- Escurecimento excessivo: usar baixa opacidade.

## Impactos esperados
- Seleção menos branca, sem alterações de navegação.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD; fallback sem backdrop-filter.

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
Restaurar somente os valores alterados de .glass-selection, preservando trabalho anterior.

## Observações
Ajuste visual solicitado diretamente; autorização anterior para implementar após auditoria mantida. Sem commit/push/backend.

## Resultado
Ajuste aplicado apenas à seleção em styles.css. Lint, typecheck, 35 testes, build frontend e diff-check aprovados. Nenhum Go alterado. Trabalho permanece na dev, sem commit/push.

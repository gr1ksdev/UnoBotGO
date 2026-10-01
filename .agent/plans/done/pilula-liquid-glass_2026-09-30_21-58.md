# Plano: pilula-liquid-glass

## Pedido do usuário
Separar a bottom navigation do rodapé: pílula flutuante com aparência inspirada no Liquid Glass da Apple.

## Objetivo
Mudar somente acabamento/posicionamento da barra, preservando links, estado URL e comportamento.

## Contexto atual
BottomNavigation renderiza links Grupos/Players somente no global. CSS fixed ocupa toda a largura do app e fica colado ao rodapé. Safe area usa variável Telegram e env; reserva de conteúdo precisa acompanhar a nova margem/frame.

## Arquivos analisados
- web/src/styles.css
- web/src/components/BottomNavigation.tsx
- .agent/context.md

## Arquivos que poderão ser modificados
- web/src/styles.css
- .agent/context.md
- .agent/memory/memory.md
- .agent/decisions.md

## Estratégia de implementação
Pílula fixed visualmente flutuante com max-width340px, folga lateral e inferior, safe area fora do corpo da cápsula. Blur/saturação/transparência e reflexos de borda suaves; fallback opaco sem suporte. Seleção como subpílula discreta, com labels/ícones existentes. Reserva de conteúdo compartilha medidas de altura/frame/margem/safe. Não introduzir estado/lógica ou dependência.

## Passos detalhados
1. Salvar plano e seguir autorização da tarefa.
2. Ajustar geometria, vidro e destaque ativo somente em CSS.
3. Conferir mobile/desktop/safe area, últimos itens e navegação existente.
4. Rodar checks, documentar e concluir plano.

## Riscos
- Transparência pode prejudicar contraste: blur, base branca e fallback opaco.
- Nova folga altera área coberta: atualizar reserva e medir último card.

## Impactos esperados
- Cápsula destacada do rodapé, com vidro translúcido, sem alterar telas aprovadas.

## Compatibilidade
- Linux, macOS, Windows, Docker e CI/CD: CSS padrão, sem dependências; fallback quando backdrop-filter indisponível.

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
Validar browser com fixtures locais; sem backend/migrations.

## Rollback
Reverter apenas hunks desta rodada, preservando working tree anterior.

## Observações
Interpretado 'não preso' como descolado visualmente do rodapé, permanecendo acessível ao rolar. Sem commit/push/main/deploy/backend/API/ranking/auth. Não se promete reprodução do material nativo Apple: efeito CSS inspirado.


## Resultado
Pílula max340, height70, radius999, margens laterais20+ e inferior12+safe (desktop+20). Vidro CSS com blur22/saturate180, translucidez/reflexo/sombra e fallback branco96; seleção interna arredondada. Reserva global98+safe. Somente styles.css em produto.

Lint/typecheck/test35/build/diff check e go build aprovados. Browser320/390/430/1280 e safe0/34 confirmou nav menor que app, distância inferior, último card44px acima no scroll final e sem overflow. Refresh, Enter, sistemas/URLs, back UI/Telegram, ocultação no detalhe, estados loading/error/empty e paginação também aprovados com fixtures. Capturas /tmp/unobot-visual-validation/glass-pill-{players,mobile-safe,desktop}.png.

Nenhum commit/push/main/deploy/backend/API/ranking/auth/migration. Working tree anterior preservado; homologação visual com usuário.

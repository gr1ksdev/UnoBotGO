# Plano: restaurar-proporcoes-referencia

## Pedido do usuário
Reprovação da miniaturização anterior. Recuperar escala, hierarquia, espaçamento e profundidade da referência aprovada, sem card separado no hero e sem mudanças backend/API/ranking. Sem commit/push.

## Objetivo
Reproduzir a presença visual da referência em mobile 390–430 CSS px e manter escala equivalente no container desktop.

## Contexto atual
Hero 192px sem insets, avatar 64px, nome 20px, score 26px, cartas 32x50px, sheet raio 20px, título 13px, rows 62px, avatars 38px, medalhas 23x30px, sombras removidas. Captura anterior e nova referência inspecionadas visualmente lado a lado. Referência recebida em Ranking do Grupo em Estilo UNO.png; não há segundo novo anexo acessível, mas captura anterior representa o working tree atual.

## Arquivos analisados
- web/src/styles.css
- web/src/pages/Rankings.tsx
- web/src/components/Ranking.tsx
- web/src/components/Ranking.test.tsx
- web/package.json
- .agent/context.md
- Ranking do Grupo em Estilo UNO.png
- /tmp/unobot-visual-validation/detail-390.png

## Arquivos que poderão ser modificados
- web/src/styles.css
- web/src/pages/Rankings.tsx
- .agent/context.md
- .agent/decisions.md
- .agent/memory/memory.md

## Estratégia de implementação
Alterar somente apresentação do detalhe; substituir regras compactas existentes, mantendo composição integrada. Escalar cada elemento individualmente. Avatar 96px, nome 28px, score 44px como pontos de partida. Cartas físicas grandes parcialmente fora da viewport, em área reservada à direita. Sheet raio 36px e sobreposição 22px; rows 88px, avatars 56px, medalhas 30x40px. Sombras leves e contraste da referência. Conservar fallback anterior, dados, formatação e fluxo.

## Passos detalhados
1. Registrar plano e mover para approved pela autorização de continuidade da tarefa.
2. Ajustar hero, tipografia, cards decorativos e sobreposição de sheet.
3. Restaurar escala e acabamento das linhas, medalhas e avatars.
4. Renderizar com dados locais simulados e comparar visualmente com referência em mobile e desktop.
5. Verificar responsividade, nomes longos, scores grandes, ausência de overflow de conteúdo e integração do hero; decoração pode ultrapassar viewport como solicitado.
6. Executar validações, documentar resultados e mover plano para done.

## Riscos
- Cartas grandes podem competir com textos; reservar margem e manter decoração atrás da sheet.
- Score excepcionalmente grande precisa de quebra, sem escala global.
- Referência inclui barra nativa do telefone; não recriar chrome do iOS no app.

## Impactos esperados
- Hierarquia recuperada e menos miniaturização; densidade deixa de ser objetivo.
- Sem alteração de API/ranking/score/auth/navegação.

## Compatibilidade
- Linux, macOS, Windows, Docker e CI/CD: CSS/React padrão.
- Mobile first; desktop com max-width existente e mesmas dimensões internas.

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
API e Telegram simulados apenas no navegador local, sem backend/migrations.

## Rollback
Reverter apenas hunks desta rodada, preservando alterações anteriores e referência fornecida.

## Observações
Tarefa autoriza alteração e validação seguidas de nova homologação visual. Sem commit/push/deploy. Referência agora disponível e inspecionada; prevalece sobre objetivos anteriores de compactação.


## Resultado e comparação objetiva

| Elemento | Rodada compacta reprovada | Nova implementação em 390–430 CSS px |
| --- | --- | --- |
| Header sem insets | 192px | 286px |
| Avatar do grupo | 64px | 96px |
| Nome do grupo | 20px | 28px |
| ID | 12px | 15px |
| Pontuação | 26px | 44px |
| Período | 11px | 14px |
| Cartas | 32x50px em coluna interna | 112x174px, três cartas sobrepostas, borda direita cortada |
| Sheet / overlap | 20px / 12px | 36px / 22px |
| Título / ícone | 13px / 16px | 17px / 22px |
| Rows | 62px | 88px |
| Avatares jogadores | 38px | 56px |
| Medalhas | 23x30px | 30x40px |
| Gap vertical rows | 6px | 10px |
| Profundidade | Sem sombras específicas | Sombras leves em cards, avatars, medalhas, cartas |

- Referência, captura anterior e ajuste comparados lado a lado em HTML autossuficiente e imagem: /tmp/unobot-visual-validation/comparacao-proporcoes.html e comparacao-proporcoes.png. Referência inclui barra nativa; não foi reproduzida no app.
- Arquivos de produto alterados nesta rodada: web/src/styles.css e web/src/pages/Rankings.tsx. Documentação atualizada sem apagar histórico anterior.
- Validado em 280/320/360/390/430px e desktop 1280px com container 480px: conteúdo sem overflow horizontal, nomes longos e scores int64 máximos preservados com quebra de linha, navegação de volta válida. Decoração ultrapassa borda intencionalmente, contida pelo header.
- npm lint/typecheck/test (31 testes)/build e git diff --check aprovados com Node 24.21.0; Go test -count=1 ./... e Go build ./... aprovados após geração de assets. Integrações PostgreSQL dependem de banco configurado; nenhum banco/produção consultado.
- Sem commit/push/deploy/migration, somente dev. Homologação visual pendente com usuário; implementação encerrada no working tree.

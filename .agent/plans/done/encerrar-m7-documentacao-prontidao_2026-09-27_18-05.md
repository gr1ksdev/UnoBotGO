# Plano: encerrar-m7-documentacao-prontidao

## Pedido do usuário
Encerrar formalmente a Milestone M7 no escopo atual na branch `dev`:
- Registrar na documentação técnica a conclusão da M7 e a homologação manual real no Telegram (Legacy N=2 e N=3; Updated N=2 e N=3; Abandono definitivo fora do ranking com 0 pontos; Saída + reentrada concluindo e pontuando normalmente; /config com botões inline, modo e ranking selecionáveis, defaults operacionais e bloqueio de troca de ranking com histórico acumulado).
- Marcar explicitamente a importação de ranking antigo como `DEFERRED` ("Old ranking import is deferred to a future milestone"), mantendo a base de código já existente (parser/staging/reconciliação) sem criar novos comandos, sem aplicar pontuações e sem conversões prematuras.
- Atualizar `docs/project-status.md` e `docs/m7-persistence.md` com os status reais (`IMPLEMENTED + HOMOLOGATED`, `IMPLEMENTED BUT NOT HOMOLOGATED`, `DEFERRED`, `OUT OF SCOPE`), mantendo Polling como padrão e recomendado.
- Atualizar `README.md` público com requisitos de runtime do PostgreSQL, migrations prévias, variáveis relevantes, comando `/config`, modos e rankings configuráveis, sem mencionar artefatos internos ou `.agent`.
- Realizar auditoria de prontidão e segurança de branch para promoção para `main` (respeitando `docs/branching.md`), identificando arquivos permitidos e proibidos, impactos de runtime/Docker/CI e estratégia de publicação sem alterar a `main` e sem push.

## Objetivo
Formalizar o fechamento da M7 na branch `dev` por meio de atualizações documentais limpas, rastreabilidade na memória persistente, garantia de integridade da suíte de testes e emissão do relatório de prontidão para promoção para a `main`.

## Contexto atual
- HEAD inicial da `dev`: `bb352b31a6dd735cbbd1f7209c811d13fc83fa07`.
- Árvore de trabalho limpa, 16 commits à frente de `origin/dev`.
- Todos os testes unitários, testes com race detector, testes com debugcards, compilação de pacotes e testes de integração com PostgreSQL 17 executados com sucesso (100% PASS).
- A branch `main` (`origin/main`) está no commit `62fc344` e possui histórico independente.

## Arquivos analisados
- `AGENTS.md`
- `README.md`
- `docs/project-status.md`
- `docs/m7-persistence.md`
- `docs/branching.md`
- `.github/workflows/public-tree.yml` (em `origin/main`)
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`

## Arquivos que poderão ser modificados
- `docs/m7-persistence.md`
- `docs/project-status.md`
- `README.md`
- `.agent/context.md`
- `.agent/memory/memory.md`
- `.agent/decisions.md`

## Estratégia de implementação

1. **Documentação M7 (`docs/m7-persistence.md`)**:
   - Atualizar status de conclusão da milestone M7.
   - Detalhar a seção de homologação real executada no Telegram com todas as evidências listadas.
   - Declarar explicitamente `Old ranking import is deferred to a future milestone`, documentando que o parser/staging/reconciliação existente permanece como fundação técnica sem execução no runtime atual.

2. **Status do Projeto (`docs/project-status.md`)**:
   - Atualizar a matriz com as categorias rigorosas: `IMPLEMENTED + HOMOLOGATED`, `IMPLEMENTED BUT NOT HOMOLOGATED`, `DEFERRED`, `OUT OF SCOPE`.
   - Marcar persistência PostgreSQL, snapshots, elegibilidade, finalização síncrona pós-commit e UX de `/config` como `IMPLEMENTED + HOMOLOGATED`.
   - Marcar ranking import como `DEFERRED`.
   - Manter Polling como modo recomendado/homologado e Webhook como experimental.

3. **README Público (`README.md`)**:
   - Ajustar a seção de requisitos e execução: documentar a necessidade do PostgreSQL (`DATABASE_URL`) no runtime do V2 e a execução de migrations (`go run ./cmd/migrate` ou container).
   - Atualizar tabela de comandos incluindo `/config` e suas permissões.
   - Citar os modos de jogo (Clássico e Caseiro) e sistemas de ranking (Legado e Atualizado).
   - Manter linguagem limpa e sem referências a artefatos internos de agentes.

4. **Memória Persistente e Decisões (`.agent/`)**:
   - Registrar em `.agent/context.md`, `.agent/memory/memory.md` e `.agent/decisions.md` o encerramento da M7 e a decisão de diferimento do import antigo.

5. **Auditoria de Prontidão e Segurança para Promoção (`main`)**:
   - Auditar a lista exata de arquivos a serem promovidos para `main` (código de produção V2, migrations SQL, testes de regressão V2, documentação técnica pública).
   - Auditar a lista exata de arquivos que NÃO podem entrar na `main` (`.agent/`, `AGENTS.md`, `.reports/`, `codemaps/`, código V1 na raiz, `debugcards.go`).
   - Mapear a estratégia recomendada de publicação conforme `docs/branching.md` (árvore pública gerada de forma limpa sem merge de histórico).
   - Identificar requisitos para o CI da `main` (ex: disponibilização de serviço PostgreSQL para os testes de integração do storage).

## Passos detalhados
1. Submeter o plano para aprovação explícita do usuário.
2. Após aprovação, mover o plano para `.agent/plans/approved/`.
3. Atualizar `docs/m7-persistence.md` com a homologação real e nota de adiamento do import.
4. Atualizar `docs/project-status.md` com a matriz de status categorizada.
5. Atualizar `README.md` com requisitos de runtime, migrations e documentação dos modos/ranking.
6. Atualizar `.agent/context.md`, `.agent/memory/memory.md` e `.agent/decisions.md`.
7. Executar a suíte de validação:
   - `go test -count=1 ./...`
   - `go test -count=1 -race ./...`
   - `go test -tags debugcards ./...`
   - `go vet ./...`
   - `go build ./...`
   - `go build -tags debugcards ./...`
   - `git diff --check`
   - Testes de integração PostgreSQL com `TEST_DATABASE_URL`.
8. Criar commits pequenos e descritivos na branch `dev` (sem push).
9. Mover o plano para `.agent/plans/done/`.
10. Apresentar o relatório completo de fechamento da M7 e o relatório de prontidão para promoção para a `main`.

## Riscos
- **Inclusão acidental de arquivos proibidos na árvore pública**: Mitigado pela validação prévia com a regra estrita do workflow `public-tree.yml` da `main`.
- **Quebra do CI da `main` por ausência de PostgreSQL**: No CI de `dev` já foi adicionado o container de serviço PostgreSQL (`.github/workflows/dev-ci.yml`); será avaliado e reportado se o workflow de `main` requer adição do serviço para testar `internal/storage/postgres`.

## Impactos esperados
- M7 formalmente documentada e homologada na `dev`.
- Documentação pública e técnica alinhada com o estado real do projeto.
- Estrutura pronta para a etapa de promoção limpa para a branch `main`.

## Compatibilidade
- Linux, macOS, Windows, Docker, CI/CD.

## Como testar
### Build
```bash
go build ./...
go build -tags debugcards ./...
```
### Testes
```bash
go test -count=1 ./...
go test -count=1 -race ./...
go test -tags debugcards ./...
go vet ./...
git diff --check
```

## Rollback
Desfazer as alterações documentais com `git restore` antes do commit, ou `git revert` caso commitado.

## Observações
- Não altera `main`.
- Não faz push sem autorização explícita.
- Não implementa features adicionais.

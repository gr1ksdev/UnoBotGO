# Plano: commit-push-dev-seguro

## Pedido do usuário
Fazer o git commit e push para a branch `dev`, com cuidado estrito para não enviar arquivos sensíveis.

## Objetivo
Commitar com segurança todas as implementações validadas (salas livres, turnos, sincronização de jogo, refinamentos de interface e áudio), garantindo que arquivos temporários, capturas de tráfego de rede (.har), arquivos de sistema (.DS_Store), backups (.zip/.patch) e relatórios binários volumosos não sejam adicionados ao repositório, e realizar o push para `origin/dev`.

## Contexto atual
- Branch atual: `dev` (sincronizada com `origin/dev`).
- As suites de testes passaram com sucesso: Go backend (`go test ./...`) e Frontend (`vitest run`, 139 testes passando).
- Existem 37 arquivos de código rastreados modificados (`internal/...`, `web/...`, `.agent/...`).
- Existem arquivos não rastreados que contêm código/testes/migrações legítimas (`0012_free_rooms.up.sql`, componentes e bibliotecas em `web/src/`, áudios e documentação em `docs/`).
- Existem arquivos não rastreados que **NÃO DEVEM** ser commitados:
  - `matcho.dotvhs.com.har` (2.4 MB de captura de tráfego de rede HTTP - sensível).
  - `.env` (credenciais locais - já ignorado).
  - `.DS_Store` e `.reports/.DS_Store` (metadados do macOS).
  - `unobotgo-*.zip` e `unobotgo-*.patch` (arquivos temporários).
  - `.reports/desempenho-local/`, `.reports/diagnostico-fluidez/`, `.reports/experiencia-local/` (mais de 320 MB de gravações de vídeo locais e screenshots que excedem cotas e não devem poluir o histórico git).

## Arquivos analisados
- `.gitignore`
- `.env.example`
- `matcho.dotvhs.com.har`
- `internal/storage/postgres/migrations/0012_free_rooms.up.sql`
- `internal/game/...`, `internal/httpapi/...`, `internal/ranking/...`, `internal/storage/...`, `internal/uno/...`
- `web/src/...`, `web/scripts/...`, `web/public/assets/...`

## Arquivos que poderão ser modificados
- `.gitignore`
- Arquivos de código, testes, migração e documentação a serem commitados:
  - `.agent/context.md`
  - `.agent/decisions.md`
  - `.agent/memory/memory.md`
  - `.agent/plans/done/*`
  - `docs/*`
  - `internal/game/*`
  - `internal/httpapi/*`
  - `internal/ranking/*`
  - `internal/storage/postgres/*`
  - `internal/uno/*`
  - `web/public/assets/*`
  - `web/scripts/*`
  - `web/src/*`

## Estratégia de implementação
1. Atualizar `.gitignore` para bloquear explicitamente:
   - `.DS_Store`
   - `*.har`
   - `*.zip`
   - `*.patch`
   - `.reports/desempenho-local/`
   - `.reports/diagnostico-fluidez/`
   - `.reports/experiencia-local/`
2. Adicionar os arquivos de funcionalidade, testes, migração, documentação e histórico do agente ao stage (`git add`).
3. Inspecionar `git status` e `git diff --cached` para garantir ausência absoluta de segredos, tokens ou arquivos indesejados.
4. Realizar o commit descritivo:
   `feat(webapp): implementa salas livres, turnos autoritativos e refinamentos de jogo`
5. Realizar o push para o remote na branch `dev`:
   `git push origin dev`
6. Verificar o status pós-push.

## Passos detalhados
1. Atualizar `.gitignore` com as exclusões seguras.
2. Executar `git add .gitignore docs/ internal/ web/ .agent/` (e migração correspondente).
3. Confirmar que `git status` não contém arquivos indesejados no stage e que `matcho.dotvhs.com.har`, `.env`, `.DS_Store` e os `.zip` permanecem ignorados ou intocados.
4. Executar `git commit -m "..."`.
5. Executar `git push origin dev`.
6. Confirmar que o commit remoto em `origin/dev` foi atualizado.

## Riscos
- **Inclusão de arquivo sensível**: Mitigado adicionando exclusões estritas no `.gitignore` antes de fazer `git add` e verificando o staged diff.
- **Push para branch incorreta**: Apenas a branch `dev` em `origin` será alvo do comando.

## Impactos esperados
- Código das novas funcionalidades (salas livres, turnos, interface do WebApp, áudio, migração de banco) versionado e sincronizado com o GitHub na branch `dev`.
- Nenhuma informação privada, credencial ou dump de rede vazado.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- CI/CD

## Como testar

### Build
```bash
go build ./... && (cd web && npm run build)
```

### Testes
```bash
go test ./... && (cd web && npm test -- --run)
```

### Execução
```bash
git status
git log -1 origin/dev
```

## Rollback
Caso necessário desfazer o commit antes ou após o push:
```bash
git reset --soft HEAD~1
```

## Observações
O arquivo `.env` permanece estritamente ignorado e ausente do commit. O arquivo `matcho.dotvhs.com.har` (captura de rede externa) será completamente excluído.

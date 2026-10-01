# Plano: commit e push na dev

## Pedido do usuário
Fazer commit e push na branch dev. Autorização explícita substitui restrição anterior de não publicar.

## Objetivo
Versionar alterações acumuladas e enviar somente para origin/dev.

## Contexto atual
Branch dev sincronizada com origin/dev antes do commit. Refinamentos frontend, assets e documentação; ferramenta devseed de homologação local também presente.

## Arquivos analisados
- git status e diff de todos os arquivos rastreados
- Makefile, .gitignore, cmd/devseed/main.go, internal/devseed/seed.go
- plano de homologação local

## Arquivos que poderão ser modificados
- Este registro de execução; conteúdo existente será versionado sem novas mudanças funcionais.

## Estratégia de implementação
Verificar branch, diff, remoto e testes, adicionar arquivos revisados, criar commit e push explícito HEAD:dev sem force.

## Passos detalhados
1. Auditar status e sincronização com origin/dev.
2. Validar diff e Go; aproveitar checks frontend já aprovados.
3. Criar commit e enviar para dev.
4. Confirmar status e igualdade do commit remoto.

## Riscos
- Alterações acumuladas de várias rodadas: escopo comunicado antes do commit.

## Impactos esperados
Histórico e branch dev atualizados; main e deploy preservados.

## Compatibilidade
Linux, macOS, Windows, Docker e CI/CD sem alteração neste passo.

## Como testar

### Build
```bash
go build ./...
```

### Testes
```bash
go test -count=1 ./...
git diff --check
```
Frontend previamente aprovado: lint, typecheck, 37 testes e build.

### Execução
```bash
git push origin HEAD:dev
```

## Rollback
Se necessário, reverter commit posteriormente com autorização, sem reset/force push.

## Observações
Testes Go e build aprovados antes do commit. Nenhum deploy ou migration executado.

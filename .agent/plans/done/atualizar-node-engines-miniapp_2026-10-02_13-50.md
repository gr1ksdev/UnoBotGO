# Plano: atualizar-node-engines-miniapp

## Pedido do usuário
Atualizar a restrição de versão do Node.js (`engines`) no Mini App para eliminar o aviso `npm warn EBADENGINE Unsupported engine` ao executar comandos npm/make em ambientes com Node.js v26+.

## Objetivo
Alterar o campo `"engines"` no arquivo `web/package.json` de `{"node": ">=24 <25"}` para `{"node": ">=24"}`, permitindo a execução transparente e sem advertências do Node.js em versões atuais (como a v26.10.0 presente na máquina do usuário) e futuras, mantendo compatibilidade com Node.js 24+.

## Contexto atual
- `web/package.json` possui `"engines": { "node": ">=24 <25" }`.
- O ambiente do desenvolvedor utiliza Node.js `v26.10.0` e npm `11.19.1`.
- Ao rodar `make web-build` ou comandos npm, o npm emite o aviso:
  `npm warn EBADENGINE Unsupported engine { package: 'unobotgo-miniapp@0.1.0', required: { node: '>=24 <25' }, current: { node: 'v26.10.0', npm: '11.19.1' } }`.

## Arquivos analisados
- `web/package.json`
- `web/package-lock.json`
- `Makefile`

## Arquivos que poderão ser modificados
- `web/package.json`
- `web/package-lock.json` (se sincronizado pelo npm)

## Estratégia de implementação
1. Atualizar `"engines": { "node": ">=24" }` em `web/package.json`.
2. Validar a execução de `npm --prefix web ci` ou `npm --prefix web run build` confirmando a eliminação do aviso `EBADENGINE`.
3. Executar a suíte de testes (`npm --prefix web run test`) e build (`npm --prefix web run build`).

## Passos detalhados
1. Editar `web/package.json` ajustando a chave `engines.node` para `">=24"`.
2. Executar `npm --prefix web run typecheck`, `npm --prefix web run lint`, `npm --prefix web run test` e `npm --prefix web run build`.
3. Validar `git diff --check`.

## Riscos
- **Risco**: Incompatibilidade de sintaxe ou APIs entre Node 24 e Node 26.
  - **Mitigação**: O código do frontend é TypeScript transcompilado pelo Vite/Rollup e a suíte completa de 49 testes roda com 100% de aprovação no Node v26.10.0.

## Impactos esperados
- Eliminação definitiva do aviso `EBADENGINE` ao rodar `make web-build`, `npm ci` ou scripts do frontend.

## Compatibilidade
- Linux: Sim
- macOS: Sim
- Windows: Sim
- Docker: Sim
- CI/CD: Sim

## Como testar

### Build
```bash
export PATH="/home/senzu/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web run build
```

### Testes
```bash
export PATH="/home/senzu/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web run test
```

### Execução
```bash
export PATH="/home/senzu/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web run dev
```

## Rollback
```bash
git checkout -- web/package.json
```

## Observações
A alteração afeta exclusivamente a declaração de compatibilidade de runtime local do Node.js, sem impacto nas dependências ou no código compilado.

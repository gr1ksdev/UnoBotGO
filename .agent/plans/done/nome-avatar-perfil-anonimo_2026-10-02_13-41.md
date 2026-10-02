# Plano: nome-avatar-perfil-anonimo

## Pedido do usuário
Na parte de Perfil, exibir o nome e o avatar do usuário. Quando ele ativar o modo anônimo, a foto e o nome devem ser convertidos para anônimo.

## Objetivo
1. Integrar os dados do usuário autenticado no Telegram (`window.Telegram.WebApp.initDataUnsafe.user`) à tela de Perfil (`/profile`):
   - **Quando público (`anonymous === false`)**:
     - Exibir o nome real do usuário (`first_name` + `last_name`, ou `username`, com fallback seguro).
     - Exibir a foto do usuário (`photo_url`) ou iniciais estilizadas no contêiner `.avatar`.
     - Subtítulo com `@username` ou indicador de visibilidade pública.
   - **Quando anônimo (`anonymous === true`)**:
     - Nome exibido como `"Anônimo"`.
     - Foto convertida imediatamente para o avatar neutro estilizado de silhueta (`.avatar-anonymous`), sem disparar requisições ou exibir imagem do usuário.
     - Subtítulo indicando `"Modo anônimo ativado • Oculto no ranking"`.
2. Assegurar tipagem estrita no TypeScript através da interface `TelegramUser` no SDK do Telegram WebApp.
3. Atualizar os testes unitários do frontend (`web/src/pages/Profile.test.tsx`) cobrindo transições de nome e foto entre os estados público e anônimo.

## Contexto atual
- A tela de Perfil (`web/src/pages/Profile.tsx`) possui um card de status com título fixo `<h2>Modo Anônimo</h2>` e um ícone SVG simples.
- Os dados reais do usuário (nome, username e foto) são fornecidos nativamente pelo cliente Telegram via `window.Telegram.WebApp.initDataUnsafe.user`.
- No Ranking Global, jogadores anônimos já são projetados com `Name: "Anônimo"` e ícone neutro `AnonymousIcon` sem avatar visível. A tela de Perfil deve espelhar esse comportamento dinamicamente ao alternar o switch.

## Arquivos analisados
- `web/src/pages/Profile.tsx`
- `web/src/pages/Profile.test.tsx`
- `web/src/lib/telegram.ts`
- `web/src/components/Ranking.tsx`
- `web/src/styles.css`

## Arquivos que poderão ser modificados
- `web/src/lib/telegram.ts`
- `web/src/pages/Profile.tsx`
- `web/src/pages/Profile.test.tsx`
- `web/src/styles.css`

## Estratégia de implementação

### 1. Tipagem em `web/src/lib/telegram.ts`
- Adicionar interface `TelegramUser`:
  ```ts
  export interface TelegramUser {
    id?: number
    first_name?: string
    last_name?: string
    username?: string
    photo_url?: string
  }
  ```
- Adicionar `initDataUnsafe?: { user?: TelegramUser }` na interface `TelegramApp`.

### 2. Lógica e Renderização em `web/src/pages/Profile.tsx`
- Obter o usuário do Telegram:
  ```ts
  const user = typeof window !== 'undefined' ? window.Telegram?.WebApp?.initDataUnsafe?.user : undefined
  const realName = [user?.first_name, user?.last_name].filter(Boolean).join(' ') || user?.username || 'Jogador'
  ```
- Gerenciar estado de erro de imagem (`const [imgError, setImgError] = useState(false)`).
- Calcular iniciais do nome real:
  ```ts
  const initials = realName.trim().split(/\s+/u).slice(0, 2).map(p => Array.from(p)[0]).join('').toUpperCase() || 'J'
  ```
- Renderização dinâmica do avatar:
  - Se `isAnonymous`: renderizar ícone neutro de silhueta (`.avatar-anonymous`).
  - Se público: se `user?.photo_url && !imgError`, renderizar `<img src={user.photo_url} alt="" onError={() => setImgError(true)} />`; caso contrário, renderizar `initials`.
- Renderização dinâmica do nome e subtítulo:
  - Nome: `isAnonymous ? 'Anônimo' : realName`.
  - Subtítulo: `isAnonymous ? 'Modo anônimo ativado • Oculto no ranking' : (user?.username ? '@' + user.username : 'Visível publicamente no ranking')`.

### 3. Ajustes de Estilo em `web/src/styles.css`
- Garantir que `.profile-avatar-box` se comporte como avatar circular completo (`border-radius: 50%`, `overflow: hidden`, imagem com `object-fit: cover`).
- Assegurar transição suave de opacidade/cor ao alternar entre foto real e silhueta anônima.

### 4. Testes em `web/src/pages/Profile.test.tsx`
- Mockar `initDataUnsafe: { user: { id: 123, first_name: 'Gabriel', last_name: 'Silva', username: 'gabrielsilva', photo_url: 'https://example.com/photo.jpg' } }`.
- Testar:
  - Inicialmente (público): renderiza nome `"Gabriel Silva"`, `@gabrielsilva` e imagem da foto.
  - Ao alternar para anônimo: nome passa a ser `"Anônimo"`, avatar vira anônimo, texto passa a `"Modo anônimo ativado • Oculto no ranking"`.
  - Ao alternar de volta para público: nome e foto são restaurados.

## Passos detalhados
1. Atualizar `web/src/lib/telegram.ts` com a tipagem de `TelegramUser` e `initDataUnsafe`.
2. Atualizar `web/src/pages/Profile.tsx` para exibir dinamicamente o nome, avatar (com fallback de iniciais e erro de imagem) e subtítulo conforme o estado de `isAnonymous`.
3. Ajustar `web/src/styles.css` caso necessário para garantir o encaixe da imagem e tipografia do nome do usuário.
4. Atualizar `web/src/pages/Profile.test.tsx` com cenários de usuário real e transição anônima.
5. Executar `npm --prefix web test` e `npm --prefix web run build`.
6. Executar suíte completa `make check` e `git diff --check`.
7. Atualizar memória e decisões em `.agent/memory/memory.md` e `.agent/decisions.md`.

## Riscos
- **Risco**: Usuário abre o Mini App fora do Telegram ou sem `photo_url`.
  - **Mitigação**: Fallback resiliente que monta o nome a partir de first/last name, username ou `"Jogador"`, e exibe iniciais com gradiente de alta definição caso a foto inexista ou falhe ao carregar.

## Impactos esperados
- A aba de Perfil refletirá fielmente a identidade do usuário logado quando público, e converterá instantaneamente nome e foto para `"Anônimo"` e avatar neutro ao ativar o modo anônimo.

## Compatibilidade
- Linux
- macOS
- Windows
- Docker
- Telegram iOS / Android / Desktop / Web

## Como testar

### Build
```bash
export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web run build
```

### Testes
```bash
export PATH="/home/gabriel/.local/share/nvm/v26.10.0/bin:$PATH"
npm --prefix web test
make check
```

## Rollback
Desfazer alterações via `git checkout web/src/lib/telegram.ts web/src/pages/Profile.tsx web/src/pages/Profile.test.tsx web/src/styles.css`.

## Observações
Trabalho estritamente na branch `dev`, mantido no working tree sem commit ou push.

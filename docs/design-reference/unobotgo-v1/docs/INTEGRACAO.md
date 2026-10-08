# Integração com o UnoBotGO real

Este documento define invariantes e um contrato conceitual. Não afirma que os endpoints, tabelas ou tipos abaixo existem. O repositório real não foi disponibilizado nesta execução; inspecioná-lo antes de alterar APIs.

## Ranking único

| Conceito | Regra |
| --- | --- |
| Jogador | Mesma identidade interna mapeada ao Telegram user ID em ambos os transportes |
| Regras | Mesma engine e mesma política de score existente |
| Origem | `inline` ou `webapp` como metadado, não ranking separado |
| Grupo | Identificador Telegram confirmado pelo backend; nunca confiar em chat ID enviado livremente pelo cliente |
| Resultado | Uma identidade de partida e um registro final canônico |
| Pontos | Aplicar uma vez por resultado elegível usando a transação/idempotência já existente |
| Histórico | Um resultado exibido uma vez, com modalidade, colocação e contexto |
| Notificação | Reutilizar comportamento oficial do projeto após commit confirmado |

O WebApp deve chamar o serviço de partidas e o mesmo caminho de persistência de resultados utilizado pelo Inline. Se esse caminho ainda depender do transporte, refatorar o mínimo necessário para extrair a operação comum sem modificar pontuação. Não criar uma segunda engine, contador paralelo de pontos ou worker com regra independente.

Não inferir identidade pela URL ou username. Validar a autenticação Telegram no backend usando o mecanismo suportado pelo projeto e conferir sua documentação oficial durante a implementação. A forma concreta de validação deve ser testada, incluindo payload modificado e payload expirado.

## Contexto de grupo

Uma sala criada a partir de um grupo precisa carregar vínculo autorizado no servidor. Pode-se usar uma referência opaca de sala criada pelo bot ou um convite validado, conforme a arquitetura atual. Abrir uma Mini App fora de um grupo não comprova vínculo com nenhum chat.

Partida privada pode contribuir ao ranking individual se atender às regras de elegibilidade existentes. Só contribuir ao ranking de grupo quando o backend identificar grupo válido. Não assumir que a simples presença do jogador num grupo basta; a partida deve ter contexto daquele grupo. Respeitar exclusões e preferências de privacidade já implementadas.

## Dados de interface — exemplo conceitual

```ts
type RankingView = {
  category: 'players' | 'groups';
  entries: Array<{
    entityId: string;
    position: number;
    displayName: string;
    avatarUrl?: string;
    points: string; // formato exato do backend; converter somente para exibição
    matches?: number;
  }>;
  myEntry?: { entityId: string; position: number; points: string };
  nextCursor?: string;
};
```

Pontuação monetária não existe aqui. Se o projeto representa pontos em centésimos, manter inteiros no cálculo e formatar com vírgula para a UI. Não somar floats no frontend. Os `+10,00` do protótipo são exemplos; não alterar o modo Legacy ou Updated do projeto para fazê-los coincidir.

Renderizar displayName como texto; validar avatarUrl conforme a política do projeto. Não interpolar nomes vindos da API em HTML como os fixtures locais estáticos deste protótipo. Perfis ocultos/anônimos devem continuar ocultos.

## Partida em tempo real

O HAR mostra recursos Socket.IO e mensagens WebSocket no Matcho. Isso prova o transporte observado nessa captura; não descreve nem autoriza reutilização do protocolo interno. O UnoBotGO pode usar o transporte compatível com sua arquitetura; a necessidade é sincronização confiável, não copiar Socket.IO.

Modelo conceitual para adaptar aos tipos reais:

```ts
type Command = {
  gameId: string;
  requestId: string;          // único para ação; retry reutiliza o mesmo
  expectedRevision: number;
  action: unknown;           // union tipada derivada da engine real
};
type ServerMessage =
  | { type: 'snapshot'; gameId: string; revision: number; view: unknown }
  | { type: 'event'; gameId: string; revision: number; eventId: string; event: unknown }
  | { type: 'rejected'; requestId: string; reason: string }
  | { type: 'result'; gameId: string; resultId: string; committed: boolean };
```

`unknown` indica trecho a definir com tipos reais; não é contrato de produção. Se a engine já usa IDs, revisions, eventos e snapshots, reutilizá-los. O servidor decide turno, regras, compra, timer e resultado. A UI apresenta intenção e estado confirmado.

Cada cliente recebe somente sua mão e informações públicas dos oponentes. Não enviar cartas ocultas e confiar que o CSS vai escondê-las. Usuário autenticado pode ser membro da sala sem ser jogador ativo; observar apenas quando permitido pela regra existente.

Ao reconectar, autenticar e confirmar participação, recuperar snapshot atual e ignorar eventos antigos. Gap de revision exige recuperação, não aplicar evento fora de ordem. Eventos duplicados não repetem cartas, som e pontos. Desconectar um cliente não pode cancelar arbitrariamente a partida dos demais.

## Testes materiais para integração

1. Resultado equivalente Inline e WebApp gera mesma pontuação sob a mesma regra e elegibilidade.
2. Finalização repetida, retry e reconexão não duplicam resultado, histórico ou score.
3. Chat ID forjado no cliente não atribui score a outro grupo.
4. Player ID forjado não acessa mão de outro jogador.
5. Cliente atrasado/revision antiga recebe rejeição e snapshot consistente.
6. Encerramento com dois jogadores não mantém um turno inexistente após o término.
7. Privacidade e modos de ranking existentes continuam válidos.

Esses testes pertencem ao repositório de integração. Nenhum teste de backend foi executado neste pacote visual.

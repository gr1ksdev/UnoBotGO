import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { apiGet, apiPost, type GameView, type RoomGroup } from '../api/client'
import { Icon } from './Mobile'
export function CreateRoom({ close, created }: { close: () => void; created: (game: GameView) => void }) {
 const dialog = useRef<HTMLDialogElement>(null)
 const [group, setGroup] = useState('')
 const [mode, setMode] = useState('classic')
 const [username, setUsername] = useState('')
 const [resolved, setResolved] = useState<RoomGroup>()
 const request = useRef({ key: '', id: '' })
 const groups = useQuery({ queryKey: ['room-groups'], queryFn: ({ signal }) => apiGet<RoomGroup[]>('room-groups', signal) })
 const options = [...(groups.data ?? []), ...(resolved && !groups.data?.some(g => g.ref === resolved.ref) ? [resolved] : [])]
 const selected = group || options[0]?.ref || ''
 const create = useMutation({ mutationFn: () => {
  const key = `${selected}:${mode}`
  if (request.current.key !== key) request.current = { key, id: crypto.randomUUID() }
  return apiPost<GameView>('rooms', { group_ref: selected, mode, request_id: request.current.id })
 }, onSuccess: created })
 const resolve = useMutation({ mutationFn: () => apiPost<RoomGroup>('room-groups/resolve', { username }), onSuccess: value => { setResolved(value); setGroup(value.ref) } })
 useEffect(() => { dialog.current?.showModal() }, [])
 return <dialog ref={dialog} className="room-sheet" onCancel={close} aria-labelledby="create-title">
  <header><h2 id="create-title">Criar sala</h2><button className="icon-button" onClick={close} aria-label="Fechar criação"><Icon name="back" /></button></header>
  <p className="sub">Escolha sua mesa e convide os jogadores. Você já entra no lobby.</p>
  <label>Modo de jogo<select value={mode} onChange={e => setMode(e.target.value)} disabled={create.isPending}>
   <option value="classic">Clássico</option><option value="caseiro">Caseiro · Troca de Mãos</option>
  </select></label>
  <label>Grupo vinculado<select value={selected} onChange={e => setGroup(e.target.value)} disabled={create.isPending || groups.isPending}>
   {!options.length && <option value="">{groups.isPending ? 'Buscando grupos…' : 'Vincule um grupo abaixo'}</option>}
   {options.map(g => <option value={g.ref} key={g.ref}>{g.title}</option>)}
  </select></label>
  {options.find(g => g.ref === selected) && <p className="room-policy">Ranking {options.find(g => g.ref === selected)?.system === 'legacy' ? 'Legado' : 'Atualizado'} · até 10 jogadores</p>}
  <details><summary>Vincular outro grupo público</summary>
   <p className="sub">Você e o bot precisam participar do grupo. O servidor verifica o vínculo.</p>
   <label>Username do grupo<input value={username} onChange={e => setUsername(e.target.value)} placeholder="@seu_grupo" autoComplete="off" /></label>
   <button className="outline" disabled={!username || resolve.isPending} onClick={() => resolve.mutate()}>{resolve.isPending ? 'Verificando…' : 'Verificar grupo'}</button>
   {resolve.isError && <p role="alert">Não foi possível validar esse grupo. Confira a participação sua e do bot.</p>}
  </details>
  {groups.isError && <p role="alert">Não foi possível buscar seus grupos. Vincule um grupo público ou tente novamente.</p>}
  {!groups.isPending && !options.length && <p className="notice">Adicione o bot ao grupo. Grupos privados aparecem para o instalador ou para participantes já conhecidos pelo bot.</p>}
  {create.isError && <p role="alert">Não foi possível criar. Confira o vínculo e se o grupo já possui uma sala aberta.</p>}
  <button className="primary" disabled={!selected || create.isPending} onClick={() => create.mutate()}>{create.isPending ? 'Criando…' : 'Criar e entrar'}<Icon name="arrow" /></button>
 </dialog>
}

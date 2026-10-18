export const soundFiles = {
 select: 'card_select-YPFdVw.mp3', hover: 'card_hover-braxSA.ogg',
 return: 'card_return-i9dqNQ.mp3', draw: 'card_draw_3-oTSLjQ.mp3',
 deal: 'card_deal_1-JkrG.mp3', play: 'card_play_1-nJ4MXQ.mp3',
 color: 'action_color-KzBo.mp3', reverse: 'action_reverse-kqelcA.ogg',
 skip: 'action_skip-LB_utg.ogg', picker: 'picker_show-sBRNWA.mp3',
 pick: 'picker_select-VEXBcw.mp3', win: 'player_won-SVCulg.ogg',
 lose: 'player_lost-IRBd-Q.ogg', rematch: 'ui_rematch-FQ1XIA.mp3',
} as const
export type FeedbackKind = keyof typeof soundFiles
export type FeedbackPreferences = { sound: boolean; haptics: boolean }
type Sound = Pick<HTMLAudioElement, 'play' | 'pause' | 'addEventListener' | 'removeEventListener' | 'volume'>
const storageKey = 'unobotgo-table-feedback-v1'

// Feedback never owns a game command or delays an accepted snapshot.
export class TableFeedback {
 private preferences: FeedbackPreferences = { sound: false, haptics: true }
 private listeners = new Set<() => void>()
 private playing = new Set<Sound>()
 private last = new Map<FeedbackKind, number>()
 constructor(private createSound: (src: string) => Sound = src => new Audio(src)) {
  try {
   const saved = JSON.parse(localStorage.getItem(storageKey) ?? 'null')
   if(saved) this.preferences = { sound: saved.sound === true, haptics: saved.haptics !== false }
  } catch { /* Restricted storage must not prevent opening a table. */ }
 }
 getSnapshot = () => this.preferences
 subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener) } }
 configure(change: Partial<FeedbackPreferences>) {
  this.preferences = { ...this.preferences, ...change }
  if(!this.preferences.sound) this.stop()
  try { localStorage.setItem(storageKey, JSON.stringify(this.preferences)) } catch { /* Optional persistence. */ }
  this.listeners.forEach(listener => listener())
 }
 play(kind: FeedbackKind, tactile = false) {
  if(typeof document !== 'undefined' && document.visibilityState === 'hidden') return
  const now = performance.now()
  if(now - (this.last.get(kind) ?? -Infinity) < (kind === 'hover' ? 160 : 65)) return
  this.last.set(kind, now)
  if(tactile && this.preferences.haptics) {
   const app = window.Telegram?.WebApp
   if(app?.isVersionAtLeast?.('6.1')) {
    try { app.HapticFeedback?.impactOccurred(kind === 'play' ? 'medium' : 'light') } catch { /* Unsupported client. */ }
   }
  }
  if(!this.preferences.sound) return
  // A distribution for ten players must never start seventy simultaneous sounds.
  if(this.playing.size >= 4) this.remove(this.playing.values().next().value!)
  try {
   const sound = this.createSound(`/assets/audio/${soundFiles[kind]}`)
   sound.volume = .28
   const done = () => this.remove(sound)
   sound.addEventListener('ended', done, { once: true })
   sound.addEventListener('error', done, { once: true })
   this.cleanups.set(sound, done)
   this.playing.add(sound)
   void sound.play().catch(done)
  } catch { /* Missing assets and autoplay rejection are independent of gameplay. */ }
 }
 private cleanups = new Map<Sound, () => void>()
 private remove(sound: Sound) {
  sound.pause()
  const done = this.cleanups.get(sound)
  if(done) { sound.removeEventListener('ended', done); sound.removeEventListener('error', done) }
  this.cleanups.delete(sound); this.playing.delete(sound)
 }
 stop = () => { [...this.playing].forEach(sound => this.remove(sound)); this.last.clear() }
}
export const tableFeedback = new TableFeedback()

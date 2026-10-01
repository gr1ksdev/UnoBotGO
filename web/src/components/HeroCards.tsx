import greenCard from '../assets/cards/green-zero.webp'
import yellowCard from '../assets/cards/yellow-zero.webp'
import redCard from '../assets/cards/red-zero.webp'

export function HeroCards() {
 return <span className="uno-cards" aria-hidden="true">
   <img className="uno-card card-green" src={greenCard} alt="" width={342} height={512} draggable={false} />
   <img className="uno-card card-yellow" src={yellowCard} alt="" width={342} height={512} draggable={false} />
   <img className="uno-card card-red" src={redCard} alt="" width={342} height={512} draggable={false} />
 </span>
}

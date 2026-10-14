export type Point = { x: number; y: number }
export function dragPose(delta: Point) {
 return { rotation: Math.max(-.35, Math.min(.35, delta.x * .0015)), lift: Math.min(28, Math.hypot(delta.x, delta.y) * .12) }
}
// Release requires meaningful travel toward the public discard, not a sideways scroll.
export function releasedOnDiscard(start: Point, end: Point, discard: Point & { width: number; height: number }) {
 const travel = Math.hypot(end.x - start.x, end.y - start.y)
 const toward = Math.hypot(start.x - discard.x, start.y - discard.y) - Math.hypot(end.x - discard.x, end.y - discard.y)
 const inside = Math.abs(end.x - discard.x) <= discard.width * .85 && Math.abs(end.y - discard.y) <= discard.height * .85
 return travel >= 24 && inside && toward >= 20
}

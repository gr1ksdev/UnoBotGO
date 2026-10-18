export type CardCorners = [number, number, number, number, number, number, number, number]

// Corners stay clockwise on both faces so a revealed number is never mirrored.
// The small edge width keeps PerspectiveMesh's homography invertible at 90°.
export function cardCorners(width: number, height: number, angle: number): CardCorners {
 const half = width / 2
 const cosine = Math.cos(angle)
 const edge = half * Math.max(.015, Math.abs(cosine))
 const depth = half * Math.sin(angle) * (cosine < 0 ? -1 : 1)
 const distance = width * 4
 const left = distance / (distance - depth), right = distance / (distance + depth)
 return [-edge * left, -height / 2 * left, edge * right, -height / 2 * right,
  edge * right, height / 2 * right, -edge * left, height / 2 * left]
}

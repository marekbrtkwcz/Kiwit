// generatePassword creates a random password compliant with the backend policy:
// 8–64 chars, mixed classes, crypto-random (not Math.random).
export function generatePassword(length = 20): string {
  const upper = 'ABCDEFGHJKLMNPQRSTUVWXYZ'
  const lower = 'abcdefghijkmnpqrstuvwxyz'
  const digits = '23456789'
  const special = '!@#$%^&*-_=+?'
  const all = upper + lower + digits + special
  const rand = (arr: string) => arr[crypto.getRandomValues(new Uint32Array(1))[0] % arr.length]
  // Guarantee at least one of each class, rest random.
  const chars = [rand(upper), rand(lower), rand(digits), rand(special)]
  while (chars.length < length) chars.push(rand(all))
  // Shuffle with crypto random.
  for (let i = chars.length - 1; i > 0; i--) {
    const j = crypto.getRandomValues(new Uint32Array(1))[0] % (i + 1)
    ;[chars[i], chars[j]] = [chars[j], chars[i]]
  }
  return chars.join('')
}

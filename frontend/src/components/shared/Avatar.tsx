const COLORS = [
  'var(--slate-500)',
  'var(--slate-600)',
  'var(--slate-700)',
  'var(--indigo-500)',
  'var(--indigo-600)',
  'var(--emerald-600)',
  'var(--amber-600)',
]

function colorFromName(name: string): string {
  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash += name.charCodeAt(i)
  }
  return COLORS[hash % COLORS.length]
}

interface AvatarProps {
  name: string
  size?: number
}

export function Avatar({ name, size = 28 }: AvatarProps) {
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .map(w => w[0])
    .slice(0, 2)
    .join('')
    .toUpperCase()

  const bg = colorFromName(name)
  const fontSize = Math.round(size * 0.4)

  return (
    <span
      className="avatar"
      title={name}
      style={{
        width: size,
        height: size,
        background: bg,
        fontSize,
        color: '#fff',
      }}
    >
      {initials}
    </span>
  )
}

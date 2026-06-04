export function Avatar({ name }: { name: string }) {
  const initials = name
    .split(/\s+/)
    .map(w => w[0])
    .slice(0, 2)
    .join('')
    .toUpperCase()

  return (
    <div
      title={name}
      style={{
        width: 26,
        height: 26,
        borderRadius: '50%',
        background: 'var(--color-blue-dim)',
        color: '#fff',
        fontSize: '0.7em',
        fontWeight: 700,
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
      }}
    >
      {initials}
    </div>
  )
}

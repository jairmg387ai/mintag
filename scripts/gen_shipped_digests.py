"""Regenerate internal/skillinstall/shipped_digests.go from the git history of the bundled skills.

Run from the repository root: python scripts/gen_shipped_digests.py
The digest algorithm must match digestFS in internal/skillinstall/sync.go.
"""
import hashlib, os, subprocess, sys
BASE = 'internal/skillinstall/bundled/skills'
def git(*a): return subprocess.run(['git', *a], capture_output=True, check=True).stdout
def digest(files):  # files: {rel: bytes}
    h = hashlib.sha256()
    for rel in sorted(files):
        h.update(rel.encode()); h.update(b'\0')
        h.update(hashlib.sha256(files[rel].replace(b'\r\n', b'\n')).digest())
    return 'sha256:' + h.hexdigest()
skills = set(git('log', '--all', '--format=', '--name-only', '--', BASE).decode().split())
names = sorted({p.split('/')[4] for p in skills if p.startswith(BASE + '/') and len(p.split('/')) > 5})
names = sorted(set(names) | set(os.listdir(BASE)))
out = {}
for name in names:
    d = f'{BASE}/{name}'
    ds = set()
    for c in git('log', '--format=%H', 'HEAD', '--', d).decode().split():
        listing = git('ls-tree', '-r', '--name-only', c, '--', d + '/').decode().split('\n')
        files = {p[len(d)+1:]: git('show', f'{c}:{p}') for p in listing if p and p.count('/') == d.count('/') + 1}
        if files: ds.add(digest(files))
    if os.path.isdir(d):
        files = {f: open(os.path.join(d, f), 'rb').read() for f in os.listdir(d) if os.path.isfile(os.path.join(d, f))}
        ds.add(digest(files))
    out[name] = sorted(ds)
lines = ['// Code generated from git history of internal/skillinstall/bundled/skills; DO NOT EDIT.',
         '// Regenerate with `python scripts/gen_shipped_digests.py` when a bundled skill changes.', '',
         'package skillinstall', '', '// shippedDigests lists every content digest Mintag has shipped per skill.',
         '// Sync treats a copy matching one of them as Mintag-owned.', 'var shippedDigests = map[string][]string{']
for name, ds in out.items():
    lines.append(f'\t"{name}": {{'); lines += [f'\t\t"{x}",' for x in ds]; lines.append('\t},')
lines.append('}')
open('internal/skillinstall/shipped_digests.go', 'w', newline='\n').write('\n'.join(lines) + '\n')
print({k: len(v) for k, v in out.items()})

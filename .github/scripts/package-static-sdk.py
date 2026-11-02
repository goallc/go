#!/usr/bin/env python3
"""Create the standard Go archive and preserve static dependency provenance."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--root', type=Path, required=True)
p.add_argument('--llvm', type=Path, required=True)
p.add_argument('--version', required=True)
p.add_argument('--output', type=Path, required=True)
a = p.parse_args()
root, llvm, output = a.root.resolve(), a.llvm.resolve(), a.output.resolve()
if not re.fullmatch(r'go[0-9][0-9A-Za-z.\-]*', a.version):
    p.error('version must be a Go version name, starting with go and a digit')
manifest = dict(line.split('=', 1) for line in (llvm/'share/goallc/build-manifest').read_text().splitlines() if '=' in line)
if manifest.get('dependency_mode') != 'bundled-static':
    p.error('the LLVM payload must contain bundled static dependencies')
licenses = llvm/'share/licenses'
for name in ('goallc-deps/zlib.txt', 'goallc-deps/zstd.txt', 'llvm.txt'):
    if not (licenses/name).is_file():
        p.error('missing dependency license: ' + name)
meta = root/'lib/goallc'
meta.mkdir(parents=True, exist_ok=True)
shutil.copytree(licenses, meta/'licenses', dirs_exist_ok=True)
revision = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
metadata = {'format': 1, 'go_revision': revision,
            'go_dirty': bool(subprocess.check_output(['git', '-C', str(root), 'status', '--porcelain', '--untracked-files=no'], text=True).strip()),
            'link_mode': 'static',
            'llvm': {k: manifest[k] for k in ('llvm_revision', 'llvm_dirty', 'llvm_version', 'dependency_mode', 'zlib', 'zstd')}}
(meta/'build-manifest.json').write_text(json.dumps(metadata, indent=2)+'\n')
go = root/'bin'/('go.exe' if os.name == 'nt' else 'go')
env = dict(os.environ, GOROOT=str(root), GOTOOLCHAIN='local', GOENV='off')
actual_version = subprocess.check_output([go, 'env', 'GOVERSION'], env=env, text=True).strip()
if actual_version != a.version or (root/'VERSION').read_text().splitlines()[0] != a.version:
    p.error('set VERSION before bootstrap; binary version is ' + actual_version)
subprocess.run([go, 'run', 'cmd/distpack'], env=env, cwd=root, check=True)
goos, goarch = subprocess.check_output([go, 'env', 'GOHOSTOS', 'GOHOSTARCH'], env=env, text=True).split()
ext = '.zip' if goos == 'windows' else '.tar.gz'
archive = root/'pkg/distpack'/f'{a.version}.{goos}-{goarch}{ext}'
output.mkdir(parents=True, exist_ok=True)
shutil.copy2(archive, output/archive.name)
digest = hashlib.sha256()
with archive.open('rb') as stream:
    for block in iter(lambda: stream.read(1024 * 1024), b''):
        digest.update(block)
sha = digest.hexdigest()
(output/(archive.name+'.sha256')).write_text(f'{sha}  {archive.name}\n')
print(output/archive.name)

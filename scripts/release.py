#!/usr/bin/env python3
"""Build and package Mosaic for macOS, Windows, and Linux (amd64 and arm64)."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import os
from pathlib import Path
import re
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parents[1]
TARGETS = [(system, arch) for system in ('darwin', 'windows', 'linux')
           for arch in ('amd64', 'arm64')]
FILES = ['LICENSE', 'THIRD_PARTY_NOTICES', 'README.md', 'docs/telemetry.md',
         'docs/validation.md', 'docs/terminals.md', 'docs/chafa-logos.png',
         'docs/interaction-demo.gif', 'assets/fonts/LICENSE.txt',
         'assets/fonts/README.md', 'assets/fonts/FiraCode-Regular.ttf',
         'assets/fonts/FiraCode-Bold.ttf']
LICENSES = {'internal/ui/osart/LICENSE': 'licenses/fastfetch-MIT.txt',
            'internal/ui/osart/SIMPLE-ICONS-LICENSE.md': 'licenses/simple-icons-CC0.md'}


def package(version, target):
    system, arch = target
    label = 'macos' if system == 'darwin' else system
    executable = 'mosaic.exe' if system == 'windows' else 'mosaic'
    binary = ROOT / 'dist' / f'mosaic-{system}-{arch}{".exe" if system == "windows" else ""}'
    env = dict(os.environ, CGO_ENABLED='0', GOOS=system, GOARCH=arch)
    subprocess.run(['go', 'build', '-trimpath', '-ldflags', f'-s -w -X main.version={version}',
                    '-o', str(binary), './cmd/mosaic'], cwd=ROOT, env=env, check=True)
    archive = ROOT / 'dist' / f'mosaic-{label}-{arch}-{version}.zip'
    command = '.\\mosaic.exe --color' if system == 'windows' else './mosaic --color'
    instructions = f'''Mosaic {version} ({label} {arch})

Extract the archive, open your terminal in this folder, and run:
  {command}

Use a 120-column by 30-row terminal for the standard dashboard. Fira Code
is included for joined meters: select it in your terminal and use --fira.
The choice is remembered when you exit. --meters portable switches back.
Portable meters are the initial default for other fonts.
Chafa is not required. See README.md for controls, docs/telemetry.md for
hardware support, and docs/interaction-demo.gif for the interactive demo.

Arrows select; Enter expands; f pops out; Esc returns or docks.
Drag panel edges to resize and titles to rearrange. H hides a grid panel.
Use o to enable, disable, and reorder panels. q exits.

These binaries are unsigned. macOS binaries are not notarized.
'''
    with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED) as output:
        output.write(binary, executable)
        for name in FILES:
            output.write(ROOT / name, name)
        for source, destination in LICENSES.items():
            output.write(ROOT / source, destination)
        if system == 'windows':
            helper = ROOT / 'dist' / f'sensors-{arch}'
            if not (helper / 'Mosaic.SensorHelper.exe').is_file():
                raise RuntimeError('Build the bundled sensor helper before packaging')
            for path in sorted(helper.rglob('*')):
                if path.is_file():
                    output.write(path, 'sensors/' + path.relative_to(helper).as_posix())
            output.write(ROOT / 'docs/windows-sensors.md', 'docs/windows-sensors.md')
            instructions += "\nKeep the sensors folder next to mosaic.exe. Its runtime is included.\nNo sensor driver is installed. GPU sensors use existing vendor drivers;\nCPU temperature/power need an already available hardware-sensor provider.\n"
        output.writestr('RUN.txt', instructions)
    with zipfile.ZipFile(archive) as output:
        if output.testzip() is not None or output.read(executable) != binary.read_bytes():
            raise RuntimeError(f'Archive verification failed: {archive.name}')
    archive.chmod(0o644)
    print(archive.name, flush=True)
    return f'{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True, help='Release version, such as v0.1.0')
    args = parser.parse_args()
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]*', args.version):
        parser.error('version must contain only letters, digits, dots, underscores, and hyphens')
    (ROOT / 'dist').mkdir(exist_ok=True)
    from build_sensors import build
    for arch in ('amd64', 'arm64'):
        build(arch, ROOT / 'dist' / f'sensors-{arch}')
    with ThreadPoolExecutor(max_workers=3) as executor:
        checksums = list(executor.map(lambda target: package(args.version, target), TARGETS))
    (ROOT / 'dist' / f'SHA256SUMS-{args.version}.txt').write_text('\n'.join(checksums) + '\n')


if __name__ == '__main__':
    main()

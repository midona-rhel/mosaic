#!/usr/bin/env python3
"""Generate compact OS text with Chafa 1.14.5 and Fira Code Bold.

Sources are pinned Simple Icons 11.0.0 vectors. The app embeds the results;
neither Chafa nor image rendering is needed at runtime.
"""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile
import cairosvg

ROOT = Path(__file__).resolve().parents[1] / 'internal/ui/osart'
FONT = ROOT.parents[2] / 'assets/fonts/FiraCode-Bold.ttf'
ICONS = {
    'macos': 'apple', 'windows_11': 'windows11', 'arch': 'archlinux',
    'ubuntu': 'ubuntu', 'fedora': 'fedora', 'linuxmint': 'linuxmint',
    'debian': 'debian', 'alpine': 'alpinelinux', 'nixos': 'nixos',
    'gentoo': 'gentoo', 'manjaro': 'manjaro', 'opensuse': 'opensuse',
    'rhel': 'redhat', 'pop': 'popos',
}


def generate(names):
    catalog = json.loads((ROOT / 'compact.json').read_text())
    version = subprocess.check_output(['chafa', '--version'], text=True)
    if 'Chafa version 1.14.5\n' not in version:
        raise SystemExit('Regeneration requires Chafa 1.14.5')
    with tempfile.TemporaryDirectory() as work:
        for name in names:
            icon = ICONS[name]
            path = ROOT / ('apple.svg' if icon == 'apple' else f'icons/{icon}.svg')
            svg = path.read_text().replace('<path ', '<path fill="white" ')
            source = Path(work) / f'{icon}.png'
            cairosvg.svg2png(bytestring=svg.encode(), write_to=str(source),
                            output_width=768, output_height=768, background_color='black')
            variants = []
            for height in (8, 7, 6):
                output = subprocess.check_output([
                    'chafa', '-f', 'symbols', '--symbols', 'ascii',
                    '--glyph-file', str(FONT), '-c', 'none', '--fg-only',
                    '--size', f'{height * 2}x{height}', '--stretch', '--work', '9',
                    str(source)], text=True)
                rows = [line.rstrip() for line in output.splitlines()]
                # Trim padding only; leave all generated characters intact.
                margin = min(len(line) - len(line.lstrip()) for line in rows if line.strip())
                rows = [line[margin:] for line in rows]
                assert len(rows) == height and all(line.isascii() for line in rows)
                variants.append(rows)
            catalog[name] = variants
    (ROOT / 'compact.json').write_text(json.dumps(catalog, indent=2) + '\n')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--os', choices=ICONS, help='Regenerate one OS instead of all')
    args = parser.parse_args()
    generate([args.os] if args.os else ICONS)

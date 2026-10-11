#!/usr/bin/env python3
"""Preview every compact Chafa asset at the terminal capture's actual cell size."""
import json
from PIL import Image, ImageDraw
from terminal_render import ROOT, BACKGROUND, BOLD, SMALL, CELL_W, CELL_H

NAMES = {
    'macos': 'macOS', 'windows_11': 'Windows', 'arch': 'Arch Linux',
    'ubuntu': 'Ubuntu', 'fedora': 'Fedora', 'linuxmint': 'Linux Mint',
    'debian': 'Debian', 'alpine': 'Alpine Linux', 'nixos': 'NixOS',
    'gentoo': 'Gentoo', 'manjaro': 'Manjaro', 'opensuse': 'openSUSE',
    'rhel': 'Red Hat', 'pop': 'Pop!_OS',
}
art = json.loads((ROOT / 'internal/ui/osart/compact.json').read_text())
colors = ['#538FFF', '#6285FF', '#7977FF', '#9467FF', '#AC5EFF', '#C65AE6']
tile_w, tile_h, columns = 240, 260, 4
picture = Image.new('RGB', (columns * tile_w, ((len(NAMES)+columns-1)//columns)*tile_h+46), BACKGROUND)
draw = ImageDraw.Draw(picture)
draw.text((20, 14), 'MOSAIC / Chafa / eight-row terminal text / Fira Code', font=SMALL, fill='#aaa2bf')
for i, (name, title) in enumerate(NAMES.items()):
    x, y = i % columns * tile_w, i // columns * tile_h + 46
    draw.text((x+20, y+8), title, font=SMALL, fill='#e9e8f5')
    rows = art[name][0]
    left = x + (tile_w-max(map(len, rows))*CELL_W)/2
    for j, line in enumerate(rows):
        draw.text((left, y+60+j*CELL_H), line, font=BOLD,
                  fill=colors[min(len(colors)-1, j*len(colors)//len(rows))],
                  anchor='ls', features=['calt', 'liga'])
picture.save(ROOT / 'docs/chafa-logos.png')

#!/usr/bin/env python3
"""Turn actual --snapshot ANSI output into SVGs (requires requirements-dev.txt)."""
import html
import os
import pathlib
import subprocess
import pyte
from terminal_render import render_screen
from terminal_screen import TerminalScreen

ROOT = pathlib.Path(__file__).resolve().parents[1]


def render(width, height, name, panel="", fira=True):
    output = subprocess.check_output(
        [str(ROOT / "bin/mosaic"), "--demo", "--snapshot", "--color",
         "--width", str(width), "--height", str(height), "--panel", panel], text=True,
        env=dict(os.environ, UNICODE_PROGRESS_BAR="true" if fira else "false")
    )
    screen = TerminalScreen(width, height)
    pyte.Stream(screen).feed(output.rstrip("\n").replace("\n", "\r\n"))
    for y in range(height):
        for x in range(width):
            assert screen.buffer[y][x].bg != 'default', f'Unpainted terminal background at {x},{y}'
            assert screen.buffer[y][x].bg != '06050b', 'Removed drop shadow still appears'
    cell_w, cell_h, left, top = 9, 19, 24, 54
    sw, sh = width * cell_w + 48, height * cell_h + 72
    parts = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{sw}" height="{sh}" viewBox="0 0 {sw} {sh}">',
             '<title>Mosaic terminal dashboard — explicitly labeled demo data</title>',
             f'<rect width="{sw}" height="{sh}" rx="12" fill="#100e19"/>',
             f'<path d="M12 0H{sw-12}Q{sw} 0 {sw} 12V36H0V12Q0 0 12 0" fill="#201b2f"/>']
    for x, color in [(20, "#fb7185"), (38, "#fbbf24"), (56, "#86efac")]:
        parts.append(f'<circle cx="{x}" cy="18" r="5" fill="{color}"/>')
    parts.append(f'<text x="{sw//2}" y="23" text-anchor="middle" fill="#94a3b8" font-family="monospace" font-size="12">mosaic · {width} × {height} · demo</text>')
    parts.append('<g font-family="Fira Code,DejaVu Sans Mono,Consolas,monospace" font-size="15" xml:space="preserve" style="font-variant-ligatures:contextual;font-feature-settings:\'calt\' 1">')
    for y in range(height):
        x = 0
        while x < width:
            cell = screen.buffer[y][x]
            start, style = x, (cell.fg, cell.bg, cell.bold)
            text = ""
            while x < width:
                c = screen.buffer[y][x]
                if (c.fg, c.bg, c.bold) != style:
                    break
                text += c.data
                x += 1
            fg, bg, bold = style
            fg = "#" + fg if len(fg) == 6 else "#e9e8f5"
            bg = "#" + bg if len(bg) == 6 else "#100e19"
            px, py, length = left + start * cell_w, top + y * cell_h, (x-start) * cell_w
            if bg != "#100e19":
                parts.append(f'<rect x="{px}" y="{py-15}" width="{length}" height="19" fill="{bg}"/>')
            if text.strip():
                weight = ' font-weight="bold"' if bold else ""
                parts.append(f'<text x="{px}" y="{py}" fill="{fg}" textLength="{length}" lengthAdjust="spacingAndGlyphs"{weight}>{html.escape(text)}</text>')
    parts.append('</g></svg>')
    destination = ROOT / "docs" / name
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text("\n".join(parts) + "\n")
    png=destination.with_suffix('.png')
    render_screen(screen,f"mosaic / {width} x {height} / Fira Code / demo").save(png)
    print(destination.relative_to(ROOT))
    print(png.relative_to(ROOT))


if __name__ == "__main__":
    render(120, 30, "preview.svg")
    render(80, 24, "preview-80x24.svg")
    render(120, 30, "preview-portable.svg", fira=False)
    for width,height in [(120,30),(80,24)]:
        for panel in ["system","cpu","memory","disks","processes","network","gpu","disk-activity","battery","thermals","connections"]:
            render(width,height,f"panels/{panel}-{width}x{height}.svg",panel)
    from PIL import Image, ImageDraw
    for width,height in [(120,30),(80,24)]:
        files = [ROOT/"docs"/f"preview{'-80x24' if width==80 else ''}.png"]
        files += [ROOT/"docs"/f"panels/{panel}-{width}x{height}.png" for panel in ["system","cpu","memory","disks","processes","network","gpu","disk-activity","battery","thermals","connections"]]
        images=[Image.open(p).convert("RGB") for p in files]
        iw,ih=images[0].size
        sheet=Image.new("RGB",(iw*2,ih*((len(images)+1)//2)),"#100e19")
        for i,img in enumerate(images):sheet.paste(img,((i%2)*iw,(i//2)*ih))
        sheet.save(ROOT/"docs"/f"panels-{width}x{height}.png")

"""Render real terminal cells with Fira Code and contextual ligatures.

Used only to capture previews, not to render the application itself.
"""
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont, features

# Windows Pillow wheels may omit libraqm; block glyphs still render normally.
TEXT_FEATURES = {"features": ["calt", "liga"]} if features.check_feature("raqm") else {}

ROOT = Path(__file__).resolve().parents[1]
BACKGROUND = "#100e19"
INK = "#e9e8f5"
# Fira Code advances 1200/1950 em per cell: 19.5px gives exactly 12px.
# At 18px (11.078px/cell), rasterized progress and block glyphs develop dark
# seams as fractional advances round differently across a run.
REGULAR = ImageFont.truetype(str(ROOT / "assets/fonts/FiraCode-Regular.ttf"), 19.5)
BOLD = ImageFont.truetype(str(ROOT / "assets/fonts/FiraCode-Bold.ttf"), 19.5)
SMALL = ImageFont.truetype(str(ROOT / "assets/fonts/FiraCode-Regular.ttf"), 13)
CELL_W = REGULAR.getlength("M")
assert CELL_W.is_integer(), "Capture fonts must use whole-pixel cell widths"
# The font's full block spans -18..+6 at this size. Match that exact row
# height so partial history glyphs join the next row without horizontal seams.
CELL_H = 24
CELL_ASCENT = 18
LEFT, TOP = 24, 58


def color(value, default):
    if len(value) == 6 and all(c in "0123456789abcdefABCDEF" for c in value):
        return "#" + value
    return default


def render_screen(screen, title="Mosaic / Fira Code / demo", cursor=None, click=False, caption=""):
    width = round(screen.columns * CELL_W) + LEFT * 2
    height = screen.lines * CELL_H + TOP + 28 + (40 if caption else 0)
    image = Image.new("RGB", (width, height), BACKGROUND)
    draw = ImageDraw.Draw(image)
    draw.rounded_rectangle((0, 0, width-1, height-1), radius=12, fill=BACKGROUND)
    draw.rectangle((0, 0, width, 34), fill="#201b2f")
    for x, fill in [(19, "#ff456d"), (37, "#b45cff"), (55, "#538fff")]:
        draw.ellipse((x-5, 13, x+5, 23), fill=fill)
    draw.text((width/2, 18), title, font=SMALL, fill="#aaa2bf", anchor="mm", **TEXT_FEATURES)
    # Paint all cell backgrounds first; Fira Code's box glyphs can overlap rows.
    for y in range(screen.lines):
        for x in range(screen.columns):
            c = screen.buffer[y][x]
            bg = color(c.bg, BACKGROUND)
            if bg != BACKGROUND:
                px, py = LEFT+x*CELL_W, TOP+y*CELL_H
                draw.rectangle((px, py-CELL_ASCENT, px+CELL_W-1, py+CELL_H-CELL_ASCENT-1), fill=bg)
    for y in range(screen.lines):
        x = 0
        while x < screen.columns:
            c = screen.buffer[y][x]
            start, style, text = x, (c.fg, c.bold), ""
            while x < screen.columns:
                c = screen.buffer[y][x]
                if (c.fg, c.bold) != style:
                    break
                text += c.data
                x += 1
            if text.strip():
                draw.text((LEFT+start*CELL_W, TOP+y*CELL_H), text,
                          font=BOLD if style[1] else REGULAR, fill=color(style[0], INK),
                          anchor="ls", **TEXT_FEATURES)
    if cursor:
        x, y = LEFT+cursor[0]*CELL_W, TOP+cursor[1]*CELL_H-12
        if click:
            draw.ellipse((x-13,y-13,x+13,y+13), outline="#b45cff", width=3)
        draw.polygon([(x,y),(x+1,y+21),(x+6,y+16),(x+11,y+26),(x+15,y+24),(x+10,y+14),(x+18,y+13)],
                     fill="#f8f5ff", outline="#100e19", width=2)
    if caption:
        draw.rectangle((0,height-40,width,height), fill="#201b2f")
        draw.text((24,height-21), "Recorded input -> " + caption, font=SMALL,
                  fill="#d3c6ee", anchor="lm", **TEXT_FEATURES)
    return image

#!/usr/bin/env python3
"""Capture real System rendering with labeled OS fixtures, not native OS tests."""
import json, pathlib, subprocess, sys
import pyte
from PIL import Image, ImageDraw
from terminal_screen import TerminalScreen
from terminal_render import render_screen, SMALL, CELL_W, CELL_H, LEFT, TOP, CELL_ASCENT

ROOT=pathlib.Path(__file__).resolve().parents[1]
cards = '--cards' in sys.argv
mode = '-card' if cards else ''
frames=json.loads(subprocess.check_output(['go','run','scripts/render_os_preview.go']+(['--cards'] if cards else []),cwd=ROOT))
tiles=[]
for frame in frames:
 w,h=frame['Width'],frame['Height']
 screen=TerminalScreen(w,h)
 pyte.Stream(screen).feed(frame['Content'].replace('\n','\r\n'))
 name=frame['Platform']
 image=render_screen(screen,f'Mosaic / {name} / {w} x {h} / demo fixture')
 image.save(ROOT/'docs/panels'/f'system-{name.lower()}{mode}-{w}x{h}.png')
 if w==120:
  # Use the only border spanning >45 columns: the centered System detail view.
  candidates=[]
  for y,line in enumerate(screen.display):
   for x,ch in enumerate(line):
    if ch=='╭':
     end=line.find('╮',x)
     if end-x>45 and set(line[x+1:end])=={'─'}: candidates.append((y,x,end))
  top,left,right=candidates[0]
  bottom=next(y for y in range(top+1,h) if screen.display[y][left]=='╰')
  crop=image.crop((LEFT+left*CELL_W,TOP+top*CELL_H-CELL_ASCENT,LEFT+(right+1)*CELL_W,TOP+(bottom+1)*CELL_H-CELL_ASCENT))
  crop.save(ROOT/'docs/panels'/f'system-{name.lower()}{mode}-crop.png')
  tiles.append((name,crop))
tw=max(im.width for _,im in tiles)+24;th=max(im.height for _,im in tiles)+54
sheet=Image.new('RGB',(tw*2,th*((len(tiles)+1)//2)),'#100e19');d=ImageDraw.Draw(sheet)
for i,(name,im) in enumerate(tiles):
 x=(i%2)*tw;y=(i//2)*th
 d.text((x+16,y+10),name+' / reference artwork / demo',font=SMALL,fill='#aaa2bf')
 sheet.paste(im,(x+(tw-im.width)//2,y+38))
sheet.save(ROOT/'docs'/f'os-artwork{mode}.png')
print(f'docs/os-artwork{mode}.png and {len(frames)} OS fixture captures')

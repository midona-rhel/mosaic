#!/usr/bin/env python3
"""Record real SGR mouse input against Mosaic into Fira Code PNGs and a GIF.

The pointer/pulse and bottom input caption are recording annotations; all TUI
content is reconstructed from the running binary's terminal output.
"""
import fcntl
import os
import pty
import select
import struct
import subprocess
import termios
import time
from pathlib import Path
import pyte
from terminal_render import render_screen
from terminal_screen import TerminalScreen

ROOT=Path(__file__).resolve().parents[1]
DEST=ROOT/'docs'


def run():
    build=subprocess.check_output([str(ROOT/'bin/mosaic'),'--version'], text=True).strip()
    master,slave=pty.openpty()
    fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',30,120,0,0))
    screen=TerminalScreen(120,30)
    stream=pyte.ByteStream(screen)
    process=subprocess.Popen([str(ROOT/'bin/mosaic'),'--demo','--color','--interval','10s'],
                             stdin=slave,stdout=slave,stderr=slave,
                             env=dict(os.environ,TERM='xterm-256color',COLORTERM='truecolor',UNICODE_PROGRESS_BAR='true'))
    frames,durations=[],[]
    pointer=None

    def pump(seconds=.12):
        until=time.monotonic()+seconds
        while time.monotonic()<until:
            ready,_,_=select.select([master],[],[],min(.04,max(0,until-time.monotonic())))
            if ready:
                chunk=os.read(master,65536)
                stream.feed(chunk)
                if b'\x1b[6n' in chunk:os.write(master,b'\x1b[1;1R')

    def save(caption,duration=450,click=False,name=None):
        if name:
            for y in range(screen.lines):
                for x in range(screen.columns):
                    cell=screen.buffer[y][x]
                    assert cell.bg != 'default', f'Unpainted background at {x},{y} in {name}'
                    assert cell.bg != '06050b', f'Removed shadow color still present in {name}'
        image=render_screen(screen,f'{build} / 120 x 30 / demo data',pointer,click,caption)
        frames.append(image);durations.append(duration)
        if name:image.save(DEST/name)

    def event(kind,x,y):
        nonlocal pointer
        pointer=(x,y)
        button={'press':0,'motion':32,'hover':35,'release':0}[kind]
        suffix='m' if kind=='release' else 'M'
        os.write(master,f'\x1b[<{button};{x+1};{y+1}{suffix}'.encode())
        pump()

    def click(x,y,caption):
        event('hover',x,y);save(caption,200)
        event('press',x,y);save(caption,350,True)
        event('release',x,y);save(caption,450)

    def drag(x,y,endx,endy,caption,steps=6,preview=None):
        event('press',x,y);save(caption,200,True)
        for i in range(1,steps+1):
            xx=round(x+(endx-x)*i/steps);yy=round(y+(endy-y)*i/steps)
            event('motion',xx,yy);save(caption,130,True)
        if preview:save(caption,700,name=preview)
        event('release',endx,endy);save(caption,500)

    def key(keys,caption):
        os.write(master,keys);pump(.2);save(caption,650)

    def text():
        return '\n'.join(screen.display)

    try:
        pump(.7)
        assert 'MOSAIC' in text() and '8 DISK I/O' in text()
        save('120 x 30 / live application with demonstration data',1400)
        click(7,5,'Select System')
        key(b'\x1b[1;2C','Shift+Right / widen System by one grid column')
        assert 'Architecture' in text()
        save('Chafa OS artwork in the widened System panel',1300,name='interaction-system.png')
        key(b'\x1b[1;2D','Shift+Left / shrink System')
        drag(58,8,88,8,'Drag CPU edge / grow width by one grid column')
        drag(50,13,50,26,'Drag CPU bottom edge / grow height by one grid row')
        assert 'PER CORE' in text()
        save('CPU at 2 x 2 / displaced panels move to Hidden',1200,name='interaction-resize.png')
        key(b'o','Open panel manager / see hidden panels')
        assert 'PANELS' in text() and 'Hidden' in text()
        save('Panel manager / enabled, hidden, and removed panels',1200)
        key(b'\x1b','Close panel manager')
        key(b'0','Reset grid sizes and order')
        click(38,6,'Select CPU')
        key(b'H','Shift+H / hide the selected panel')
        key(b'o','Open panel manager to restore CPU')
        key(b'2','Select CPU in the panel manager')
        assert any('2 CPU' in row and 'Removed' in row for row in screen.display)
        save('CPU is disabled / its saved priority is retained',1100)
        key(b' ','Space / enable CPU again')
        key(b'\x1b','Return to the dashboard')
        assert '2 CPU' in text()
        click(38,6,'Select CPU')
        key(b'\r','Enter / expand CPU details')
        assert 'Total utilization' in text()
        save('Expanded CPU / utilization, temperature, power and cores',1500,name='interaction-expanded.png')
        key(b'\x1b','Esc / return to the grid')
        click(55,3,'Click the CPU pop-out button')
        assert ':: CPU' in text()
        row=next(y for y,line in enumerate(screen.display) if ':: CPU' in line)
        col=screen.display[row].index(':: CPU')+3
        drag(col,row,col+12,row-2,'Drag the pop-out / its contents follow the pointer')
        assert ':: CPU' in text() and 'Total utilization' in text()
        save('Floating CPU / independent window and controls',1300,name='interaction-popout.png')
        key(b'\x1b','Esc / dock the CPU window')
        assert ':: CPU' not in text()
        drag(39,3,70,7,'Drag CPU title onto Memory / rearrange the grid')
        assert 'MEMORY' in screen.display[3][31:59] and 'CPU' in screen.display[3][61:89]
        save('Rearranged panels / order follows the drop',1200)
        key(b'0','Reset to the default grid')
        save('Mosaic / back to the dashboard',1800)
        os.write(master,b'q')
        pump(.15)
        assert process.wait(timeout=5)==0
        palette=frames[0].quantize(colors=192)
        gif_frames=[frame.quantize(palette=palette) for frame in frames]
        gif_frames[0].save(DEST/'interaction-demo.gif',save_all=True,
                           append_images=gif_frames[1:],duration=durations,
                           loop=0,optimize=False,disposal=2)
        print(f'Captured {len(frames)} real PTY frames, {sum(durations)/1000:.1f}s -> docs/interaction-demo.gif')
    finally:
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        os.close(master)
        os.close(slave)


if __name__=='__main__':
    run()

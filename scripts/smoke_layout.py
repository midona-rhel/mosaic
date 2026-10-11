#!/usr/bin/env python3
"""Real PTY checks for outline commit/cancel and a two-column hidden layout."""
import fcntl
import os
import pty
import select
import struct
import subprocess
import termios
import time

import pyte
from terminal_screen import TerminalScreen

master, slave = pty.openpty()
screen = TerminalScreen(55, 20)
stream = pyte.ByteStream(screen)
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 20, 55, 0, 0))
p = subprocess.Popen(['./bin/mosaic','--demo','--color','--interval','10s'],
                     stdin=slave,stdout=slave,stderr=slave,
                     env=dict(os.environ,TERM='xterm-256color',COLORTERM='truecolor'))

def pump(seconds=.22):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if select.select([master],[],[],.03)[0]:
            data=os.read(master,65536)
            stream.feed(data)
            if b'\x1b[6n' in data:os.write(master,b'\x1b[1;1R')

def key(data):os.write(master,data);pump()
def mouse(button,x,y,release=False):
    key(f'\x1b[<{button};{x+1};{y+1}{"m" if release else "M"}'.encode())
def text():return '\n'.join(screen.display)
def capture(name,caption):
    if os.getenv('MOSAIC_CAPTURE_LAYOUT')=='1':
        from terminal_render import render_screen
        render_screen(screen,'mosaic / 55 x 20 / recorded demo',None,False,caption).save('docs/'+name)

try:
    pump(.8)
    assert 'MOSAIC' in text() and 'o panels' in text()
    key(b'\x1b[B')
    assert '› 3 MEMORY' in text()
    key(b'\x1b[1;2C') # Grow the bottom-left card to both columns.
    assert 'DISKS' not in text().split('o panels')[0]
    original=screen.display[3][2:-2]
    mouse(0,5,10)
    mouse(32,5,2)
    assert 'Drag edge' not in text()
    assert screen.display[2][1:54] == '╭' + '─'*51 + '╮', 'preview top border has gaps'
    assert all(screen.buffer[2][x].fg == 'e9e8f5' for x in range(1,54)), 'preview is not white'
    assert screen.display[3][2:-2]==original, 'preview moved the committed cards'
    capture('thin-resize-preview-fira.png','Two columns / grow Memory upward / destination outline')
    key(b'\x1b')
    assert 'Memory 2×2' not in text() and screen.display[3][2:-2]==original
    mouse(0,5,10);mouse(32,5,2);mouse(0,5,2,True)
    assert '› 3 MEMORY' in screen.display[3] and '1 SYSTEM' not in text(), text()
    assert 'Hidden:' in text()
    capture('thin-grid-fira.png','Memory keeps priority / displaced panels are hidden, not resized')
    key(b'o')
    assert 'Hidden' in text() and 'PANELS' in text()
    key(b'7');key(b'\r')
    assert '› 7 GPU' in screen.display[3], 'hidden panel did not return at first priority'
    key(b'0');key(b'f')
    assert ':: System' in text()
    y=next(y for y,row in enumerate(screen.display) if ':: System' in row)
    x=screen.display[y].index(':: System')
    mouse(0,x+3,y);mouse(32,x+4,y+1)
    assert ':: System' in text() and 'Architecture' in text(), 'drag lost the live window body'
    mouse(0,x+4,y+1,True)
    assert ':: System' in text() and 'Release to place' not in text()
    key(b'q')
    assert p.wait(timeout=5)==0
    print('Layout PTY passed: grid outline / live floating drag, cancel/commit, two-column displacement, hidden-panel recovery')
finally:
    if p.poll() is None:p.terminate();p.wait(timeout=5)
    os.close(master);os.close(slave)

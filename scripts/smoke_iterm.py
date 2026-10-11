#!/usr/bin/env python3
"""Check emitted iTerm controls in a PTY; not a native iTerm appearance test."""
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
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
screen = TerminalScreen(120, 30)
stream = pyte.ByteStream(screen)
transcript = bytearray()
process = subprocess.Popen(['./bin/mosaic', '--demo', '--color', '--interval', '10s'],
                           stdin=slave, stdout=slave, stderr=slave,
                           env=dict(os.environ, TERM='xterm-256color', TERM_PROGRAM='iTerm.app',
                                    UNICODE_PROGRESS_BAR='true'))

def pump(seconds=.3):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        if select.select([master], [], [], .03)[0]:
            chunk = os.read(master, 65536)
            transcript.extend(chunk)
            stream.feed(chunk)
            if b'\x1b[6n' in chunk:
                os.write(master, b'\x1b[1;1R')

def hover(x, y, shape):
    start = len(transcript)
    os.write(master, f'\x1b[<35;{x+1};{y+1}M'.encode())
    pump()
    assert f'\x1b]22;{shape}\x07'.encode() in transcript[start:], shape

try:
    pump(.8)
    assert 'MOSAIC' in '\n'.join(screen.display)
    assert b'\x1b]1337;HighlightCursorLine=0\x07' in transcript
    assert b'\x1b[?1049h\x1b]22;arrow\x07' in transcript
    assert any(0xee00 <= ord(c) <= 0xee05 for row in screen.display for c in row)
    y, row = next((y, row) for y, row in enumerate(screen.display) if '↗' in row)
    hover(row.index('↗'), y, 'hand2')
    y, row = next((y, row) for y, row in enumerate(screen.display) if '╭' in row)
    x = row.index('╭')
    hover(x, y+3, 'arrow')
    assert screen.buffer[y+4][x].fg == '73d2ff'
    hover(row.index('╭')+10, y+1, 'fleur')
    hover(x+5, y+5, 'arrow')
    assert screen.buffer[y+4][x].fg != '73d2ff'
    os.write(master, b'q')
    assert process.wait(timeout=5) == 0
    pump(.1)
    assert b'\x1b]22;\x07\x1b[?1049l' in transcript
    print('iTerm protocol smoke passed: legacy arrow/hand/move, edge color, guide suppression, Fira glyphs, cleanup')
finally:
    if process.poll() is None:
        process.terminate()
        process.wait(timeout=5)
    os.close(master)
    os.close(slave)

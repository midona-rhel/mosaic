#!/usr/bin/env python3
"""Real terminal input for optional panels, priority controls and floating views."""
import fcntl, os, pty, select, struct, subprocess, termios, time
import pyte
from terminal_screen import TerminalScreen

master,slave=pty.openpty()
fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',30,120,0,0))
screen=TerminalScreen(120,30);stream=pyte.ByteStream(screen)
p=subprocess.Popen(['./bin/mosaic','--demo','--color','--interval','10s'],stdin=slave,stdout=slave,stderr=slave,env=dict(os.environ,TERM='xterm-256color',COLORTERM='truecolor',UNICODE_PROGRESS_BAR='true'))
def pump(seconds=.22):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  if select.select([master],[],[],.03)[0]:
   data=os.read(master,65536);stream.feed(data)
   if b'\x1b[6n' in data:os.write(master,b'\x1b[1;1R')
def text():return '\n'.join(screen.display)
def key(b):os.write(master,b);pump()
def wait_for(predicate, timeout=5):
 end=time.monotonic()+timeout
 while not predicate() and time.monotonic()<end:pump(.05)
 assert predicate(), text()
def capture(name,caption):
 if os.getenv('MOSAIC_CAPTURE_PANELS')=='1':
  from terminal_render import render_screen
  render_screen(screen,'mosaic / 120 x 30 / recorded demo',None,False,caption).save('docs/'+name)
def click(x,y):key(f'\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m'.encode())
try:
 pump(.8)
 assert '8 DISK I/O' in text()
 key(b'o')
 assert all(s in text() for s in ['Battery','Thermals','Connections','Removed'])
 capture('panel-manager-fira.png','Panel manager / add, remove, reorder and sort')
 key(b'9');key(b' ')
 assert any('9 Battery' in row and 'Hidden' in row for row in screen.display), 'enabling changed priority'
 key(b'\r') # Explicitly bring first; enabling alone preserves its saved slot.
 wait_for(lambda: '9 BATTERY' in screen.display[3] and '78%' in text())
 capture('battery-card-fira.png','Battery explicitly moved first / toggling preserves priority')
 key(b'o');key(b'\x1b[1;2B')
 key(b'\x1b')
 assert '9 BATTERY' not in screen.display[3][:30], 'move down did not change priority'
 key(b'o');key(b's');key(b'\x1b')
 assert '9 BATTERY' in screen.display[3][:30], 'alphabetical sort failed'
 # Remove then restore Battery using mouse selection and the clickable action bar.
 key(b'o')
 y=next(y for y,row in enumerate(screen.display) if '9 Battery' in row)
 click(45,y)
 x=screen.display[-1].index('remove')
 click(x,29)
 assert any('9 Battery' in row and 'Removed' in row for row in screen.display)
 key(b' ');key(b'\x1b')
 # Add each remaining optional type by clicking its actual row.
 for name in ('Thermals','Connections'):
  key(b'o')
  y=next(y for y,row in enumerate(screen.display) if name in row)
  click(45,y);key(b' ');key(b'\r') # Explicit bring-first action.
  assert name.upper() in screen.display[3]
  key(b'\r')
  assert ('CPU TEMPERATURES' if name=='Thermals' else 'Reported sockets') in text()
  key(b'f')
  assert ':: '+name in text(), 'new panel failed to float'
  key(b'\x1b')
 capture('extra-panels-fira.png','Connections, Thermals and Battery / enabled cards keep priority')
 # Keep the actual floating contents visible throughout a held drag.
 key(b'd');key(b'2');key(b'f')
 y=next(y for y,row in enumerate(screen.display) if ':: CPU' in row)
 x=screen.display[y].index(':: CPU')+3
 key(f'\x1b[<0;{x+1};{y+1}M'.encode())
 key(f'\x1b[<32;{x+5};{y+2}M'.encode())
 assert ':: CPU' in screen.display[y+1] and 'Total utilization' in text(), 'window body disappeared during drag'
 capture('live-window-drag-fira.png','Live floating-window drag / contents follow the pointer')
 key(f'\x1b[<0;{x+5};{y+2}m'.encode())
 assert ':: CPU' in screen.display[y+1], 'release changed displayed destination'
 key(b'H')
 assert ':: CPU' in text(), 'floating shortcut hid its grid panel'
 key(b'\x1b');key(b'H')
 assert ':: CPU' not in text(), 'dock/hide left a window visible'
 key(b'o');key(b'2')
 assert any('2 CPU' in row and 'Removed' in row for row in screen.display), 'hide did not disable the panel'
 key(b' ');key(b'\x1b')
 assert '2 CPU' in text(), 'panel menu did not restore the hidden panel'
 key(b'q');assert p.wait(timeout=5)==0
 print('Panel PTY passed: eight equal defaults, add/remove via keys and mouse, move, sort, expanded views and pop-outs')
finally:
 if p.poll() is None:p.terminate();p.wait(timeout=5)
 os.close(master);os.close(slave)

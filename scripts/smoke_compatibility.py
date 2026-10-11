#!/usr/bin/env python3
"""Check meter overrides and opt-out of host appearance extensions (POSIX PTY)."""
import fcntl
import os
import pty
import select
import struct
import subprocess
import termios
import time

env = dict(os.environ, TERM="xterm-256color", TERM_PROGRAM="iTerm.app")
for mode, inherited in [("fira", "false"), ("portable", "true")]:
    output = subprocess.check_output(
        ["./bin/mosaic", "--demo", "--snapshot", "--meters", mode],
        env=dict(env, UNICODE_PROGRESS_BAR=inherited), text=True)
    assert any(0xEE00 <= ord(c) <= 0xEE05 for c in output) == (mode == "fira")
bad = subprocess.run(["./bin/mosaic", "--meters", "invalid"], capture_output=True)
assert bad.returncode != 0 and b"meters must be" in bad.stderr

master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 30, 120, 0, 0))
p = subprocess.Popen(["./bin/mosaic", "--demo", "--meters", "portable",
                      "--no-terminal-effects"], stdin=slave, stdout=slave, stderr=slave, env=env)
transcript = bytearray()
try:
    deadline = time.monotonic() + 5
    quit_sent = False
    while time.monotonic() < deadline:
        if select.select([master], [], [], .05)[0]:
            data = os.read(master, 65536)
            transcript.extend(data)
            if b"\x1b[6n" in data:
                os.write(master, b"\x1b[1;1R")
        if not quit_sent and b"SYSTEM" in transcript:
            os.write(master, b"q")
            quit_sent = True
        if p.poll() is not None:
            break
    assert p.wait(timeout=1) == 0
    assert b"SYSTEM" in transcript and b"\x1b[?1049h" in transcript
    assert b"\x1b[?1003h" in transcript, "mouse reporting was disabled"
    for forbidden in [b"\x1b]22;", b"HighlightCursorLine", b"\x1b]11;",
                      b"\x1b]111", b"Mosaic \xc2\xb7 system monitor"]:
        assert forbidden not in transcript, repr(forbidden)
finally:
    if p.poll() is None:
        p.kill()
        p.wait()
    os.close(master)
    os.close(slave)
print("Compatibility smoke passed: font overrides, invalid option, host-effects opt-out, mouse retained")

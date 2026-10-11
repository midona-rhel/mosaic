#!/usr/bin/env python3
"""Exercise actual Bubble Tea input, mouse reporting, resizing and terminal cleanup.

POSIX only. Install requirements-dev.txt and build bin/mosaic first.
"""
import fcntl
import os
import pty
import select
import signal
import struct
import subprocess
import termios
import time
import pyte
import re
from terminal_screen import TerminalScreen


def run():
    master, slave = pty.openpty()
    original = termios.tcgetattr(slave)

    screen = TerminalScreen(80, 24)
    stream = pyte.ByteStream(screen)

    def resize(width, height):
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        screen.resize(lines=height, columns=width)

    resize(80, 24)
    env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", TERM_PROGRAM="", ITERM_SESSION_ID="")
    process = subprocess.Popen(["./bin/mosaic", "--demo", "--color"], stdin=slave, stdout=slave, stderr=slave, env=env)
    transcript = bytearray()

    def read_until(marker, timeout=5):
        started = time.monotonic()
        deadline = started + timeout
        received = bytearray()
        while time.monotonic() < deadline:
            ready, _, _ = select.select([master], [], [], 0.1)
            if ready:
                chunk = os.read(master, 65536)
                transcript.extend(chunk)
                received.extend(chunk)
                stream.feed(chunk)
                # Answer the terminal's cursor position query if requested.
                if b"\x1b[6n" in chunk:
                    os.write(master, b"\x1b[1;1R")
            # An idempotent action may produce no cell diff. Allow input to
            # settle, then inspect the screen even when no new bytes arrived.
            if time.monotonic() - started >= 0.15 and marker.decode() in "\n".join(screen.display):
                return
        raise AssertionError(f"No {marker!r} in terminal output; tail: {received[-600:]!r}")

    def send(keys, marker):
        os.write(master, keys)
        read_until(marker)
        time.sleep(0.12)  # Keep standalone Esc separate from the next key.

    try:
        read_until(b"MOSAIC")
        assert b"\x1b]22;default\x07" in transcript, "initial arrow pointer was not requested"
        for y, line in enumerate(screen.display):
            if "↗" in line:
                x = line.index("↗")
                send(f"\x1b[<35;{x+1};{y+1}M".encode(), b"MOSAIC")
                assert b"\x1b]22;pointer\x07" in transcript, "button hover did not request a hand"
                break
        else:
            raise AssertionError("no pop-out button found")
        send(b"\x1b[<35;1;1M", b"MOSAIC")
        send(b"\x1b[C", "› 2 CPU".encode())
        send(b"\x1b[B", "5 PROCESSES".encode())
        send(b"\r", b"Filter:")
        send(b"\x1b", "5 PROCESSES".encode())
        send(b"\x1b[D", "› 4 DISKS".encode())
        send(b"\x1b[A", "› 1 SYSTEM".encode())
        send(b"\x1b[C", "› 2 CPU".encode())
        send(b"p", b"shift+arrows")
        send(b"\x1b[1;2C", "2×1".encode())
        send(b"\x1b[1;2B", "2×2".encode())
        send(b"s", "SYSTEM".encode())
        send(b"\x1b", b"o panels")
        send(b"0", "› 1 SYSTEM".encode())
        send(b"t", b"CHOOSE PANEL")
        send(b"7", b"GPU")
        send(b"\r", b"63.0%")
        send(b"t", b"CHOOSE PANEL")
        send(b"1", b"System")
        send(b"\r", b"SYSTEM")
        send(b"7", b"Provider")
        send(b"\x1b", b"SYSTEM")
        send(b"5", b"Filter:")
        send(b"/fire", b"fire")
        os.write(master, b"\x1b")
        time.sleep(0.1)  # Disambiguate a lone Esc from an escape sequence.
        send(b"\r", b"Executable")
        send(b"x", b"TERMINATE PROCESS?")
        send(b"n", b"PROCESS / ")
        send(b"\x1b", b"Filter:")
        send(b"\x1b", b"SYSTEM")
        # SGR mouse press/release inside the CPU card, with one-based coordinates.
        send(b"\x1b[<0;44;5M\x1b[<0;44;5m", "› 2 CPU".encode())
        assert "Total utilization" not in "\n".join(screen.display), "card click opened details"
        send(b"\r", b"Total utilization")
        send(b"\x1b", b"SYSTEM")
        resize(120, 30)
        process.send_signal(signal.SIGWINCH)
        read_until(b"10 CPUs")
        resize(40, 16)
        process.send_signal(signal.SIGWINCH)
        read_until(b"MOSAIC")
        send(b"6", b"Download")
        os.write(master, b"q")
        assert process.wait(timeout=5) == 0
        while select.select([master], [], [], 0.1)[0]:
            transcript.extend(os.read(master, 65536))
        assert b"\x1b[?1049h" in transcript, "alternate screen was never entered"
        assert b"\x1b[?1049l" in transcript, "alternate screen was not restored"
        assert re.search(rb"\x1b\]11;#100[eE]19(?:\x07|\x1b\\)", transcript), "temporary terminal background was not set"
        assert b"\x1b]111\x07" in transcript or b"\x1b]111\x1b\\" in transcript, "terminal background was not reset"
        assert termios.tcgetattr(slave) == original, "terminal settings were not restored"
        assert transcript.index(b"\x1b[?1049h") < transcript.index(b"\x1b]22;default\x07"), "pointer override leaked into the main screen"
        assert b"\x1b]22;\x07\x1b[?1049l" in transcript, "pointer was not reset before leaving the alternate screen"
        print("PTY smoke passed: keyboard, filter, inspect, cancel, mouse, resize, background and pointer set/reset, cleanup")
    finally:
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        os.close(master)
        os.close(slave)


if __name__ == "__main__":
    run()

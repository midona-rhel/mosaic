#!/usr/bin/env python3
"""Exercise the native Windows console through pywinpty's ConPTY backend.

Install requirements-dev.txt and build bin/mosaic.exe first.
"""
import os
from pathlib import Path
import queue
import threading
import time

import pyte
from terminal_screen import TerminalScreen


def run():
    if os.name != "nt":
        raise SystemExit("This smoke test requires native Windows; use smoke_pty.py on POSIX.")
    from winpty import PtyProcess
    from winpty.enums import Backend

    binary = str(Path(__file__).resolve().parent.parent / "bin" / "mosaic.exe")
    process = PtyProcess.spawn([binary, "--demo", "--color"],
                               dimensions=(24, 80), backend=Backend.ConPTY)
    chunks = queue.Queue()
    transcript = bytearray()
    screen = TerminalScreen(80, 24)
    stream = pyte.ByteStream(screen)
    query_tail = b""

    def send(keys):
        process.write(keys.decode("utf-8"))

    def drain():
        try:
            while True:
                chunks.put(process.read(65536).encode("utf-8"))
        except EOFError:
            pass

    def feed(chunk):
        nonlocal query_tail
        transcript.extend(chunk)
        stream.feed(chunk)
        queries = query_tail + chunk
        if b"\x1b[6n" in queries:
            send(b"\x1b[1;1R")
        query_tail = queries[-3:]

    def read_until(marker, timeout=10):
        started = time.monotonic()
        while time.monotonic() - started < timeout:
            try:
                feed(chunks.get(timeout=0.1))
            except queue.Empty:
                pass
            if time.monotonic() - started >= 0.15 and marker in "\n".join(screen.display):
                return
        raise AssertionError(f"No {marker!r}; alive={process.isalive()}, "
                             f"output={bytes(transcript[-2000:])!r}\n"
                             + "\n".join(screen.display))

    def action(keys, marker):
        send(keys)
        read_until(marker)
        time.sleep(0.12)  # Disambiguate standalone Escape.

    def drag_window():
        def title_position():
            for y, row in enumerate(screen.display):
                if ":: CPU" in row:
                    return row.index(":: CPU"), y
            return None

        resize(160, 45)
        read_until(":: CPU")
        # Drain the resize frame before taking coordinates.
        deadline = time.monotonic() + 0.3
        while time.monotonic() < deadline:
            try:
                feed(chunks.get(timeout=0.02))
            except queue.Empty:
                pass
        x, y = title_position()

        def mouse(button, dx=0, release=False):
            send(f"\x1b[<{button};{x + dx + 1};{y + 1}{'m' if release else 'M'}".encode())

        mouse(0)
        positions = set()
        started = time.monotonic()
        for step in range(60):
            mouse(32, 1 + step % 15)
            deadline = started + (step + 1) / 60
            while time.monotonic() < deadline:
                try:
                    feed(chunks.get(timeout=0.002))
                    positions.add(title_position())
                except queue.Empty:
                    pass
        mouse(0, 15, release=True)
        released = time.monotonic()
        deadline = released + 2
        while title_position() != (x + 15, y) and time.monotonic() < deadline:
            try:
                feed(chunks.get(timeout=0.02))
            except queue.Empty:
                pass
        assert title_position() == (x + 15, y), "drag did not reach the final mouse position"
        assert len(positions - {None}) >= 4, "drag did not paint intermediate positions"
        print(f"ConPTY drag: {len(positions - {None})} positions; "
              f"release-to-final-frame {(time.monotonic() - released) * 1000:.0f} ms")

    def resize(width, height):
        screen.resize(lines=height, columns=width)
        process.setwinsize(height, width)

    reader = threading.Thread(target=drain, daemon=True)
    reader.start()
    try:
        read_until("MOSAIC")
        action(b"o", "PANELS")
        action(b"9", "Battery")
        action(b" ", "Battery")
        action(b"\r", "9 BATTERY")  # Explicit bring-first, separate from enable.
        action(b"\r", "Battery power")
        action(b"\x1b", "9 BATTERY")
        action(b"o", "PANELS")
        action(b" ", "Removed")
        action(b"\x1b", "SYSTEM")
        action(b"0", "SYSTEM")
        action(b"\x1b[C", "› 2 CPU")
        action(b"p", "shift+arrows")
        action(b"\x1b[1;2C", "2×1")
        action(b"\x1b[1;2B", "2×2")
        action(b"s", "SYSTEM")
        action(b"\x1b", "o panels")
        action(b"0", "SYSTEM")
        action(b"t", "CHOOSE PANEL")
        action(b"7", "GPU")
        action(b"\r", "63.0%")
        action(b"t", "CHOOSE PANEL")
        action(b"1", "System")
        action(b"\r", "SYSTEM")
        action(b"7", "Provider")
        action(b"\x1b", "SYSTEM")
        action(b"5", "Filter:")
        action(b"/fire", "fire")
        send(b"\x1b")
        time.sleep(0.15)
        action(b"\r", "Executable")
        action(b"x", "TERMINATE PROCESS?")
        action(b"n", "PROCESS / ")
        action(b"\x1b", "Filter:")
        action(b"\x1b", "SYSTEM")
        action(b"\x1b[<0;44;5M\x1b[<0;44;5m", "› 2 CPU")
        action(b"\r", "Total utilization")
        action(b"f", ":: CPU")
        drag_window()
        action(b"d", "SYSTEM")
        resize(120, 30)
        read_until("10 CPUs")
        resize(40, 16)
        action(b"6", "Download")
        send(b"q")
        deadline = time.monotonic() + 10
        while process.isalive() and time.monotonic() < deadline:
            time.sleep(0.1)
        assert not process.isalive(), "console process did not exit"
        assert process.exitstatus == 0, f"console process exited with {process.exitstatus}"
    finally:
        process.close(force=True)
        reader.join(timeout=5)
        while not chunks.empty():
            transcript.extend(chunks.get_nowait())
    assert b"\x1b[?1049h" in transcript, "alternate screen was never entered"
    assert b"\x1b[?1049l" in transcript, "alternate screen was not restored"
    print("ConPTY smoke passed: keyboard, filter, inspect, cancel, mouse, float, resize, exit, cleanup")


if __name__ == "__main__":
    run()

#!/usr/bin/env python3
"""Native Windows terminal smoke via ConPTY (Windows 10 1809+).

Install requirements-dev.txt and build bin/mosaic.exe first. This exercises
the Windows console backend, unlike piping the program's standard streams.
"""
import ctypes as c
from ctypes import wintypes as w
import os
from pathlib import Path
import queue
import subprocess
import threading
import time

import pyte
from terminal_screen import TerminalScreen


class COORD(c.Structure):
    _fields_ = [("X", c.c_short), ("Y", c.c_short)]


class STARTUPINFO(c.Structure):
    _fields_ = [
        ("cb", w.DWORD), ("reserved", w.LPWSTR), ("desktop", w.LPWSTR),
        ("title", w.LPWSTR), ("x", w.DWORD), ("y", w.DWORD),
        ("xsize", w.DWORD), ("ysize", w.DWORD), ("xchars", w.DWORD),
        ("ychars", w.DWORD), ("fill", w.DWORD), ("flags", w.DWORD),
        ("show", w.WORD), ("reserved_size", w.WORD), ("reserved_bytes", c.c_void_p),
        ("stdin", w.HANDLE), ("stdout", w.HANDLE), ("stderr", w.HANDLE),
    ]


class STARTUPINFOEX(c.Structure):
    _fields_ = [("info", STARTUPINFO), ("attributes", c.c_void_p)]


class PROCESS_INFORMATION(c.Structure):
    _fields_ = [("process", w.HANDLE), ("thread", w.HANDLE),
                ("pid", w.DWORD), ("tid", w.DWORD)]


def run():
    if os.name != "nt":
        raise SystemExit("This smoke test requires native Windows; use smoke_pty.py on POSIX.")
    kernel = c.WinDLL("kernel32", use_last_error=True)

    def bind(name, restype, *args):
        f = getattr(kernel, name)
        f.restype, f.argtypes = restype, args
        return f

    handle_ptr = c.POINTER(w.HANDLE)
    dword_ptr = c.POINTER(w.DWORD)
    create_pipe = bind("CreatePipe", w.BOOL, handle_ptr, handle_ptr, c.c_void_p, w.DWORD)
    close = bind("CloseHandle", w.BOOL, w.HANDLE)
    create_console = bind("CreatePseudoConsole", c.c_long, COORD, w.HANDLE, w.HANDLE, w.DWORD, handle_ptr)
    resize_console = bind("ResizePseudoConsole", c.c_long, w.HANDLE, COORD)
    close_console = bind("ClosePseudoConsole", None, w.HANDLE)
    initialize = bind("InitializeProcThreadAttributeList", w.BOOL, c.c_void_p, w.DWORD, w.DWORD, c.POINTER(c.c_size_t))
    update = bind("UpdateProcThreadAttribute", w.BOOL, c.c_void_p, w.DWORD, c.c_size_t, c.c_void_p, c.c_size_t, c.c_void_p, c.c_void_p)
    delete = bind("DeleteProcThreadAttributeList", None, c.c_void_p)
    create_process = bind("CreateProcessW", w.BOOL, w.LPCWSTR, w.LPWSTR, c.c_void_p, c.c_void_p, w.BOOL,
                          w.DWORD, c.c_void_p, w.LPCWSTR, c.POINTER(STARTUPINFOEX), c.POINTER(PROCESS_INFORMATION))
    read_file = bind("ReadFile", w.BOOL, w.HANDLE, c.c_void_p, w.DWORD, dword_ptr, c.c_void_p)
    write_file = bind("WriteFile", w.BOOL, w.HANDLE, c.c_void_p, w.DWORD, dword_ptr, c.c_void_p)
    wait = bind("WaitForSingleObject", w.DWORD, w.HANDLE, w.DWORD)
    terminate = bind("TerminateProcess", w.BOOL, w.HANDLE, w.UINT)
    exit_code = bind("GetExitCodeProcess", w.BOOL, w.HANDLE, dword_ptr)

    def check(result):
        if not result:
            raise c.WinError(c.get_last_error())

    def hresult(result):
        if result < 0:
            raise OSError(f"ConPTY HRESULT 0x{result & 0xffffffff:08x}")

    input_read, input_write, output_read, output_write = (w.HANDLE() for _ in range(4))
    console = w.HANDLE()
    process = PROCESS_INFORMATION()
    attributes = None
    attributes_initialized = False
    reader = None
    chunks = queue.Queue()
    transcript = bytearray()
    screen = TerminalScreen(80, 24)
    stream = pyte.ByteStream(screen)

    def send(keys):
        data = c.create_string_buffer(keys)
        count = w.DWORD()
        check(write_file(input_write, data, len(keys), c.byref(count), None))
        assert count.value == len(keys), "partial console write"

    def drain():
        buffer = c.create_string_buffer(65536)
        count = w.DWORD()
        while read_file(output_read, buffer, len(buffer), c.byref(count), None) and count.value:
            chunks.put(buffer.raw[:count.value])

    def feed(chunk):
        transcript.extend(chunk)
        stream.feed(chunk)
        if b"\x1b[6n" in chunk:
            send(b"\x1b[1;1R")

    def read_until(marker, timeout=10):
        # Wait for new output before testing the reconstructed terminal screen.
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            try:
                feed(chunks.get(timeout=0.1))
            except queue.Empty:
                continue
            if marker in "\n".join(screen.display):
                return
        raise AssertionError(f"No {marker!r} in console screen:\n" + "\n".join(screen.display))

    def action(keys, marker):
        send(keys)
        read_until(marker)
        time.sleep(0.12)  # Disambiguate standalone Escape.

    def resize(width, height):
        screen.resize(lines=height, columns=width)
        hresult(resize_console(console, COORD(width, height)))

    try:
        check(create_pipe(c.byref(input_read), c.byref(input_write), None, 0))
        check(create_pipe(c.byref(output_read), c.byref(output_write), None, 0))
        hresult(create_console(COORD(80, 24), input_read, output_write, 0, c.byref(console)))
        size = c.c_size_t()
        initialize(None, 1, 0, c.byref(size))  # Query the required buffer size.
        attributes = c.create_string_buffer(size.value)
        check(initialize(attributes, 1, 0, c.byref(size)))
        attributes_initialized = True
        check(update(attributes, 0, 0x00020016, console, c.sizeof(w.HANDLE), None, None))
        startup = STARTUPINFOEX()
        startup.info.cb = c.sizeof(startup)
        startup.attributes = c.cast(attributes, c.c_void_p)
        binary = str(Path(__file__).resolve().parent.parent / "bin" / "mosaic.exe")
        command = c.create_unicode_buffer(subprocess.list2cmdline([binary, "--demo", "--color"]))
        # Read concurrently: ConPTY operations may block if output is not drained.
        reader = threading.Thread(target=drain, daemon=True)
        reader.start()
        check(create_process(binary, command, None, None, False, 0x00080000, None, None,
                             c.byref(startup), c.byref(process)))
        # ConPTY owns its duplicates. Keeping our output writer open would
        # prevent the reader seeing EOF after the hosted console closes.
        close(input_read)
        input_read = w.HANDLE()
        close(output_write)
        output_write = w.HANDLE()
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
        action(b"\x1b", "sort: CPU")
        action(b"\r", "Executable")
        action(b"x", "TERMINATE PROCESS?")
        action(b"n", "PROCESS / ")
        action(b"\x1b", "Filter:")
        action(b"\x1b", "SYSTEM")
        action(b"\x1b[<0;44;5M\x1b[<0;44;5m", "› 2 CPU")
        action(b"\r", "Total utilization")
        action(b"f", ":: CPU")
        action(b"d", "SYSTEM")
        resize(120, 30)
        read_until("10 CPUs")
        resize(40, 16)
        action(b"6", "Download")
        send(b"q")
        assert wait(process.process, 10000) == 0, "console process did not exit"
        code = w.DWORD()
        check(exit_code(process.process, c.byref(code)))
        assert code.value == 0, f"console process exited with {code.value}"
    finally:
        if process.process:
            if wait(process.process, 0) == 258:
                terminate(process.process, 1)
                wait(process.process, 5000)
        if console:
            close_console(console)
        if reader:
            reader.join(timeout=5)
        while not chunks.empty():
            transcript.extend(chunks.get_nowait())
        if attributes_initialized:
            delete(attributes)
        for handle in (process.thread, process.process, input_read, input_write, output_read, output_write):
            if handle:
                close(handle)
    assert b"\x1b[?1049h" in transcript, "alternate screen was never entered"
    assert b"\x1b[?1049l" in transcript, "alternate screen was not restored"
    print("ConPTY smoke passed: keyboard, filter, inspect, cancel, mouse, float, resize, exit, cleanup")


if __name__ == "__main__":
    run()

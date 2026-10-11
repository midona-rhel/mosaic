#!/usr/bin/env python3
"""Native Windows check: signed driver loads without setup and cleans up on EOF."""
import ctypes
import json
from pathlib import Path
import queue
import subprocess
import threading

ROOT = Path(__file__).resolve().parents[1]
HELPER = ROOT / 'bin/sensors/Mosaic.SensorHelper.exe'
SERVICE = 'MosaicSensors-PawnIO-2-1'
assert ctypes.windll.shell32.IsUserAnAdmin(), 'Run this lifecycle test elevated'

def service_absent():
    result = subprocess.run(['sc.exe', 'query', SERVICE], capture_output=True)
    return result.returncode == 1060

class Session:
    def __init__(self):
        self.proc = subprocess.Popen([str(HELPER), '--stdio'], stdin=subprocess.PIPE,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                     text=True, encoding='utf-8')
        self.lines = queue.Queue()
        def read():
            for line in self.proc.stdout:
                self.lines.put(line)
        threading.Thread(target=read, daemon=True).start()
    def sample(self):
        self.proc.stdin.write('sample\n')
        self.proc.stdin.flush()
        return json.loads(self.lines.get(timeout=30))
    def close(self):
        self.proc.stdin.close()
        assert self.proc.wait(timeout=10) == 0, self.proc.stderr.read()
    def kill(self):
        if self.proc.poll() is None:
            self.proc.kill()
            self.proc.wait()

assert service_absent(), 'Test must start without a Mosaic sensor service'
before = set(Path('C:/ProgramData').glob('MosaicSensors-*'))
first, second = Session(), None
try:
    a = first.sample()
    assert a['DriverMode'] == 'portable-session' and a['DriverAccessible'], a
    print('Signed portable driver loaded; CPU device accessible without installer', flush=True)
    second = Session()
    b = second.sample()
    assert b['DriverMode'] == 'portable-session' and b['DriverAccessible'], b
    first.close()
    # Closing the first instance must not interrupt the other one.
    b = second.sample()
    assert b['DriverAccessible'], b
    second.close()
finally:
    first.kill()
    if second:
        second.kill()
assert service_absent(), 'Session driver service remained after both readers exited'
assert set(Path('C:/ProgramData').glob('MosaicSensors-*')) == before, 'Session image was not removed'
print('Concurrent readers preserved; service and driver image removed on final exit', flush=True)

# A modified image must fail closed before it reaches the service manager.
image = HELPER.parent / 'driver/PawnIO.sys'
original = image.read_bytes()
try:
    image.write_bytes(b'XX' + original[2:])
    result = subprocess.run([str(HELPER), '--once'], capture_output=True, text=True, timeout=30)
    report = json.loads(result.stdout)
    assert not report['DriverAccessible'] and any('checksum mismatch' in p for p in report['Problems']), report
    assert service_absent()
finally:
    image.write_bytes(original)
print('Modified driver rejected before loading', flush=True)

# Simulate an existing provider. Mosaic must borrow, not replace/stop it.
external = 'MosaicTestExternalPawnIO'
created = False
try:
    subprocess.run(['sc.exe', 'create', external, 'type=', 'kernel', 'start=', 'demand',
                    'binPath=', str(image)], check=True, capture_output=True)
    created = True
    subprocess.run(['sc.exe', 'start', external], check=True, capture_output=True)
    result = subprocess.run([str(HELPER), '--once'], capture_output=True, text=True, timeout=30)
    report = json.loads(result.stdout)
    assert report['DriverMode'] == 'existing' and report['DriverAccessible'], report
    assert service_absent(), 'Borrowing a provider created a competing service'
    # Ask the SCM for numeric state rather than depending on localized text.
    state = subprocess.run(['powershell.exe', '-NoProfile', '-Command',
                            f'(Get-Service {external}).Status.value__'],
                           capture_output=True, text=True, check=True)
    assert state.stdout.strip() == '4', state.stdout
    print('Existing provider left running and unchanged', flush=True)
finally:
    if created:
        subprocess.run(['sc.exe', 'stop', external], capture_output=True)
        subprocess.run(['sc.exe', 'delete', external], capture_output=True)

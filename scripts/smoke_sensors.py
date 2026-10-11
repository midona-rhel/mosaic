#!/usr/bin/env python3
"""Check the bundled helper on Windows with its normal session-driver lifecycle."""
import json
import os
from pathlib import Path
import queue
import subprocess
import threading
import time

ROOT = Path(__file__).resolve().parents[1]
helper = ROOT / 'bin/sensors/Mosaic.SensorHelper.exe'
env = dict(os.environ, DOTNET_ROOT=str(ROOT / 'no-installed-runtime'))
process = subprocess.Popen([str(helper), '--stdio'], stdin=subprocess.PIPE,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                           text=True, encoding='utf-8', env=env)
results = queue.Queue()

def read():
    for line in process.stdout:
        results.put(line)

threading.Thread(target=read, daemon=True).start()
try:
    reports = []
    for _ in range(2):
        process.stdin.write('sample\n')
        process.stdin.flush()
        report = json.loads(results.get(timeout=20))
        assert report['Protocol'] == 1
        assert isinstance(report['Sensors'], list)
        assert isinstance(report['DriverAccessible'], bool)
        if not report['DriverAccessible']:
            cpu = [s for s in report['Sensors'] if s['HardwareType'] == 'Cpu'
                   and s['Type'] in ('Temperature', 'Power', 'Clock')]
            assert all(s['Value'] is None for s in cpu), 'inaccessible CPU registers reported measurements'
        reports.append(report)
        time.sleep(0.1)
    assert reports[0]['At'] != reports[1]['At']
    process.stdin.close()
    assert process.wait(timeout=5) == 0
    print('Sensor helper: repeated samples, self-contained runtime, missing-driver handling, EOF cleanup passed')
finally:
    if process.poll() is None:
        process.kill()
        process.wait()

result = subprocess.run([str(ROOT / 'bin/mosaic.exe'), '--cpu-diagnostics'],
                        capture_output=True, text=True, encoding='utf-8', timeout=30)
assert result.returncode == 0, result.stderr
report = json.loads(result.stdout)
assert report['SensorBackend']['Protocol'] == 1, report
print('Mosaic consumed the bundled helper through its native Windows collector')

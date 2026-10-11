#!/usr/bin/env python3
"""Native availability/schema smoke. This does not certify absent hardware."""
import json, math, os, subprocess, sys
binary='./bin/mosaic.exe' if sys.platform=='win32' else './bin/mosaic'
s=json.loads(subprocess.check_output([binary,'--json','--interval','250ms'],timeout=25))
r=s['Extras']
assert all(r['Collected'].values()), 'JSON must request all extra collectors'
assert r['At'] and r['Connections']['Source']
for b in r['Battery']['Devices'] or []:
 for k in ('Percent','Health','Watts','Voltage','Cycles'):
  if b[k] is not None:assert math.isfinite(b[k]) and b[k]>=0,(k,b[k])
 if b['Percent'] is not None:assert b['Percent']<=100
for f in r['Fans']['Devices'] or []:assert 0<=f['RPM']<=100000
for c in r['Connections']['Entries'] or []:
 assert c['Protocol'] in ('TCP','UDP') and c['PID']>=0 and c['Local']
print('Native collector schema passed on',sys.platform)
for key,field in [('Battery','Devices'),('Fans','Devices'),('Connections','Entries')]:
 report=r[key];print(key, len(report[field] or []),'readings;',report['Problem'] or 'no provider error')

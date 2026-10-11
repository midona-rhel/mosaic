#!/usr/bin/env python3
"""Import pinned, MIT-licensed Fastfetch artwork from its source tarball."""
import json
import pathlib
import re
import sys
import tarfile

REVISION = '3453de61f82e4cdd746cf60534e943a842e6b199'
ROOT = pathlib.Path(__file__).resolve().parents[1] / 'internal/ui/osart'


def normalize(value):
    return ''.join(c for c in value.lower() if c.isalnum())


def main(archive):
    assets, aliases = {}, {}
    with tarfile.open(archive) as source:
        for member in source.getmembers():
            if '/src/logo/ascii/' not in member.name or not member.isfile():
                continue
            value = source.extractfile(member).read().decode()
            if member.name.endswith('.txt'):
                assets[pathlib.Path(member.name).stem] = value
            elif member.name.endswith('.inc'):
                pattern = r'\.names\s*=\s*\{(.*?)\}.*?\.lines\s*=\s*FASTFETCH_DATATEXT_LOGO_(\w+)'
                for names, asset in re.findall(pattern, value, re.S):
                    for name in re.findall(r'"([^"\n]+)"', names):
                        aliases[normalize(name)] = asset.lower()
    assert all(v in assets for v in aliases.values())
    catalog = {'revision': REVISION, 'art': assets, 'aliases': aliases}
    (ROOT / 'catalog.json').write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + '\n')
    print(f'Imported {len(assets)} artworks and {len(aliases)} aliases')


if __name__ == '__main__':
    main(sys.argv[1])

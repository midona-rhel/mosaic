#!/usr/bin/env python3
"""Publish Mosaic's self-contained Windows sensor helper (requires .NET 10 SDK)."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
PROJECT = ROOT / 'windows/SensorHelper/Mosaic.SensorHelper.csproj'


def build(arch, output):
    rid = {'amd64': 'win-x64', 'arm64': 'win-arm64'}[arch]
    # Publish sequentially: both RIDs share the project's restore directory.
    subprocess.run(['dotnet', 'restore', str(PROJECT), '--locked-mode'], check=True)
    subprocess.run(['dotnet', 'publish', str(PROJECT), '-c', 'Release', '-r', rid,
                    '--self-contained', 'true', '--no-restore', '-o', str(output)], check=True)
    if not (output / 'Mosaic.SensorHelper.exe').is_file() or not (output / 'coreclr.dll').is_file():
        raise RuntimeError('Self-contained helper publication is incomplete')
    licenses = output / 'licenses'
    shutil.copytree(ROOT / 'windows/licenses', licenses, dirs_exist_ok=True)
    # Include exact dependency metadata, embedded license files and source URLs.
    assets = json.loads((PROJECT.parent / 'obj/project.assets.json').read_text())
    package_roots = list(assets['packageFolders'])
    inventory = []
    for name, library in assets['libraries'].items():
        if library['type'] != 'package':
            continue
        folder = next((Path(root) / library['path'] for root in package_roots
                       if (Path(root) / library['path']).is_dir()), None)
        if folder is None:
            raise RuntimeError(f'Missing dependency metadata: {name}')
        safe_name = name.replace('/', '-')
        metadata = next(folder.glob('*.nuspec'))
        shutil.copy2(metadata, licenses / f'{safe_name}.nuspec')
        tree = ET.parse(metadata)
        ns = {'n': 'http://schemas.microsoft.com/packaging/2013/05/nuspec.xsd'}
        entry = {'package': name, 'nuget': f'https://www.nuget.org/packages/{name}'}
        for field in ('license', 'projectUrl', 'repository'):
            element = tree.find(f'.//n:{field}', ns)
            if element is not None:
                entry[field] = {'text': element.text, **element.attrib}
                if field == 'license' and element.get('type') == 'file':
                    source = folder / element.text
                    if source.is_file():
                        shutil.copy2(source, licenses / f'{safe_name}-{source.name}')
        inventory.append(entry)
    (licenses / 'dependencies.json').write_text(json.dumps(inventory, indent=2) + '\n')
    shutil.copy2(PROJECT.parent / 'packages.lock.json', licenses / 'packages.lock.json')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--arch', choices=['amd64', 'arm64'], required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    build(args.arch, args.output.resolve())


if __name__ == '__main__':
    main()

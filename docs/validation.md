# Validation

## Release checks

The local release checks run on Linux with Go 1.26.9:

- `go test -race ./...` and `go vet ./...`.
- Real PTY keyboard/mouse input, resizing, layout persistence, floating windows,
  panel management, terminal setup and cleanup, and compatibility options.
- Live collector schema checks and an owned local TCP listener/process fixture.
- Pixel checks for meter coverage, graph baselines, and adjacent cell bounds.
- CGO-disabled builds for macOS, Windows, and Linux on amd64 and arm64.
- Release ZIP integrity and binary-content verification; SHA-256 checksums.

[GitHub Actions](https://github.com/midona-rhel/mosaic/actions) runs the native
Linux, macOS, and Windows matrix. Windows jobs include PowerShell 5.1/7 and
ConPTY checks. Runner success verifies those environments, not every terminal,
driver, GPU, sensor, or OS release.

## Coverage

UI tests exercise all eleven panel types, configurable content, grid spans,
collision-free placement, priority and overflow hiding, resizing cancellation,
spatial navigation, panel replacement, saved layouts, process sorting and
selection, scrollbars, pop-outs, window stacking, hover isolation, and terminal
pointer cleanup. Process termination tests operate only on owned child fixtures
and verify identity checks against PID reuse.

Telemetry fixtures cover CPU accounting and baselines, counter resets and units,
optional values, cache/history behavior, GPU engine aggregation, Apple sensor
keys and energy channels, Linux sysfs/powercap, Windows WMI/PDH parsing, battery
capacity/current units, fan readings, and socket ownership. Source references and
provider-specific limits are in [telemetry.md](telemetry.md).

Live Linux collection has been checked against `/proc/meminfo` and `statvfs` for
memory and filesystem totals. The development VM has no GPU, battery, or thermal
hardware exposed. Missing readings are reported as unavailable. A native M5 Pro
report confirmed CPU temperature and package-power readings on macOS 27; this is
not an independent calibration of those readings.

## Reproducing checks

```sh
go test -race ./...
go vet ./...
go build -o bin/mosaic ./cmd/mosaic
python3 -m venv .venv
.venv/bin/pip install -r requirements-dev.txt
.venv/bin/python scripts/smoke_pty.py
.venv/bin/python scripts/smoke_iterm.py
.venv/bin/python scripts/smoke_compatibility.py
.venv/bin/python scripts/smoke_layout.py
.venv/bin/python scripts/smoke_panels.py
.venv/bin/python scripts/smoke_collectors.py
.venv/bin/python scripts/test_terminal_render.py
python3 scripts/release.py --version v0.1.0
```

## Visual fixtures

The panel galleries render actual application output at 120×30 and 80×24 using
labeled demo data. The interaction GIF records actual keyboard and mouse input
at 120×30; its pointer, click pulses, and action captions are annotations added
to the decoded terminal output. Run `scripts/capture_interactions.py` after
building `bin/mosaic` to reproduce it.

All 14 compact Chafa logos have six-, seven-, and eight-row variants. The 42
assets reproduce byte-for-byte with `scripts/generate_os_ascii.py`, Chafa 1.14.5,
the vendored Simple Icons vectors, and Fira Code Bold. See the
[artwork provenance](../internal/ui/osart/README.md). Other distro logos and
larger details use the embedded Fastfetch catalogue.

## Remaining hardware and terminal limits

GPU memory, temperature, power, fan speeds, and CPU energy counters depend on
hardware, driver, OS version, and permissions. Linux Intel global GPU activity
and some Apple/WDDM optional counters remain unavailable on unsupported systems.
Native Apple GPU memory/temperature and battery-power accuracy require further
hardware comparisons. A missing reading is never replaced with a guessed zero.

Fira Code meters need the corresponding font glyphs. Use `--meters portable` with
other fonts, `--no-terminal-effects` when terminal-specific pointer/background
controls are unwanted, and `--snapshot` or `--json` outside an interactive console.
See [terminals.md](terminals.md) for supported environments and fallbacks.

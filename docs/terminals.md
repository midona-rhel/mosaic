# Terminal compatibility and verification

The executable is a native Go console application. It does not depend on Bash, Make, Python or a web browser at runtime. The optional Windows GPU collector starts the system's noninteractive PowerShell itself, independent of the shell from which Mosaic was launched.

| Environment | Behavior / requirement | Verification status |
| --- | --- | --- |
| Linux terminal with Bash/zsh/fish | Unicode, VT input/output; detected color profile; keyboard controls | Native Linux + real POSIX PTY checked locally; individual emulator brands not exhaustively tested |
| macOS Terminal.app, iTerm2, WezTerm, kitty | Same VT UI; font ligatures depend on host configuration | x64/ARM64 builds and native macOS PTY suite passed; individual emulator brands not exhaustively tested |
| Windows Terminal with PowerShell 5.1/7 or cmd | Native `.exe`; VT input/output enabled by Bubble Tea; Windows 10 1809+ / Windows 11 targeted | x64/ARM64 builds, PowerShell 5.1/7 and native ConPTY suite passed |
| Modern Windows console host | Native VT console path; font and mouse presentation may differ | Native ConPTY suite passed; classic host UI presentation not independently checked |
| VS Code integrated terminal | Run executable in Terminal, not an output-only panel | Intended compatibility; native host testing pending |
| SSH / tmux / screen | Terminal description and mouse/color passthrough must be configured by the host | Keyboard fallback provided; these host combinations not tested locally |
| PowerShell ISE / legacy or dumb output hosts | Use `--snapshot` / `--json` | Full-screen interactive support not promised |

Mouse dragging requires the host to report press, motion and release coordinates. The app handles SGR mouse messages through Bubble Tea and always offers keyboard equivalents: `f`, `m`, `d`, `Esc`, Tab and Enter. `--no-mouse` disables capture. Ligatures and Fira Code are optional visual features and do not change character-cell positions or click rectangles.

Mosaic requests contextual mouse pointers with OSC 22: an arrow over ordinary content, a pointing hand on buttons, grab/grabbing over draggable titles, and directional resize arrows on borders. Overrides are applied after entering the alternate screen and cleared before leaving, so hosts with separate screen pointer state retain the shell's pointer. `--no-mouse` sends no pointer overrides. Terminals without OSC 22 support keep their own pointer while keyboard and mouse actions continue to work. Native pointer appearance in individual terminal versions remains unverified here.

For iTerm, Mosaic uses its supported X11 shape names (`arrow`, `hand2`, `fleur`, `sb_h_double_arrow`, `sb_v_double_arrow`); CSS names from the earlier build reset iTerm to its text cursor. macOS has native diagonal cursors, but the inspected iTerm OSC 22 mapping does not expose diagonal cursor names. Mosaic now retains an ordinary arrow at corners in iTerm instead of the previous crosshair, and marks the hovered corner with a diagonal arrow in the TUI. Hosts supporting CSS diagonal shapes still receive `nwse-resize` / `nesw-resize`. This is an emulator protocol limitation, not a claim about macOS's cursor APIs. On entering Mosaic, `OSC 1337;HighlightCursorLine=0` disables the cursor-guide stripe in the current iTerm session. It does not edit the saved profile or guess the previous setting on exit; View → Show Cursor Guide can re-enable it. Other hosts receive CSS pointer names.

Protocol references: [OSC 22 pointer shape names and reset behavior](https://github.com/kovidgoyal/kitty/blob/master/docs/pointer-shapes.rst), [iTerm's OSC 22 parser](https://github.com/gnachman/iTerm2/blob/5c60f13237314a7d4747f36d3d18c0e4aac2bfd8/sources/VT100/VT100XtermParser.m#L291), and [iTerm's actual X11 pointer map](https://github.com/gnachman/iTerm2/blob/5c60f13237314a7d4747f36d3d18c0e4aac2bfd8/sources/PTYSession/PTYSession.m#L18969).

Mosaic temporarily sets the terminal background to its canvas color (`#100E19`) through Bubble Tea's `View.BackgroundColor` (OSC 11). This lets newly exposed cells and terminal padding match the app while resize frames are pending. Bubble Tea sends OSC 111 on normal shutdown to reset the terminal background. The POSIX PTY smoke test checks both sequences and resize handling; native iTerm resize behavior still needs verification. Hosts that ignore OSC 11 retain their own background outside painted cells.

The Windows test harness creates an actual [ConPTY](https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session), launches `bin/mosaic.exe` with console handles, feeds keyboard/SGR mouse input, changes the console dimensions, reconstructs the screen with pyte and checks successful exit and alternate-screen restoration. It exercises the Windows terminal backend rather than substituting a pipe for a console. It cannot prove every emulator's rendering or mouse translation, and it has not run locally on this Linux machine.

## Windows checks

```powershell
go build -trimpath -o .\bin\mosaic.exe .\cmd\mosaic
.\scripts\smoke_powershell.ps1
python -m pip install -r requirements-dev.txt
python .\scripts\smoke_conpty.py
.\bin\mosaic.exe --demo --color
```

Run the launch script under both Windows PowerShell 5.1 and PowerShell 7. For native visual acceptance, resize the terminal, click the CPU card’s ↗ button, rearrange dashboard cards by dragging their titles, resize each edge and a corner, open Processes, inspect a process, cancel its termination prompt and quit. Confirm that the original console and text selection behavior are restored. Do this in both Windows Terminal and the modern console host before calling them fully validated.

References: [Bubble Tea's pinned Windows console setup](https://github.com/charmbracelet/bubbletea/blob/v2.1.0/tty_windows.go), [Windows virtual terminal sequences](https://learn.microsoft.com/en-us/windows/console/console-virtual-terminal-sequences), [ConPTY creation/lifecycle](https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session), and [Microsoft on interactive console apps in ISE](https://devblogs.microsoft.com/powershell/console-application-non-support-in-the-ise/).

## Fira Code typography

Install the bundled Regular/Bold fonts and choose **Fira Code** in the host terminal’s font settings. In Windows Terminal, set the profile’s `font.face` to `Fira Code`; PowerShell does not select fonts. The upstream [terminal support list](https://github.com/tonsky/FiraCode#terminal-compatibility-list) includes Windows Terminal, WezTerm, kitty and iTerm2; Windows Console itself does not shape ligatures. The latter still supports standard-width monitoring and mouse controls where VT mouse reporting is provided.

Portable meters are the default. `--meters fira` or `UNICODE_PROGRESS_BAR=true` opts into Fira Code's U+EE00–U+EE05 joined glyphs after you select that font. `--meters portable` overrides an inherited font opt-in. Arrow and punctuation ligatures are separate font features and retain their cell counts.

The portable meter paints foreground and background together so the muted track and fill have the same height. ASCII brackets and shaded empty glyphs remain meaningful without color.

## Explicit compatibility controls

Font coverage cannot be reliably queried across terminals. The default therefore uses standard Unicode blocks and brackets. The `--meters` flags override `UNICODE_PROGRESS_BAR` and work identically from PowerShell, cmd, Bash and zsh.

Use `--no-terminal-effects` when host extensions are unavailable or undesirable. It suppresses Mosaic's OSC pointer changes, iTerm cursor-guide command, default background changes and title override. It retains ordinary mouse input and keyboard navigation. Use `--no-mouse` separately when mouse reporting is unavailable. Bubble Tea still uses the normal terminal initialization and VT screen/input protocols. Portable meters require standard Unicode block glyphs; this is not an ASCII-only or legacy non-VT frontend.

Example with another font and without host appearance overrides:

```sh
./mosaic --meters portable --no-terminal-effects
```

No terminal font flag can add mouse reporting, ligatures or pointer-shape support to an emulator which does not implement them. In output-only hosts use `--json`; use `--snapshot --meters portable` for a static Unicode dashboard. Cross-platform binaries and protocol tests do not establish hands-on compatibility with every terminal version or multiplexer configuration.

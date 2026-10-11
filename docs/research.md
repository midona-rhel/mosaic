# Research and design decisions

Investigated 10 October 2026. The implementation follows the requested equal rounded boxes, with a single expanded detail view and layered dialogs.

## Library choice

The library recalled as “Lipstick” is [Lip Gloss](https://github.com/charmbracelet/lipgloss). It handles styling, Unicode cell measurements, borders, composition, and layout utilities. [Bubble Tea](https://github.com/charmbracelet/bubbletea) supplies the application event loop, keyboard/mouse events, resizing, terminal modes, and rendering. Neither is a system metrics collector.

The [Lip Gloss v2 beta 2 announcement](https://charm.land/blog/lipgloss-v2-beta-2/) describes the layer compositing feature that inspired this project. [Charm's v2 announcement](https://charm.land/blog/v2/) confirms the libraries left beta on 23 February 2026. The repository's branch literally named [`beta`](https://github.com/charmbracelet/lipgloss/tree/beta) was also checked; its old module path and dependency set are not the current stable v2 API. The project therefore pins **Lip Gloss v2.0.6** and **Bubble Tea v2.1.0**, using their `charm.land/.../v2` module paths rather than a moving beta branch.

There is an API difference worth preserving: early examples use `NewCanvas(layers...)`. The pinned release uses a sized `NewCanvas(width, height)` and `Compose`. Mosaic composes a `NewCompositor` into that bounded canvas, so layers clip to the terminal. Mouse coordinates are mapped to the same computed rectangles as rendering; header/footer cover cards that extend beyond the scrolling viewport. Explicit modal state blocks interactions with underlying cards.

[gopsutil v4](https://github.com/shirou/gopsutil) provides common host, CPU, memory, disk, network, and process APIs across the three requested OS families. The pinned source was audited for each OS; [telemetry.md](telemetry.md) records the native APIs, semantics and gaps. GPU adds separate driver/OS adapters with optional values, bounded commands and source labels. UI code does not parse Linux `/proc`, localized shell tables or driver output.

## Inspiration

| Application | Useful pattern | Decision for Mosaic |
| --- | --- | --- |
| [btop](https://github.com/aristocratos/btop) | Rounded resource panels, colored usage graphs, mouse actions | Use rounded cards, bars, sparklines and visible focus; retain equal card sizes |
| [bottom](https://github.com/ClementTsang/bottom) | Cross-platform widgets, expanded views, process searching and sorting | Expand one card, filter by name/PID, keep process selection stable during refresh |
| [htop](https://github.com/htop-dev/htop) | Dense process table and deliberate process actions | Show PID/CPU/RSS, inspect before acting, require an explicit confirmation |

These inform the interaction patterns, rather than prescribing the geometry. btop's main codebase is not a Windows portability model; bottom and the selected Go collector better match that requirement.

## Geometry

The default design budgets two header and two footer rows, one-cell side margins, two-column gutters, and one-row gutters. Minimum cards are 32 columns wide when multiple columns fit, and six rows high. A single column can shrink further. At 80×24 this yields two columns of 38×6 cards, arranged in three rows with no scrolling. At 120×30 it yields three columns of 38×12 cards.

Card size uses integer division, with leftover cells absorbed by gaps or margins. All six grid cards share a single width and height. A column breakpoint trades card width for fewer rows: 68 columns allows two columns; 102 allows three. Inside each range, available width and height grow the boxes together. Detached detail windows can be resized independently; docking restores the equal-card grid. Short terminals keep readable cards and scroll rather than squeeze every panel below its useful height.

This is deliberately a layout solver, separate from styles and telemetry. It makes resize behavior testable without a terminal and preserves keyboard/mouse agreement. Focus follows the same panel across breakpoints. Expanded views use all available content space; returning to the grid restores its focused card.

## Interaction flow

1. Start on the live six-card overview. Select with arrows or Tab; Enter/click opens the card. Number keys open a panel directly.
2. Expanded resource views scroll through extra fields, cores, mounts or interfaces. Esc returns to the overview. Tab changes panels without collapsing the detail view.
   Drag a card title or press `f` to detach a window. Its title moves it, any of its four borders or corners resizes it, `m` maximizes it, and `d` docks the workspace. Raising windows updates focus and hit-test order together. Header/footer remain above floating content; dialogs remain above the workspace.
3. Processes opens a sorted table. `/` edits a filter; typing—including `q`—changes the filter instead of executing shortcuts. Enter/Esc finishes editing. Selection follows PID plus start time across sorting and refresh.
4. Enter/click on a process opens a layered inspection dialog. Missing permissions or exited/reused processes produce a visible result.
5. `x` opens a confirmation dialog; `y` requests termination. `n`/Esc cancels. No default affirmative action, force-kill, priority changes, suspend, or resume operations are included.

“Basic things” in other panels currently means inspect, scroll, pause, change sampling frequency, and refresh. System-wide mutations such as freeing memory or unmounting disks are outside this first implementation.

## Reference, typography and color refinement

The second refinement studies Charm's actual [layout example](https://github.com/charmbracelet/lipgloss/blob/19f011f1cc573e5d2bf9d7a9c4f14d429647759e/examples/layout/main.go), [blended border example](https://github.com/charmbracelet/lipgloss/blob/19f011f1cc573e5d2bf9d7a9c4f14d429647759e/examples/blending/border-blend-rotation/bubbletea/main.go), and [brightness example](https://github.com/charmbracelet/lipgloss/blob/19f011f1cc573e5d2bf9d7a9c4f14d429647759e/examples/brightness/main.go). Mosaic adopts filled badges, blue/purple/lilac border blends and a brightness-based modal spotlight. These are actual pinned Lip Gloss v2 capabilities. The offset dark drop-shadow layers are removed. Every final canvas cell receives an explicit background, including nested ANSI runs; full graph cells use background fills so they do not inherit font seams or terminal-default black. These effects do not require a timer that redraws an otherwise idle dashboard.

The user-provided SCTT screenshot was inspected on 10 October 2026. Its dense hierarchy and overlapping detail window guided the refinements. Mosaic uses a darker plum background, blue low-utilization values, neon purple as usage rises, and red for saturation/low-space warnings. The continuous ramp is shared by utilization bars, history columns and percentages. Network throughput uses a relative blue/purple scale, avoiding a red health warning solely for high traffic.

[Fira Code](https://github.com/tonsky/FiraCode) provides monospaced contextual ligatures and good console glyph coverage. Version 6.2 Regular/Bold fonts are bundled with their OFL license for reproducible previews. ASCII `->` and `//` remain the underlying text; the font/terminal shapes their ligatures without altering the Go app's cell positions. Because the terminal owns font selection, the live app also supports ordinary monospace fonts and emits private-use progress glyphs by default at the user’s request, with `UNICODE_PROGRESS_BAR=false` for other fonts. The screenshot renderer enables `calt`/`liga`; its pixels come from terminal output parsed by pyte, not a hand-built mock UI.

Action buttons and mouse hit rectangles share a single definition. Title presses wait for release before opening, allowing an actual drag to detach the panel. Floating windows use unique panel IDs, preserve their own scroll offsets, and stay within the content bounds. A single focused window remains selected when Tab is pressed. Confirmation dialogs block underlying pointer actions; the dangerous operation always requires a separate explicit click or `y` key.

## Collection behavior and next decisions

CPU percentages derive from counter differences, excluding double-counted Linux guest time. Per-process CPU uses deltas of user+system seconds divided by elapsed wall time. Network and disk rates handle counter resets and preserve the last successful timestamp. Collection commands never overlap; timer generations prevent manual refresh and pause/resume from creating parallel sampling loops. Histories retain at most 120 samples, and the first interactive rate sample displays warmup.

The working name is **Mosaic**. Network and GPU have their own cards, making seven resource panels. The `t` picker separates a card’s identity and grid position from its content type; duplicate types are supported. Card layouts persist in the user config directory. Slower GPU providers have a separate five-second cache, timestamp and history. The natural follow-up choices are configurable themes, per-panel visibility, and separating slower filesystem/process collection from fast CPU updates after profiling on machines with thousands of processes.

## Quieter controls and Fira Code meters (11 October 2026)

The footer keeps only the actions relevant to the current view, aligned to the right with muted text and hover feedback. Help holds the full shortcut map. Window-local move/resize instruction lines and the dedicated corner marker were removed. Border presses resize floating panels and process inspectors; interior title presses move them. Each moving edge is clamped independently against its opposite edge, so hitting the minimum size or screen boundary does not unexpectedly move the window. Escape restores the gesture’s original rectangle.

The upstream [ligature specimen](https://github.com/tonsky/FiraCode/blob/master/extras/ligatures.png) verifies `->`, `::`, `//`, extendable arrows and joined line sequences. The font’s [progress specimen](https://github.com/tonsky/FiraCode/blob/master/extras/progress.png) documents a separate six-glyph meter: U+EE00/EE01/EE02 for empty start/middle/end, U+EE03/EE04/EE05 for filled start/middle/end. Mosaic now defaults to these actual glyphs; neither the terminal nor Go can reliably detect the selected font. `UNICODE_PROGRESS_BAR=false` selects thick Unicode blocks inside ASCII brackets. Both paths occupy exactly the requested character-cell width; no guessed ligatures or font-dependent mouse coordinates are used.

## Rearranging, visible keys and surfaces (11 October 2026)

The latest user feedback supersedes the outlined cards and the previous footer placement. The OK/BUSY/HIGH arrow legend and routine lower-left status messages are removed. Keyboard shortcuts now remain at the bottom left as bright keys plus muted verbs; they remain clickable. Cards use filled tonal surfaces and corner cutouts, and meters use continuous background fills rather than outlining every cell or drawing horizontal rails. Optional Fira Code U+EE00/U+EE02 caps preserve the requested bracket ends.

Credible references inspected directly: [Charm’s layout example](https://github.com/charmbracelet/lipgloss/blob/main/examples/layout/main.go), specifically its filled history panels and active/inactive button styles; [Bubbles’ help source](https://github.com/charmbracelet/bubbles/blob/main/help/help.go), specifically ShortHelpView’s separate key/description styles and width handling; [Bubble Tea’s mouse example](https://github.com/charmbracelet/bubbletea/blob/main/examples/mouse/main.go), which enables AllMotion hover reporting; and [Lazygit’s published keybindings](https://github.com/jesseduffield/lazygit/blob/master/docs/keybindings/Keybindings_en.md), which documents visible context commands, focus movement and panel actions. These are upstream maintainer sources, not generated design references. The implementation uses these patterns with Mosaic’s shared render/hit-test geometry.

Card-title dragging inserts a card at the drop target and shifts intervening cards. Pop-out is a separate real ↗ action: hover shows a highlight and hint; clicking opens or raises the unique panel window. The keyboard equivalent is f. Keyboard rearrange mode uses p, arrows, Enter to commit and Esc to restore the original order. A stable panel identity is distinct from its screen slot, so 1–6 hotkeys and telemetry retain their meaning after moving cards. Resize preserves the in-session order. Pop-outs have explicit maximize/dock controls; inspectors and termination dialogs have actions inside their own surfaces. Dangerous controls still require a separate explicit confirmation, and underlying pointer targets remain blocked by dialogs.

## Restore the reference borders and meters (11 October 2026)

The surface-only styling above overcorrected the request to remove stray lines. The rounded Lip Gloss frames and the original six-glyph Fira Code meters are restored, including the blue-to-purple-to-red fill and joined rails. The separate screenshot raster fix remains: exact 12px cell advances prevent fractional-pixel dark seams. The visible lower-left hotkeys, removed health legend, shared button hit targets and card rearrangement remain unchanged.

## Individual grid spans and quieter data (11 October 2026)

The user clarified that less noise should retain data density. Overview panels again show multiple volumes and process rows, use short names with labeled numeric columns, and communicate disk occupancy through color rather than a warning banner. Process and mount inspection retains the original full paths.

Each card now owns width and height spans over the responsive base grid. Packing scans in reading order and never overlaps cards. Effective spans clamp to the current viewport, while requested sizes survive temporary narrowing. Edge drags snap to cell pitches; Shift+arrows provide the same operation. Layout mode groups moving, resizing, title sorting, reset and cancel. Directional focus uses rectangle overlap rather than array arithmetic to navigate around multi-cell panels. The larger CPU panel reserves room for per-core readings and uses the remaining height for its chart.


The latest viewport refinement supersedes the original grid sizing: 80×24 uses three columns/three rows, and 120×30 uses four columns/two rows. Packing is bounded by the viewport, with top-left reading order as priority. Extra cards hide instead of scrolling. Requested spans and relative order survive responsive changes; additional rows are introduced in taller windows when useful. Hover paints only the edge under the pointer cyan, distinct from the selection gradient. See the current README and validation notes for defaults and measurements.

## OS reference artwork (polish-14)

Use Fastfetch's upstream ASCII catalogue and name aliases at revision
[3453de61f82e4cdd746cf60534e943a842e6b199](https://github.com/fastfetch-cli/fastfetch/tree/3453de61f82e4cdd746cf60534e943a842e6b199/src/logo/ascii).
The embedded catalogue retains all 531 artwork strings and 615 aliases, including
Windows and Linux distributions. OS/platform identity comes from the host
collector; normalized aliases match punctuation/case variants. Unknown Linux
IDs use Linux artwork. Full and small source variants fit the available space;
we do not invent replacement logos or crop a logo to force it into a small card.
Fastfetch's license is included in THIRD_PARTY_NOTICES and osart/LICENSE.
The import script and pinned revision make updates reproducible.

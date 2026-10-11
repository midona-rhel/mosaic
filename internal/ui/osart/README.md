# OS artwork

ASCII artwork from [Fastfetch](https://github.com/fastfetch-cli/fastfetch/tree/3453de61f82e4cdd746cf60534e943a842e6b199/src/logo/ascii), revision `3453de61f82e4cdd746cf60534e943a842e6b199`, used under the accompanying MIT license.

The catalogue contains 531 artworks and 615 aliases from the upstream ASCII files and alias tables. Artwork strings are preserved verbatim. Regenerate with `python3 scripts/import_os_art.py fastfetch-3453de61f82e4cdd746cf60534e943a842e6b199.tar.gz`. Mosaic removes Fastfetch color markers and applies its own palette when rendering.

`compact.json` contains static ASCII strings. Every compact mark is at most eight rows; the renderer reserves a row beneath it and never resamples the text.

All 14 compact marks are generated with [Chafa 1.14.5](https://github.com/hpjansson/chafa/tree/1.14.5) from [Simple Icons 11.0.0](https://github.com/simple-icons/simple-icons/tree/11.0.0/icons) vectors, using the bundled Fira Code Bold glyphs. Sources are vendored in `icons/` and `apple.svg` under CC0 (`SIMPLE-ICONS-LICENSE.md`).

The marks cover macOS, Windows, Arch, Ubuntu, Fedora, Linux Mint, Debian, Alpine, NixOS, Gentoo, Manjaro, openSUSE, Red Hat, and Pop!_OS. `scripts/generate_os_ascii.py` rasterizes the vectors with CairoSVG and calls Chafa with `--symbols ascii --fg-only -c none --work 9`. Eight-row marks use a 16×8 grid; smaller alternatives use 14×7 and 12×6. Only empty outer columns are trimmed. No generated characters are manually edited. The Apple output preserves the sample selected by the user.

Regeneration requires CairoSVG and Chafa 1.14.5 on PATH. `scripts/generate_apple_ascii.py` is a compatibility entry point to regenerate Apple alone. `scripts/render_chafa_gallery.py` previews every compact asset using the same Fira Code cells and colors as the terminal captures.

Larger details retain the full Fastfetch catalogue. Other distributions retain their matching small reference when it fits. Compact artwork is plain ASCII by default; `--ascii-logos` also restricts expanded artwork to ASCII. The application embeds text and needs no Chafa installation, image protocol, runtime generation or download.

"""Pixel regressions for terminal captures, independent of native host rendering."""
import unittest
import os
import pathlib
import subprocess
import pyte
from PIL import ImageColor
from pyte.screens import Char
from terminal_screen import TerminalScreen
from terminal_render import render_screen, LEFT, TOP, CELL_W, CELL_H, CELL_ASCENT


class CellBoundsTest(unittest.TestCase):
    def test_portable_meter_track_and_fill_cover_complete_cells(self):
        binary = pathlib.Path(__file__).resolve().parents[1] / 'bin' / 'mosaic'
        output = subprocess.check_output([str(binary), '--demo', '--snapshot', '--color',
                                          '--width', '120', '--height', '30'], text=True,
                                         env=dict(os.environ, UNICODE_PROGRESS_BAR='false'))
        screen = TerminalScreen(120, 30)
        pyte.Stream(screen).feed(output.rstrip('\n').replace('\n', '\r\n'))
        picture = render_screen(screen)
        filled = empty = 0
        for y in range(screen.lines):
            for x in range(screen.columns):
                cell = screen.buffer[y][x]
                if cell.data not in ('█', '░'):
                    continue
                if cell.data == '░':
                    empty += 1
                elif cell.bg != cell.fg:
                    continue  # unrelated partial graph glyph
                else:
                    filled += 1
                self.assertEqual(cell.fg, cell.bg)
                expected = ImageColor.getrgb('#' + cell.bg)
                for px in range(int(LEFT+x*CELL_W), int(LEFT+(x+1)*CELL_W)):
                    for py in range(TOP-CELL_ASCENT+y*CELL_H, TOP-CELL_ASCENT+(y+1)*CELL_H):
                        self.assertEqual(picture.getpixel((px, py)), expected)
        self.assertGreater(filled, 0)
        self.assertGreater(empty, 0)

    def test_all_graph_heights_share_a_baseline(self):
        for bold in (False, True):
            screen = TerminalScreen(8, 2)
            for x, glyph in enumerate('▁▂▃▄▅▆▇█'):
                screen.buffer[0][x] = Char(glyph, fg='538fff', bg='538fff' if x == 7 else '100e19', bold=bold)
            picture = render_screen(screen)
            background = ImageColor.getrgb('#100e19')
            for x in range(8):
                for px in range(int(LEFT+x*CELL_W), int(LEFT+(x+1)*CELL_W)):
                    colored = [y for y in range(TOP-CELL_ASCENT, TOP+CELL_H-CELL_ASCENT+2)
                               if picture.getpixel((px,y)) != background]
                    self.assertEqual(max(colored), TOP+CELL_H-CELL_ASCENT-1)

    def test_background_does_not_spill_into_adjacent_cells(self):
        screen = TerminalScreen(2, 2)
        screen.buffer[0][0] = Char(' ', fg='538fff', bg='538fff')
        picture = render_screen(screen)
        background = ImageColor.getrgb('#100e19')
        self.assertEqual(picture.getpixel((int(LEFT+CELL_W), TOP)), background)
        self.assertEqual(picture.getpixel((LEFT, TOP+CELL_H-CELL_ASCENT)), background)


if __name__ == '__main__':
    unittest.main()

"""pyte screen with xterm background-color erase for ICH and DCH.

pyte 0.8.2 erases with active attributes for EL/ECH, but inserts default cells
for ICH/DCH. Bubble Tea's renderer uses these VT operations with the active
background; emulate that behavior so previews don't invent black strips.
"""
import pyte


class TerminalScreen(pyte.Screen):
    def insert_characters(self, count=None):
        self.dirty.add(self.cursor.y)
        count = min(count or 1, max(0, self.columns-self.cursor.x))
        line = self.buffer[self.cursor.y]
        blank = self.cursor.attrs._replace(data=" ")
        for x in range(self.columns-1, self.cursor.x-1, -1):
            line[x] = line[x-count] if x >= self.cursor.x+count else blank

    def delete_characters(self, count=None):
        self.dirty.add(self.cursor.y)
        count = min(count or 1, max(0, self.columns-self.cursor.x))
        line = self.buffer[self.cursor.y]
        blank = self.cursor.attrs._replace(data=" ")
        for x in range(self.cursor.x, self.columns):
            line[x] = line[x+count] if x+count < self.columns else blank

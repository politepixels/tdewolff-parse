package parse

import (
	"testing"

	"github.com/tdewolff/test"
)

// These tests validate the incremental line/column tracking that Input now maintains
// while moving forward and backward, including across newlines and multi-byte runes.
func TestInputPositionMoveAndRewind(t *testing.T) {
	// α (2 bytes), β (2), γ (2), δ (2)
	s := "α\nβγ\nδ"
	z := NewInputString(s)

	// start
	line, col := z.Position()
	test.T(t, line, 1, "start line")
	test.T(t, col, 1, "start col")

	// move over α (one rune)
	z.MoveRune()
	line, col = z.Position()
	test.T(t, line, 1, "after α line")
	test.T(t, col, 2, "after α col")

	// cross first newline
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 2, "after first newline line")
	test.T(t, col, 1, "after first newline col")

	// mark position just after first newline
	mark := z.Pos()
	expectLine, expectCol := z.Position()

	// move over β and γ (two runes)
	z.MoveRune()
	z.MoveRune()
	line, col = z.Position()
	test.T(t, line, 2, "after βγ line")
	test.T(t, col, 3, "after βγ col")

	// cross second newline and move over δ
	z.Move(1)
	z.MoveRune()
	line, col = z.Position()
	test.T(t, line, 3, "after second newline+δ line")
	test.T(t, col, 2, "after second newline+δ col")

	// Move back to the mark using negative Move
	back := z.Pos() - mark
	z.Move(-back)
	line, col = z.Position()
	test.T(t, line, expectLine, "after negative Move line")
	test.T(t, col, expectCol, "after negative Move col")

	// Move forward again and then Rewind to mark
	z.Move(back)
	z.Rewind(mark)
	line, col = z.Position()
	test.T(t, line, expectLine, "after Rewind line")
	test.T(t, col, expectCol, "after Rewind col")

	// Reset should go back to 1:1
	z.Reset()
	line, col = z.Position()
	test.T(t, line, 1, "after Reset line")
	test.T(t, col, 1, "after Reset col")
}

// These tests validate PositionAt on fixed byte offsets over ASCII, 2-byte and 4-byte runes and newlines.
func TestInputPositionAt(t *testing.T) {
	// a (1) \n (1) æ (2) \n (1) \U00100000 (4) \n (1) b (1)
	s := "a\næ\n\U00100000\nb"
	z := NewInputString(s)

	// Offsets chosen to land on rune boundaries and after newlines
	cases := []struct {
		off      int
		wantLine int
		wantCol  int
	}{
		{0, 1, 1},  // at 'a'
		{1, 1, 2},  // between 'a' and first newline
		{2, 2, 1},  // just after first newline
		{4, 2, 2},  // just after æ
		{5, 3, 1},  // just after second newline
		{9, 3, 2},  // just after 4-byte rune
		{10, 4, 1}, // just after third newline
		{11, 4, 2}, // at 'b'+1 (end)
	}

	for _, tc := range cases {
		line, col := z.PositionAt(tc.off)
		test.T(t, line, tc.wantLine, "line at offset")
		test.T(t, col, tc.wantCol, "col at offset")
	}
}

// Only '\n' counts as a newline for Input's Position/PositionAt. Validate CR and CRLF behavior.
func TestInputPositionNewlinesVariants(t *testing.T) {
	s := "a\r\nb\n\rc\n\n"
	z := NewInputString(s)

	// Start
	line, col := z.Position()
	test.T(t, line, 1)
	test.T(t, col, 1)

	// 'a'
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 1)
	test.T(t, col, 2)

	// '\r' does NOT increment line
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 1)
	test.T(t, col, 3)

	// '\n' increments line, resets column
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 2)
	test.T(t, col, 1)

	// 'b'
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 2)
	test.T(t, col, 2)

	// '\n' -> new line 3
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 3)
	test.T(t, col, 1)

	// '\r' does NOT increment line
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 3)
	test.T(t, col, 2)

	// 'c'
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 3)
	test.T(t, col, 3)

	// '\n' -> new line 4
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 4)
	test.T(t, col, 1)

	// '\n' -> new line 5
	z.Move(1)
	line, col = z.Position()
	test.T(t, line, 5)
	test.T(t, col, 1)
}

// Rewind should clamp to [0,len-1] and recompute line/col correctly.
func TestInputRewindClampingAndConsistency(t *testing.T) {
	s := "ab\ncd"
	z := NewInputString(s)

	// Rewind before start from start=0 -> clamp to 0
	z.Rewind(-1000)
	line, col := z.Position()
	test.T(t, z.Offset(), 0)
	test.T(t, line, 1)
	test.T(t, col, 1)

	// Rewind beyond end -> clamp to len(buf)-1 which is the sentinel NULL (EOF position)
	z.Rewind(1000)
	// Expected equals PositionAt(Len()) which returns the EOF position
	expLine, expCol := z.PositionAt(z.Len())
	line, col = z.Position()
	test.T(t, line, expLine)
	test.T(t, col, expCol)

	// Now move forward a bit and set start, then test relative Rewind clamping
	z.Reset()
	z.Move(3) // at 'c'
	z.Skip()  // start=3
	// Relative rewind far negative -> clamp to 0
	z.Rewind(-1000)
	test.T(t, z.Offset(), 0)
	line, col = z.Position()
	test.T(t, line, 1)
	test.T(t, col, 1)
}

// For many offsets, Position() after moving should match PositionAt(offset).
func TestInputPositionConsistencyWithPositionAt(t *testing.T) {
	s := "A\r\nBæ\n\U00100000C\rD\n\nE"
	z := NewInputString(s)
	n := z.Len()

	for off := 0; off < n; off++ { // exclude n because Move(n) clamps to n-1
		z.Reset()
		z.Move(off)
		l1, c1 := z.Position()
		l2, c2 := z.PositionAt(off)
		if l1 != l2 || c1 != c2 {
			t.Fatalf("mismatch at off=%d: Position()=%d:%d PositionAt()=%d:%d", off, l1, c1, l2, c2)
		}
	}

	// Also check offsets >= n are clamped by PositionAt
	lEnd, cEnd := z.PositionAt(n)
	z.Reset()
	z.Move(1 << 28) // large move clamps to n-1
	lPos, cPos := z.PositionAt(z.Offset())
	// lPos/cPos reflect position at n-1; ensure PositionAt(n) does not panic and returns sane values
	test.That(t, lEnd >= 1, "end line >= 1")
	test.That(t, cEnd >= 1, "end col >= 1")
	test.T(t, lPos >= 1, true)
	test.T(t, cPos >= 1, true)
}

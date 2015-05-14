package tktext

import (
	"sort"
	"testing"
)

func poscmp(t *testing.T, got Position, wantLine, wantChar int) {
	if wantLine != got.Line || wantChar != got.Char {
		t.Errorf("got %d.%d, want %d.%d", got.Line, got.Char,
			wantLine, wantChar)
	}
}

func strcmp(t *testing.T, got, want string) {
	if want != got {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func intcmp(t *testing.T, got, want int) {
	if want != got {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestNew(t *testing.T) {
	if text := New(); text == nil {
		t.FailNow()
	}
}

func TestParse(t *testing.T) {
	text := New()
	strings := []string{"bad", "1.bad", "10000000000000000000.1",
		"1.0+10000000000000000000c", "1.0+1characters", "1.0 bad",
		"1.0 linesoup", "1.0 wordeater", "@10000000000000000000,0",
		"@0,10000000000000000000"}
	for _, pos := range strings {
		func() {
			defer func() {
				if err := recover(); err == nil {
					t.Error("Bad position did not cause panic")
				}
			}()
			text.Get("1.0", pos)
		}()
	}
}

func TestBBox(t *testing.T) {
	text := New()
	if x, y := text.BBox("1.0"); x != 0 || y != 0 {
		t.Errorf("BBox() == %d, %d; want %d, %d", x, y, 0, 0)
	}
	text.SetSize(3, 1)
	text.Insert("end", "hello\nworld")
	text.XViewScroll(2)
	text.YViewScroll(1)
	if x, y := text.BBox("1.0"); x != -2 || y != -1 {
		t.Errorf("BBox() == %d, %d; want %d, %d", x, y, -2, -1)
	}
	text.SetWrap(Char)
	text.XViewMoveTo(0)
	text.YViewMoveTo(0)
	if x, y := text.BBox("2.4"); x != 1 || y != 3 {
		t.Errorf("BBox() == %d, %d; want %d, %d", x, y, 1, 3)
	}
	text.Replace("1.0", "end", "hello\tworld")
	text.Replace("1.0", "end", "hello        world")
	if x, y := text.BBox("end"); x != 0 || y != 6 {
		t.Errorf("BBox() == %d, %d; want %d, %d", x, y, 0, 6)
	}
}

func TestCompare(t *testing.T) {
	text := New()
	text.Insert("1.0", "hello\nworld")
	if text.Compare("1.0", "end -15c") != 0 {
		t.Error("Compare did not return zero for identical indices")
	}
	if text.Compare("end linestart -1c", "1.0 +1l") >= 0 {
		t.Error("Compare did not return negative for index1 < index2")
	}
	if text.Compare("1.0 wordend", "1.5 linestart") <= 0 {
		t.Error("Compare did not return positive for index1 > index2")
	}
}

func TestCount(t *testing.T) {
	text := New()
	text.Insert("end", // Haiku by Robert Spiess
		"an aging willow--\nits image unsteady\nin the flowing stream")

	intcmp(t, text.CountChars("1.5", "1.5"), 0)
	intcmp(t, text.CountChars("2.3", "2.10"), 7)
	intcmp(t, text.CountChars("3.0", "2.18"), -1)
	intcmp(t, text.CountChars("1.0", "end"), len(text.Get("1.0", "end")))
	intcmp(t, text.CountChars("end", "1.0"), -len(text.Get("1.0", "end")))

	intcmp(t, text.CountLines("2.0", "2.18"), 0)
	intcmp(t, text.CountLines("2.0", "1.17"), -1)
	intcmp(t, text.CountLines("1.0", "3.0"), 2)
	intcmp(t, text.CountLines("end", "1.0"), -2)

	intcmp(t, text.CountDisplayLines("1.0", "end"), 2)
	text.SetSize(80, 0)
	text.SetWrap(Char)
	intcmp(t, text.CountDisplayLines("1.0", "end"), 2)
	text.SetSize(9, 0)
	intcmp(t, text.CountDisplayLines("end", "1.0"), -6)
	intcmp(t, text.CountDisplayLines("1.0", "1.9"), 0)
	intcmp(t, text.CountDisplayLines("1.9", "1.10"), 1)
	intcmp(t, text.CountDisplayLines("1.10", "1.end"), 0)
}

func TestDLineInfo(t *testing.T) {
	text := New()
	if x, y, w := text.DLineInfo("1.0"); x != 0 || y != 0 || w != 0 {
		t.Errorf("BBox() == %#v; want %d, %d, %d", []int{x, y, w}, 0, 0, 0)
	}
	text.SetSize(6, 2)
	text.SetWrap(Char)
	text.Insert("end", "hello world")
	if x, y, w := text.DLineInfo("1.3"); x != 3 || y != 0 || w != 6 {
		t.Errorf("BBox() == %#v; want %d, %d, %d", []int{x, y, w}, 3, 0, 6)
	}
	if x, y, w := text.DLineInfo("1.8"); x != 2 || y != 1 || w != 5 {
		t.Errorf("BBox() == %#v; want %d, %d, %d", []int{x, y, w}, 1, 1, 5)
	}
	if x, y, w := text.DLineInfo("1.11"); x != 5 || y != 1 || w != 5 {
		t.Errorf("BBox() == %#v; want %d, %d, %d", []int{x, y, w}, 5, 1, 5)
	}
	text.Replace("1.0", "end", "\thi")
	if x, y, w := text.DLineInfo("1.1"); x != 2 || y != 1 || w != 4 {
		t.Errorf("BBox() == %#v; want %d, %d, %d", []int{x, y, w}, 2, 1, 4)
	}
}

func TestEditModified(t *testing.T) {
	text := New()
	if text.EditGetModified() {
		t.Error("EditGetModified returned true for new buffer")
	}
	text.Insert("end", "hello")
	if !text.EditGetModified() {
		t.Error("EditGetModified returned false for changed buffer")
	}
	text.EditSetModified(false)
	if text.EditGetModified() {
		t.Error("EditGetModified returned true after flag was set to false")
	}
	text.Replace("1.0", "end", "world")
	if !text.EditGetModified() {
		t.Error("EditGetModified returned false for changed buffer")
	}
	text.EditSetModified(true)
	if !text.EditGetModified() {
		t.Error("EditGetModified returned false after flag was set to true")
	}
}

func TestGetScreenLines(t *testing.T) {
	text := New()
	lines := text.GetScreenLines()
	if len(lines) != 0 {
		t.Errorf("GetScreenLines returned %#v for sizeless buffer", lines)
	}
	text.SetSize(5, 3)
	lines = text.GetScreenLines()
	if len(lines) != 1 || lines[0] != "" {
		t.Errorf("GetScreenLines returned %#v for empty buffer", lines)
	}
	text.Insert("end", "ape\nhippopotamus\nzebra")
	lines = text.GetScreenLines()
	if len(lines) != 3 || lines[0] != "ape" || lines[1] != "hippo" ||
		lines[2] != "zebra" {
		t.Errorf("GetScreenLines returned %#v for non-wrapping buffer", lines)
	}
	text.XViewScroll(4)
	lines = text.GetScreenLines()
	if len(lines) != 3 || lines[0] != "" || lines[1] != "opota" ||
		lines[2] != "a" {
		t.Errorf("GetScreenLines returned %#v for x-scrolled buffer", lines)
	}
	text.XViewMoveTo(0)
	text.SetWrap(Char)
	lines = text.GetScreenLines()
	if len(lines) != 3 || lines[0] != "ape" || lines[1] != "hippo" ||
		lines[2] != "potam" {
		t.Errorf("GetScreenLines returned %#v for wrapping buffer", lines)
	}
	text.Insert("3.0", "\n")
	text.YViewScroll(4)
	lines = text.GetScreenLines()
	if len(lines) != 2 || lines[0] != "" || lines[1] != "zebra" {
		t.Errorf("GetScreenLines returned %#v for y-scrolled buffer", lines)
	}
}

func TestIndex(t *testing.T) {
	text := New()
	text.Insert("1.0", "hello\nworld")

	// Base line.char indices
	poscmp(t, text.Index("0.0"), 1, 0)
	poscmp(t, text.Index("1.3"), 1, 3)
	poscmp(t, text.Index("1.9"), 1, 5)
	poscmp(t, text.Index("5.0"), 2, 5)

	// Char/index count modifiers
	poscmp(t, text.Index("1.1 +0c"), 1, 1)
	poscmp(t, text.Index("1.0 -5c"), 1, 0)
	poscmp(t, text.Index("1.0 --1i + 2 chars"), 1, 3)
	poscmp(t, text.Index("1.0 + 6c"), 2, 0)
	poscmp(t, text.Index("2.0 +-1 c"), 1, 5)
	poscmp(t, text.Index("2.0+ 10 indices-1c"), 2, 4)

	// Line count modifiers
	poscmp(t, text.Index("1.2 + 1 lines"), 2, 2)
	poscmp(t, text.Index("2.2-0l"), 2, 2)
	poscmp(t, text.Index("2.2-2l"), 1, 2)
	poscmp(t, text.Index("1.5+2l"), 2, 5)
	text.Insert("1.5", " there")
	poscmp(t, text.Index("1.8+1l"), 2, 5)
	poscmp(t, text.Index("1.1+1l+10c-1l-1c"), 1, 4)

	// Line start/end modifiers
	poscmp(t, text.Index("1.5 linestart"), 1, 0)
	poscmp(t, text.Index("1.5 lineend"), 1, 11)
	poscmp(t, text.Index("2.0 lines"), 2, 0)
	poscmp(t, text.Index("2.5 linee"), 2, 5)

	// Word start/end modifiers
	poscmp(t, text.Index("1.5 wordstart"), 1, 0)
	poscmp(t, text.Index("2.2 wordend"), 2, 5)
	poscmp(t, text.Index("2.0 wordstart"), 2, 0)
	poscmp(t, text.Index("1.6 wordend"), 1, 11)

	// Chain
	poscmp(t, text.Index("1.2 linestart lineend +1c wordend -1l"), 1, 5)

	// @x,y indices
	text.Replace("1.0", "end", "hello\nworld")
	text.SetSize(3, 5)
	poscmp(t, text.Index("@-1,-1"), 1, 0)
	poscmp(t, text.Index("@0,0"), 1, 0)
	poscmp(t, text.Index("@8,2"), 2, 5)
	poscmp(t, text.Index("@0,3"), 2, 0)
	text.SetWrap(Char)
	poscmp(t, text.Index("@0,1"), 1, 3)
	poscmp(t, text.Index("@8,2"), 2, 2)
	poscmp(t, text.Index("@0,5"), 2, 3)
	text.Replace("1.0", "end", "\t\thello")
	poscmp(t, text.Index("@2,1"), 1, 0)
	poscmp(t, text.Index("@2,2"), 1, 1)
	poscmp(t, text.Index("@2,5"), 1, 3)
}

func TestGet(t *testing.T) {
	text := New()
	text.Insert("1.0", "hello")
	strcmp(t, text.Get("1.1", "1.1"), "")
	strcmp(t, text.Get("end", "1.0"), "")
	strcmp(t, text.Get("1.1", "1.4"), "ell")
	strcmp(t, text.Get("1.1", "1.end"), "ello")
	strcmp(t, text.Get("1.0", "end"), "hello")
	text.Insert("end", "\nworld")
	strcmp(t, text.Get("2.0", "end"), "world")
}

func TestInsert(t *testing.T) {
	text := New()
	text.Insert("1.0", "")
	strcmp(t, text.Get("1.0", "end"), "")
	text.Insert("1.0", "alpha")
	strcmp(t, text.Get("1.0", "end"), "alpha")
	text.Insert("1.0", "beta ")
	strcmp(t, text.Get("1.0", "end"), "beta alpha")
	text.Insert("1.5", "gamma ")
	strcmp(t, text.Get("1.0", "end"), "beta gamma alpha")
	text.Insert("2.0", " delta")
	strcmp(t, text.Get("1.0", "end"), "beta gamma alpha delta")

	text = New()
	text.Insert("1.0", "alpha\nbeta gamma\ndelta")
	strcmp(t, text.Get("1.0", "end"), "alpha\nbeta gamma\ndelta")
	text.Insert("2.5", "epsilon\nzeta ")
	strcmp(t, text.Get("1.0", "end"),
		"alpha\nbeta epsilon\nzeta gamma\ndelta")
	text.Insert("2.5", "eta ")
	strcmp(t, text.Get("1.0", "end"),
		"alpha\nbeta eta epsilon\nzeta gamma\ndelta")
}

func TestDelete(t *testing.T) {
	text := New()
	text.Insert("1.0", "chased")
	text.Delete("1.2", "1.1")
	text.Delete("1.2", "1.2")
	strcmp(t, text.Get("1.0", "end"), "chased")
	text.Delete("1.3", "1.5")
	strcmp(t, text.Get("1.0", "end"), "chad")
	text.Delete("1.0", "end")
	strcmp(t, text.Get("1.0", "end"), "")
	text.Insert("1.0", "alpha\nbeta\ngamma\ndelta")
	text.Delete("2.3", "4.3")
	strcmp(t, text.Get("1.0", "end"), "alpha\nbetta")
}

func TestReplace(t *testing.T) {
	text := New()
	text.Replace("1.0", "1.0", "hello")
	strcmp(t, text.Get("1.0", "end"), "hello")
	text.Replace("1.1", "1.4", "ipp")
	strcmp(t, text.Get("1.0", "end"), "hippo")
	text.Replace("1.4", "1.5", "o\npotamus")
	strcmp(t, text.Get("1.0", "end"), "hippo\npotamus")
	text.Replace("1.1", "2.6", "and")
	strcmp(t, text.Get("1.0", "end"), "hands")
	text.Replace("end", "1.0", " down")
	strcmp(t, text.Get("1.0", "end"), "hands down")
	text.Replace("1.0", "end", "hello\nworld")
	text.Replace("1.end", "2.0", "\t")
	strcmp(t, text.Get("1.0", "end"), "hello\tworld")
}

func TestMarkGravity(t *testing.T) {
	text := New()
	if _, err := text.MarkGetGravity("1"); err == nil {
		t.Error("MarkGetGravity did not return error for new TkText")
	}
	text.MarkSet("1", "1.0")
	if g, err := text.MarkGetGravity("1"); err == nil {
		if g != Right {
			t.Error("Default mark gravity not set to Right")
		}
	} else {
		t.Error("MarkGetGravity returned error for valid mark name")
	}
	if err := text.MarkSetGravity("1", Right); err != nil {
		t.Error("MarkSetGravity returned error for valid mark name")
	}
	if g, _ := text.MarkGetGravity("1"); g != Right {
		t.Error("MarkSetGravity did not change mark gravity")
	}
	if err := text.MarkSetGravity("2", Right); err == nil {
		t.Error("MarkSetGravity did not return error for invalid mark name")
	}

	text.Insert("1", "hello")
	poscmp(t, text.Index("1"), 1, 5)
	text.MarkSetGravity("1", Left)
	text.Insert("1", " world")
	poscmp(t, text.Index("1"), 1, 5)
}

func TestMarkNames(t *testing.T) {
	text := New()
	if len(text.MarkNames()) != 0 {
		t.Error("MarkNames returned non-empty slice for new TkText")
	}
	names1 := []string{"1", "2", "3"}
	for _, name := range names1 {
		text.MarkSet(name, "1.0")
	}
	names2 := text.MarkNames()
	sort.StringSlice(names2).Sort()
	for i := range names1 {
		if names1[i] != names2[i] {
			t.Error("MarkNames did not return correct names")
		}
	}
}

func TestMarkNextPrevious(t *testing.T) {
	text := New()
	text.Insert("1.0", "hello\nworld")
	text.MarkSet("4", "1.0")
	text.MarkSet("2", "1.5")
	text.MarkSet("3", "1.5")
	text.MarkSet("1", "2.0")

	strcmp(t, text.MarkNext("1.0"), "4")
	strcmp(t, text.MarkNext("4"), "2")
	strcmp(t, text.MarkNext("1.1"), "2")
	strcmp(t, text.MarkNext("2"), "3")
	strcmp(t, text.MarkNext("3"), "1")
	strcmp(t, text.MarkNext("1"), "")
	strcmp(t, text.MarkNext("2.1"), "")

	strcmp(t, text.MarkPrevious("1.0"), "4")
	strcmp(t, text.MarkPrevious("4"), "")
	strcmp(t, text.MarkPrevious("1.5"), "3")
	strcmp(t, text.MarkPrevious("3"), "2")
	strcmp(t, text.MarkPrevious("2"), "4")
	strcmp(t, text.MarkPrevious("2.0"), "1")
	strcmp(t, text.MarkPrevious("1"), "3")
}

func TestMarkSet(t *testing.T) {
	text := New()
	text.Insert("end", "hello")
	text.MarkSet("1", "1.1")
	text.MarkSet("2", "1.4")
	strcmp(t, text.Get("1", "2"), "ell")

	text.Insert("1.0", "\n")
	strcmp(t, text.Get("1", "2"), "ell")
	text.Insert("1.0", "\n")
	strcmp(t, text.Get("1", "2"), "ell")
	text.Insert("3.2", "y he")
	strcmp(t, text.Get("1", "2"), "ey hell")
	text.Insert("3.4", "and\n")
	strcmp(t, text.Get("1", "2"), "ey and\nhell")

	text.Delete("1.0", "2.0")
	strcmp(t, text.Get("1", "2"), "ey and\nhell")
	text.Delete("1", "3.1")
	strcmp(t, text.Get("1", "2"), "ell")
	text.Delete("1", "1+1c")
	strcmp(t, text.Get("1", "2"), "ll")
	text.Delete("1", "2")
	strcmp(t, text.Get("1", "2"), "")
	text.Delete("1.0", "end")
	strcmp(t, text.Get("1", "2"), "")
}

func TestMarkUnset(t *testing.T) {
	text := New()
	names := []string{"1", "2", "3"}
	for _, name := range names {
		text.MarkSet(name, "1.0")
	}
	text.MarkUnset()
	text.MarkUnset("1")
	text.MarkUnset("2", "3")
	for _, name := range names {
		func() {
			defer func() {
				if err := recover(); err == nil {
					t.Error("MarkUnset did not remove mark")
				}
			}()
			text.Get(name, name)
		}()
	}
}

func TestUndo(t *testing.T) {
	text := New()
	text.EditSeparator()
	if text.EditUndo() {
		t.Error("EditUndo returned true for new TkText")
	}
	if text.EditRedo() {
		t.Error("EditRedo returned true for new TkText")
	}
	text.Insert("1.0", "hello")
	if !text.EditUndo() {
		t.Error("EditUndo returned false for non-empty stack")
	}
	if text.EditUndo() {
		t.Error("EditUndo returned true for empty stack")
	}
	strcmp(t, text.Get("1.0", "end"), "")
	if !text.EditRedo() {
		t.Error("EditRedo returned false for non-empty stack")
	}
	if text.EditRedo() {
		t.Error("EditRedo returned true for empty stack")
	}
	strcmp(t, text.Get("1.0", "end"), "hello")
	text.EditUndo()
	strcmp(t, text.Get("1.0", "end"), "")

	text.Insert("1.0", "there")
	if text.EditRedo() {
		t.Error("EditRedo returned true after edit operation")
	}
	text.Insert("1.0", "hello ")
	text.Insert("end", " world")
	text.EditUndo()
	strcmp(t, text.Get("1.0", "end"), "")
	text.EditRedo()
	strcmp(t, text.Get("1.0", "end"), "hello there world")
	text.EditSeparator()
	text.EditSeparator()
	text.Delete("1.8", "1.10")
	text.Delete("1.8", "1.10")
	text.Delete("1.4", "1.8")

	text.EditUndo()
	strcmp(t, text.Get("1.0", "end"), "hello there world")
	text.EditRedo()
	strcmp(t, text.Get("1.0", "end"), "hellworld")
	text.EditUndo()
	text.EditUndo()
	strcmp(t, text.Get("1.0", "end"), "")
	text.EditRedo()
	strcmp(t, text.Get("1.0", "end"), "hello there world")
	text.EditRedo()
	strcmp(t, text.Get("1.0", "end"), "hellworld")

	text.EditReset()
	if text.EditUndo() {
		t.Error("EditUndo returned true after TkText reset")
	}
	if text.EditRedo() {
		t.Error("EditRedo returned true after TkText reset")
	}

	text = New()
	text.SetUndo(false)
	text.Insert("1.0", "hello")
	if text.EditUndo() {
		t.Error("EditUndo returned true when undo mechanism is disabled")
	}
}

func TestSee(t *testing.T) {
	text := New()
	text.Insert("end",
		"hello, world\n\n\nhello there, hi\n\nhello, hello\n\n")
	text.SetSize(3, 3)

	text.See("4.13")
	if left, right := text.XView(); left != 0.8 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.8, 1.0)
	}
	if top, bot := text.YView(); top != 0.125 || bot != 0.5 {
		t.Errorf("YView() == %f, %f; want %f, %f", top, bot, 0.125, 0.5)
	}

	text.See("4.11")
	if left, right := text.XView(); left != 11.0/15 || right != 14.0/15 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 11.0/15,
			14.0/15)
	}

	text.See("8.end")
	if left, right := text.XView(); left != 0 || right != 0.2 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 0, 2)
	}
	if top, bot := text.YView(); top != 0.75 || bot != 1 {
		t.Errorf("YView() == %f, %f; want %f, %f", top, bot, 0.75, 1.0)
	}

	text.See("1.0")
	if left, right := text.XView(); left != 0 || right != 0.2 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 0, 2)
	}
	if top, bot := text.YView(); top != 0 || bot != 0.375 {
		t.Errorf("YView() == %f, %f; want %f, %f", top, bot, 0.0, 0.375)
	}
}

func TestXView(t *testing.T) {
	text := New()
	text.XView() // Undefined, but make sure there's no panic
	text.SetSize(9, 1)
	if left, right := text.XView(); left != 0 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 1.0)
	}
	text.Insert("end", "\nampersand")
	if left, right := text.XView(); left != 0 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 1.0)
	}
	text.Insert("end", ".")
	if left, right := text.XView(); left != 0 || right != 0.9 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 0.9)
	}
	text.Replace("1.0", "end", "\thi")
	if left, right := text.XView(); left != 0 || right != 0.9 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 0.9)
	}
	text.SetTabStop(4)
	if left, right := text.XView(); left != 0 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 1.0)
	}
	text.SetTabStop(8)
	text.SetWrap(Char)
	if left, right := text.XView(); left != 0 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 1.0)
	}

	// XViewMoveTo
	text.Replace("1.0", "end", "rhinoceros")
	text.SetWrap(None)
	text.XViewMoveTo(1)
	if left, right := text.XView(); left != 1 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 1.0, 1.0)
	}
	text.XViewMoveTo(0.1)
	if left, right := text.XView(); left != 0.1 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.1, 1.0)
	}
	text.XViewMoveTo(0)
	if left, right := text.XView(); left != 0 || right != 0.9 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 0.9)
	}

	// XViewScroll
	text.XViewScroll(1)
	if left, right := text.XView(); left != 0.1 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.1, 1.0)
	}
	text.XViewScroll(10)
	if left, right := text.XView(); left != 1 || right != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 1.0, 1.0)
	}
	text.XViewScroll(-11)
	if left, right := text.XView(); left != 0 || right != 0.9 {
		t.Errorf("XView() == %f, %f; want %f, %f", left, right, 0.0, 0.9)
	}
}

func TestYView(t *testing.T) {
	text := New()
	text.YView() // Undefined, but make sure there's no panic
	text.SetSize(3, 3)
	if top, bot := text.YView(); top != 0 || bot != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", top, bot, 0.0, 1.0)
	}
	text.Insert("end", "hello\nthere\nworld")
	if top, bot := text.YView(); top != 0 || bot != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", top, bot, 0.0, 1.0)
	}
	text.SetWrap(Char)
	if top, bot := text.YView(); top != 0 || bot != 0.5 {
		t.Errorf("XView() == %f, %f; want %f, %f", top, bot, 0.0, 0.5)
	}
	text.YViewMoveTo(0.5)
	if top, bot := text.YView(); top != 0.5 || bot != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", top, bot, 0.5, 1.0)
	}
	text.YViewScroll(10)
	if top, bot := text.YView(); top != 1 || bot != 1 {
		t.Errorf("XView() == %f, %f; want %f, %f", top, bot, 1.0, 1.0)
	}
	text.YViewScroll(-10)
	if top, bot := text.YView(); top != 0 || bot != 0.5 {
		t.Errorf("XView() == %f, %f; want %f, %f", top, bot, 0.0, 0.5)
	}
}

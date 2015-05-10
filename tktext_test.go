package tktext

import "testing"

func poscmp(t *testing.T, got Position, wantRow, wantCol int) {
	if wantRow != got.Row || wantCol != got.Col {
		t.Errorf("got %d.%d, want %d.%d", got.Row, got.Col, wantRow, wantCol)
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
	strings := []string{"bad", "1.bad"}
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

func TestIndex(t *testing.T) {
	text := New()
	text.Insert("1.0", "hello\nworld")
	poscmp(t, text.Index("0.0"), 1, 0)
	poscmp(t, text.Index("1.3"), 1, 3)
	poscmp(t, text.Index("1.9"), 1, 5)
	poscmp(t, text.Index("5.0"), 2, 5)
	poscmp(t, text.Index("1.0 -5"), 1, 0)
	poscmp(t, text.Index("1.0 +3"), 1, 3)
	poscmp(t, text.Index("1.0 +6"), 2, 0)
	poscmp(t, text.Index("2.0 -1"), 1, 5)
	poscmp(t, text.Index("2.0 +9"), 2, 5)
}

func TestGet(t *testing.T) {
	text := New()
	text.Insert("1.0", "hello")
	strcmp(t, text.Get("1.1", "1.1").String(), "")
	strcmp(t, text.Get("1.1", "1.4").String(), "ell")
	strcmp(t, text.Get("1.1", "1.end").String(), "ello")
	strcmp(t, text.Get("1.0", "end").String(), "hello")
	text.Insert("end", "\nworld")
	strcmp(t, text.Get("2.0", "end").String(), "world")
}

func TestInsert(t *testing.T) {
	text := New()
	text.Insert("1.0", "")
	strcmp(t, text.Get("1.0", "end").String(), "")
	text.Insert("1.0", "alpha")
	strcmp(t, text.Get("1.0", "end").String(), "alpha")
	text.Insert("1.0", "beta ")
	strcmp(t, text.Get("1.0", "end").String(), "beta alpha")
	text.Insert("1.5", "gamma ")
	strcmp(t, text.Get("1.0", "end").String(), "beta gamma alpha")
	text.Insert("2.0", " delta")
	strcmp(t, text.Get("1.0", "end").String(), "beta gamma alpha delta")

	text = New()
	text.Insert("1.0", "alpha\nbeta gamma\ndelta")
	strcmp(t, text.Get("1.0", "end").String(), "alpha\nbeta gamma\ndelta")
	text.Insert("2.5", "epsilon\nzeta ")
	strcmp(t, text.Get("1.0", "end").String(),
		"alpha\nbeta epsilon\nzeta gamma\ndelta")
	text.Insert("2.5", "eta ")
	strcmp(t, text.Get("1.0", "end").String(),
		"alpha\nbeta eta epsilon\nzeta gamma\ndelta")
}

func TestDelete(t *testing.T) {
	text := New()
	text.Insert("1.0", "chased")
	text.Delete("1.2", "1.2")
	strcmp(t, text.Get("1.0", "end").String(), "chased")
	text.Delete("1.3", "1.5")
	strcmp(t, text.Get("1.0", "end").String(), "chad")
	text.Delete("1.0", "end")
	strcmp(t, text.Get("1.0", "end").String(), "")
	text.Insert("1.0", "alpha\nbeta\ngamma\ndelta")
	text.Delete("2.3", "4.3")
	strcmp(t, text.Get("1.0", "end").String(), "alpha\nbetta")
}

func TestReplace(t *testing.T) {
	text := New()
	text.Replace("1.0", "1.0", "hello")
	strcmp(t, text.Get("1.0", "end").String(), "hello")
	text.Replace("1.1", "1.4", "ipp")
	strcmp(t, text.Get("1.0", "end").String(), "hippo")
	text.Replace("1.4", "1.5", "o\npotamus")
	strcmp(t, text.Get("1.0", "end").String(), "hippo\npotamus")
	text.Replace("1.1", "2.6", "and")
	strcmp(t, text.Get("1.0", "end").String(), "hands")
}

func TestMarkSet(t *testing.T) {
	text := New()
	text.Insert("end", "hello")
	text.MarkSet("1", "1.1")
	text.MarkSet("2", "1.4")
	strcmp(t, text.Get("1", "2").String(), "ell")

	text.Insert("1.0", "\n")
	strcmp(t, text.Get("1", "2").String(), "ell")
	text.Insert("1.0", "\n")
	strcmp(t, text.Get("1", "2").String(), "ell")
	text.Insert("3.2", "y he")
	strcmp(t, text.Get("1", "2").String(), "ey hell")
	text.Insert("3.4", "and\n")
	strcmp(t, text.Get("1", "2").String(), "ey and\nhell")

	text.Delete("1.0", "2.0")
	strcmp(t, text.Get("1", "2").String(), "ey and\nhell")
	text.Delete("1", "3.1")
	strcmp(t, text.Get("1", "2").String(), "ell")
	text.Delete("1", "2")
	strcmp(t, text.Get("1", "2").String(), "")
	text.Delete("1.0", "end")
	strcmp(t, text.Get("1", "2").String(), "")
}

func TestMarkUnset(t *testing.T) {
	text := New()
	text.MarkSet("1", "1.0")
	text.MarkUnset("1")
	defer func() {
		if err := recover(); err == nil {
			t.Error("MarkUnset did not remove mark")
		}
	}()
	text.Get("1", "1")
}

func TestNumLines(t *testing.T) {
	text := New()
	intcmp(t, text.NumLines(), 1)
	text.Insert("1.0", "hello\nworld\n")
	intcmp(t, text.NumLines(), 3)
}

func TestUndo(t *testing.T) {
	text := New()
	text.MarkSet("mark", "end")
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
	strcmp(t, text.Get("1.0", "end").String(), "")
	if !text.EditRedo() {
		t.Error("EditRedo returned false for non-empty stack")
	}
	if text.EditRedo() {
		t.Error("EditRedo returned true for empty stack")
	}
	strcmp(t, text.Get("1.0", "end").String(), "hello")
	text.EditUndo()
	strcmp(t, text.Get("1.0", "end").String(), "")

	text.Insert("1.0", "there")
	if text.EditRedo() {
		t.Error("EditRedo returned true after edit operation")
	}
	text.Insert("1.0", "hello ")
	text.Insert("end", " world")
	text.EditUndo("mark")
	strcmp(t, text.Get("1.0", "end").String(), "")
	text.EditRedo("mark")
	strcmp(t, text.Get("1.0", "end").String(), "hello there world")
	text.EditSeparator()
	text.EditSeparator()
	text.Delete("1.8", "1.10")
	text.Delete("1.8", "1.10")
	text.Delete("1.4", "1.8")
	text.EditUndo("mark")
	strcmp(t, text.Get("1.0", "end").String(), "hello there world")
	text.EditRedo("mark")
	strcmp(t, text.Get("1.0", "end").String(), "hellworld")
	text.EditUndo()
	text.EditUndo()
	strcmp(t, text.Get("1.0", "end").String(), "")
	text.EditRedo()
	strcmp(t, text.Get("1.0", "end").String(), "hello there world")
	text.EditRedo()
	strcmp(t, text.Get("1.0", "end").String(), "hellworld")

	text.EditReset()
	if text.EditUndo() {
		t.Error("EditUndo returned true after TkText reset")
	}
	if text.EditRedo() {
		t.Error("EditRedo returned true after TkText reset")
	}
}

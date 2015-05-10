// Package tktext implements a text-editing buffer with an interface like that
// of the Tcl/Tk text widget. The buffer is thread-safe.
package tktext

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var indexRegexp = regexp.MustCompile(`(\d+)\.(\w+)`)

// Position represents a position in a text buffer.
type Position struct {
	Row, Col int
}

// String returns a string representation of the position that can be used as
// an index in buffer functions.
func (p Position) String() string {
	return fmt.Sprintf("%d.%d", p.Row, p.Col)
}

type insertOp struct {
	sp, ep, s string
}

type deleteOp struct {
	sp, ep, s string
}

type separator bool

// Text represents a text buffer.
type Text struct {
	lines, undoStack, redoStack *list.List
	marks                       map[string]*Position
	mutex                       *sync.RWMutex
}

// New returns an initialized Text object.
func New() *Text {
	b := Text{list.New(), list.New(), list.New(), make(map[string]*Position),
		&sync.RWMutex{}}
	b.lines.PushBack("")
	return &b
}

func mustParseInt(s string) int {
	n, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		panic(err)
	}
	return int(n)
}

func (t *Text) getLine(n int) *list.Element {
	i, line := 1, t.lines.Front()
	for i < n {
		line = line.Next()
		i++
	}
	return line
}

// Index returns the row and column numbers of an index into b.
func (t *Text) Index(index string) Position {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var pos Position
	words := strings.Split(index, " ")

	// Parse initial index
	if words[0] == "end" {
		// End keyword
		pos.Row = t.lines.Len()
		pos.Col = len(t.lines.Back().Value.(string))
	} else if markPos, ok := t.marks[words[0]]; ok {
		// Marks
		pos.Row = markPos.Row
		pos.Col = markPos.Col
	} else {
		// Match "row.col" format
		matches := indexRegexp.FindStringSubmatch(words[0])
		if matches == nil {
			panic(errors.New(fmt.Sprintf("Bad index: %#v", index)))
		}

		// Parse row
		pos.Row = mustParseInt(matches[1])
		if pos.Row < 1 {
			pos.Row = 1
			pos.Col = 0
		} else if pos.Row > t.lines.Len() {
			pos.Row = t.lines.Len()
			pos.Col = len(t.lines.Back().Value.(string))
		} else {
			// Parse col
			length := len(t.getLine(pos.Row).Value.(string))
			if matches[2] == "end" {
				pos.Col = length
			} else {
				pos.Col = mustParseInt(matches[2])
				if pos.Col > length {
					pos.Col = length
				}
			}
		}
	}

	// Parse offsets
	for _, word := range words[1:] {
		// Keep in mind that a newline counts as a character
		offset := mustParseInt(word)
		if offset >= 0 {
			line := t.getLine(pos.Row)
			length := len(line.Value.(string))
			for offset+pos.Col > length && line.Next() != nil {
				offset -= length - pos.Col + 1
				pos.Row++
				pos.Col = 0
				line = line.Next()
				length = len(line.Value.(string))
			}
			if offset+pos.Col <= length {
				pos.Col += offset
			} else {
				pos.Col = length
			}
		} else {
			offset = -offset
			for offset > pos.Col && pos.Row > 1 {
				offset -= pos.Col + 1
				pos.Row--
				pos.Col = len(t.getLine(pos.Row).Value.(string))
			}
			if offset <= pos.Col {
				pos.Col -= offset
			} else {
				pos.Col = 0
			}
		}
	}

	return pos
}

// Get returns the text from start to end indices in b.
func (t *Text) Get(startIndex, endIndex string) *bytes.Buffer {
	// Parse indices
	start := t.Index(startIndex)
	end := t.Index(endIndex)

	t.mutex.RLock()
	defer t.mutex.RUnlock()

	// Find starting line
	i, line := 1, t.lines.Front()
	for i < start.Row {
		line = line.Next()
		i++
	}

	// Write text to buffer
	var text bytes.Buffer
	for i <= end.Row {
		if i != start.Row {
			text.WriteString("\n")
		}
		s := line.Value.(string)
		if i == start.Row {
			if i == end.Row {
				text.WriteString(s[start.Col:end.Col])
			} else {
				text.WriteString(s[start.Col:])
			}
		} else if i == end.Row {
			text.WriteString(s[:end.Col])
		} else {
			text.WriteString(s)
		}
		line = line.Next()
		i++
	}

	return &text
}

func (t *Text) del(startIndex, endIndex string, undo bool) {
	// Parse indices
	start := t.Index(startIndex)
	end := t.Index(endIndex)

	t.mutex.Lock()

	// Find starting line
	i, line := 1, t.lines.Front()
	for i < start.Row {
		line = line.Next()
		i++
	}

	// Delete text
	b := &bytes.Buffer{}
	for i <= end.Row {
		if i == start.Row {
			s := line.Value.(string)
			if i == end.Row {
				line.Value = s[:start.Col] + s[end.Col:]
				b.WriteString(s[start.Col:end.Col])
			} else {
				line.Value = s[:start.Col]
				b.WriteString(s[start.Col:] + "\n")
			}
		} else if i == end.Row {
			endLine := line.Next()
			line.Value = line.Value.(string) + endLine.Value.(string)[end.Col:]
			b.WriteString(endLine.Value.(string)[:end.Col])
			t.lines.Remove(endLine)
		} else {
			next := line.Next()
			b.WriteString(next.Value.(string) + "\n")
			t.lines.Remove(next)
		}
		i++
	}

	// Update marks
	for _, pos := range t.marks {
		if pos.Row == end.Row && pos.Col >= end.Col {
			pos.Col += start.Col - end.Col
		} else if pos.Row == start.Row && pos.Col >= start.Col {
			pos.Col = start.Col
		}
		if start.Row != end.Row &&
			((pos.Row == start.Row && pos.Col > start.Col) ||
				(pos.Row > start.Row && pos.Row < end.Row) ||
				(pos.Row == end.Row && pos.Col < end.Col)) {
			pos.Col = start.Col
		}
		if pos.Row >= end.Row {
			pos.Row -= end.Row - start.Row
		}
	}

	t.mutex.Unlock()

	if undo {
		sp := start.String()
		ep := end.String()
		t.mutex.Lock()
		front := t.undoStack.Front()
		collapsed := false
		if front != nil {
			switch v := front.Value.(type) {
			case deleteOp:
				if v.sp == sp {
					ep = fmt.Sprintf("%s +%d", ep, len(v.s))
					front.Value = deleteOp{sp, ep, v.s + b.String()}
					collapsed = true
				} else if v.sp == ep {
					ep = fmt.Sprintf("%s +%d", ep, len(v.s))
					front.Value = deleteOp{sp, ep, b.String() + v.s}
					collapsed = true
				}
			}
		}
		if !collapsed {
			t.undoStack.PushFront(deleteOp{sp, ep, b.String()})
		}
		t.mutex.Unlock()
	}
}

// Delete deletes the text from start to end indices in b.
func (t *Text) Delete(startIndex, endIndex string) {
	if t.Index(startIndex) != t.Index(endIndex) {
		t.del(startIndex, endIndex, true)
		t.mutex.Lock()
		t.redoStack.Init()
		t.mutex.Unlock()
	}
}

func (t *Text) insert(index, s string, undo bool) {
	start := t.Index(index)

	t.mutex.Lock()

	// Find insert index
	i, line := 1, t.lines.Front()
	for i < start.Row && line.Next() != nil {
		line = line.Next()
		i++
	}

	// Insert lines
	startLine := line
	lines := strings.Split(s, "\n")
	for _, insertLine := range lines {
		line = t.lines.InsertAfter(insertLine, line)
	}

	// Update marks
	for _, pos := range t.marks {
		if pos.Row > start.Row {
			pos.Row += len(lines) - 1
		} else if pos.Row == start.Row && pos.Col >= start.Col {
			pos.Row += len(lines) - 1
			if len(lines) == 1 {
				pos.Col += len(s)
			} else {
				pos.Col += len(line.Value.(string)) - start.Col
			}
		}
	}

	// Splice initial line together with inserted lines
	line.Value = line.Value.(string) + startLine.Value.(string)[start.Col:]
	startLine.Value = startLine.Value.(string)[:start.Col] +
		t.lines.Remove(startLine.Next()).(string)

	t.mutex.Unlock()

	if undo {
		sp := start.String()
		end := t.Index(fmt.Sprintf("%s +%d", start.String(), len(s)))
		ep := end.String()
		t.mutex.Lock()
		front := t.undoStack.Front()
		collapsed := false
		if front != nil {
			switch v := front.Value.(type) {
			case insertOp:
				if v.ep == sp {
					front.Value = insertOp{v.sp, ep, v.s + s}
					collapsed = true
				} else if v.sp == sp {
					t.mutex.Unlock()
					end = t.Index(fmt.Sprintf("%s +%d", index, len(s+v.s)))
					t.mutex.Lock()
					ep = end.String()
					front.Value = insertOp{sp, ep, s + v.s}
					collapsed = true
				}
			}
		}
		if !collapsed {
			t.undoStack.PushFront(insertOp{sp, ep, s})
		}
		t.mutex.Unlock()
	}
}

// Insert inserts text at an index in b.
func (t *Text) Insert(index, s string) {
	if s != "" {
		t.insert(index, s, true)
		t.mutex.Lock()
		t.redoStack.Init()
		t.mutex.Unlock()
	}
}

// Replace replaces the text from start to end indices in b with string s.
func (t *Text) Replace(startIndex, endIndex, s string) {
	t.Delete(startIndex, endIndex)
	t.Insert(startIndex, s)
}

// MarkSet associates a name with an index into b. The name must not contain a
// space character.
func (t *Text) MarkSet(name, index string) {
	pos := t.Index(index)
	t.mutex.Lock()
	t.marks[name] = &pos
	t.mutex.Unlock()
}

// MarkUnset removes a mark from b.
func (t *Text) MarkUnset(name string) {
	t.mutex.Lock()
	delete(t.marks, name)
	t.mutex.Unlock()
}

// NumLines returns the number of lines of text in the buffer.
func (t *Text) NumLines() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.lines.Len()
}

// EditUndo undoes changes to the buffer until a separator is encountered or
// the undo stack is empty. Undone changes are pushed onto the redo stack. The
// given marks are placed at position of the redone operation.
func (t *Text) EditUndo(marks ...string) bool {
	i, loop := 0, true
	for loop {
		t.mutex.RLock()
		front := t.undoStack.Front()
		t.mutex.RUnlock()
		if front == nil {
			break
		}
		switch v := front.Value.(type) {
		case separator:
			if i != 0 {
				loop = false
			}
		case insertOp:
			t.del(v.sp, v.ep, false)
			pos := t.Index(v.sp)
			t.mutex.Lock()
			for _, k := range marks {
				t.marks[k] = &Position{pos.Row, pos.Col}
			}
			t.mutex.Unlock()
		case deleteOp:
			t.insert(v.sp, v.s, false)
			pos := t.Index(v.ep)
			t.mutex.Lock()
			for _, k := range marks {
				t.marks[k] = &Position{pos.Row, pos.Col}
			}
			t.mutex.Unlock()
		}
		if loop {
			t.mutex.Lock()
			t.redoStack.PushFront(t.undoStack.Remove(front))
			t.mutex.Unlock()
			i++
		}
	}
	return i > 0
}

// EditRedo redoes changes to the buffer until a separator is encountered or
// the undo stack is empty. Redone changes are pushed onto the undo stack. The
// given marks are placed at position of the redone operation.
func (t *Text) EditRedo(marks ...string) bool {
	i, loop, redone := 0, true, false
	for loop {
		t.mutex.RLock()
		front := t.redoStack.Front()
		t.mutex.RUnlock()
		if front == nil {
			break
		}
		switch v := front.Value.(type) {
		case separator:
			if i != 0 {
				loop = false
			}
		case insertOp:
			t.insert(v.sp, v.s, false)
			pos := t.Index(v.ep)
			t.mutex.Lock()
			for _, k := range marks {
				t.marks[k] = &Position{pos.Row, pos.Col}
			}
			t.mutex.Unlock()
			redone = true
		case deleteOp:
			t.del(v.sp, v.ep, false)
			pos := t.Index(v.sp)
			t.mutex.Lock()
			for _, k := range marks {
				t.marks[k] = &Position{pos.Row, pos.Col}
			}
			t.mutex.Unlock()
			redone = true
		}
		if loop {
			t.mutex.Lock()
			t.undoStack.PushFront(t.redoStack.Remove(front))
			t.mutex.Unlock()
			i++
		}
	}
	return redone
}

// EditSeparator pushes an edit separator onto the undo stack if a separator is
// not already on top.
func (t *Text) EditSeparator() {
	t.mutex.Lock()
	front := t.undoStack.Front()
	var sep separator
	if front != nil {
		switch front.Value.(type) {
		case separator:
			// Do nothing
		default:
			t.undoStack.PushFront(sep)
		}
	}
	t.mutex.Unlock()
}

// EditReset clears the buffer's undo and redo stacks.
func (t *Text) EditReset() {
	t.mutex.Lock()
	t.undoStack.Init()
	t.redoStack.Init()
	t.mutex.Unlock()
}

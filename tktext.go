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

var lineCharRegexp = regexp.MustCompile(`^(\d+)\.(\w+)`)
var countRegexp = regexp.MustCompile(`^ ?([+-]) ?(-?\d+) ?([cil]\w*)`)

// Position represents a position in a text buffer.
type Position struct {
	Line, Char int
}

// String returns a string representation of the position that can be used as
// an index in buffer functions.
func (p Position) String() string {
	return fmt.Sprintf("%d.%d", p.Line, p.Char)
}

type insertOp struct {
	sp, ep, s string
}

type deleteOp struct {
	sp, ep, s string
}

type separator bool

// TkText represents a text buffer.
type TkText struct {
	lines, undoStack, redoStack *list.List
	marks                       map[string]*Position
	mutex                       *sync.RWMutex
}

// New returns an initialized TkText object.
func New() *TkText {
	b := TkText{list.New(), list.New(), list.New(), make(map[string]*Position),
		&sync.RWMutex{}}
	b.lines.PushBack("")
	return &b
}

func (t *TkText) getLine(n int) *list.Element {
	i, line := 1, t.lines.Front()
	for i < n {
		line = line.Next()
		i++
	}
	return line
}

func (t *TkText) parseLineChar(index string) (Position, int, error) {
	var pos Position

	// Match <line>.<char> format
	match := lineCharRegexp.FindStringSubmatch(index)
	if match == nil {
		err := errors.New("Bad line.char index: " + index)
		return Position{}, 0, err
	}

	// Parse line
	if line, err := strconv.ParseInt(match[1], 10, 0); err == nil {
		pos.Line = int(line)
	} else {
		return Position{}, 0, err
	}
	if pos.Line < 1 {
		pos.Line = 1
		pos.Char = 0
	} else if pos.Line > t.lines.Len() {
		pos.Line = t.lines.Len()
		pos.Char = len(t.lines.Back().Value.(string))
	} else {
		// Parse char
		length := len(t.getLine(pos.Line).Value.(string))
		if match[2] == "end" {
			pos.Char = length
		} else {
			if char, err := strconv.ParseInt(match[2], 10, 0); err == nil {
				pos.Char = int(char)
			} else {
				return Position{}, 0, err
			}
			if pos.Char > length {
				pos.Char = length
			}
		}
	}

	return pos, len(match[0]), nil
}

// Index parses a string index and returns an equivalent Position. If the index
// is badly formed, panic.
func (t *TkText) Index(index string) Position {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var pos Position

	// Todo list -- don't remove until they're tested
	// TODO: Support linestart, lineend, wordstart, wordend modifiers
	// TODO: Allow unambiguous abbreviation of modifier words

	// Parse base
	if lineCharPos, length, err := t.parseLineChar(index); err == nil {
		// <line>.<char>
		pos = lineCharPos
		index = index[length:]
	} else if strings.HasPrefix(index, "end") {
		// end
		pos.Line = t.lines.Len()
		pos.Char = len(t.lines.Back().Value.(string))
		index = index[3:]
	} else {
		// <mark>
		for mark, markPos := range t.marks {
			if strings.HasPrefix(index, mark) {
				pos = *markPos
				index = index[len(mark):]
				break
			}
		}
	}

	if pos.Line == 0 {
		panic(errors.New("Bad index base: " + index))
	}

	// Parse modifiers
	for index != "" {
		if match := countRegexp.FindStringSubmatch(index); match != nil {
			// +/- <count> chars/indices/lines
			index = index[len(match[0]):]
			n, err := strconv.ParseInt(match[2], 10, 0)
			if err != nil {
				panic(err)
			}
			delta := int(n)
			if match[1] == "-" {
				delta = -delta
			}
			if strings.HasPrefix("chars", match[3]) ||
				strings.HasPrefix("indices", match[3]) {
				if delta >= 0 {
					line := t.getLine(pos.Line)
					length := len(line.Value.(string))
					for delta+pos.Char > length && line.Next() != nil {
						delta -= length - pos.Char + 1
						pos.Line++
						pos.Char = 0
						line = line.Next()
						length = len(line.Value.(string))
					}
					if delta+pos.Char <= length {
						pos.Char += delta
					} else {
						pos.Char = length
					}
				} else {
					delta = -delta
					for delta > pos.Char && pos.Line > 1 {
						delta -= pos.Char + 1
						pos.Line--
						pos.Char = len(t.getLine(pos.Line).Value.(string))
					}
					if delta <= pos.Char {
						pos.Char -= delta
					} else {
						pos.Char = 0
					}
				}
			} else if strings.HasPrefix("lines", match[3]) {
				pos.Line += delta
				if pos.Line < 1 {
					pos.Line = 1
				} else if pos.Line > t.lines.Len() {
					pos.Line = t.lines.Len()
				}
				length := len(t.getLine(pos.Line).Value.(string))
				if pos.Char >= length {
					pos.Char = length
				}
			} else {
				panic(errors.New("Bad count type: " + match[3]))
			}
		} else {
			panic(errors.New("Bad index modifier: " + index))
		}
	}

	return pos
}

// Get returns the text from start to end indices in b.
func (t *TkText) Get(startIndex, endIndex string) *bytes.Buffer {
	// Parse indices
	start := t.Index(startIndex)
	end := t.Index(endIndex)

	t.mutex.RLock()
	defer t.mutex.RUnlock()

	// Find starting line
	i, line := 1, t.lines.Front()
	for i < start.Line {
		line = line.Next()
		i++
	}

	// Write text to buffer
	var text bytes.Buffer
	for i <= end.Line {
		if i != start.Line {
			text.WriteString("\n")
		}
		s := line.Value.(string)
		if i == start.Line {
			if i == end.Line {
				text.WriteString(s[start.Char:end.Char])
			} else {
				text.WriteString(s[start.Char:])
			}
		} else if i == end.Line {
			text.WriteString(s[:end.Char])
		} else {
			text.WriteString(s)
		}
		line = line.Next()
		i++
	}

	return &text
}

func (t *TkText) del(startIndex, endIndex string, undo bool) {
	// Parse indices
	start := t.Index(startIndex)
	end := t.Index(endIndex)

	t.mutex.Lock()

	// Find starting line
	i, line := 1, t.lines.Front()
	for i < start.Line {
		line = line.Next()
		i++
	}

	// Delete text
	b := &bytes.Buffer{}
	for i <= end.Line {
		if i == start.Line {
			s := line.Value.(string)
			if i == end.Line {
				line.Value = s[:start.Char] + s[end.Char:]
				b.WriteString(s[start.Char:end.Char])
			} else {
				line.Value = s[:start.Char]
				b.WriteString(s[start.Char:] + "\n")
			}
		} else if i == end.Line {
			endLine := line.Next()
			line.Value = line.Value.(string) +
				endLine.Value.(string)[end.Char:]
			b.WriteString(endLine.Value.(string)[:end.Char])
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
		if pos.Line == end.Line && pos.Char >= end.Char {
			pos.Char += start.Char - end.Char
		} else if pos.Line == start.Line && pos.Char >= start.Char {
			pos.Char = start.Char
		}
		if start.Line != end.Line &&
			((pos.Line == start.Line && pos.Char > start.Char) ||
				(pos.Line > start.Line && pos.Line < end.Line) ||
				(pos.Line == end.Line && pos.Char < end.Char)) {
			pos.Char = start.Char
		}
		if pos.Line >= end.Line {
			pos.Line -= end.Line - start.Line
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
					ep = fmt.Sprintf("%s +%dc", ep, len(v.s))
					front.Value = deleteOp{sp, ep, v.s + b.String()}
					collapsed = true
				} else if v.sp == ep {
					ep = fmt.Sprintf("%s +%dc", ep, len(v.s))
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
func (t *TkText) Delete(startIndex, endIndex string) {
	if t.Index(startIndex) != t.Index(endIndex) {
		t.del(startIndex, endIndex, true)
		t.mutex.Lock()
		t.redoStack.Init()
		t.mutex.Unlock()
	}
}

func (t *TkText) insert(index, s string, undo bool) {
	start := t.Index(index)

	t.mutex.Lock()

	// Find insert index
	i, line := 1, t.lines.Front()
	for i < start.Line && line.Next() != nil {
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
		if pos.Line > start.Line {
			pos.Line += len(lines) - 1
		} else if pos.Line == start.Line && pos.Char >= start.Char {
			pos.Line += len(lines) - 1
			if len(lines) == 1 {
				pos.Char += len(s)
			} else {
				pos.Char += len(line.Value.(string)) - start.Char
			}
		}
	}

	// Splice initial line together with inserted lines
	line.Value = line.Value.(string) + startLine.Value.(string)[start.Char:]
	startLine.Value = startLine.Value.(string)[:start.Char] +
		t.lines.Remove(startLine.Next()).(string)

	t.mutex.Unlock()

	if undo {
		sp := start.String()
		end := t.Index(fmt.Sprintf("%s +%dc", start.String(), len(s)))
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
					end = t.Index(fmt.Sprintf("%s +%dc", index, len(s+v.s)))
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
func (t *TkText) Insert(index, s string) {
	if s != "" {
		t.insert(index, s, true)
		t.mutex.Lock()
		t.redoStack.Init()
		t.mutex.Unlock()
	}
}

// Replace replaces the text from start to end indices in b with string s.
func (t *TkText) Replace(startIndex, endIndex, s string) {
	t.Delete(startIndex, endIndex)
	t.Insert(startIndex, s)
}

// MarkSet associates a name with an index into b. The name must not contain a
// space character.
func (t *TkText) MarkSet(name, index string) {
	pos := t.Index(index)
	t.mutex.Lock()
	t.marks[name] = &pos
	t.mutex.Unlock()
}

// MarkUnset removes a mark from b.
func (t *TkText) MarkUnset(name string) {
	t.mutex.Lock()
	delete(t.marks, name)
	t.mutex.Unlock()
}

// NumLines returns the number of lines of text in the buffer.
func (t *TkText) NumLines() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.lines.Len()
}

// EditUndo undoes changes to the buffer until a separator is encountered or
// the undo stack is empty. Undone changes are pushed onto the redo stack. The
// given marks are placed at position of the redone operation.
func (t *TkText) EditUndo(marks ...string) bool {
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
				t.marks[k] = &Position{pos.Line, pos.Char}
			}
			t.mutex.Unlock()
		case deleteOp:
			t.insert(v.sp, v.s, false)
			pos := t.Index(v.ep)
			t.mutex.Lock()
			for _, k := range marks {
				t.marks[k] = &Position{pos.Line, pos.Char}
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
func (t *TkText) EditRedo(marks ...string) bool {
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
				t.marks[k] = &Position{pos.Line, pos.Char}
			}
			t.mutex.Unlock()
			redone = true
		case deleteOp:
			t.del(v.sp, v.ep, false)
			pos := t.Index(v.sp)
			t.mutex.Lock()
			for _, k := range marks {
				t.marks[k] = &Position{pos.Line, pos.Char}
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
func (t *TkText) EditSeparator() {
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
func (t *TkText) EditReset() {
	t.mutex.Lock()
	t.undoStack.Init()
	t.redoStack.Init()
	t.mutex.Unlock()
}

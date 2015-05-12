// Package tktext implements a text-editing buffer with an API and capabilities
// like that of the Tcl/Tk text widget. The buffer is thread-safe.
//
// Note that any function that takes an index string as a parameter will panic
// if the index is badly formed. For documentation on index syntax, see
// http://www.tcl.tk/man/tcl8.5/TkCmd/text.htm#M7.
package tktext

import (
	"bytes"
	"container/list"
	"crypto/md5"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Gravity determines the behavior of a mark during insertions at its position.
// Right gravity is the default.
type Gravity uint8

const (
	Right Gravity = iota // Mark remains to right of inserted text.
	Left                 // Mark remains to left of inserted text.
)

// WrapMode determines the behavior of a text display when a line is too long
// to fit in the window.
type WrapMode uint8

const (
	None WrapMode = iota // Lines are not wrapped.
	Char                 // Wrapping line breaks may occur at any character.
	Word                 // Wrapping line breaks will not occur within a word.

)

var lineCharRegexp = regexp.MustCompile(`^(\d+)\.(\w+)`)
var countRegexp = regexp.MustCompile(`^ ?([+-]) ?(-?\d+) ?([cil]\w*)`)
var startEndRegexp = regexp.MustCompile(`^ ?(line|word)([se]\w*)`)
var wordRegexp = regexp.MustCompile(`^\w$`)

// Position denotes a position in a text buffer.
type Position struct {
	Line, Char int
}

// String returns a string representation of the Position that can be used as
// an index in TkText functions.
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

type mark struct {
	Position
	gravity Gravity
	name    string
}

type markSort []*mark

func (a markSort) Len() int      { return len(a) }
func (a markSort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func (a markSort) Less(i, j int) bool {
	if a[i].Line != a[j].Line {
		return a[i].Line < a[j].Line
	}
	if a[i].Char != a[j].Char {
		return a[i].Char < a[j].Char
	}
	return a[i].name < a[j].name
}

// TkText is a text buffer. Internally, the contents are stored as a doubly
// linked list of line strings.
type TkText struct {
	lines                *list.List
	undoStack, redoStack *list.List
	marks                map[string]*mark
	mutex                *sync.RWMutex
	undo, modified       bool
	saveEndPos           Position
	checksum             [md5.Size]byte
	width, height        int
	tabStop              int
	wrapMode             WrapMode
}

// New returns an initialized and empty TkText buffer.
func New() *TkText {
	b := TkText{
		list.New(),
		list.New(), list.New(),
		make(map[string]*mark),
		&sync.RWMutex{},
		true, false,
		Position{1, 0},
		md5.Sum([]byte{}),
		0, 0,
		8,
		None,
	}
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

func comparePos(pos1, pos2 Position) int {
	if pos1.Line != pos2.Line {
		return pos1.Line - pos2.Line
	}
	return pos1.Char - pos2.Char
}

// Compare returns a positive integer if index1 is greater than index2, a
// negative integer if index1 is less than index2, and zero if the indices are
// equal.
func (t *TkText) Compare(index1, index2 string) int {
	return comparePos(t.Index(index1), t.Index(index2))
}

// CountChars returns the number of UTF-8 characters between two indices. If
// index1 is after index2, the result will be a negative number.
func (t *TkText) CountChars(index1, index2 string) int {
	pos1, pos2 := t.Index(index1), t.Index(index2)
	reverse := comparePos(pos1, pos2) > 0
	if reverse {
		pos1, pos2 = pos2, pos1
	}
	line := t.getLine(pos1.Line)
	n := 0
	for pos1.Line < pos2.Line {
		n += len(line.Value.(string)) + 1
		pos1.Line++
		line = line.Next()
	}
	n += pos2.Char - pos1.Char
	if reverse {
		n = -n
	}
	return n
}

// CountLines returns the number of line breaks between two indices. If index1
// is after index2, the result will be a negative number (or zero).
func (t *TkText) CountLines(index1, index2 string) int {
	pos1, pos2 := t.Index(index1), t.Index(index2)
	return pos2.Line - pos1.Line
}

// Index parses a string index and returns an equivalent valid Position in the
// text buffer.
func (t *TkText) Index(index string) Position {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var pos Position

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
		// <mark> - pick the longest mark that matches the index
		prefixLen := 0
		for k, v := range t.marks {
			if strings.HasPrefix(index, k) && len(k) > prefixLen {
				pos = v.Position
				prefixLen = len(k)
				index = index[prefixLen:]
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
		} else if match := startEndRegexp.FindStringSubmatch(
			index); match != nil {
			// line/word start/end
			if match[1] == "line" {
				if strings.HasPrefix("start", match[2]) {
					pos.Char = 0
				} else if strings.HasPrefix("end", match[2]) {
					pos.Char = len(t.getLine(pos.Line).Value.(string))
				} else {
					panic(errors.New("Bad index modifier: " + index))
				}
			} else { // match[1] == "word"
				line := t.getLine(pos.Line).Value.(string)
				if strings.HasPrefix("start", match[2]) {
					for pos.Char > 0 &&
						wordRegexp.MatchString(line[pos.Char-1:pos.Char]) {
						pos.Char--
					}
				} else if strings.HasPrefix("end", match[2]) {
					for pos.Char < len(line) &&
						wordRegexp.MatchString(line[pos.Char:pos.Char+1]) {
						pos.Char++
					}
				} else {
					panic(errors.New("Bad index modifier: " + index))
				}
			}
			index = index[len(match[0]):]
		} else {
			panic(errors.New("Bad index modifier: " + index))
		}
	}

	return pos
}

// Get returns the text between two indices as a string. If index1 is after
// index2, an empty string will be returned.
func (t *TkText) Get(index1, index2 string) string {
	// Parse indices
	start, end := t.Index(index1), t.Index(index2)
	if comparePos(start, end) >= 0 {
		return ""
	}

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

	return text.String()
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
	for _, m := range t.marks {
		if comparePos(start, m.Position) <= 0 {
			if comparePos(m.Position, end) <= 0 {
				m.Position = start
			} else {
				if m.Line == end.Line && start.Line == end.Line {
					m.Char -= end.Char - start.Char
				}
				m.Line -= end.Line - start.Line
			}
		}
	}

	undo = undo && t.undo
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

// Delete deletes the text from index1 to index2. If index1 is after index2, no
// text is deleted.
func (t *TkText) Delete(index1, index2 string) {
	if t.Compare(index1, index2) < 0 {
		t.del(index1, index2, true)
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
	for _, m := range t.marks {
		if m.Line > start.Line {
			m.Line += len(lines) - 1
		} else if m.Line == start.Line && m.Char >= start.Char {
			m.Line += len(lines) - 1
			if m.gravity == Right || m.Char > start.Char {
				if len(lines) == 1 {
					m.Char += len(s)
				} else {
					m.Char += len(line.Value.(string)) - start.Char
				}
			}
		}
	}

	// Splice initial line together with inserted lines
	line.Value = line.Value.(string) + startLine.Value.(string)[start.Char:]
	startLine.Value = startLine.Value.(string)[:start.Char] +
		t.lines.Remove(startLine.Next()).(string)

	undo = undo && t.undo
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

// Insert inserts the given text at the given index.
func (t *TkText) Insert(index, s string) {
	if s != "" {
		t.insert(index, s, true)
		t.mutex.Lock()
		t.redoStack.Init()
		t.mutex.Unlock()
	}
}

// Replace replaces the text from index1 to index2 with the given text. If
// index1 is after index2, the operation is equivalent to an insertion at
// index1.
func (t *TkText) Replace(index1, index2, s string) {
	t.Delete(index1, index2)
	t.Insert(index1, s)
}

// MarkGetGravity returns the gravity of the mark with the given name, or an
// error if a mark with the given name is not set.
func (t *TkText) MarkGetGravity(name string) (Gravity, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	m := t.marks[name]
	if m == nil {
		return Right, fmt.Errorf("mark does not exist: %s", name)
	}
	return m.gravity, nil
}

// MarkSetGravity sets the gravity of the mark with the given name, or returns
// an error if a mark with the given name is not set.
func (t *TkText) MarkSetGravity(name string, direction Gravity) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	m := t.marks[name]
	if m == nil {
		return fmt.Errorf("mark does not exist: %s", name)
	}
	m.gravity = direction
	return nil
}

// MarkNames returns a slice of names of marks that are currently set.
func (t *TkText) MarkNames() []string {
	t.mutex.RLock()
	names := make([]string, len(t.marks))
	i := 0
	for k := range t.marks {
		names[i] = k
		i++
	}
	t.mutex.RUnlock()
	return names
}

func (t *TkText) sortedMarks(reverse bool) []*mark {
	marks := make([]*mark, len(t.marks))
	i := 0
	for _, m := range t.marks {
		marks[i] = m
		i++
	}
	if reverse {
		sort.Sort(sort.Reverse(markSort(marks)))
	} else {
		sort.Sort(markSort(marks))
	}
	return marks
}

// MarkNext returns the name of the next mark at or after the given index. If
// the given index is a mark, that mark will not be returned. An empty string
// is returned if no mark is found. This function can be used to step through
// all set marks in order.
func (t *TkText) MarkNext(index string) string {
	pos := t.Index(index)
	marks := t.sortedMarks(false)
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	indexIsMark := t.marks[index] != nil
	for _, m := range marks {
		if m.Line > pos.Line || (m.Line == pos.Line && (m.Char > pos.Char ||
			(m.Char == pos.Char && (!indexIsMark || m.name > index)))) {
			return m.name
		}
	}
	return ""
}

// MarkPrevious returns the name of the next mark at or before the given index.
// If the given index is a mark, that mark will not be returned. An empty
// string is returned if no mark is found. This function can be used to step
// through all set marks in reverse order.
func (t *TkText) MarkPrevious(index string) string {
	pos := t.Index(index)
	marks := t.sortedMarks(true)
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	indexIsMark := t.marks[index] != nil
	for _, m := range marks {
		if m.Line < pos.Line || (m.Line == pos.Line && (m.Char < pos.Char ||
			(m.Char == pos.Char && (!indexIsMark || m.name < index)))) {
			return m.name
		}
	}
	return ""
}

// MarkSet sets a mark with the given name at the given index. If a mark with
// the given name is already set, its position is updated.
func (t *TkText) MarkSet(name, index string) {
	pos := t.Index(index)
	t.mutex.Lock()
	t.marks[name] = &mark{pos, Right, name}
	t.mutex.Unlock()
}

// MarkUnset removes the marks with the given names. It is not an error to
// remove a mark that is not set.
func (t *TkText) MarkUnset(name ...string) {
	t.mutex.Lock()
	for _, k := range name {
		delete(t.marks, k)
	}
	t.mutex.Unlock()
}

// EditGetModified returns true if and only if the buffer contents differ from
// the last point at which the modified flag was set to false, or if the
// modified flag is set to true.
func (t *TkText) EditGetModified() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	if t.modified {
		return true
	}
	endPos := Position{t.lines.Len(), len(t.lines.Back().Value.(string))}
	if endPos != t.saveEndPos {
		return true
	}
	return t.checksum != md5.Sum([]byte(t.Get("1.0", "end")))
}

// EditSetModified sets the modified flag of the widget. If the flag is set to
// false, EditGetModified compares the buffer contents to the current state to
// determine whether the buffer was modified. If the flag is set to true,
// EditGetModified always returns true.
func (t *TkText) EditSetModified(modified bool) {
	t.mutex.Lock()
	t.modified = modified
	if !modified {
		t.saveEndPos = Position{t.lines.Len(),
			len(t.lines.Back().Value.(string))}
		t.mutex.Unlock()
		contents := t.Get("1.0", "end")
		t.mutex.Lock()
		t.checksum = md5.Sum([]byte(contents))
	}
	t.mutex.Unlock()
}

// EditUndo undoes changes to the buffer until a separator is encountered after
// at least one change, or until the undo stack is empty. Undone changes are
// pushed onto the redo stack. Returns true if and only if a change was undone.
// If the undo mechanism is disabled for the buffer, the function returns false
// with no effect.
func (t *TkText) EditUndo() bool {
	if !t.undo {
		return false
	}
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
		case deleteOp:
			t.insert(v.sp, v.s, false)
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

// EditRedo redoes changes to the buffer until a separator is encountered after
// at least one change, or until the redo stack is empty. Redone changes are
// pushed onto the undo stack. Returns true if and only if a change was redone.
// If the undo mechanism is disabled for the buffer, the function returns false
// with no effect.
func (t *TkText) EditRedo() bool {
	if !t.undo {
		return false
	}
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
			redone = true
		case deleteOp:
			t.del(v.sp, v.ep, false)
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

// EditSeparator pushes a separator onto the undo stack if a separator is not
// already on top and the stack is not empty.
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

// EditReset clears the undo and redo stacks.
func (t *TkText) EditReset() {
	t.mutex.Lock()
	t.undoStack.Init()
	t.redoStack.Init()
	t.mutex.Unlock()
}

// SetSize sets the text display's width and height in characters and lines,
// respectively.
func (t *TkText) SetSize(width, height int) {
	t.mutex.Lock()
	t.width, t.height = width, height
	t.mutex.Unlock()
}

// SetTabStop sets the width in characters of the text display's tab stops.
// The default is 8.
func (t *TkText) SetTabStop(width int) {
	t.mutex.Lock()
	t.tabStop = width
	t.mutex.Unlock()
}

// SetUndo enables or disables the undo mechanism for the buffer. The mechanism
// is enabled by default.
func (t *TkText) SetUndo(enabled bool) {
	t.mutex.Lock()
	t.undo = enabled
	t.mutex.Unlock()
}

// SetWrap sets the wrap mode of the text display. The default is None.
func (t *TkText) SetWrap(mode WrapMode) {
	t.mutex.Lock()
	t.wrapMode = mode
	t.mutex.Unlock()
}

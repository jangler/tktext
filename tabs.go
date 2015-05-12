package tktext

import "sync"

var p []rune                // Reuse the same array for efficiency's sake
var mutex = &sync.RWMutex{} // But that means we have to put a mutex on it

// Expand tabs to spaces
func expand(s string, tabStop int) string {
	mutex.Lock()
	if len(p) < tabStop*len(s) {
		p = make([]rune, tabStop*len(s))
	}
	col := 0

	for _, ch := range s {
		if ch == '\t' {
			p[col] = ' '
			col++
			for col%tabStop != 0 {
				p[col] = ' '
				col++
			}
		} else {
			p[col] = ch
			col++
		}
	}

	s = string(p[:col])
	mutex.Unlock()
	return s
}

// Return width of expanded string in columns
func columns(s string, tabStop int) int {
	col := 0
	for _, ch := range s {
		if ch == '\t' {
			col += tabStop - col%tabStop
		} else {
			col++
		}
	}
	return col
}

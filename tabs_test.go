package tktext

import "testing"

func TestExpand(t *testing.T) {
	strcmp(t, expand("hello", 8), "hello")
	strcmp(t, expand("hello\tworld", 8), "hello   world")
	strcmp(t, expand("hello\t\tworld,  response\tnil", 8),
		"hello           world,  response        nil")
}

func TestColumns(t *testing.T) {
	intcmp(t, columns("", 8), 0)
	intcmp(t, columns("hello", 8), 5)
	intcmp(t, columns("\thello", 8), 13)
	intcmp(t, columns("hello\t", 8), 8)
}

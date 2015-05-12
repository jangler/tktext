TkText
======
A Go library for text editing with an API that imitates that of the Tcl/Tk
text widget. Note that this library does *not* provide a GUI like Tk does.
Displaying the text is left up to the client, although the library provides
functions to assist drawing fixed-width text views.

Currently, this library supports equivalents to the following Tk text widget
commands:

- bbox
- compare
- count
- delete
- dlineinfo
- edit
- get
- index
- insert
- mark
- replace
- xview
- yview

Installation
------------
	go get -u github.com/jangler/tktext

Documentation
-------------
- [GoDoc](http://godoc.org/github.com/jangler/tktext)
- [Tcl/Tk text manual page](http://www.tcl.tk/man/tcl8.5/TkCmd/text.htm)

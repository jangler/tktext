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
- see
- xview
- yview

Notably missing are the search and tag commands, which I don't plan to
implement unless someone else has a use case for them.

At this point the public API should be stable, but there are probably bugs yet
to be uncovered. Performance has room for improvement, so benchmarking and
optimization are likely next steps.

Installation
------------
	go get -u github.com/jangler/tktext

Documentation
-------------
- [GoDoc](http://godoc.org/github.com/jangler/tktext)
- [Tcl/Tk text manual page](http://www.tcl.tk/man/tcl8.5/TkCmd/text.htm)

Bugs
----
Unicode is not handled correctly in some situations. See
<http://godoc.org/github.com/jangler/edit> for a similar package that handles
Unicode correctly in addition to being faster and offering built-in
functionality for syntax highlighting (but lacking some niceties of the Tk
text widget interface).

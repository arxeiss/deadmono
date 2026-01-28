package analysis

// Package represents a Go package with its dead functions.
type Package struct {
	Name  string      // declared name
	Path  string      // full import path
	Funcs []*Function // list of dead functions within it
}

// Function represents a dead function within a Go package with all details.
type Function struct {
	Name      string   // name (sans package qualifier)
	Position  Position // file/line/column of function declaration
	Generated bool     // function is declared in a generated .go file
	Marker    bool     // function is a marker interface method
}

// Position represents a position in a source file.
type Position struct {
	File      string // name of file
	Line, Col int    // line and byte index, both 1-based
}

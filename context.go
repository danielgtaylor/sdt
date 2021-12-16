package sdt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"
)

func refToPath(ref string) string {
	// Convert schema $ref to JSON/YAML Path.
	// #/foo/bar/0/baz => $.'foo'.'bar'[0].'baz'
	p := "$"
	for _, item := range strings.Split(strings.Trim(ref, "/"), "/") {
		if _, err := strconv.Atoi(item); err == nil {
			p += "[" + item + "]"
			continue
		}
		p += ".'" + item + "'"
	}
	return p
}

type ContextError interface {
	Error() string
	Message() string
	Path() string
	Offset() int
	Line() int
	Column() int
	Length() int
	Source() string
}

type contextError struct {
	err    error
	path   string
	offset int
	line   int
	column int
	length int
	source string
}

func (e *contextError) Error() string {
	return fmt.Sprintf("%s: %s\n%s", e.path, e.err, e.source)
}

func (e *contextError) Message() string {
	return e.err.Error()
}

func (e *contextError) Path() string {
	return e.path
}

func (e *contextError) Offset() int {
	return e.offset
}

func (e *contextError) Line() int {
	return e.line
}

func (e *contextError) Column() int {
	return e.column
}

func (e *contextError) Length() int {
	return e.length
}

func (e *contextError) Source() string {
	return e.source
}

type contextMeta struct {
	Errors             []ContextError
	TemplateComplexity int
}

type context struct {
	Filename string
	Path     string
	Meta     *contextMeta
	AST      *ast.File
}

func newContext(filename string, astFile *ast.File, path ...string) *context {
	return &context{
		Filename: filename,
		Path:     "/" + strings.Join(path, "/"),
		Meta:     &contextMeta{},
		AST:      astFile,
	}
}

func (c *context) WithPath(path interface{}) *context {
	return &context{
		Filename: c.Filename,
		Path:     strings.TrimRight(c.Path, "/") + "/" + fmt.Sprintf("%v", path),
		Meta:     c.Meta,
		AST:      c.AST,
	}
}

// FullPath returns the full path to the context, including the filename if
// one was given.
func (c *context) FullPath() string {
	if c.Filename != "" {
		if strings.Contains(c.Filename, "#") {
			return c.Filename + c.Path
		}
		return c.Filename + "#" + c.Path
	}
	return c.Path
}

// AddError adds an error into the rendering context at the current path. As
// a convenience it returns nil.
func (c *context) AddError(value error) interface{} {
	return c.AddErrorOffset(value, 0, 0)
}

// AddErrorOffset adds an error into the rendering context at the current path
// plus an additional offset. As a convenience it returns nil.
func (c *context) AddErrorOffset(value error, offset uint16, length uint8) interface{} {
	source := ""
	posOffset := 0
	line := 0
	col := 0
	if c.AST != nil {
		path, err := yaml.PathString(refToPath(c.Path))
		if err != nil {
			panic(err)
		}

		if node, err := path.FilterFile(c.AST); err == nil {
			switch node.GetToken().Type {
			case token.DoubleQuoteType, token.SingleQuoteType:
				// Account for quoted strings
				offset += 1
			}

			// Modify and then return the token position based on the offset.
			pos := node.GetToken().Position
			pos.Column += int(offset)

			posOffset = pos.Offset + int(offset)
			line = pos.Line
			col = pos.Column + int(offset)

			if length == 0 {
				length = uint8(len(node.GetToken().Value))
			}

			// var pp printer.Printer
			var pp Printer
			source = pp.PrintErrorToken(node.GetToken(), int(length))

			pos.Column -= int(offset)
		}
	}

	c.Meta.Errors = append(c.Meta.Errors, &contextError{
		err:    value,
		path:   c.FullPath(),
		offset: posOffset,
		line:   line,
		column: col,
		length: int(length),
		source: source,
	})
	return nil
}

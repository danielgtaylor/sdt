package sdt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/printer"
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

type contextMeta struct {
	Errors             []error
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
	return c.AddErrorOffset(value, 0)
}

// AddErrorOffset adds an error into the rendering context at the current path
// plus an additional offset. As a convenience it returns nil.
func (c *context) AddErrorOffset(value error, offset int) interface{} {
	source := ""
	if c.AST != nil {
		path, err := yaml.PathString(refToPath(c.Path))
		if err != nil {
			panic(err)
		}

		if node, err := path.FilterFile(c.AST); err == nil {
			// Modify and then return the token position based on the offset.
			pos := node.GetToken().Position
			pos.Column += offset

			var pp printer.Printer
			source = pp.PrintErrorToken(node.GetToken(), false)

			pos.Column -= offset
		}
	}

	c.Meta.Errors = append(c.Meta.Errors, fmt.Errorf("%s: %w\n%s", c.FullPath(), value, string(source)))
	return nil
}

package sdt

import (
	"fmt"
	"strings"
)

type contextMeta struct {
	Errors             []error
	TemplateComplexity int
}

type context struct {
	Filename string
	Path     string
	Meta     *contextMeta
}

func newContext(filename string, path ...string) *context {
	return &context{
		Filename: filename,
		Path:     "/" + strings.Join(path, "/"),
		Meta:     &contextMeta{},
	}
}

func (c *context) WithPath(path interface{}) *context {
	return &context{
		Filename: c.Filename,
		Path:     strings.TrimRight(c.Path, "/") + "/" + fmt.Sprintf("%v", path),
		Meta:     c.Meta,
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
func (c *context) AddError(err error) interface{} {
	c.Meta.Errors = append(c.Meta.Errors, fmt.Errorf("%s: %w", c.FullPath(), err))
	return nil
}

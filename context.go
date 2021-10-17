package sdt

import (
	"fmt"
	"strings"
)

type errors struct {
	Value []error
}

type context struct {
	Path   string
	Errors *errors
}

func newContext() *context {
	return &context{
		Path:   "/",
		Errors: &errors{},
	}
}

func (c *context) WithPath(path interface{}) *context {
	return &context{
		Path:   strings.TrimRight(c.Path, "/") + "/" + fmt.Sprintf("%v", path),
		Errors: c.Errors,
	}
}

// AddError adds an error into the rendering context at the current path. As
// a convenience it returns nil.
func (c *context) AddError(err error) interface{} {
	c.Errors.Value = append(c.Errors.Value, fmt.Errorf("%s: %w", c.Path, err))
	return nil
}

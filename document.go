package sdt

import (
	"fmt"
	"io/ioutil"
	"path"

	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
	"gopkg.in/yaml.v3"
)

// Schemas describe the structured data template's inputs and outputs.
type Schemas struct {
	// Dialect sets the default JSON Schema dialect to use when no explicit
	// $schema property is given at the root of any passed or referenced schema.
	Dialect string                 `json:"dialect" yaml:"dialect"`
	Input   map[string]interface{} `json:"input" yaml:"input"`
	Output  map[string]interface{} `json:"output" yaml:"output"`
}

// Document is a combination of input/output schemas and a template to
// render out a data structure. Context data that is described by the schema
// is passed as input when rendering.
type Document struct {
	Filename string      `json:"-" yaml:"-"`
	Schemas  *Schemas    `json:"schemas" yaml:"schemas"`
	Template interface{} `json:"template" yaml:"template"`
}

// New creates a new document.
func New(filename string) *Document {
	return &Document{
		Filename: filename,
		Schemas:  &Schemas{},
	}
}

// NewFromFile loads a document instance from a file.
func NewFromFile(filename string) (*Document, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	doc := &Document{}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}

	doc.Filename = filename

	return doc, nil
}

// Validate that input params are correct based on the schema.
func (doc *Document) ValidateInput(params map[string]interface{}) error {
	if doc.Schemas == nil || doc.Schemas.Input == nil {
		return nil
	}

	doc.Schemas.Input["type"] = "object"
	if doc.Schemas.Input["additionalProperties"] == nil {
		// Input should be strict!
		doc.Schemas.Input["additionalProperties"] = false
	}
	s, err := compileSchema(path.Join(doc.Filename, "schemas", "input"), doc.Schemas.Dialect, doc.Schemas.Input)
	if err != nil {
		return fmt.Errorf("error compiling schema: %w", err)
	}
	err = s.Validate(params)
	if err != nil {
		return fmt.Errorf("error validating params against schema: %w", err)
	}

	setDefaults(s, params)
	return nil
}

func (doc *Document) ValidateTemplate() []error {
	if doc.Schemas == nil || doc.Schemas.Input == nil {
		return []error{fmt.Errorf("input schema required")}
	}

	if doc.Schemas.Output == nil {
		return nil
	}

	doc.Schemas.Input["type"] = "object"
	sin, err := compileSchema(path.Join(doc.Filename, "schemas", "input"), doc.Schemas.Dialect, doc.Schemas.Input)
	if err != nil {
		return []error{fmt.Errorf("error validating template: %w", err)}
	}

	sout, err := compileSchema(path.Join(doc.Filename, "schemas", "output"), doc.Schemas.Dialect, doc.Schemas.Output)
	if err != nil {
		return []error{fmt.Errorf("error validating template: %w", err)}
	}

	ctx := newContext(doc.Filename, "template")
	example, err := generateExample(sin)
	if err != nil {
		return []error{fmt.Errorf("error validating template: %w", err)}
	}
	validateTemplate(ctx, sout, doc.Template, example.(map[string]interface{}))

	return ctx.Errors.Value
}

func (doc *Document) ValidateOutput(output interface{}) error {
	if doc.Schemas == nil || doc.Schemas.Output == nil {
		return nil
	}

	s, err := compileSchema(path.Join(doc.Filename, "schemas", "output"), doc.Schemas.Dialect, doc.Schemas.Output)
	if err != nil {
		return fmt.Errorf("error compiling schema: %w", err)
	}

	err = s.Validate(output)
	if err != nil {
		return fmt.Errorf("error validating output against schema: %w", err)
	}

	return nil
}

// Render the template into a data structure.
func (doc *Document) Render(params map[string]interface{}) (interface{}, []error) {
	ctx := newContext(doc.Filename, "template")

	// Built-in function definitions.
	params["append"] = func(a []interface{}, b []interface{}) []interface{} {
		return append(a, b...)
	}

	return render(ctx, doc.Template, params), ctx.Errors.Value
}

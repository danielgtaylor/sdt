package sdt

import (
	"fmt"
	"io/ioutil"
	"path"

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

	return NewFromBytes(filename, b)
}

func NewFromBytes(filename string, data []byte) (*Document, error) {
	doc := &Document{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	doc.Filename = filename

	return doc, nil
}

// ValidateInput validates that input params are correct based on the schema.
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

// ValidateTemplate validates that the template is structurally and semantically
// correct based on the given schemas.
func (doc *Document) ValidateTemplate() ([]error, []error) {
	if doc.Schemas == nil || doc.Schemas.Input == nil {
		return nil, []error{fmt.Errorf("input schema required")}
	}

	doc.Schemas.Input["type"] = "object"
	sin, err := compileSchema(path.Join(doc.Filename, "schemas", "input"), doc.Schemas.Dialect, doc.Schemas.Input)
	if err != nil {
		return nil, []error{fmt.Errorf("error validating template: %w", err)}
	}

	if doc.Schemas.Output == nil {
		return nil, nil
	}

	sout, err := compileSchema(path.Join(doc.Filename, "schemas", "output"), doc.Schemas.Dialect, doc.Schemas.Output)
	if err != nil {
		return nil, []error{fmt.Errorf("error validating template: %w", err)}
	}

	ctx := newContext(doc.Filename, "template")
	example, err := generateExample(sin)
	if err != nil {
		return nil, []error{fmt.Errorf("error validating template: %w", err)}
	}
	validateTemplate(ctx, sout, doc.Template, example.(map[string]interface{}))

	warnings := []error{}
	if ctx.Meta.TemplateComplexity > 50 {
		warnings = append(warnings, fmt.Errorf("template complexity is high: %d", ctx.Meta.TemplateComplexity))
	}

	return warnings, ctx.Meta.Errors
}

// ValidateOutput validates the rendered output against the given output schema.
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

	return render(ctx, doc.Template, params), ctx.Meta.Errors
}

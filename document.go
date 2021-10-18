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
	Schemas  *Schemas    `json:"schemas" yaml:"schemas"`
	Template interface{} `json:"template" yaml:"template"`
}

// New creates a new document.
func New() *Document {
	return &Document{
		Schemas: &Schemas{},
	}
}

// NewFromFile loads a document instance from a file.
func NewFromFile(filename string) (*Document, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	d := &Document{}
	if err := yaml.Unmarshal(b, &d); err != nil {
		return nil, err
	}

	return d, nil
}

// Validate that input params are correct based on the schema.
func (bp *Document) ValidateInput(filename string, params map[string]interface{}) error {
	if bp.Schemas == nil || bp.Schemas.Input == nil {
		return nil
	}

	bp.Schemas.Input["type"] = "object"
	if bp.Schemas.Input["additionalProperties"] == nil {
		// Input should be strict!
		bp.Schemas.Input["additionalProperties"] = false
	}
	s, err := compileSchema(path.Join(filename, "schemas", "input"), bp.Schemas.Dialect, bp.Schemas.Input)
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

func (bp *Document) ValidateTemplate(filename string) []error {
	if bp.Schemas == nil || bp.Schemas.Input == nil {
		return []error{fmt.Errorf("input schema required")}
	}

	if bp.Schemas.Output == nil {
		return nil
	}

	bp.Schemas.Input["type"] = "object"
	sin, err := compileSchema(path.Join(filename, "schemas", "input"), bp.Schemas.Dialect, bp.Schemas.Input)
	if err != nil {
		return []error{fmt.Errorf("error validating template: %w", err)}
	}

	sout, err := compileSchema(path.Join(filename, "schemas", "output"), bp.Schemas.Dialect, bp.Schemas.Output)
	if err != nil {
		return []error{fmt.Errorf("error validating template: %w", err)}
	}

	ctx := newContext()
	example, err := generateExample(sin)
	if err != nil {
		return []error{fmt.Errorf("error validating template: %w", err)}
	}
	validateTemplate(ctx, sout, bp.Template, example.(map[string]interface{}))

	return ctx.Errors.Value
}

func (bp *Document) ValidateOutput(filename string, output interface{}) error {
	if bp.Schemas == nil || bp.Schemas.Output == nil {
		return nil
	}

	s, err := compileSchema(path.Join(filename, "schemas", "output"), bp.Schemas.Dialect, bp.Schemas.Output)
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
func (bp *Document) Render(params map[string]interface{}) (interface{}, []error) {
	ctx := newContext()

	// Built-in function definitions.
	params["append"] = func(a []interface{}, b []interface{}) []interface{} {
		return append(a, b...)
	}

	return render(ctx, bp.Template, params), ctx.Errors.Value
}

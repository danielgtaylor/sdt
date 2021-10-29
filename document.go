package sdt

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/santhosh-tekuri/jsonschema/v5"
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

	inputSchema  *jsonschema.Schema
	outputSchema *jsonschema.Schema
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
	doc := New(filename)
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	return doc, nil
}

func (doc *Document) LoadSchemas() error {
	if doc.Schemas == nil {
		return nil
	}

	if doc.inputSchema == nil && doc.Schemas.Input != nil {
		doc.Schemas.Input["type"] = "object"
		if doc.Schemas.Input["additionalProperties"] == nil {
			// Input should be strict!
			doc.Schemas.Input["additionalProperties"] = false
		}
		s, err := compileSchema(path.Join(doc.Filename, "schemas", "input"), doc.Schemas.Dialect, doc.Schemas.Input)
		if err != nil {
			return fmt.Errorf("error compiling input schema: %w", err)
		}
		doc.inputSchema = s
	}

	if doc.outputSchema == nil && doc.Schemas.Output != nil {
		s, err := compileSchema(path.Join(doc.Filename, "schemas", "output"), doc.Schemas.Dialect, doc.Schemas.Output)
		if err != nil {
			return fmt.Errorf("error compiling output schema: %w", err)
		}
		doc.outputSchema = s
	}

	return nil
}

func (doc *Document) Example() (interface{}, error) {
	if doc.Schemas == nil || doc.Schemas.Input == nil {
		return nil, nil
	}
	doc.LoadSchemas()
	return generateExample(doc.inputSchema)
}

// ValidateInput validates that input params are correct based on the schema.
func (doc *Document) ValidateInput(params map[string]interface{}) error {
	if doc.Schemas == nil || doc.Schemas.Input == nil {
		return nil
	}

	doc.LoadSchemas()

	err := doc.inputSchema.Validate(params)
	if err != nil {
		return fmt.Errorf("error validating params against schema: %w", err)
	}

	setDefaults(doc.inputSchema, params)
	return nil
}

// ValidateTemplate validates that the template is structurally and semantically
// correct based on the given schemas.
func (doc *Document) ValidateTemplate() ([]error, []error) {
	if doc.Schemas == nil || doc.Schemas.Input == nil {
		return nil, []error{fmt.Errorf("input schema required")}
	}

	doc.LoadSchemas()

	if doc.Schemas.Output == nil {
		return nil, nil
	}

	ctx := newContext(doc.Filename, "template")
	example, err := generateExample(doc.inputSchema)
	if err != nil {
		return nil, []error{fmt.Errorf("error validating template: %w", err)}
	}
	validateTemplate(ctx, doc.outputSchema, doc.Template, example.(map[string]interface{}))

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

	doc.LoadSchemas()

	err := doc.outputSchema.Validate(output)
	if err != nil {
		return fmt.Errorf("error validating output against schema: %w", err)
	}

	return nil
}

// Render the template into a data structure.
func (doc *Document) Render(params map[string]interface{}) (interface{}, []error) {
	doc.LoadSchemas()
	setDefaults(doc.inputSchema, params)
	ctx := newContext(doc.Filename, "template")
	return render(ctx, doc.Template, params), ctx.Meta.Errors
}

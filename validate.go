package sdt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/danielgtaylor/mexpr"
	"github.com/santhosh-tekuri/jsonschema/v5"

	// Enable loading schemas via HTTP references.
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader"
)

func hasType(s *jsonschema.Schema, typ string) bool {
	for _, t := range s.Types {
		if t == typ {
			return true
		}
	}
	return false
}

func getKeys(m map[string]*jsonschema.Schema) []string {
	props := []string{}
	for k := range m {
		props = append(props, k)
	}
	return props
}

// getItems returns the item schema of an array, supporting multiple versions
// of JSON Schema including 2020. If items is an array, then only the first
// item schema is returned.
func getItems(s *jsonschema.Schema) *jsonschema.Schema {
	if s.Items != nil {
		if tmp, ok := s.Items.(*jsonschema.Schema); ok {
			return tmp
		} else if tmp, ok := s.Items.([]*jsonschema.Schema); ok {
			return tmp[0]
		}
	}
	if s.Items2020 != nil {
		return s.Items2020
	}
	return &jsonschema.Schema{}
}

// Copied and modified from:
// https://github.com/santhosh-tekuri/jsonschema/blob/master/draft.go
func findDraft(url string) *jsonschema.Draft {
	if strings.HasPrefix(url, "http://") {
		url = "https://" + strings.TrimPrefix(url, "http://")
	}
	if strings.HasSuffix(url, "#") || strings.HasSuffix(url, "#/") {
		url = url[:strings.IndexByte(url, '#')]
	}
	switch url {
	case "https://json-schema.org/schema":
		return jsonschema.Draft2020
	case "https://json-schema.org/draft/2020-12/schema":
		return jsonschema.Draft2020
	case "https://json-schema.org/draft/2019-09/schema":
		return jsonschema.Draft2019
	case "https://json-schema.org/draft-07/schema":
		return jsonschema.Draft7
	case "https://json-schema.org/draft-06/schema":
		return jsonschema.Draft6
	case "https://json-schema.org/draft-04/schema":
		return jsonschema.Draft4
	case "openapi-3.0":
		return jsonschema.Draft4
	case "openapi-3.1":
		// TODO: should this point to the OpenAPI specific dialect?
		// https://spec.openapis.org/oas/3.1/dialect/base
		return jsonschema.Draft2020
	}
	return nil
}

func compileSchema(filename string, dialect string, input interface{}) (*jsonschema.Schema, error) {
	// This is inefficient, but the input may be YAML so we first need to ensure
	// we are working with JSON (also why this can't use json.RawMessage).
	j, _ := json.Marshal(input)
	c := jsonschema.NewCompiler()
	if d := findDraft(dialect); d != nil {
		c.Draft = d
	}
	c.ExtractAnnotations = true
	fixed := strings.Replace(filename, "#", "/", -1)
	c.AddResource(fixed, bytes.NewReader(j))
	return c.Compile(fixed)
}

func getJSONType(value interface{}) string {
	return map[reflect.Kind]string{
		reflect.Bool:    "boolean",
		reflect.Int:     "number",
		reflect.Int64:   "number",
		reflect.Float64: "number",
		reflect.String:  "string",
		reflect.Slice:   "array",
		reflect.Map:     "object",
	}[reflect.TypeOf(value).Kind()]
}

// wrongTypeError adds an error to the context with information about the
// expected type. If it's an array or object, additional info about the
// items and properties is included to help with debugging.
func wrongTypeError(ctx *context, found string, expected *jsonschema.Schema) {
	extra := ""
	if hasType(expected, "object") {
		extra = fmt.Sprintf(" with properties %v", getKeys(expected.Properties))
	}
	if hasType(expected, "array") {
		items := getItems(expected)
		extra = fmt.Sprintf(" with %v items", items.Types)
		if hasType(items, "object") {
			extra += fmt.Sprintf(" with properties %v", getKeys(items.Properties))
		}
	}

	ctx.AddError(fmt.Errorf("error validating template: type %s not allowed, expecting %v%s", found, expected.Types, extra))
}

func validateString(ctx *context, s *jsonschema.Schema, template interface{}, paramsExample map[string]interface{}) {
	matches := interpolationRe.FindAllStringIndex(template.(string), -1)

	if len(matches) > 0 {
		for _, match := range matches {
			expr := template.(string)[match[0]+2 : match[1]-1]
			ctx.Meta.TemplateComplexity++
			_, err := mexpr.Parse(expr, paramsExample)
			if err != nil {
				ctx.AddErrorOffset(fmt.Errorf("error validating template: unable to compile expression '%s': %v", expr, err), match[0]+err.Offset()+2)
				if len(matches) == 1 && match[0] == 0 && match[1] == len(template.(string)) {
					return
				}
			}
		}
	}

	if len(matches) == 1 && matches[0][0] == 0 && matches[0][1] == len(template.(string)) {
		// This is a single value string template that can return any type.
		t := template.(string)
		out, err := mexpr.Eval(t[2:len(t)-1], paramsExample)
		if err != nil {
			ctx.AddErrorOffset(fmt.Errorf("error validating template: unable to eval expression '%s': %v", t[2:len(t)-1], err), err.Offset()+2)
			return
		}
		outJSONType := getJSONType(out)
		if !hasType(s, outJSONType) {
			if outJSONType == "number" && hasType(s, "integer") {
				// Parsed static numbers are always float64 for some input formats
				// like JSON. We can safely ignore this because it should render
				// correctly in the output.
				return
			}
			ctx.AddError(fmt.Errorf("error validating template: expression '%s' results in %s but expecting %v", t[2:len(t)-1], outJSONType, s.Types))
		}
		return
	}

	// This will result in a string as output.
	if !hasType(s, "string") {
		wrongTypeError(ctx, "string", s)
	}
}

func validateBranch(ctx *context, s *jsonschema.Schema, t map[string]interface{}, paramsExample map[string]interface{}) {
	if s, ok := t["$if"].(string); ok {
		if !strings.HasPrefix(s, "${") {
			ctx.WithPath("$if").AddError(fmt.Errorf("error validating template: $if expression must use ${...} interpolation syntax"))
		} else {
			_, err := mexpr.Parse(s[2:len(s)-1], paramsExample)
			if err != nil {
				ctx.WithPath("$if").AddErrorOffset(fmt.Errorf("error validating template: unable to test $if expression: %v", err), err.Offset()+2)
			}
		}
	}
	if t["$then"] == nil {
		ctx.AddError(fmt.Errorf("error validating template:  $then clause is required for $if branching"))
	} else {
		ctx.Meta.TemplateComplexity++
		validateTemplate(ctx.WithPath("$then"), s, t["$then"], paramsExample)
	}
	if t["$else"] != nil {
		ctx.Meta.TemplateComplexity++
		validateTemplate(ctx.WithPath("$else"), s, t["$else"], paramsExample)
	}
}

func validateLoop(ctx *context, s *jsonschema.Schema, t map[string]interface{}, paramsExample map[string]interface{}) {

	var item interface{}
	switch v := t["$for"].(type) {
	case string:
		ctx.Meta.TemplateComplexity++
		if !strings.HasPrefix(v, "${") {
			ctx.WithPath("$for").AddError(fmt.Errorf("error validating template: $for expression must use ${...} interpolation syntax"))
		} else {
			results, err := mexpr.Eval(v[2:len(v)-1], paramsExample)
			if err != nil {
				ctx.WithPath("$for").AddErrorOffset(fmt.Errorf("error validating template: unable to test $for expression: %v", err), err.Offset()+2)
			} else {
				if a, ok := results.([]interface{}); ok {
					item = a[0]
				} else {
					ctx.WithPath("$for").AddError((fmt.Errorf("error validating template: $for expresssion must result in an array but found '%v'", results)))
				}
			}
		}
	case []interface{}:
		item = v[0]
	default:
		ctx.WithPath("$for").AddError(fmt.Errorf("error validating template: $for expression must be an array or string"))
	}

	if t["$each"] == nil {
		ctx.AddError(fmt.Errorf("error validating template: $each clause is required for $for looping"))
	} else {
		ctx.Meta.TemplateComplexity++
		paramsCopy := map[string]interface{}{}
		for k, v := range paramsExample {
			paramsCopy[k] = v
		}

		as := "item"
		if t["$as"] != nil {
			if s, ok := t["$as"].(string); ok {
				as = s
			} else {
				ctx.WithPath("$as").AddError(fmt.Errorf("error validating template: $as must be a string"))
				return
			}
		}

		paramsCopy[as] = item

		loop := "loop"
		if as != "item" {
			loop += "_" + as
		}
		paramsCopy[loop] = map[string]interface{}{
			"index": 0,
			"first": true,
			"last":  false,
		}

		validateTemplate(ctx.WithPath("$each"), getItems(s), t["$each"], paramsCopy)
	}
}

func validateFlatten(ctx *context, s *jsonschema.Schema, t map[string]interface{}, paramsExample map[string]interface{}) {
	ctx.Meta.TemplateComplexity++
	switch flat := t["$flatten"].(type) {
	case []interface{}:
		for i, item := range flat {
			validateTemplate(ctx.WithPath(fmt.Sprintf("$flatten/%d", i)), s, item, paramsExample)
		}
		// TODO: disallow extra properties?
		return
	case map[string]interface{}:
		if flat["$for"] != nil {
			// This is okay because it'll evaluate to an array eventually. So the
			// schema validation keeps working, wrap it in an array.
			wrapped := &jsonschema.Schema{
				Types: []string{"array"},
				Items: s,
			}
			validateTemplate(ctx.WithPath("$flatten"), wrapped, flat, paramsExample)
			return
		}
	}
	ctx.WithPath("$flatten").AddError(fmt.Errorf("$flatten must be an array or contain a $for clause"))
}

func validateTemplate(ctx *context, s *jsonschema.Schema, template interface{}, paramsExample map[string]interface{}) {
	if s == nil {
		return
	}
	if s.Ref != nil {
		validateTemplate(ctx, s.Ref, template, paramsExample)
		return
	}

	jsonType := getJSONType(template)

	// Special case: string template
	if jsonType == "string" {
		validateString(ctx, s, template, paramsExample)
		return
	}

	// Special case: if/for logic
	if jsonType == "object" {
		t := template.(map[string]interface{})

		if t["$if"] != nil {
			validateBranch(ctx, s, t, paramsExample)
			return
		}

		if t["$for"] != nil {
			validateLoop(ctx, s, t, paramsExample)
			return
		}

		if t["$flatten"] != nil {
			validateFlatten(ctx, s, t, paramsExample)
			return
		}
	}

	found := false
	for _, typ := range s.Types {
		if typ == "integer" {
			typ = "number"
		}
		if typ == jsonType {
			found = true
			break
		}
	}

	if !found {
		wrongTypeError(ctx, jsonType, s)
		return
	}

	switch jsonType {
	case "array":
		for i, item := range template.([]interface{}) {
			validateTemplate(ctx.WithPath(i), getItems(s), item, paramsExample)
		}
	case "object":
		for k, v := range template.(map[string]interface{}) {
			propSchema := s.Properties[k]
			if propSchema == nil {
				// Additional properties can describe props with a variable name.
				if addl, ok := s.AdditionalProperties.(*jsonschema.Schema); ok {
					validateTemplate(ctx.WithPath(k), addl, v, paramsExample)
					continue
				}

				ctx.WithPath(k).AddError(fmt.Errorf("error validating template: property %s not in allowed set %v", k, getKeys(s.Properties)))
				continue
			}

			validateTemplate(ctx.WithPath(k), propSchema, v, paramsExample)
		}
	}
}

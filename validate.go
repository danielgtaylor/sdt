package sdt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/antonmedv/expr"
	"github.com/santhosh-tekuri/jsonschema/v5"
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
		matches := interpolationRe.FindAllString(template.(string), -1)

		if len(matches) > 0 {
			for _, match := range matches {
				_, err := expr.Compile(match[2:len(match)-1], expr.Env(paramsExample))
				if err != nil {
					ctx.AddError(fmt.Errorf("error validating template: unable to compile expression '%s': %v", match, err))
				}
			}
		}

		if len(matches) == 1 && len(matches[0]) == len(template.(string)) {
			// This is a single value string template that can return any type.
			t := template.(string)
			out, err := expr.Eval(t[2:len(t)-1], paramsExample)
			if err != nil {
				ctx.AddError(fmt.Errorf("error validating template: unable to eval expression '%s': %v", t[2:len(t)-1], err))
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
		} else {
			// This will result in a string as output.
			if !hasType(s, "string") {
				ctx.AddError(fmt.Errorf("error validating template: string not allowed, expecting %v", s.Types))
			}
		}
		return
	}

	// Special case: if/for logic
	if jsonType == "object" {
		t := template.(map[string]interface{})

		if t["$if"] != nil {
			if t["$then"] == nil {
				ctx.AddError(fmt.Errorf("error validating template:  $then clause if required for $if branching"))
			} else {
				validateTemplate(ctx.WithPath("$then"), s, t["$then"], paramsExample)
			}
			if t["$else"] != nil {
				validateTemplate(ctx.WithPath("$else"), s, t["$else"], paramsExample)
			}
			// TODO: disallow extra properties?
			return
		}

		if t["$for"] != nil {
			var item interface{}
			switch v := t["$for"].(type) {
			case string:
				if !strings.HasPrefix(v, "${") {
					ctx.AddError(fmt.Errorf("error validating template: $for expression must use ${...} interpolation syntax"))
					return
				}
				results, err := expr.Eval(v[2:len(v)-1], paramsExample)
				if err != nil {
					ctx.AddError(fmt.Errorf("error validating template: unable to test $for expression: %v", err))
					return
				}

				if a, ok := results.([]interface{}); ok {
					item = a[0]
				} else {
					ctx.AddError((fmt.Errorf("error validating template: $for expresssion must result in an array but found '%v'", results)))
					return
				}
			case []interface{}:
				item = v[0]
			default:
				ctx.AddError(fmt.Errorf("error validating template: $for expression must be an array or string"))
				return
			}

			if t["$each"] == nil {
				ctx.AddError(fmt.Errorf("error validating template: $each clause is required for $for looping"))
			} else {
				paramsCopy := map[string]interface{}{}
				for k, v := range paramsExample {
					paramsCopy[k] = v
				}

				as := "item"
				if t["$as"] != nil {
					if s, ok := t["$as"].(string); ok {
						as = s
					} else {
						ctx.AddError(fmt.Errorf("error validating template: $as must be a string"))
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

				// If the $each results in an array, we merge the items into the final
				// array, allowing one $each to generate multiple outputs.
				if a, ok := t["$each"].([]interface{}); ok {
					for i, eachItem := range a {
						validateTemplate(ctx.WithPath(fmt.Sprintf("$each/%d", i)), getItems(s), eachItem, paramsCopy)
					}
				} else {
					if m, ok := t["$each"].(map[string]interface{}); ok {
						parts := strings.Split(ctx.Path, "/")
						if m["$for"] != nil && (len(parts) < 2 || parts[len(parts)-2] != "$each") {
							// Special case: nested $for loops.
							validateTemplate(ctx.WithPath("$each"), s, t["$each"], paramsCopy)
							return
						}
					}
					validateTemplate(ctx.WithPath("$each"), getItems(s), t["$each"], paramsCopy)
				}
			}
			// TODO: disallow extra properties?
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
		extra := ""
		if hasType(s, "object") {
			extra = fmt.Sprintf(" with properties %v", getKeys(s.Properties))
		}
		if hasType(s, "array") {
			extra = fmt.Sprintf(" with %v items", getItems(s).Types)
		}

		ctx.AddError(fmt.Errorf("error validating template: type %s not allowed, expecting %v%s", jsonType, s.Types, extra))
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

				ctx.AddError(fmt.Errorf("error validating template: property %s not in allowed set %v", k, getKeys(s.Properties)))
				continue
			}

			validateTemplate(ctx.WithPath(k), propSchema, v, paramsExample)
		}
	}
}

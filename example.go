package sdt

import (
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// generateExample will output an example data structure given a schema that
// sets a non-zero value for all properties/items. The purpose is to have an
// instance in Go with discrete types that can be used for the expression
// type checker.
func generateExample(s *jsonschema.Schema) interface{} {
	if s.Ref != nil {
		return generateExample(s.Ref)
	}

	if len(s.Types) > 1 {
		panic(fmt.Errorf("multiple types not supported"))
	}

	switch s.Types[0] {
	case "boolean":
		return true
	case "integer":
		return 1
	case "number":
		return 1.0
	case "string":
		return "string"
	case "array":
		return []interface{}{generateExample(getItems(s))}
	case "object":
		if s.AdditionalProperties != nil {
			panic(fmt.Errorf("additionalProperties not supported"))
		}

		tmp := map[string]interface{}{}
		for k, v := range s.Properties {
			tmp[k] = generateExample(v)
		}
		return tmp
	}

	return nil
}

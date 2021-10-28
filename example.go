package sdt

import (
	"fmt"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// generateExample will output an example data structure given a schema that
// sets a non-zero value for all properties/items. The purpose is to have an
// instance in Go with discrete types that can be used for the expression
// type checker.
func generateExample(s *jsonschema.Schema) (interface{}, error) {
	if s.Ref != nil {
		return generateExample(s.Ref)
	}

	if len(s.Types) > 1 {
		return nil, fmt.Errorf("multiple types not supported")
	}

	if len(s.Examples) > 0 {
		return convertNumberIfNeeded(s.Examples[0], s), nil
	}

	if s.Default != nil {
		return convertNumberIfNeeded(s.Default, s), nil
	}

	if len(s.Enum) > 0 {
		return convertNumberIfNeeded(s.Enum[0], s), nil
	}

	switch s.Types[0] {
	case "boolean":
		return true, nil
	case "integer":
		return 1, nil
	case "number":
		return 1.0, nil
	case "string":
		return "string", nil
	case "array":
		example, err := generateExample(getItems(s))
		if err != nil {
			return nil, err
		}
		return []interface{}{example}, nil
	case "object":
		if s.AdditionalProperties != nil {
			// Ignore `additionalProperties: false`, fail everything else.
			if _, ok := s.AdditionalProperties.(bool); !ok {
				return nil, fmt.Errorf("additionalProperties not supported")
			}
		}

		tmp := map[string]interface{}{}
		for k, v := range s.Properties {
			example, err := generateExample(v)
			if err != nil {
				return nil, err
			}
			tmp[k] = example
		}
		return tmp, nil
	}

	return nil, nil
}

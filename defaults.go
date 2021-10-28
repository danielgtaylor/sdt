package sdt

import (
	"encoding/json"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// convertNumberIfNeeded will convert `json.Number` objects into either
// an int64 or float64 depending on what the schema calls for.
func convertNumberIfNeeded(v interface{}, s *jsonschema.Schema) interface{} {
	switch d := v.(type) {
	case json.Number:
		if hasType(s, "integer") {
			di, _ := d.Int64()
			return di
		} else {
			df, _ := d.Float64()
			return df
		}
	}
	return v
}

// setDefaults takes user-provided input and traverses it along with the input
// schema to determine if unset values exist which have a default that should
// be set, then sets them. Params are modified in-place.
func setDefaults(s *jsonschema.Schema, params map[string]interface{}) {
	for s.Ref != nil {
		s = s.Ref
	}

	for k, v := range s.Properties {
		for v.Ref != nil {
			v = v.Ref
		}
		if v.Default != nil {
			if _, ok := params[k]; !ok {
				// Handle arbitrary JSON numbers by converting to the closest Go
				// type so that expressions work as expected. E.g. a json.Number can't
				// be added to an int, so this fixes that.
				params[k] = convertNumberIfNeeded(v.Default, v)
			}
		}

		switch pv := params[k].(type) {
		case map[string]interface{}:
			// This is a nested object.
			setDefaults(v, pv)
		case []interface{}:
			// This is an array
			// TODO handle arrays of arrays
			for _, item := range pv {
				if o, ok := item.(map[string]interface{}); ok {
					setDefaults(getItems(v), o)
				}
			}
		}
	}
}
